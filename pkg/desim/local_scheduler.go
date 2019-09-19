package desim

import (
	"log"
	"math/rand"
	"time"
)

const D = false

var (
	_ Scheduler       = (*localScheduler)(nil)
	_ SchedulerClient = (*localScheduler)(nil)
)

type waitingRequest struct {
	envelope *chanReq
	timeout  *Event
	async    bool
}

func NewLocalScheduler(actorCount int, resources []Resource) (Scheduler, SchedulerClient) {
	res := make(map[string]Resource)
	for _, r := range resources {
		res[r.id()] = r
	}
	schd := &localScheduler{
		actorCount:              actorCount,
		resources:               res,
		queue:                   make(chan *chanReq, actorCount),
		eventHeap:               newEventHeap(),
		pendingResponse:         make(map[int]*chanReq),
		actorsWaitingForService: make(map[string]*waitingRequest),
	}
	return schd, schd
}

type localScheduler struct {
	actorCount int
	resources  map[string]Resource
	queue      chan *chanReq

	currentTime             time.Time
	eventID                 int
	eventHeap               *eventHeap
	pendingResponse         map[int]*chanReq
	actorsWaitingForService map[string]*waitingRequest
}

func (schd *localScheduler) Schedule(req *Request) *Response {
	envelope := &chanReq{
		req: req,
		res: make(chan *chanRes),
	}
	schd.queue <- envelope
	res := <-envelope.res
	return res.res
}

func (schd *localScheduler) Run(r *rand.Rand, start, end time.Time) []*Event {
	schd.currentTime = start

	var (
		// requests that are waiting for some condition to occur
		// before being scheduled in the future
		actorsRunning = schd.actorCount
	)
	defer func() {
		for _, pending := range schd.pendingResponse {
			select {
			case pending.res <- &chanRes{res: &Response{
				Now:  schd.currentTime,
				Done: true,
			}}:
			default:
			}
		}
	}()

	recvRequest := func(envelope *chanReq) {
		req := envelope.req
		reqType := req.Type
		switch {
		case reqType.Done != nil:
			schd.handleRequestTypeDone(envelope)
		case reqType.Delay != nil:
			schd.handleRequestTypeDelay(envelope)
		case reqType.AcquireResource != nil:
			schd.handleRequestTypeAcquireResource(envelope)
		case reqType.ReleaseResource != nil:
			schd.handleRequestTypeReleaseResource(envelope)
		}
	}

	var history []*Event
	moreEvents := true
	for {

		if moreEvents && len(schd.pendingResponse) != actorsRunning {
			// wait til all actors have made an action
			var polledCount int
			for moreEvents && len(schd.pendingResponse) != actorsRunning {
				env, ok := <-schd.queue
				if !ok {
					moreEvents = false
				} else {
					recvRequest(env)
					polledCount++
				}
			}
			if D {
				log.Printf("scheduler: received events: %d", polledCount)
			}
		}

		if !moreEvents {
			return history
		}

		if schd.eventHeap.Len() == 0 {
			return history
		}

		nextEvent := schd.eventHeap.Pop()

		if !end.IsZero() && nextEvent.Time.After(end) {
			return history
		}

		if D {
			log.Printf("scheduler: performing next event: %v", nextEvent.Labels)
		}

		schd.currentTime = nextEvent.Time // advance time

		if nextEvent.onHandle != nil {
			nextEvent.onHandle()
		}

		if nextEvent.Signals.Has(SignalActorDone) {
			delete(schd.actorsWaitingForService, nextEvent.Actor)
			actorsRunning--
		}

		res := &Response{
			Now:         nextEvent.Time,
			Interrupted: nextEvent.Interrupted,
			Timedout:    nextEvent.Timedout,
		}
		if pending, ok := schd.pendingResponse[nextEvent.ID]; ok {
			pending.res <- &chanRes{res: res}
		}

		// cleanup
		delete(schd.pendingResponse, nextEvent.ID)

		history = append(history, nextEvent)
	}
}

func (schd *localScheduler) newEvent(req *Request, happensAt time.Time, kind string) *Event {
	schd.eventID++
	actor := req.Actor
	return &Event{
		Actor:       actor,
		ID:          schd.eventID,
		Priority:    req.Priority,
		Time:        happensAt,
		TieBreakers: req.TieBreakers,
		Signals:     req.Signals,
		Labels:      req.Labels,
		Kind:        kind,
	}
}

func (schd *localScheduler) handleRequestTypeDone(envelope *chanReq) {
	req := envelope.req
	// schedule an immediate "done" event
	ev := schd.newEvent(req, schd.currentTime, "actor is done")
	ev.Signals.Set(SignalActorDone)
	schd.eventHeap.Push(ev)
	schd.pendingResponse[ev.ID] = envelope
	return
}

func (schd *localScheduler) handleRequestTypeDelay(envelope *chanReq) {
	req := envelope.req
	reqType := req.Type.Delay
	// simply schedule an event to wake up
	ev := schd.newEvent(req, schd.currentTime.Add(reqType.Delay), "waited a delay")
	schd.eventHeap.Push(ev)
	schd.pendingResponse[ev.ID] = envelope
	return
}

func (schd *localScheduler) handleRequestTypeAcquireResource(envelope *chanReq) {
	req := envelope.req
	acquire := req.Type.AcquireResource
	actor := req.Actor
	// lookup the resource
	resource, ok := schd.resources[acquire.ResourceID]
	if !ok {
		panic("asking to acquire a resource that doesn't exist")
	}
	acquired := resource.acquireOrEnqueue(actor)
	if acquired {
		// schedule an immediate event
		ev := schd.newEvent(req, schd.currentTime, "acquired resource immediately")
		schd.eventHeap.Push(ev)
		schd.pendingResponse[ev.ID] = envelope
		return
	}
	// schedule a timeout
	timeoutEvent := schd.newEvent(req, schd.currentTime.Add(acquire.Timeout), "timed out waiting for resource")
	timeoutEvent.Timedout = true
	schd.eventHeap.Push(timeoutEvent)
	schd.pendingResponse[timeoutEvent.ID] = envelope

	// keep the actor waiting, somewhere we can grab it back
	// when its turns come
	schd.actorsWaitingForService[actor] = &waitingRequest{
		envelope: envelope,
		timeout:  timeoutEvent,
		async:    false, // we are actively waiting for the response
	}
	return
}

func (schd *localScheduler) releaseResource(resource Resource, actor string) {
	resource.release(actor, func(nextActorInLine string) (stillWaiting bool) {
		waitingRequest, ok := schd.actorsWaitingForService[nextActorInLine]
		if !ok {
			// actor timed out/is gone
			return false
		}
		// remove actor from the waiting list
		delete(schd.actorsWaitingForService, nextActorInLine)
		// remove the actor's pending timeout
		timeoutEvent := waitingRequest.timeout
		schd.eventHeap.Remove(timeoutEvent)
		delete(schd.pendingResponse, timeoutEvent.ID)

		// schedule an immediate event to wake up the actor
		// it has acquired the resource
		ev := schd.newEvent(waitingRequest.envelope.req, schd.currentTime, "acquired resource after waiting")

		schd.eventHeap.Push(ev)
		if !waitingRequest.async {
			schd.pendingResponse[ev.ID] = waitingRequest.envelope
		}
		return true
	})
}

func (schd *localScheduler) handleRequestTypeReleaseResource(envelope *chanReq) {
	req := envelope.req
	release := req.Type.ReleaseResource
	actor := req.Actor
	// lookup the resource
	resource, ok := schd.resources[release.ResourceID]
	if !ok {
		panic("asking to release a resource that doesn't exist")
	}

	if req.Async {
		// schedule an event in the future to release the resource
		ev := schd.newEvent(req, schd.currentTime.Add(req.AsyncDelay), "released resource async")
		// trigger the release when the event occurs
		ev.onHandle = func() {
			schd.releaseResource(resource, actor)
		}
		schd.eventHeap.Push(ev)
		// return control immediately
		envelope.res <- &chanRes{
			res: &Response{Now: schd.currentTime},
		}
		return
	}

	// schedule an immediate event to release the resource
	ev := schd.newEvent(req, schd.currentTime, "released resource")
	ev.onHandle = func() {
		schd.releaseResource(resource, actor)
	}
	schd.eventHeap.Push(ev)
	schd.pendingResponse[ev.ID] = envelope

	return
}

type chanReq struct {
	req *Request
	res chan *chanRes
}

type chanRes struct {
	res *Response
}

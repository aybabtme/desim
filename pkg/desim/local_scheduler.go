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

	var (

		// requests that are waiting for some condition to occur
		// before being scheduled in the future
		actorsRunning = schd.actorCount
		currentTime   = start
		eventID       = 0
	)
	defer func() {
		for _, pending := range schd.pendingResponse {
			select {
			case pending.res <- &chanRes{res: &Response{
				Now:  currentTime,
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
			eventID = schd.handleRequestTypeDone(eventID, currentTime, envelope)
		case reqType.Delay != nil:
			eventID = schd.handleRequestTypeDelay(eventID, currentTime, envelope)
		case reqType.AcquireResource != nil:
			eventID = schd.handleRequestTypeAcquireResource(eventID, currentTime, envelope)
		case reqType.ReleaseResource != nil:
			eventID = schd.handleRequestTypeReleaseResource(eventID, currentTime, envelope)
		case reqType.UseResource != nil:
			panic("TODO")
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

		currentTime = nextEvent.Time // advance time

		if nextEvent.Signals.Has(SignalActorDone) {
			delete(schd.actorsWaitingForService, nextEvent.Actor)
			actorsRunning--
		}

		res := &Response{
			Now:         nextEvent.Time,
			Interrupted: nextEvent.Interrupted,
			Timedout:    nextEvent.Timedout,
		}
		schd.pendingResponse[nextEvent.ID].res <- &chanRes{res: res}

		// cleanup
		delete(schd.pendingResponse, nextEvent.ID)

		history = append(history, nextEvent)
	}
}

func newEvent(eventID int, req *Request, happensAt time.Time, kind string) (int, *Event) {
	eventID++
	actor := req.Actor
	return eventID, &Event{
		Actor:       actor,
		ID:          eventID,
		Priority:    req.Priority,
		Time:        happensAt,
		TieBreakers: req.TieBreakers,
		Signals:     req.Signals,
		Labels:      req.Labels,
		Kind:        kind,
	}
}

func (schd *localScheduler) handleRequestTypeDone(eventID int, currentTime time.Time, envelope *chanReq) int {
	req := envelope.req
	// schedule an immediate "done" event
	eventID, ev := newEvent(eventID, req, currentTime, "actor is done")
	ev.Signals.Set(SignalActorDone)
	schd.eventHeap.Push(ev)
	schd.pendingResponse[ev.ID] = envelope
	return eventID
}

func (schd *localScheduler) handleRequestTypeDelay(eventID int, currentTime time.Time, envelope *chanReq) int {
	req := envelope.req
	reqType := req.Type.Delay
	// simply schedule an event to wake up
	eventID, ev := newEvent(eventID, req, currentTime.Add(reqType.Delay), "waited a delay")
	schd.eventHeap.Push(ev)
	schd.pendingResponse[ev.ID] = envelope
	return eventID
}

func (schd *localScheduler) handleRequestTypeAcquireResource(eventID int, currentTime time.Time, envelope *chanReq) int {
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
		eventID, ev := newEvent(eventID, req, currentTime, "acquired resource immediately")
		schd.eventHeap.Push(ev)
		schd.pendingResponse[ev.ID] = envelope
		return eventID
	}
	// schedule a timeout
	eventID, timeoutEvent := newEvent(eventID, req, currentTime.Add(acquire.Timeout), "timed out waiting for resource")
	timeoutEvent.Timedout = true
	schd.eventHeap.Push(timeoutEvent)
	schd.pendingResponse[timeoutEvent.ID] = envelope

	// keep the actor waiting, somewhere we can grab it back
	// when its turns come
	schd.actorsWaitingForService[actor] = &waitingRequest{
		envelope: envelope,
		timeout:  timeoutEvent,
	}
	return eventID
}

func (schd *localScheduler) handleRequestTypeReleaseResource(eventID int, currentTime time.Time, envelope *chanReq) int {
	req := envelope.req
	release := req.Type.ReleaseResource
	actor := req.Actor
	// lookup the resource
	resource, ok := schd.resources[release.ResourceID]
	if !ok {
		panic("asking to release a resource that doesn't exist")
	}
	// schedule an immediate event to release the resource
	eventID, ev := newEvent(eventID, req, currentTime, "released resource")
	schd.eventHeap.Push(ev)
	schd.pendingResponse[ev.ID] = envelope

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
		eventID, ev = newEvent(eventID, waitingRequest.envelope.req, currentTime, "acquired resource after waiting")

		schd.eventHeap.Push(ev)
		schd.pendingResponse[ev.ID] = waitingRequest.envelope

		return true
	})
	return eventID
}

type chanReq struct {
	req *Request
	res chan *chanRes
}

type chanRes struct {
	res *Response
}

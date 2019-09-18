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

func NewLocalScheduler(actorCount int, resources []Resource) (Scheduler, SchedulerClient) {
	res := make(map[string]Resource)
	for _, r := range resources {
		res[r.id()] = r
	}
	schd := &localScheduler{
		actorCount: actorCount,
		resources:  res,
		queue:      make(chan *chanReq, actorCount),
	}
	return schd, schd
}

type localScheduler struct {
	actorCount int
	resources  map[string]Resource
	queue      chan *chanReq
}

type waitingRequest struct {
	envelope *chanReq
	timeout  *Event
}

func (schd *localScheduler) Run(r *rand.Rand, start, end time.Time) []*Event {

	var (
		eventHeap       = newEventHeap()
		pendingResponse = make(map[int]*chanReq)
		// requests that are waiting for some condition to occur
		// before being scheduled in the future
		actorsWaitingForService = make(map[string]*waitingRequest)
		actorsRunning           = schd.actorCount
		resources               = schd.resources
		currentTime             = start
		eventID                 = 0
	)
	defer func() {
		for _, pending := range pendingResponse {
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
		actor := req.Actor
		switch {
		case reqType.Done != nil:
			// schedule an immediate "done" event
			eventID++
			ev := &Event{
				Actor:       actor,
				ID:          eventID,
				Priority:    req.Priority,
				Time:        currentTime,
				TieBreakers: req.TieBreakers,
				Signals:     req.Signals.Set(SignalActorDone),
				Labels:      req.Labels,
				Kind:        "actor is done",
			}
			eventHeap.Push(ev)
			pendingResponse[ev.ID] = envelope
		case reqType.Delay != nil:
			// simply schedule an event to wake up
			eventID++
			ev := &Event{
				Actor:       actor,
				ID:          eventID,
				Priority:    req.Priority,
				Time:        currentTime.Add(reqType.Delay.Delay),
				TieBreakers: req.TieBreakers,
				Signals:     req.Signals,
				Labels:      req.Labels,
				Kind:        "waited a delay",
			}
			eventHeap.Push(ev)
			pendingResponse[ev.ID] = envelope
		case reqType.AcquireResource != nil:
			// lookup the resource
			acquire := reqType.AcquireResource
			resource, ok := resources[acquire.ResourceID]
			if !ok {
				panic("asking to acquire a resource that doesn't exist")
			}
			acquired := resource.acquireOrEnqueue(actor)
			if acquired {
				// schedule an immediate event
				eventID++
				ev := &Event{
					Actor:       actor,
					ID:          eventID,
					Priority:    req.Priority,
					Time:        currentTime, // right now
					TieBreakers: req.TieBreakers,
					Signals:     req.Signals,
					Labels:      req.Labels,
					Kind:        "acquired resource immediately",
				}
				eventHeap.Push(ev)
				pendingResponse[ev.ID] = envelope
				return
			}
			// schedule a timeout
			eventID++
			timeoutEvent := &Event{
				Actor:       actor,
				ID:          eventID,
				Priority:    req.Priority,
				Time:        currentTime.Add(acquire.Timeout),
				TieBreakers: req.TieBreakers,
				Signals:     req.Signals,
				Labels:      req.Labels,
				Kind:        "timed out waiting for resource",
				Timedout:    true,
			}
			eventHeap.Push(timeoutEvent)
			pendingResponse[timeoutEvent.ID] = envelope
			// keep the actor waiting, somewhere we can grab it back
			// when its turns come
			actorsWaitingForService[actor] = &waitingRequest{
				envelope: envelope,
				timeout:  timeoutEvent,
			}

		case reqType.ReleaseResource != nil:
			// lookup the resource
			release := reqType.ReleaseResource
			resource, ok := resources[release.ResourceID]
			if !ok {
				panic("asking to release a resource that doesn't exist")
			}
			// schedule an immediate event to release the resource
			eventID++
			ev := &Event{
				Actor:       actor,
				ID:          eventID,
				Priority:    req.Priority,
				Time:        currentTime, // right now
				TieBreakers: req.TieBreakers,
				Signals:     req.Signals,
				Labels:      req.Labels,
				Kind:        "released resource",
			}
			eventHeap.Push(ev)
			pendingResponse[ev.ID] = envelope
			resource.release(actor, func(nextActorInLine string) (stillWaiting bool) {
				waitingRequest, ok := actorsWaitingForService[nextActorInLine]
				if !ok {
					// actor timed out/is gone
					return false
				}
				// remove actor from the waiting list
				delete(actorsWaitingForService, nextActorInLine)
				// remove the actor's pending timeout
				timeoutEvent := waitingRequest.timeout
				eventHeap.Remove(timeoutEvent)
				delete(pendingResponse, timeoutEvent.ID)

				// schedule an immediate event to wake up the actor
				// it has acquired the resource
				eventID++
				ev := &Event{
					Actor:       nextActorInLine,
					ID:          eventID,
					Priority:    waitingRequest.envelope.req.Priority,
					Time:        currentTime, // right now
					TieBreakers: waitingRequest.envelope.req.TieBreakers,
					Signals:     waitingRequest.envelope.req.Signals,
					Labels:      waitingRequest.envelope.req.Labels,
					Kind:        "acquired resource after waiting",
				}
				eventHeap.Push(ev)
				pendingResponse[ev.ID] = waitingRequest.envelope

				return true
			})
		}
	}

	var history []*Event
	moreEvents := true
	for {

		if moreEvents && len(pendingResponse) != actorsRunning {
			// wait til all actors have made an action
			var polledCount int
			for moreEvents && len(pendingResponse) != actorsRunning {
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

		if eventHeap.Len() == 0 {
			return history
		}

		nextEvent := eventHeap.Pop()

		if !end.IsZero() && nextEvent.Time.After(end) {
			return history
		}

		if D {
			log.Printf("scheduler: performing next event: %v", nextEvent.Labels)
		}

		currentTime = nextEvent.Time // advance time

		if nextEvent.Signals.Has(SignalActorDone) {
			delete(actorsWaitingForService, nextEvent.Actor)
			actorsRunning--
		}

		res := &Response{
			Now:         nextEvent.Time,
			Interrupted: nextEvent.Interrupted,
			Timedout:    nextEvent.Timedout,
		}
		pendingResponse[nextEvent.ID].res <- &chanRes{res: res}

		// cleanup
		delete(pendingResponse, nextEvent.ID)

		history = append(history, nextEvent)
	}
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

type chanReq struct {
	req *Request
	res chan *chanRes
}

type chanRes struct {
	res *Response
}

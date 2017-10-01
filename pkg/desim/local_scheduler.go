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

func NewLocalScheduler(actorCount int) (Scheduler, SchedulerClient) {
	schd := &localScheduler{actorCount: actorCount, queue: make(chan *chanReq, actorCount)}
	return schd, schd
}

type localScheduler struct {
	actorCount int
	queue      chan *chanReq
}

func (schd *localScheduler) Run(r *rand.Rand, start, end time.Time) []*Event {

	eventHeap := newEventHeap()

	pendingResponse := make(map[int]*chanReq)

	actorsRunning := schd.actorCount

	currentTime := start

	eventID := 0

	recvEvent := func(envelope *chanReq) {
		req := envelope.req
		eventID++
		ev := &Event{
			ID:          eventID,
			Priority:    req.Priority,
			Time:        currentTime.Add(req.Delay),
			TieBreakers: req.TieBreakers,
			Signals:     req.Signals,
			Labels:      req.Labels,
		}

		eventHeap.Push(ev)
		pendingResponse[ev.ID] = envelope
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
					recvEvent(env)
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
			actorsRunning--
		}

		res := &Response{
			Now:         nextEvent.Time,
			Interrupted: false, // don't support interruptions just yet
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

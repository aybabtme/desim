package desim

import (
	"log"
	"math/rand"
	"time"
)

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

func (schd *localScheduler) Run(r *rand.Rand, start, end time.Time) {

	eventHeap := NewEventHeap()

	pendingResponse := make(map[int]*chanReq)

	actorsRunning := schd.actorCount

	currentTime := start

	eventID := 0
	log.Print("start of simulation")

	recvEvent := func(envelope *chanReq) {
		req := envelope.req
		eventID++
		ev := &Event{
			ID:       eventID,
			Priority: req.Priority,
			Time:     currentTime.Add(req.Delay),
			TieBreaker: func() int32 {
				return req.TieBreaker(r)
			},
			Signals: req.Signals,
		}
		log.Printf("<- received an event %d: %+v", eventID, ev)

		eventHeap.Push(ev)
		pendingResponse[ev.ID] = envelope
	}

	moreEvents := true
	for {

		// wait til all actors have made an action
		log.Printf("waiting for actors to send create events")
		for moreEvents && len(pendingResponse) != actorsRunning {
			env, ok := <-schd.queue
			if !ok {
				moreEvents = false
			} else {
				recvEvent(env)
			}
		}
		log.Printf("all actors are waiting")

		if !moreEvents {
			log.Print("end of simulation: no more events")
			return
		}

		if eventHeap.Len() == 0 {
			log.Print("deadlock: all actors are waiting for a response but no events are planned")
			return
		}

		nextEvent := eventHeap.Pop()

		if !end.IsZero() && nextEvent.Time.After(end) {
			log.Print("end of simulation: reached end of time")
			return
		}

		log.Printf("processing event: %+v", nextEvent)

		currentTime = nextEvent.Time // advance time

		if nextEvent.Signals.Has(SignalActorDone) {
			log.Printf("an actor reported that it will not produce anymore events")
			actorsRunning--
		}

		res := &Response{
			Now:         nextEvent.Time,
			Interrupted: false, // don't support interruptions just yet
		}
		reschan, ok := pendingResponse[nextEvent.ID]
		if !ok {
			log.Panicf("no one waiting for %d", nextEvent.ID)
		}
		reschan.res <- &chanRes{res: res}

		// cleanup
		delete(pendingResponse, nextEvent.ID)
	}
}

func (schd *localScheduler) Schedule(req *Request) *Response {
	log.Printf("requesting scheduling of an event")
	envelope := &chanReq{
		req: req,
		res: make(chan *chanRes),
	}
	schd.queue <- envelope
	log.Printf("event scheduled")
	res := <-envelope.res
	log.Printf("event processed")
	return res.res
}

type chanReq struct {
	req *Request
	res chan *chanRes
}

type chanRes struct {
	res *Response
}

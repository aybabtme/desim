package desim

import (
	"math/rand"
	"time"
)

var _ Scheduler = (*LocalScheduler)(nil)

type LocalScheduler struct {
	queue chan *chanReq
}

func NewLocalScheduler() *LocalScheduler {
	return &LocalScheduler{
		queue: make(chan *chanReq),
	}
}

func (client *LocalScheduler) Start(r *rand.Rand, start time.Time) {

	eventHeap := NewEventHeap()

	pendingResponse := make(map[*Event]*chanReq)

	currentTime := start

	for eventID := 0; ; eventID++ {

	process_scheduled_events:
		for eventHeap.Len() > 0 {
			nextEvent := eventHeap.Peek()
			if !nextEvent.Time.Before(currentTime) {
				break process_scheduled_events
			}
			res := &Response{
				Now:         nextEvent.Time,
				Interrupted: false, // don't support interruptions just yet
			}
			pendingResponse[nextEvent].res <- &chanRes{res: res}

			// cleanup
			eventHeap.Remove(nextEvent)
			delete(pendingResponse, nextEvent)
			currentTime = nextEvent.Time
		}

		// receive an event request
		envelope, more := <-client.queue
		if !more {
			return
		}
		req := envelope.req

		ev := &Event{
			ID:         eventID,
			Priority:   req.Priority,
			Time:       currentTime.Add(req.Delay),
			TieBreaker: req.TieBreaker,
		}

		pendingResponse[ev] = envelope
	}
}

func (client *LocalScheduler) Schedule(req *Request) *Response {
	envelope := &chanReq{
		req: req,
		res: make(chan *chanRes),
	}
	client.queue <- envelope
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

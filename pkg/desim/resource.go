package desim

import (
	"fmt"
)

type reservationKey string

type reservation struct {
	seq   int
	actor string
}

func (res reservation) key() reservationKey {
	return reservationKey(fmt.Sprintf("%d-%s", res.seq, res.actor))
}

// A Resource can be acquired and released by reservations. It has a capacity of 1 or more
// slots. The priority in which reservations get to acquire resources depends on the resource
// implementation.
type Resource interface {
	id() string
	acquireOrEnqueue(byActor string) *reservation
	release(res reservationKey, notifyNextInLine func(*reservation) (stillWaiting bool))
}

// MakeFIFOResource makes a resource that is acquired in first-in
// first out order.
func MakeFIFOResource(name string, capacity int) Resource {
	return &fifoResource{
		name:         name,
		capacity:     capacity,
		reservations: make(map[reservationKey]*reservation),
		queue:        newReservationQueue(capacity),
	}
}

type fifoResource struct {
	seq      int
	name     string
	capacity int

	reservations map[reservationKey]*reservation

	queue *reservationQueue
}

func (fifo *fifoResource) id() string { return fifo.name }

func (fifo *fifoResource) acquireOrEnqueue(byActor string) *reservation {
	fifo.seq++
	res := &reservation{seq: fifo.seq, actor: byActor}
	if len(fifo.reservations) >= fifo.capacity {
		fifo.queue.Push(res)
		return nil
	}
	fifo.reservations[res.key()] = res
	return res
}

func (fifo *fifoResource) release(resKey reservationKey, notifyNextInLine func(*reservation) bool) {
	_, ok := fifo.reservations[resKey]
	if !ok {
		panic("can't release reservation that was never acquired")
	}
	delete(fifo.reservations, resKey)
	if len(fifo.reservations) < fifo.capacity {
		for fifo.queue.Len() > 0 {
			nextInLine := fifo.queue.Pop()
			accepted := notifyNextInLine(nextInLine)
			if accepted {
				fifo.reservations[nextInLine.key()] = nextInLine
				return
			}
		}
	}
}

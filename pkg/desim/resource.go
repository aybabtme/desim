package desim

import (
	"github.com/aybabtme/desim/pkg/desim/internal/ds"
)

// A Resource can be acquired and released by actors. It has a capacity of 1 or more
// slots. The priority in which actors get to acquire resources depends on the resource
// implementation.
type Resource interface {
	id() string
	acquireOrEnqueue(byActor string) (acquired bool)
	release(byActor string, notifyNextInLine func(actor string) (stillWaiting bool))
}

// MakeFIFOResource makes a resource that is acquired in first-in
// first out order.
func MakeFIFOResource(name string, capacity int) Resource {
	return &fifoResource{
		name:     name,
		capacity: capacity,
		actors:   make(map[string]struct{}),
		queue:    ds.NewStringQueue(capacity),
	}
}

type fifoResource struct {
	name     string
	capacity int

	actors map[string]struct{}

	queue *ds.StringQueue
}

func (fifo *fifoResource) id() string { return fifo.name }

func (fifo *fifoResource) acquireOrEnqueue(byActor string) bool {
	if len(fifo.actors) >= fifo.capacity {
		fifo.queue.Push(byActor)
		return false
	}
	fifo.actors[byActor] = struct{}{}
	return true
}

func (fifo *fifoResource) release(byActor string, notifyNextInLine func(actor string) bool) {
	delete(fifo.actors, byActor)
	if len(fifo.actors) < fifo.capacity {
		for fifo.queue.Len() > 0 {
			nextInLine := fifo.queue.Pop()
			accepted := notifyNextInLine(nextInLine)
			if accepted {
				fifo.actors[nextInLine] = struct{}{}
				return
			}
		}
	}
}

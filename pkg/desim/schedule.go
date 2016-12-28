package desim

import (
	"fmt"
	"time"
)

type Scheduler interface {
	Schedule(*Request) *Response
}

type Request struct {
	Delay      time.Duration
	Priority   int32
	TieBreaker func() int32
}

type Response struct {
	Now         time.Time
	Interrupted bool
}

type Event struct {
	ID         int
	Priority   int32
	Time       time.Time
	TieBreaker func() int32
}

func (e *Event) Compare(other *Event) int {

	if e.Priority > other.Priority {
		return 1
	}
	if e.Priority < other.Priority {
		return -1
	}
	if e.Time.Before(other.Time) {
		return 1
	}
	if other.Time.Before(e.Time) {
		return -1
	}

	// same priority and time, use tie breaker

	for i := 0; i < 100; i++ {
		self := e.TieBreaker()
		other := other.TieBreaker()
		if self > other {
			return 1
		}
		if other > self {
			return -1
		}
	}
	panic(fmt.Sprintf("can't resolve tie between two events, are they duplicates?\nself=%#v\nother=%#v", e, other))
}

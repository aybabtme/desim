package desim

import (
	"fmt"
	"math/rand"
	"time"
)

type Scheduler interface {
	Run(r *rand.Rand, start, end time.Time) []*Event
}

type SchedulerClient interface {
	Schedule(*Request) *Response
}

type Request struct {
	Actor       string
	Priority    int32
	Signals     Signal
	TieBreakers [4]int32
	Labels      map[string]string
	Type        *RequestType

	Async      bool
	AsyncDelay time.Duration
}

type RequestType struct {
	// oneof
	Done            *RequestDone
	Delay           *RequestDelay
	AcquireResource *RequestAcquireResource
	ReleaseResource *RequestReleaseResource
}

type RequestDone struct{}

type RequestDelay struct {
	Delay time.Duration
}

type RequestAcquireResource struct {
	ResourceID string
	Timeout    time.Duration
}

type RequestReleaseResource struct {
	ResourceID     string
	ReservationKey string
}

type Response struct {
	Now         time.Time
	Interrupted bool
	Timedout    bool
	Done        bool

	ReservationKey string
}

type Event struct {
	Actor       string
	ID          int
	Time        time.Time
	Priority    int32
	Signals     Signal
	TieBreakers [4]int32
	Labels      map[string]string

	Kind        string
	Interrupted bool
	Timedout    bool
	// TODO: these need to be some kind of return value
	ReservationKey string

	onHandle func()
}

func (e *Event) compare(other *Event) int {
	if e.Time.Before(other.Time) {
		return 1
	}
	if other.Time.Before(e.Time) {
		return -1
	}
	if e.Priority > other.Priority {
		return 1
	}
	if e.Priority < other.Priority {
		return -1
	}
	if e.Actor < other.Actor {
		return 1
	}
	if e.Actor > e.Actor {
		return -1
	}
	if e.ID < other.ID {
		return 1
	}
	if e.ID > other.ID {
		return -1
	}

	// same time and priority, use tie breakers
	for i, eBreaker := range e.TieBreakers {
		oBreaker := other.TieBreakers[i]
		if eBreaker > oBreaker {
			return 1
		} else if eBreaker < oBreaker {
			return -1
		}
	}

	if e.ID == other.ID {
		return 0
	}

	panic(fmt.Sprintf(`can't resolve tie between two events, are they duplicates?
        self = %#v
        other= %#v`,
		e, other,
	))
}

type Signal uint8

const (
	SignalAbort Signal = 1 << iota
	SignalActorDone
)

func (r Signal) Set(f Signal) Signal { r = r | f; return r }
func (r Signal) Has(f Signal) bool   { return (r & f) == f }

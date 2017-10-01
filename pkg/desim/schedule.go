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
	Delay       time.Duration
	Priority    int32
	Signals     Signal
	TieBreakers [4]int32
	Labels      map[string]string
}

type Response struct {
	Now         time.Time
	Interrupted bool
}

type Event struct {
	ID          int
	Time        time.Time
	Priority    int32
	Signals     Signal
	TieBreakers [4]int32
	Labels      map[string]string
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

	// same time and priority, use tie breakers
	for i, eBreaker := range e.TieBreakers {
		oBreaker := other.TieBreakers[i]
		if eBreaker > oBreaker {
			return 1
		} else if eBreaker < oBreaker {
			return -1
		}
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

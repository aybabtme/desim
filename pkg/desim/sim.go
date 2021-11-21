// Package desim allows one to quickly create actor-based discrete event simulations.
package desim

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/aybabtme/desim/pkg/gen"
)

type SchedulerFn func(actorCount int, res []Resource) (Scheduler, SchedulerClient)

type Simulation interface {
	Run([]*Actor, []Resource, Logger) []*Event
}

type Actor struct {
	name   string
	action Action
}

func MakeActor(name string, action Action) *Actor {
	return &Actor{
		name:   name,
		action: action,
	}
}

type Action func(Env) bool

type Env interface {
	Now() time.Time
	Rand() *rand.Rand

	IsRunning() bool

	Sleep(gen.Duration) (interrupted bool)
	Abort()
	Done(gen.Duration)

	Acquire(res Resource, timeout gen.Duration) (release func(), obtained bool)
	UseAsync(res Resource, duration, timeout gen.Duration) (obtained bool)

	Log() Logger
}

type Logger interface {
	KV(string, string) Logger
	KVi(string, int) Logger
	KVf(string, float64) Logger
	Event(string)
}

// New creates a simulation that will start from the given time.
func New(mkSchd SchedulerFn, r *rand.Rand, start, end gen.Time) Simulation {
	return &sim{mkSchd: mkSchd, r: r, start: start, end: end}
}

type sim struct {
	mkSchd     SchedulerFn
	r          *rand.Rand
	start, end gen.Time
}

func (sim *sim) Run(actors []*Actor, resources []Resource, actorlog Logger) []*Event {

	var (
		r     = rand.New(rand.NewSource(sim.r.Int63()))
		start = sim.start.Gen()
		end   = sim.end.Gen()
	)
	schd, client := sim.mkSchd(len(actors), resources)

	var wg sync.WaitGroup
	for id, actor := range actors {
		wg.Add(1)
		env := makeEnv(r.Int63(), start, client, actorlog.KV("actor", actor.name), actor.name)
		go func(id int, env Env, actor *Actor) {
			defer wg.Done()
			defer func() {
				if e := recover(); e != nil {
					if e == stopAllActors {
						// the simulation stopped
						return
					}
					panic(e)
				}
			}()
			for env.IsRunning() {
				if !actor.action(env) {
					env.Done(gen.StaticDuration(0))
					return
				}
			}
		}(id, env, actor)
	}

	history := schd.Run(r, start, end)
	wg.Wait()
	return history
}

func makeEnv(seed int64, now time.Time, schd SchedulerClient, log Logger, actorName string) *env {
	r := rand.New(rand.NewSource(seed))
	return &env{r: r, now: now, schd: schd, log: log, actorName: actorName, aborted: false, stopped: false}
}

var _ Env = (*env)(nil)

type env struct {
	r    *rand.Rand
	now  time.Time
	schd SchedulerClient
	log  Logger

	actorName string

	aborted bool
	stopped bool
}

func (env *env) Now() time.Time   { return env.now }
func (env *env) Rand() *rand.Rand { return env.r }
func (env *env) Log() Logger      { return env.log.KV("time", env.now.Format(time.RFC3339Nano)) }
func (env *env) IsRunning() bool  { return !env.aborted || !env.stopped }

func (env *env) Sleep(d gen.Duration) (interrupted bool) {
	resp := env.send(0, &RequestType{
		Delay: &RequestDelay{Delay: d.Gen()},
	}, false, 0)
	return resp.Interrupted
}

func (env *env) Abort() {
	env.stopped = true
	env.aborted = true
	_ = env.send(SignalAbort, &RequestType{
		Abort: &RequestAbort{},
	}, false, 0)
}

func (env *env) Done(d gen.Duration) {
	env.stopped = true
	_ = env.send(SignalActorDone, &RequestType{
		Done: &RequestDone{},
	}, false, 0)
}

func (env *env) Acquire(res Resource, timeout gen.Duration) (release func(), obtained bool) {
	resp := env.send(0, &RequestType{
		AcquireResource: &RequestAcquireResource{
			ResourceID: res.id(),
			Timeout:    timeout.Gen(),
		},
	}, false, 0)
	if resp.Timedout {
		return nil, false
	}
	releaseReq := &RequestType{
		ReleaseResource: &RequestReleaseResource{
			ResourceID:     res.id(),
			ReservationKey: resp.ReservationKey,
		},
	}
	releaseFn := func() {
		_ = env.send(0, releaseReq, false, 0)
	}

	return releaseFn, true
}

func (env *env) UseAsync(res Resource, duration, timeout gen.Duration) (obtained bool) {
	resp := env.send(0, &RequestType{
		AcquireResource: &RequestAcquireResource{
			ResourceID: res.id(),
			Timeout:    timeout.Gen(),
		},
	}, false, 0)
	if resp.Timedout {
		return false
	}
	// we don't wait
	_ = env.send(0, &RequestType{
		ReleaseResource: &RequestReleaseResource{
			ResourceID:     res.id(),
			ReservationKey: resp.ReservationKey,
		},
	}, true, duration.Gen())

	return true
}

var stopAllActors = struct{}{}

func (env *env) send(sig Signal, reqType *RequestType, async bool, asyncDelay time.Duration) *Response {
	if D {
		log.Printf("%q: sending an event", env.actorName)
	}
	resp := env.schd.Schedule(&Request{
		Actor:    env.actorName,
		Type:     reqType,
		Priority: 0,
		TieBreakers: [4]int32{
			env.r.Int31(),
			env.r.Int31(),
			env.r.Int31(),
			env.r.Int31(),
		},
		Signals:    sig,
		Labels:     map[string]string{"name": env.actorName},
		Async:      async,
		AsyncDelay: asyncDelay,
	})
	env.now = resp.Now
	if resp.Done {
		if !env.stopped {
			env.stopped = resp.Done
		}
		if !env.aborted {
			env.aborted = resp.Done
		}
		panic(stopAllActors)
	}
	return resp
}

package desim

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/aybabtme/desim/pkg/gen"
)

type SchedulerFn func(actorCount int) (Scheduler, SchedulerClient)

type Simulation interface {
	Run([]*Actor, Logger)
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
	Abort(gen.Duration)
	Done(gen.Duration)

	Log() Logger
}

type Logger interface {
	KV(string, string) Logger
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

func (sim *sim) Run(actors []*Actor, actorlog Logger) {

	var (
		r     = rand.New(rand.NewSource(sim.r.Int63()))
		start = sim.start.Gen()
		end   = sim.end.Gen()
	)
	schd, client := sim.mkSchd(len(actors))

	var wg sync.WaitGroup
	for id, actor := range actors {
		wg.Add(1)
		env := makeEnv(r.Int63(), start, client, actorlog.KV("actor", actor.name))
		go func(id int, env Env, actor *Actor) {
			log.Printf("%d: actor starting: %q", id, actor.name)

			actionCount := 0
			defer func() { log.Printf("%d: actor stopping: %q performed %d actions", id, actor.name, actionCount) }()

			defer wg.Done()
			for env.IsRunning() {
				actionCount++
				if !actor.action(env) {
					log.Printf("%d: actor purposefully stopped", id)
					env.Done(gen.StaticDuration(0))
					return
				}
			}
			log.Printf("%d: environment was aborted", id)
		}(id, env, actor)
	}

	schd.Run(r, start, end)
	wg.Wait()
}

func makeEnv(seed int64, now time.Time, schd SchedulerClient, log Logger) *env {
	r := rand.New(rand.NewSource(seed))
	return &env{r: r, now: now, schd: schd, log: log, aborted: false}
}

var _ Env = (*env)(nil)

type env struct {
	r    *rand.Rand
	now  time.Time
	schd SchedulerClient
	log  Logger

	aborted bool
	stopped bool
}

func (env *env) Now() time.Time   { return env.now }
func (env *env) Rand() *rand.Rand { return env.r }
func (env *env) Log() Logger      { return env.log.KV("time", env.now.Format(time.RFC3339Nano)) }
func (env *env) IsRunning() bool  { return !env.aborted || !env.stopped }

func (env *env) Sleep(d gen.Duration) (interrupted bool) {
	resp := env.send(d, 0)
	return resp.Interrupted
}

func (env *env) Abort(d gen.Duration) {
	env.aborted = true
	_ = env.send(d, SignalAbort)
}

func (env *env) Done(d gen.Duration) {
	env.stopped = true
	_ = env.send(d, SignalActorDone)
}

func (env *env) send(d gen.Duration, sig Signal) *Response {
	resp := env.schd.Schedule(&Request{
		Delay:    d.Gen(),
		Priority: 0,
		TieBreaker: func(r *rand.Rand) int32 {
			return r.Int31()
		},
		Signals: sig,
	})
	env.now = resp.Now
	return resp
}

package desim

import (
	"math/rand"
	"time"

	"github.com/aybabtme/desim/pkg/gen"
)

type Simulation interface {
	Start(Observer, []*Actor)
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

type Action func(Env)

type Env interface {
	Now() time.Time
	Rand() *rand.Rand
	Sleep(gen.Duration) (interupted bool)
	Log() Logger
}

type Logger interface {
	Event(string)
}

type Observer interface {
}

// New creates a simulation that will start from the given time.
func New(start gen.Time) Simulation {
	return &sim{start: start}
}

type sim struct {
	r     *rand.Rand
	start gen.Time
}

func (sim *sim) Start(observer Observer, actors []*Actor) {

	r := sim.r
	start := sim.start.Gen()
	schd := NewLocalScheduler()

	for _, actor := range actors {
		seed := r.Int63()
		env := makeEnv(seed, start, schd)
		go func(env Env, actor *Actor) {
			for {
				actor.action(env)
			}

		}(env, actor)
	}

	schd.Start(func(req *Request) *Response {
		// before processing the event, see if pending events
		// can be done
	})
}

func makeEnv(seed int64, now time.Time, schd Scheduler) *env {
	r := rand.New(rand.NewSource(seed))
	return &env{r: r, now: now, schd: schd}
}

var _ Env = (*env)(nil)

type env struct {
	r    *rand.Rand
	now  time.Time
	schd Scheduler
	log  Logger
}

func (env *env) Now() time.Time   { return env.now }
func (env *env) Rand() *rand.Rand { return env.r }

func (env *env) Sleep(d gen.Duration) (interupted bool) {
	resp := env.schd.Schedule(&Request{
		Ev:    &Event{Message: "sleep"},
		Delay: d.Gen(),
	})
	env.now = resp.Now
	return resp.Interrupted
}

func (env *env) Log() Logger { return env.log }

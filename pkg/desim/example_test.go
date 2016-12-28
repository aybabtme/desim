package desim_test

import (
	"time"

	"github.com/aybabtme/desim/pkg/desim"
	"github.com/aybabtme/desim/pkg/gen"
)

func Example() {
	start := gen.StaticTime(time.Now())

	sim := desim.New(start)

	slowSleep := gen.StaticDuration(1000 * time.Millisecond)
	slowActor := desim.MakeActor("slow", func(env desim.Env) {
		if !env.Sleep(slowSleep) {
			env.Log().Event("couldn't sleep")
		}
	})

	fastSleep := gen.StaticDuration(500 * time.Millisecond)
	fastActor := desim.MakeActor("fast", func(env desim.Env) {
		if !env.Sleep(fastSleep) {
			env.Log().Event("couldn't sleep")
		}
	})

	sim.Play(
		nil,
		slowActor, fastActor,
	)
}

package desim_test

import (
	"math/rand"
	"os"
	"time"

	"github.com/aybabtme/desim/pkg/desim"
	"github.com/aybabtme/desim/pkg/gen"
)

func ExampleNew() {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(10 * time.Second)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)

	slowActor := desim.MakeActor("slow", clock(1*time.Second))
	fastActor := desim.MakeActor("fast", clock(500*time.Millisecond))

	sim.Run(
		[]*desim.Actor{
			slowActor,
			fastActor,
		},
		desim.LogJSON(os.Stdout),
	)

	// Output:
	// {"actor":"fast","time":"1970-01-01T00:00:00Z","event":"woke up, about to sleep"}
	// {"actor":"slow","time":"1970-01-01T00:00:00Z","event":"woke up, about to sleep"}
	// {"actor":"fast","time":"1970-01-01T00:00:00.5Z","event":"woke up, about to sleep"}
	// {"actor":"slow","time":"1970-01-01T00:00:01Z","event":"woke up, about to sleep"}
	// {"actor":"fast","time":"1970-01-01T00:00:01Z","event":"woke up, about to sleep"}
	// {"actor":"fast","time":"1970-01-01T00:00:01.5Z","event":"woke up, about to sleep"}
	// {"actor":"slow","time":"1970-01-01T00:00:02Z","event":"woke up, about to sleep"}
	// {"actor":"fast","time":"1970-01-01T00:00:02Z","event":"woke up, about to sleep"}
	// {"actor":"fast","time":"1970-01-01T00:00:02.5Z","event":"woke up, about to sleep"}
	// {"actor":"slow","time":"1970-01-01T00:00:03Z","event":"woke up, about to sleep"}
	// {"actor":"slow","time":"1970-01-01T00:00:04Z","event":"woke up, about to sleep"}
	// {"actor":"slow","time":"1970-01-01T00:00:05Z","event":"woke up, about to sleep"}
}

func clock(dur time.Duration) desim.Action {
	iter := 6
	pdur := gen.StaticDuration(dur)
	return func(env desim.Env) bool {
		iter--
		env.Log().Event("woke up, about to sleep")

		if env.Sleep(pdur) {
			env.Log().Event("couldn't sleep")
			return false
		}
		return iter > 0
	}
}

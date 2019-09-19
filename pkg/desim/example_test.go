package desim_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
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

	slowActor := desim.MakeActor("slow", clock(6, 900*time.Millisecond))
	fastActor := desim.MakeActor("fast", clock(6, 400*time.Millisecond))

	evs := sim.Run(
		[]*desim.Actor{
			slowActor,
			fastActor,
		},
		nil,
		desim.LogJSON(ioutil.Discard),
	)
	for _, ev := range evs {
		fmt.Printf("%v: %v\n", ev.Time, ev.Labels["name"])
	}

	// Output:
	// 1970-01-01 00:00:00.4 +0000 UTC: fast
	// 1970-01-01 00:00:00.8 +0000 UTC: fast
	// 1970-01-01 00:00:00.9 +0000 UTC: slow
	// 1970-01-01 00:00:01.2 +0000 UTC: fast
	// 1970-01-01 00:00:01.6 +0000 UTC: fast
	// 1970-01-01 00:00:01.8 +0000 UTC: slow
	// 1970-01-01 00:00:02 +0000 UTC: fast
	// 1970-01-01 00:00:02.4 +0000 UTC: fast
	// 1970-01-01 00:00:02.4 +0000 UTC: fast
	// 1970-01-01 00:00:02.7 +0000 UTC: slow
	// 1970-01-01 00:00:03.6 +0000 UTC: slow
	// 1970-01-01 00:00:04.5 +0000 UTC: slow
	// 1970-01-01 00:00:05.4 +0000 UTC: slow
	// 1970-01-01 00:00:05.4 +0000 UTC: slow
}

func ExampleNewByTime() {
	var (
		r     = rand.New(rand.NewSource(42))
		begin = time.Unix(0, 0).UTC()
		end   = begin.Add(24 * time.Hour)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(begin),
		gen.StaticTime(end),
	)

	slowActor := desim.MakeActor("slow", randomRateClock(r.Int63(), 900*time.Millisecond))
	fastActor := desim.MakeActor("fast", randomRateClock(r.Int63(), 400*time.Millisecond))

	start := time.Now()
	evs := sim.Run(
		[]*desim.Actor{
			slowActor,
			fastActor,
		},
		nil,
		desim.LogMute(),
		// desim.LogPretty(ioutil.Discard),
		// desim.LogJSON(ioutil.Discard),
	)
	duration := time.Since(start)
	speedup := float64(end.Sub(begin)) / float64(duration)
	eventsRate := float64(len(evs)) / duration.Seconds()
	log.Printf("simulated %v (%d events) in %v - %.1fx factor, %.1f events/s", end.Sub(begin), len(evs), duration, speedup, eventsRate)

	// Output:
	//
}

func ExampleResourceSharing() {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(500 * time.Millisecond)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)

	mutex := desim.MakeFIFOResource("mutex", 1)

	raceForMutex := func(env desim.Env) bool {
		release, timedout := env.Acquire(mutex, gen.StaticDuration(time.Second))
		if !timedout {
			env.Log().Event("timed out waiting for mutex")
			return false
		}
		env.Sleep(gen.StaticDuration(100 * time.Millisecond))
		release()
		return true
	}

	racer1 := desim.MakeActor("racer1", raceForMutex)
	racer2 := desim.MakeActor("racer2", raceForMutex)

	evs := sim.Run(
		[]*desim.Actor{
			racer1,
			racer2,
		},
		[]desim.Resource{mutex},
		desim.LogJSON(ioutil.Discard),
	)
	for _, ev := range evs {
		fmt.Printf("%v: %s\n", ev.Time, ev.Kind)
	}

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: acquired resource immediately
	// 1970-01-01 00:00:00.1 +0000 UTC: waited a delay
	// 1970-01-01 00:00:00.1 +0000 UTC: released resource
	// 1970-01-01 00:00:00.1 +0000 UTC: acquired resource after waiting
	// 1970-01-01 00:00:00.2 +0000 UTC: waited a delay
	// 1970-01-01 00:00:00.2 +0000 UTC: released resource
	// 1970-01-01 00:00:00.2 +0000 UTC: acquired resource after waiting
	// 1970-01-01 00:00:00.3 +0000 UTC: waited a delay
	// 1970-01-01 00:00:00.3 +0000 UTC: released resource
	// 1970-01-01 00:00:00.3 +0000 UTC: acquired resource after waiting
	// 1970-01-01 00:00:00.4 +0000 UTC: waited a delay
	// 1970-01-01 00:00:00.4 +0000 UTC: released resource
	// 1970-01-01 00:00:00.4 +0000 UTC: acquired resource after waiting
	// 1970-01-01 00:00:00.5 +0000 UTC: waited a delay
	// 1970-01-01 00:00:00.5 +0000 UTC: released resource
	// 1970-01-01 00:00:00.5 +0000 UTC: acquired resource after waiting
}

func ExampleResourceTimeout() {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(100 * time.Millisecond)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)

	mutex := desim.MakeFIFOResource("mutex", 1)

	raceForMutex := func(env desim.Env) bool {
		release, timedout := env.Acquire(mutex, gen.StaticDuration(50*time.Millisecond))
		if !timedout {
			env.Log().Event("timed out waiting for mutex")
			return false
		}
		env.Sleep(gen.StaticDuration(100 * time.Millisecond))
		release()
		return false
	}

	racer1 := desim.MakeActor("racer1", raceForMutex)
	racer2 := desim.MakeActor("racer2", raceForMutex)

	evs := sim.Run(
		[]*desim.Actor{
			racer1,
			racer2,
		},
		[]desim.Resource{mutex},
		desim.LogJSON(ioutil.Discard),
	)
	for _, ev := range evs {
		fmt.Printf("%v: %s\n", ev.Time, ev.Kind)
	}

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: acquired resource immediately
	// 1970-01-01 00:00:00.05 +0000 UTC: timed out waiting for resource
	// 1970-01-01 00:00:00.05 +0000 UTC: actor is done
	// 1970-01-01 00:00:00.1 +0000 UTC: waited a delay
	// 1970-01-01 00:00:00.1 +0000 UTC: released resource
	// 1970-01-01 00:00:00.1 +0000 UTC: actor is done
}

func ExampleAsyncResourceUsage() {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(500 * time.Millisecond)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)

	pool := desim.MakeFIFOResource("pool", 1)

	var (
		requestAt  time.Time
		obtainedAt time.Time
	)

	raceForPool := func(env desim.Env) bool {

		useFor := gen.StaticDuration(100 * time.Millisecond)
		timeoutAfter := gen.StaticDuration(time.Second)

		requestAt = env.Now()
		obtained := env.UseAsync(pool, useFor, timeoutAfter)
		if !obtained {
			env.Log().Event("timed out waiting to use pool")
			return false
		}
		obtainedAt = env.Now()
		return false
	}

	racer1 := desim.MakeActor("racer1", raceForPool)

	evs := sim.Run(
		[]*desim.Actor{
			racer1,
		},
		[]desim.Resource{pool},
		desim.LogJSON(ioutil.Discard),
	)
	fmt.Printf("requested at %v\n", requestAt)
	fmt.Printf("obtained at %v\n", obtainedAt)
	for _, ev := range evs {
		fmt.Printf("%v: %s\n", ev.Time, ev.Kind)
	}

	// Output:
	// requested at 1970-01-01 00:00:00 +0000 UTC
	// obtained at 1970-01-01 00:00:00 +0000 UTC
	// 1970-01-01 00:00:00 +0000 UTC: acquired resource immediately
	// 1970-01-01 00:00:00 +0000 UTC: actor is done
	// 1970-01-01 00:00:00.1 +0000 UTC: released resource async
}

func ExampleAsyncAndSyncResourceUsage() {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(500 * time.Millisecond)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)

	useFor := gen.StaticDuration(100 * time.Millisecond)
	syncUserStartDelay := gen.StaticDuration(10 * time.Millisecond)
	timeoutAfter := gen.StaticDuration(time.Second)

	pool := desim.MakeFIFOResource("pool", 1)

	asyncUser := func(env desim.Env) bool {
		obtained := env.UseAsync(pool, useFor, timeoutAfter)
		if !obtained {
			env.Log().Event("timed out waiting to use pool")
			return false
		}
		return false
	}

	syncUser := func(env desim.Env) bool {
		if interrupted := env.Sleep(syncUserStartDelay); interrupted {
			panic("shouldn't be interrupted")
		}
		release, obtained := env.Acquire(pool, timeoutAfter)
		if !obtained {
			env.Log().Event("timed out waiting to use pool")
			return false
		}
		if interrupted := env.Sleep(useFor); interrupted {
			panic("shouldn't be interrupted")
		}
		release()
		return false
	}

	evs := sim.Run(
		[]*desim.Actor{
			desim.MakeActor("async-user", asyncUser),
			desim.MakeActor("sync-user", syncUser),
		},
		[]desim.Resource{pool},
		desim.LogJSON(ioutil.Discard),
	)
	for _, ev := range evs {
		fmt.Printf("%v: %s - %s\n", ev.Time, ev.Actor, ev.Kind)
	}

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: async-user - acquired resource immediately
	// 1970-01-01 00:00:00 +0000 UTC: async-user - actor is done
	// 1970-01-01 00:00:00.01 +0000 UTC: sync-user - waited a delay
	// 1970-01-01 00:00:00.1 +0000 UTC: async-user - released resource async
	// 1970-01-01 00:00:00.1 +0000 UTC: sync-user - acquired resource after waiting
	// 1970-01-01 00:00:00.2 +0000 UTC: sync-user - waited a delay
	// 1970-01-01 00:00:00.2 +0000 UTC: sync-user - released resource
	// 1970-01-01 00:00:00.2 +0000 UTC: sync-user - actor is done
}

func ExampleAsyncReentrantUsage() {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(500 * time.Millisecond)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)

	useFor := gen.StaticDuration(100 * time.Millisecond)
	timeoutAfter := gen.StaticDuration(time.Second)

	pool := desim.MakeFIFOResource("pool", 2)

	var (
		firstJob  time.Time
		secondJob time.Time
		thirdJob  time.Time
	)

	user := func(env desim.Env) bool {
		// send one task to the pool
		obtained := env.UseAsync(pool, useFor, timeoutAfter)
		if !obtained {
			env.Log().Event("timed out waiting to use pool")
			return false
		}
		firstJob = env.Now()
		// send another one immediately
		obtained = env.UseAsync(pool, useFor, timeoutAfter)
		if !obtained {
			env.Log().Event("timed out waiting to use pool")
			return false
		}
		secondJob = env.Now()
		// then wait a bit to send the final job
		obtained = env.UseAsync(pool, useFor, timeoutAfter)
		if !obtained {
			env.Log().Event("timed out waiting to use pool")
			return false
		}
		thirdJob = env.Now()
		return false
	}

	evs := sim.Run(
		[]*desim.Actor{
			desim.MakeActor("user", user),
		},
		[]desim.Resource{pool},
		desim.LogJSON(ioutil.Discard),
	)
	fmt.Printf("first job sent at %v\n", firstJob)
	fmt.Printf("second job sent at %v\n", secondJob)
	fmt.Printf("third job sent at %v\n", thirdJob)
	for _, ev := range evs {
		fmt.Printf("%v: %s - %s\n", ev.Time, ev.Actor, ev.Kind)
	}

	// Output:
	// first job sent at 1970-01-01 00:00:00 +0000 UTC
	// second job sent at 1970-01-01 00:00:00 +0000 UTC
	// third job sent at 1970-01-01 00:00:00.1 +0000 UTC
	// 1970-01-01 00:00:00 +0000 UTC: user - acquired resource immediately
	// 1970-01-01 00:00:00 +0000 UTC: user - acquired resource immediately
	// 1970-01-01 00:00:00.1 +0000 UTC: user - released resource async
	// 1970-01-01 00:00:00.1 +0000 UTC: user - released resource async
	// 1970-01-01 00:00:00.1 +0000 UTC: user - acquired resource after waiting
	// 1970-01-01 00:00:00.1 +0000 UTC: user - actor is done
	// 1970-01-01 00:00:00.2 +0000 UTC: user - released resource async
}

func clock(iter int, dur time.Duration) desim.Action {
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

func infiniteclock(dur time.Duration) desim.Action {
	pdur := gen.StaticDuration(dur)
	return func(env desim.Env) bool {
		env.Log().Event("woke up, about to sleep")
		return !env.Sleep(pdur)
	}
}

func randomRateClock(seed int64, avgRate time.Duration) desim.Action {
	r := rand.New(rand.NewSource(seed))
	pdur := gen.ExpDuration(r, avgRate)
	return func(env desim.Env) bool {
		env.Log().Event("woke up, about to sleep")
		return !env.Sleep(pdur)
	}
}

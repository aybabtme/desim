package desim_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/aybabtme/benchkit"
	"github.com/aybabtme/benchkit/benchplot"
	"github.com/aybabtme/desim/pkg/desim"
	"github.com/aybabtme/desim/pkg/gen"
)

func BenchmarkRun(b *testing.B) {
	actors := []*desim.Actor{
		desim.MakeActor("fast", infiniteclock(500*time.Millisecond)),
		// desim.MakeActor("fast2", infiniteclock(300*time.Millisecond)),
		desim.MakeActor("slow", infiniteclock(1*time.Second)),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doBenchmark(b, time.Hour, actors)
	}
}

func doBenchmark(b *testing.B, maxHistory time.Duration, actors []*desim.Actor) {
	var (
		r     = rand.New(rand.NewSource(42))
		begin = time.Unix(0, 0).UTC()
		end   = begin.Add(maxHistory)
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(begin),
		gen.StaticTime(end),
	)

	start := time.Now()
	evs := sim.Run(
		actors,
		nil,
		desim.LogMute(),
	)

	duration := time.Since(start)
	simulRate := (end.Sub(begin) / duration) * time.Second
	speedup := float64(end.Sub(begin)) / float64(duration)
	eventsRate := float64(len(evs)) / duration.Seconds()
	b.Logf("simulated %v (%d events) in %v - %.1fx factor, %v/s, %.1f events/s", end.Sub(begin), len(evs), duration, speedup, simulRate, eventsRate)

}

func TestGenerateTimeComplexityByActor(t *testing.T) {

	maxHistory := 1 * time.Minute
	frequency := 500 * time.Millisecond
	var (
		begin = time.Unix(0, 0).UTC()
		end   = begin.Add(maxHistory)
	)

	actorCount := 200
	reproductions := 30

	times := benchkit.Bench(benchkit.Time(actorCount, reproductions)).Each(func(each benchkit.BenchEach) {
		for repeat := 0; repeat < reproductions; repeat++ {
			for i := 0; i < actorCount; i++ {
				sim := desim.New(
					desim.NewLocalScheduler,
					rand.New(rand.NewSource(42)),
					gen.StaticTime(begin),
					gen.StaticTime(end),
				)
				var actors []*desim.Actor
				for actorID := 0; actorID < i; actorID++ {
					actors = append(actors,
						desim.MakeActor(
							fmt.Sprintf("actor%d", actorID),
							infiniteclock(frequency),
						),
					)
				}

				each.Before(i)

				_ = sim.Run(actors, nil, desim.LogMute())

				each.After(i)
			}
		}

	}).(*benchkit.TimeResult)

	p, err := benchplot.PlotTime(
		fmt.Sprintf("time to simulate %v of events", maxHistory),
		"actor count",
		times, false,
	)
	if err != nil {
		panic(err)
	}
	if err := p.Save(1270, 960, "time_to_simulate_by_actor_count.png"); err != nil {
		panic(err)
	}
}

func TestGenerateTimeComplexityByEventFrequency(t *testing.T) {

	maxHistory := 1 * time.Minute
	var (
		begin = time.Unix(0, 0).UTC()
		end   = begin.Add(maxHistory)
	)

	eventPerSec := 400
	reproductions := 30

	times := benchkit.Bench(benchkit.Time(eventPerSec, reproductions)).Each(func(each benchkit.BenchEach) {
		for repeat := 0; repeat < reproductions; repeat++ {
			for i := 0; i < eventPerSec; i++ {
				if i == 0 {
					each.Before(i)
					each.After(i)
					continue
				}
				frequency := time.Duration(float64(time.Second) / float64(i))
				sim := desim.New(
					desim.NewLocalScheduler,
					rand.New(rand.NewSource(42)),
					gen.StaticTime(begin),
					gen.StaticTime(end),
				)
				actors := []*desim.Actor{
					desim.MakeActor("actor", infiniteclock(frequency)),
				}

				each.Before(i)

				_ = sim.Run(actors, nil, desim.LogMute())

				each.After(i)
			}
		}

	}).(*benchkit.TimeResult)

	p, err := benchplot.PlotTime(
		fmt.Sprintf("time to simulate %v of events", maxHistory),
		"events per simulated second",
		times, false,
	)
	if err != nil {
		panic(err)
	}
	if err := p.Save(1270, 960, "time_to_simulate_by_frequency.png"); err != nil {
		panic(err)
	}
}

package desim_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/aybabtme/desim/pkg/desim"
	"github.com/aybabtme/desim/pkg/gen"
	"github.com/stretchr/testify/require"
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

	slowActor := desim.MakeActor("slow", clock(6, 1*time.Second))
	fastActor := desim.MakeActor("fast", clock(6, 500*time.Millisecond))

	evs := sim.Run(
		[]*desim.Actor{
			slowActor,
			fastActor,
		},
		desim.LogJSON(ioutil.Discard),
	)
	for _, ev := range evs {
		fmt.Printf("%v: %v\n", ev.Time, ev.Labels["name"])
	}

	// Output:
	// 1970-01-01 00:00:00.5 +0000 UTC: fast
	// 1970-01-01 00:00:01 +0000 UTC: slow
	// 1970-01-01 00:00:01 +0000 UTC: fast
	// 1970-01-01 00:00:01.5 +0000 UTC: fast
	// 1970-01-01 00:00:02 +0000 UTC: fast
	// 1970-01-01 00:00:02 +0000 UTC: slow
	// 1970-01-01 00:00:02.5 +0000 UTC: fast
	// 1970-01-01 00:00:03 +0000 UTC: slow
	// 1970-01-01 00:00:03 +0000 UTC: fast
	// 1970-01-01 00:00:03 +0000 UTC: fast
	// 1970-01-01 00:00:04 +0000 UTC: slow
	// 1970-01-01 00:00:05 +0000 UTC: slow
	// 1970-01-01 00:00:06 +0000 UTC: slow
	// 1970-01-01 00:00:06 +0000 UTC: slow
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

	// slowActor := desim.MakeActor("slow", infiniteclock(1*time.Second))
	// fastActor := desim.MakeActor("fast", infiniteclock(500*time.Millisecond))
	slowActor := desim.MakeActor("slow", randomRateClock(r.Int63(), 1*time.Second))
	fastActor := desim.MakeActor("fast", randomRateClock(r.Int63(), 500*time.Millisecond))

	start := time.Now()
	evs := sim.Run(
		[]*desim.Actor{
			slowActor,
			fastActor,
		},
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

func TestRunSim(t *testing.T) {
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Unix(0, 0).UTC()
		end   = start.Add(10 * time.Second)

		add500ms = 500 * time.Millisecond

		want = []*desim.Event{
			{ID: 1,
				Time:     start.Add(add500ms),
				Priority: 0, Signals: 0x0,
				TieBreakers: [4]int32{2042833275, 380326700, 1175016362, 23962569},
				Labels:      map[string]string{"name": "fast"}},
			{ID: 2,
				Time:     start.Add(2 * add500ms),
				Priority: 0, Signals: 0x0,
				TieBreakers: [4]int32{810344218, 783138040, 1244615815, 1913476789},
				Labels:      map[string]string{"name": "slow"}},
			{ID: 3,
				Time:     start.Add(2 * add500ms),
				Priority: 0, Signals: 0x0,
				TieBreakers: [4]int32{473422256, 1766685442, 199037783, 491052927},
				Labels:      map[string]string{"name": "fast"}},
			{ID: 5,
				Time:     start.Add(3 * add500ms),
				Priority: 0, Signals: 0x0,
				TieBreakers: [4]int32{1552131012, 93140978, 909994430, 772630341},
				Labels:      map[string]string{"name": "fast"}},
			{ID: 6,
				Time:     start.Add(3 * add500ms),
				Priority: 0, Signals: 0x2,
				TieBreakers: [4]int32{2000019877, 2087901139, 1947051166, 1016903292},
				Labels:      map[string]string{"name": "fast"}},
			{ID: 4,
				Time:     start.Add(4 * add500ms),
				Priority: 0, Signals: 0x0,
				TieBreakers: [4]int32{922880067, 1610263671, 143869070, 261388404},
				Labels:      map[string]string{"name": "slow"}},
			{ID: 7,
				Time:     start.Add(6 * add500ms),
				Priority: 0, Signals: 0x0,
				TieBreakers: [4]int32{1105892420, 1893608637, 790905086, 1809097032},
				Labels:      map[string]string{"name": "slow"}},
			{ID: 8,
				Time:     start.Add(6 * add500ms),
				Priority: 0, Signals: 0x2,
				TieBreakers: [4]int32{1586173079, 1996033081, 592272731, 2058568556},
				Labels:      map[string]string{"name": "slow"}},
		}
	)

	sim := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	)
	got := sim.Run(
		[]*desim.Actor{
			desim.MakeActor("slow", clock(3, 1*time.Second)),
			desim.MakeActor("fast", clock(3, 500*time.Millisecond)),
		},
		desim.LogMute(),
	)

	sort.Slice(want, func(i, j int) bool { return want[i].Time.Before(want[j].Time) })
	sort.Slice(got, func(i, j int) bool { return got[i].Time.Before(got[j].Time) })

	require.Equal(t, want, got)
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

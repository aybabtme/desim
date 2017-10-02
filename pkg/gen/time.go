package gen

import (
	"math/rand"
	"time"
)

/*
Times
*/

// Time is a generator for Time values.
type Time interface {
	Gen() time.Time
}

// StaticTime generates a static Time.
func StaticTime(t time.Time) Time {
	return TimeFunc(func() time.Time { return t })
}

// NormalTime generates a Time from a normal distribution,
// centered on the given mean and with the given standard deviation.
func NormalTime(r *rand.Rand, mean time.Time, stdDev time.Duration) Time {
	dur := NormalDuration(r, 0, stdDev)
	return TimeFunc(func() time.Time {
		return mean.Add(dur.Gen())
	})
}

// ExpTime generates a Time from a exponential distribution,
// given a desired average rate.
func ExpTime(r *rand.Rand, now time.Time, avgRate time.Duration) Time {
	dur := ExpDuration(r, avgRate)
	return TimeFunc(func() time.Time {
		return now.Add(dur.Gen())
	})
}

// UniformTime generates a Time from a uniform distribution,
// given a desired range.
func UniformTime(r *rand.Rand, from, to time.Time) Time {
	dur := UniformDuration(r, 0, to.Sub(from))
	return TimeFunc(func() time.Time {
		return from.Add(dur.Gen())
	})
}

// TimeFunc generates Time by invoking a given function.
type TimeFunc func() time.Time

// Gen generates a Time.
func (gen TimeFunc) Gen() time.Time { return gen() }

/*
Durations
*/

// Duration is a generator for Duration values.
type Duration interface {
	Gen() time.Duration
}

// StaticDuration generates a static Duration.
func StaticDuration(t time.Duration) Duration {
	return DurationFunc(func() time.Duration { return t })
}

// NormalDuration generates a Duration from a normal distribution,
// centered on the given mean and with the given standard deviation.
func NormalDuration(r *rand.Rand, mean, stdDev time.Duration) Duration {
	desiredStdDevInSec := stdDev.Seconds()
	desiredMeanInSec := mean.Seconds()
	return DurationFunc(func() time.Duration {
		sampleInSec := r.NormFloat64()*desiredStdDevInSec + desiredMeanInSec
		return time.Duration(sampleInSec * float64(time.Second))
	})
}

// ExpDuration generates a Duration from a exponential distribution,
// given a desired average rate.
func ExpDuration(r *rand.Rand, avgRate time.Duration) Duration {
	avgRateInSec := avgRate.Seconds()

	return DurationFunc(func() time.Duration {
		sampleInSec := r.ExpFloat64() / avgRateInSec
		return time.Duration(sampleInSec * float64(time.Second))
	})
}

// UniformDuration generates a Duration from a uniform distribution,
// given a desired range.
func UniformDuration(r *rand.Rand, from, to time.Duration) Duration {
	fromInSec := from.Seconds()
	toInSec := to.Seconds()
	return DurationFunc(func() time.Duration {
		sampleInSec := (r.Float64() * (toInSec - fromInSec)) * fromInSec
		return time.Duration(sampleInSec * float64(time.Second))
	})
}

// DurationFunc generates Duration by invoking a given function.
type DurationFunc func() time.Duration

// Gen generates a Duration.
func (gen DurationFunc) Gen() time.Duration { return gen() }

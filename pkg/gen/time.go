package gen

import (
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

// DurationFunc generates Duration by invoking a given function.
type DurationFunc func() time.Duration

// Gen generates a Duration.
func (gen DurationFunc) Gen() time.Duration { return gen() }

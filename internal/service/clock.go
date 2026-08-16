package service

import "time"

// Clock abstracts time access so services and workers can be tested deterministically.
type Clock interface {
	Now() time.Time
}

// RealClock returns the actual wall-clock time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// FakeClock is a controllable clock for tests.
type FakeClock struct {
	t time.Time
}

// NewFakeClock creates a fake clock starting at the given instant.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{t: start}
}

func (c *FakeClock) Now() time.Time { return c.t }

// Advance moves the clock forward by d.
func (c *FakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// Set replaces the clock's current time.
func (c *FakeClock) Set(t time.Time) { c.t = t }

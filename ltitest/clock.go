package ltitest

import (
	"sync"
	"time"
)

// Clock is a manually-advanceable implementation of lti.Clock, for
// deterministic tests of expiry/leeway logic.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock returns a Clock fixed at t.
func NewClock(t time.Time) *Clock {
	return &Clock{now: t}
}

// Now implements lti.Clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

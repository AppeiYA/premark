package testsupport

import (
	"sync"
	"time"

	"premark/internal/ports"
)

var _ ports.Clock = (*FixedClock)(nil)

type FixedClock struct {
	mu  sync.RWMutex
	now time.Time
}

func NewFixedClock(t time.Time) *FixedClock {
	return &FixedClock{now: t}
}

func (c *FixedClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *FixedClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *FixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

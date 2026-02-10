package service

import (
	"sync"
	"sync/atomic"
)

// ConcurrencyLimiter limits the number of concurrent tasks globally.
type ConcurrencyLimiter struct {
	max     int32
	current int32
}

// NewConcurrencyLimiter creates a new limiter with the provided max.
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	if max < 0 {
		max = 0
	}
	return &ConcurrencyLimiter{max: int32(max)}
}

// TryAcquire attempts to acquire one slot without blocking.
// It returns a release function and ok=false if no slot is available.
func (l *ConcurrencyLimiter) TryAcquire() (release func(), ok bool) {
	if l == nil {
		return nil, false
	}
	max := atomic.LoadInt32(&l.max)
	if max <= 0 {
		return nil, false
	}
	for {
		current := atomic.LoadInt32(&l.current)
		if current >= max {
			return nil, false
		}
		if atomic.CompareAndSwapInt32(&l.current, current, current+1) {
			var once sync.Once
			release = func() {
				once.Do(func() {
					atomic.AddInt32(&l.current, -1)
				})
			}
			return release, true
		}
	}
}

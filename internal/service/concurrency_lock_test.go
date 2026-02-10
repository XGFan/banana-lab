package service

import "testing"

func TestConcurrencyLock(t *testing.T) {
	limiter := NewConcurrencyLimiter(1)

	release, ok := limiter.TryAcquire()
	if !ok {
		t.Fatalf("expected first TryAcquire to succeed")
	}

	if _, ok := limiter.TryAcquire(); ok {
		t.Fatalf("expected second TryAcquire to fail before release")
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("release should be idempotent, got panic: %v", r)
			}
		}()
		release()
		release()
	}()

	releaseAgain, ok := limiter.TryAcquire()
	if !ok {
		t.Fatalf("expected TryAcquire to succeed after release")
	}
	releaseAgain()
}

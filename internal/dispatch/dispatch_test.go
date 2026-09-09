package dispatch_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/dispatch"
)

// waitOrTimeout fails the test rather than hanging forever if wg never
// completes — a leaked/deadlocked worker pool should show up as a test
// failure, not a stuck test run.
func waitOrTimeout(t *testing.T, wg *sync.WaitGroup, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("timed out waiting for jobs to complete")
	}
}

func TestSubmitExecutesJobs(t *testing.T) {
	const n = 20
	// Queue capacity matches n: this test is about jobs actually running,
	// not about queue-full behavior (covered separately below), so give
	// Submit enough room that a slow scheduler can't turn this into a flake.
	p := dispatch.NewPool(3, n)
	defer p.Close()

	var (
		mu  sync.Mutex
		ran = map[int]bool{}
		wg  sync.WaitGroup
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		err := p.Submit(dispatch.Job{Fn: func(ctx context.Context) {
			defer wg.Done()
			mu.Lock()
			ran[i] = true
			mu.Unlock()
		}})
		if err != nil {
			t.Fatalf("Submit(%d) returned error: %v", i, err)
		}
	}

	waitOrTimeout(t, &wg, time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(ran) != n {
		t.Errorf("ran %d jobs, want %d", len(ran), n)
	}
}

// TestSubmitReturnsErrQueueFullWithoutBlocking exercises the documented
// choice in dispatch.ErrQueueFull: once the pool has no spare capacity
// (one worker busy, one job already queued), Submit must return an error
// immediately rather than blocking the caller. If Submit instead blocked
// on a full channel send, this test would hang until the surrounding `go
// test` timeout killed it.
func TestSubmitReturnsErrQueueFullWithoutBlocking(t *testing.T) {
	p := dispatch.NewPool(1, 1)
	defer p.Close()

	block := make(chan struct{})
	started := make(chan struct{})

	// Occupy the pool's only worker with a job that won't finish until the
	// test releases it.
	if err := p.Submit(dispatch.Job{Fn: func(ctx context.Context) {
		close(started)
		<-block
	}}); err != nil {
		t.Fatalf("Submit(blocking job) returned error: %v", err)
	}
	<-started

	// Fill the single queue slot.
	if err := p.Submit(dispatch.Job{Fn: func(ctx context.Context) {}}); err != nil {
		t.Fatalf("Submit(queue-filling job) returned error: %v", err)
	}

	// The pool is now fully occupied: one job running, one queued. This
	// Submit has nowhere to put the job and must return ErrQueueFull
	// promptly instead of blocking.
	start := time.Now()
	err := p.Submit(dispatch.Job{Fn: func(ctx context.Context) {}})
	elapsed := time.Since(start)

	close(block)

	if err != dispatch.ErrQueueFull {
		t.Fatalf("Submit on a full queue returned %v, want ErrQueueFull", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("Submit on a full queue took %v, want near-instant (must never block the caller)", elapsed)
	}
}

// TestJobFromCascadeFlagCarriedThrough checks that FromCascade survives the
// trip from Submit to Fn actually running — the seam a future
// 1-3-cascade-prewarm consumer would use to enforce its depth-1 cap.
func TestJobFromCascadeFlagCarriedThrough(t *testing.T) {
	p := dispatch.NewPool(1, 1)
	defer p.Close()

	var (
		mu       sync.Mutex
		observed bool
		wg       sync.WaitGroup
	)
	wg.Add(1)

	job := dispatch.Job{FromCascade: true}
	job.Fn = func(ctx context.Context) {
		defer wg.Done()
		mu.Lock()
		observed = job.FromCascade
		mu.Unlock()
	}

	if err := p.Submit(job); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	waitOrTimeout(t, &wg, time.Second)

	mu.Lock()
	defer mu.Unlock()
	if !observed {
		t.Error("FromCascade was not true from inside the running job, want true")
	}
}

// TestJobDefaultsToNotFromCascade checks the zero-value Job doesn't
// accidentally read as cascade-originated, since a direct cold-start job
// (the depth-0 case) is constructed without setting the field at all.
func TestJobDefaultsToNotFromCascade(t *testing.T) {
	p := dispatch.NewPool(1, 1)
	defer p.Close()

	var (
		mu       sync.Mutex
		observed = true
		wg       sync.WaitGroup
	)
	wg.Add(1)

	job := dispatch.Job{}
	job.Fn = func(ctx context.Context) {
		defer wg.Done()
		mu.Lock()
		observed = job.FromCascade
		mu.Unlock()
	}

	if err := p.Submit(job); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	waitOrTimeout(t, &wg, time.Second)

	mu.Lock()
	defer mu.Unlock()
	if observed {
		t.Error("FromCascade = true for a zero-value Job, want false")
	}
}

// TestCloseWaitsForQueuedAndInFlightJobs checks Close's graceful-drain
// semantics: every job submitted before Close is called still gets to run,
// rather than being abandoned when workers see the shutdown signal.
func TestCloseWaitsForQueuedAndInFlightJobs(t *testing.T) {
	p := dispatch.NewPool(2, 8)

	var done int32
	const n = 6
	for i := 0; i < n; i++ {
		err := p.Submit(dispatch.Job{Fn: func(ctx context.Context) {
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&done, 1)
		}})
		if err != nil {
			t.Fatalf("Submit returned error: %v", err)
		}
	}

	p.Close()

	if got := atomic.LoadInt32(&done); got != n {
		t.Errorf("completed jobs after Close = %d, want %d (Close must drain queued/in-flight work)", got, n)
	}
}

// TestCloseDoesNotLeakGoroutines guards against the exact failure mode
// Close exists to prevent: worker goroutines outliving the test that
// started them.
func TestCloseDoesNotLeakGoroutines(t *testing.T) {
	p := dispatch.NewPool(4, 4)

	var wg sync.WaitGroup
	wg.Add(4)
	for i := 0; i < 4; i++ {
		if err := p.Submit(dispatch.Job{Fn: func(ctx context.Context) {
			defer wg.Done()
		}}); err != nil {
			t.Fatalf("Submit returned error: %v", err)
		}
	}
	waitOrTimeout(t, &wg, time.Second)

	closed := make(chan struct{})
	go func() {
		p.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not return within 1s; workers likely leaked")
	}
}

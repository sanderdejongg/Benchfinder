// Package dispatch is a generic, in-process background job dispatch
// mechanism: a bounded, goroutine-based worker pool.
//
// Per 1-4-dispatch-mechanism D1, this deliberately does not pull in a job
// queue library or a message broker (NATS, SQS, Redis) — at MVP scale that
// buys nothing over goroutines and channels, and building the pool directly
// is also a vehicle for learning Go's concurrency primitives. Revisit only
// if a *reliability* requirement shows up (jobs surviving a process
// restart, retry-with-backoff, dead-lettering) — not for throughput alone.
//
// This package has no consumer wired up yet. It is built to be consumed by
// the future 1-3-cascade-prewarm background jobs, but has no dependency on
// that (or any other ingestion) package — see the design doc's Dependencies
// section.
package dispatch

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned by Submit when the bounded queue has no spare
// capacity. Per 1-4-dispatch-mechanism's constraint that submitting a job
// must never block the triggering caller, a full queue is surfaced as an
// immediate error rather than either (a) blocking Submit until a slot frees
// up, which would defeat the whole point of dispatching in the background,
// or (b) silently dropping the job, which would hide the overload from the
// caller entirely. Returning an error lets the caller decide — log it, drop
// it, or (in a future design) apply backpressure upstream.
var ErrQueueFull = errors.New("dispatch: queue full")

// Job is a unit of background work submitted to a Pool.
type Job struct {
	// Fn is the work to run. It receives the Pool's shutdown context, so
	// long-running work should watch ctx.Done() to exit promptly on Close.
	Fn func(ctx context.Context)

	// FromCascade marks a job as originating from a cascade pre-warm job
	// rather than a direct cold-start request. Per 1-3-cascade-prewarm D2,
	// cascade fan-out is capped at depth 1: a cell warmed via cascade must
	// never itself trigger a further cascade. This package only carries the
	// flag through to wherever Fn runs — enforcing the cap is the future
	// cascade consumer's responsibility, not this mechanism's.
	FromCascade bool
}

// Pool is a bounded, goroutine-based worker pool for background jobs.
type Pool struct {
	jobs chan Job
	wg   sync.WaitGroup

	cancel context.CancelFunc
}

// NewPool starts a Pool with workers goroutines pulling from a queue that
// holds up to queueCapacity pending jobs. Workers start immediately; there
// is no separate Start step, matching how the rest of this codebase
// constructs ready-to-use types (see coldstart.NewFetcher).
func NewPool(workers, queueCapacity int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		jobs:   make(chan Job, queueCapacity),
		cancel: cancel,
	}

	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker(ctx)
	}
	return p
}

// worker drains the queue until it is empty and ctx has been cancelled by
// Close. The queued-job case is checked first on every iteration so a
// worker never abandons already-submitted work in favour of exiting early —
// Close's shutdown is a graceful drain, not an abort.
func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case job := <-p.jobs:
			job.Fn(ctx)
			continue
		default:
		}

		select {
		case job := <-p.jobs:
			job.Fn(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// Submit enqueues job for execution by the next free worker. See
// ErrQueueFull's doc comment for why a full queue is an immediate error
// rather than a block or a silent drop.
func (p *Pool) Submit(job Job) error {
	select {
	case p.jobs <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// Close signals workers to stop once the queue is drained, then blocks
// until every worker goroutine has returned. It exists so callers (chiefly
// tests) can shut a Pool down cleanly without leaking goroutines; the
// running API process has no equivalent need at MVP since it lives for the
// process lifetime.
//
// Close does not stop Submit from accepting new jobs — those would just sit
// unprocessed once workers exit. That's an acceptable gap for a shutdown
// path with no current caller outside tests; guarding against it would be
// speculative for a need this package doesn't have yet.
func (p *Pool) Close() {
	p.cancel()
	p.wg.Wait()
}

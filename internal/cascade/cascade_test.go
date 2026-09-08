package cascade_test

import (
	"context"
	"testing"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/cascade"
	"github.com/sanderdejongg/Benchfinder/internal/coldstart"
	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
	"github.com/sanderdejongg/Benchfinder/internal/overpass"
)

// fakeCellStore is an in-memory h3grid.PolledCellStore, same shape as
// coldstart_test.go's fake.
type fakeCellStore struct {
	polledAt map[string]time.Time
}

func (f *fakeCellStore) PolledAt(_ context.Context, h3Index string) (time.Time, bool, error) {
	t, ok := f.polledAt[h3Index]
	return t, ok, nil
}

func (f *fakeCellStore) MarkPolled(_ context.Context, h3Index string, _ int, polledAt time.Time) error {
	if f.polledAt == nil {
		f.polledAt = map[string]time.Time{}
	}
	f.polledAt[h3Index] = polledAt
	return nil
}

// fakeSource is an in-memory coldstart.BenchFetcher that always reports one
// bench found, so every EnsureCellFresh call it backs performs a genuine
// (non-empty) ingest.
type fakeSource struct {
	calls int
}

func (f *fakeSource) FetchBenches(_ context.Context, bbox h3grid.BBox) ([]overpass.Node, error) {
	f.calls++
	lat := (bbox.South + bbox.North) / 2
	lon := (bbox.West + bbox.East) / 2
	return []overpass.Node{{ID: int64(f.calls), Lat: lat, Lon: lon}}, nil
}

// fakeWriter is an in-memory coldstart.BenchWriter.
type fakeWriter struct {
	calls int
}

func (f *fakeWriter) UpsertBenches(_ context.Context, benches []coldstart.Bench) error {
	f.calls++
	return nil
}

// fakeDispatcher is a hand-written in-memory cascade.Dispatcher. Dispatch
// only records the job -- it never runs it -- so tests can assert the
// triggering call doesn't wait on execution, and can run a captured job's
// payload later, on demand, to inspect what it does.
type fakeDispatcher struct {
	jobs []cascade.Job
}

func (d *fakeDispatcher) Dispatch(job cascade.Job) {
	d.jobs = append(d.jobs, job)
}

const (
	lat = 52.3731
	lng = 4.8926
)

var now = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

func newPrewarmer(cells *fakeCellStore, source *fakeSource, writer *fakeWriter, dispatcher *fakeDispatcher) *cascade.Prewarmer {
	fetcher := coldstart.NewFetcher(cells, source, writer)
	return cascade.NewPrewarmer(cells, fetcher, dispatcher)
}

// 1. A genuine cold-start ingest of a previously-unfresh origin cell
// dispatches jobs for its k=2..3 annulus neighbours.
func TestEnsureCellFreshDispatchesAnnulusOnGenuineIngest(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{}
	writer := &fakeWriter{}
	dispatcher := &fakeDispatcher{}

	p := newPrewarmer(cells, source, writer, dispatcher)
	if err := p.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	if len(dispatcher.jobs) == 0 {
		t.Fatal("no jobs dispatched for a genuine ingest of a previously-unfresh cell")
	}

	origin, err := h3grid.CellForPoint(lat, lng)
	if err != nil {
		t.Fatalf("CellForPoint returned error: %v", err)
	}
	for _, j := range dispatcher.jobs {
		if j.Cell == origin {
			t.Errorf("dispatched job for the origin cell %s itself, want only neighbours", origin)
		}
		if !j.CascadeOrigin {
			t.Errorf("job for cell %s has CascadeOrigin=false, want true (D2 flag)", j.Cell)
		}
	}
}

// 2. An origin cell that was already fresh (cache-hit, no real ingest
// happened) dispatches nothing.
func TestEnsureCellFreshNoDispatchWhenOriginAlreadyFresh(t *testing.T) {
	ctx := context.Background()
	origin, err := h3grid.CellForPoint(lat, lng)
	if err != nil {
		t.Fatalf("CellForPoint returned error: %v", err)
	}

	cells := &fakeCellStore{polledAt: map[string]time.Time{origin: now}}
	source := &fakeSource{}
	writer := &fakeWriter{}
	dispatcher := &fakeDispatcher{}

	p := newPrewarmer(cells, source, writer, dispatcher)
	if err := p.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	if len(dispatcher.jobs) != 0 {
		t.Errorf("dispatched %d jobs for an already-fresh origin cell, want 0", len(dispatcher.jobs))
	}
	if source.calls != 0 {
		t.Errorf("Overpass was called %d times for an already-fresh origin cell, want 0", source.calls)
	}
}

// 3. Neighbours that are already fresh are skipped (not dispatched).
func TestEnsureCellFreshSkipsFreshNeighbours(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{}
	writer := &fakeWriter{}
	dispatcher := &fakeDispatcher{}

	p := newPrewarmer(cells, source, writer, dispatcher)

	// First call: genuine ingest, dispatches the full annulus.
	if err := p.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}
	fullAnnulus := len(dispatcher.jobs)
	if fullAnnulus == 0 {
		t.Fatal("no jobs dispatched on the first (genuine ingest) call")
	}

	// Mark one dispatched neighbour as already fresh, then trigger a fresh
	// origin ingest again (different point, same cell would just no-op) --
	// simplest way to re-exercise dispatchNeighbours is to directly mark a
	// neighbour fresh and reset state, then re-run against a *new* origin
	// whose annulus overlaps. Instead, exercise the skip directly: mark the
	// first captured neighbour fresh, reset the origin's own poll record so
	// the origin is unfresh again, and re-run.
	skippedCell := dispatcher.jobs[0].Cell
	cells.polledAt[skippedCell] = now

	origin, err := h3grid.CellForPoint(lat, lng)
	if err != nil {
		t.Fatalf("CellForPoint returned error: %v", err)
	}
	delete(cells.polledAt, origin)
	dispatcher.jobs = nil

	if err := p.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error (second call): %v", err)
	}

	if len(dispatcher.jobs) != fullAnnulus-1 {
		t.Errorf("second call dispatched %d jobs, want %d (annulus minus the one pre-marked fresh)", len(dispatcher.jobs), fullAnnulus-1)
	}
	for _, j := range dispatcher.jobs {
		if j.Cell == skippedCell {
			t.Errorf("dispatched a job for neighbour %s despite it being marked fresh", skippedCell)
		}
	}
}

// 4. A job dispatched via cascade, when its payload is run, does not itself
// originate a further cascade (depth-1 cap holds, D2).
func TestDispatchedJobDoesNotOriginateFurtherCascade(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{}
	writer := &fakeWriter{}
	dispatcher := &fakeDispatcher{}

	p := newPrewarmer(cells, source, writer, dispatcher)
	if err := p.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	jobs := dispatcher.jobs
	if len(jobs) == 0 {
		t.Fatal("no jobs dispatched to exercise")
	}
	dispatcher.jobs = nil // isolate: only see dispatches caused by running a job

	if err := jobs[0].Run(ctx); err != nil {
		t.Fatalf("dispatched job Run returned error: %v", err)
	}

	if len(dispatcher.jobs) != 0 {
		t.Errorf("running a cascade-dispatched job itself dispatched %d further jobs, want 0 (D2 depth-1 cap violated)", len(dispatcher.jobs))
	}
}

// 5. The triggering call's dispatch step doesn't block/wait on the fake
// dispatcher actually running the jobs.
func TestDispatchDoesNotBlockOnJobExecution(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{}
	writer := &fakeWriter{}
	dispatcher := &fakeDispatcher{}

	p := newPrewarmer(cells, source, writer, dispatcher)

	done := make(chan error, 1)
	go func() {
		done <- p.EnsureCellFresh(ctx, lat, lng, now)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EnsureCellFresh returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("EnsureCellFresh did not return promptly -- it appears to block on job execution")
	}

	// Jobs were captured but never run by Dispatch itself.
	if len(dispatcher.jobs) == 0 {
		t.Fatal("no jobs were dispatched")
	}
}

// Package cascade implements cascade pre-warming as described in
// .claude/architecture/designs/1-3-cascade-prewarm.md: after a cell
// finishes a genuine cold-start ingest (internal/coldstart), pre-warm its
// H3 neighbours in the background so a user who walks or scrolls outward
// moves into already-warm cells instead of re-paying the cold-start cost.
package cascade

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/h3-go/v4"

	"github.com/sanderdejongg/Benchfinder/internal/coldstart"
	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
)

// annulusInnerK, annulusOuterK bound the H3 grid-distance annulus this
// design pre-warms around a freshly-ingested cell: k=2..3 at res 8, roughly
// 900m-1400m outward, sized on plausible human walking distance (D1). Not
// configurable -- per .claude/rules/code-style.md, this design calls for
// exactly this range, not a general ring-size knob.
const (
	annulusInnerK = 2
	annulusOuterK = 3
)

// Job is one unit of background pre-warm work: a plain cold-start ingest
// for a single cascade neighbour cell.
//
// Per 1-3-cascade-prewarm D2, a cell warmed via cascade must never itself
// trigger a further cascade -- only a cell reached by a direct/origin user
// query originates one. Run is built to call coldstart.Fetcher.
// EnsureCellFresh directly (the plain ingest), never Prewarmer.
// EnsureCellFresh (the cascading entry point below), which enforces the cap
// structurally. CascadeOrigin carries the same fact as an explicit flag, as
// called for by the design doc's "jobs dispatched by cascade pre-warm carry
// a flag" language -- the future 1-4 worker pool checks it before deciding
// whether to re-dispatch.
type Job struct {
	// Cell is the H3 index of the neighbour being pre-warmed.
	Cell string
	// CascadeOrigin is always true for jobs this package dispatches. See
	// the type doc comment and 1-3-cascade-prewarm D2.
	CascadeOrigin bool
	// Run performs the ingest. Takes only a context so the future
	// dispatcher (1-4-dispatch-mechanism) can invoke it without knowing
	// anything about coldstart or H3 -- the closure captures what it needs
	// at dispatch time.
	Run func(ctx context.Context) error
}

// Dispatcher is the seam this package needs from a background job runner:
// hand it a Job and return without waiting for it to complete. It will be
// satisfied by the worker pool in 1-4-dispatch-mechanism once that design
// lands; until then, this package depends only on this local interface,
// never on a concrete implementation -- same pattern as BenchFetcher /
// BenchWriter in internal/coldstart.
type Dispatcher interface {
	Dispatch(job Job)
}

// Prewarmer runs the direct/origin cold-start flow and, after a genuine
// ingest (not a freshness-cache no-op), dispatches background pre-warm jobs
// for the ingested cell's k=2..3 annulus of H3 neighbours (D1). This is the
// only entry point that originates a cascade -- see Job's doc comment for
// the depth-1 cap this relies on (D2).
type Prewarmer struct {
	cells      h3grid.PolledCellStore
	fetcher    *coldstart.Fetcher
	dispatcher Dispatcher
}

// NewPrewarmer returns a Prewarmer backed by cells for freshness checks,
// fetcher for the actual cold-start ingestion (both the origin's and, via
// dispatched jobs, its neighbours'), and dispatcher for background fan-out.
func NewPrewarmer(cells h3grid.PolledCellStore, fetcher *coldstart.Fetcher, dispatcher Dispatcher) *Prewarmer {
	return &Prewarmer{cells: cells, fetcher: fetcher, dispatcher: dispatcher}
}

// EnsureCellFresh is the cold-start entry point for a direct/origin user
// query: it delegates to coldstart.Fetcher.EnsureCellFresh for the actual
// ingest, then -- if that ingest was a genuine fetch rather than an
// already-fresh no-op -- dispatches cascade pre-warm jobs for the cell's
// neighbours. Dispatch never blocks this call (constraint in the design
// doc): jobs are handed to the Dispatcher and this method returns
// immediately after, without waiting for them to run.
//
// coldstart.Fetcher's public contract reports only success/failure, not
// whether it performed a real fetch. This method recovers that distinction
// itself: it checks freshness before calling EnsureCellFresh, and treats
// "was not fresh before, no error after" as the genuine-ingest signal.
func (p *Prewarmer) EnsureCellFresh(ctx context.Context, lat, lng float64, now time.Time) error {
	wasFresh, cell, err := h3grid.IsFresh(ctx, p.cells, lat, lng, now)
	if err != nil {
		return fmt.Errorf("cascade: freshness check: %w", err)
	}

	if err := p.fetcher.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		return fmt.Errorf("cascade: ensure cell fresh: %w", err)
	}

	if wasFresh {
		// Cache-hit no-op: no genuine ingest happened, so there's nothing
		// new to pre-warm around.
		return nil
	}

	p.dispatchNeighbours(ctx, cell, now)
	return nil
}

// dispatchNeighbours computes cell's k=2..3 annulus and dispatches a
// background pre-warm Job for each neighbour that isn't already fresh.
// Errors here (annulus computation, freshness lookups) are deliberately
// swallowed rather than returned: cascade pre-warm is a best-effort
// optimization layered on top of an ingest that has already succeeded, and
// a Job's own EnsureCellFresh call re-checks freshness before doing
// anything -- so failing open (dispatching a job we weren't sure was
// necessary) costs at most a redundant no-op, never incorrect behaviour.
func (p *Prewarmer) dispatchNeighbours(ctx context.Context, cell string, now time.Time) {
	neighbours, err := annulus(cell)
	if err != nil {
		return
	}

	for _, n := range neighbours {
		if p.isFresh(ctx, n, now) {
			continue
		}
		p.dispatcher.Dispatch(p.jobFor(n, now))
	}
}

// isFresh reports whether neighbour cell n is already fresh, so
// dispatchNeighbours can skip it. Mirrors h3grid.IsFresh's freshness
// comparison, but against a known cell index rather than a lat/lng point.
// On lookup error, reports not-fresh -- see dispatchNeighbours' comment on
// why failing open here is safe.
func (p *Prewarmer) isFresh(ctx context.Context, n string, now time.Time) bool {
	polledAt, found, err := p.cells.PolledAt(ctx, n)
	if err != nil || !found {
		return false
	}
	return now.Sub(polledAt) < h3grid.FreshnessWindow
}

// jobFor builds the pre-warm Job for neighbour cell n. Run resolves n to a
// point inside it and calls the plain coldstart ingest for that point --
// never Prewarmer.EnsureCellFresh, per the depth-1 cap (D2).
func (p *Prewarmer) jobFor(n string, now time.Time) Job {
	fetcher := p.fetcher
	return Job{
		Cell:          n,
		CascadeOrigin: true,
		Run: func(ctx context.Context) error {
			lat, lng, err := cellCenter(n)
			if err != nil {
				return fmt.Errorf("cascade: cell center for %s: %w", n, err)
			}
			if err := fetcher.EnsureCellFresh(ctx, lat, lng, now); err != nil {
				return fmt.Errorf("cascade: run job for %s: %w", n, err)
			}
			return nil
		},
	}
}

// annulus returns the H3 cell indices at grid distance annulusInnerK..
// annulusOuterK from originIndex. GridDiskDistances buckets cells by exact
// grid distance from the origin, which sidesteps the pentagon-adjacency
// edge cases a plain "ring at distance k" call can hit near one of H3's 12
// pentagons -- we only want the annulus, not a plain ring.
func annulus(originIndex string) ([]string, error) {
	origin := h3.CellFromString(originIndex)
	if !origin.IsValid() {
		return nil, fmt.Errorf("cascade: invalid origin cell %q", originIndex)
	}

	rings, err := origin.GridDiskDistances(annulusOuterK)
	if err != nil {
		return nil, fmt.Errorf("cascade: grid disk distances for %q: %w", originIndex, err)
	}

	var neighbours []string
	for k := annulusInnerK; k <= annulusOuterK && k < len(rings); k++ {
		for _, c := range rings[k] {
			if !c.IsValid() {
				// Zero cell left by pentagon distortion -- see h3-go's
				// GridDiskDistances doc comment -- not a real neighbour.
				continue
			}
			neighbours = append(neighbours, c.String())
		}
	}
	return neighbours, nil
}

// cellCenter returns a point inside the H3 cell identified by index, so
// coldstart.Fetcher.EnsureCellFresh (which takes lat/lng, not a cell index)
// can be called for a specific neighbour cell without changing its public
// contract.
func cellCenter(index string) (lat, lng float64, err error) {
	cell := h3.CellFromString(index)
	if !cell.IsValid() {
		return 0, 0, fmt.Errorf("cascade: invalid cell %q", index)
	}
	center, err := cell.LatLng()
	if err != nil {
		return 0, 0, fmt.Errorf("cascade: cell center for %q: %w", index, err)
	}
	return center.Lat, center.Lng, nil
}

package coldstart

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
	"github.com/sanderdejongg/Benchfinder/internal/overpass"
)

// bboxPadding expands the primary Overpass query beyond the exact H3 cell
// boundary, so a bench just across a cell edge is still caught by the first
// attempt. Reasonable default, not derived — same convention as the
// constants in 2-nearby-search.
const bboxPadding = 0.25

// BenchFetcher queries an external bench data source (Overpass in
// production) for a bounding box. An interface so cold-start orchestration
// can be tested without a network dependency.
type BenchFetcher interface {
	FetchBenches(ctx context.Context, bbox h3grid.BBox) ([]overpass.Node, error)
}

// BenchWriter upserts fetched benches, keyed on (source, source_id) so
// re-polling a cell is always safe — see 1-ingestion-pipeline D8.
type BenchWriter interface {
	UpsertBenches(ctx context.Context, benches []Bench) error
}

// Fetcher runs the cold-start flow: if the H3 cell containing a point is
// already fresh, EnsureCellFresh is a no-op. Otherwise it blocks on a
// synchronous Overpass fetch, upserts the results, and marks the cell
// polled — but only on success (D2).
type Fetcher struct {
	cells   h3grid.PolledCellStore
	source  BenchFetcher
	benches BenchWriter
}

// NewFetcher returns a Fetcher backed by cells for freshness tracking,
// source for the live Overpass fetch, and benches for the upsert.
func NewFetcher(cells h3grid.PolledCellStore, source BenchFetcher, benches BenchWriter) *Fetcher {
	return &Fetcher{cells: cells, source: source, benches: benches}
}

// EnsureCellFresh makes sure the H3 cell containing (lat, lng) is fresh as
// of now. On failure or timeout, the cell is left unpolled — matching the
// "200 with an empty array, not marked polled" contract from D2 — and it is
// the caller's job (the future 2-nearby-search HTTP handler) to turn that
// error into the empty-result response; this package only reports it.
func (f *Fetcher) EnsureCellFresh(ctx context.Context, lat, lng float64, now time.Time) error {
	fresh, cell, err := h3grid.IsFresh(ctx, f.cells, lat, lng, now)
	if err != nil {
		return fmt.Errorf("coldstart: freshness check: %w", err)
	}
	if fresh {
		return nil
	}

	nodes, err := f.fetchCell(ctx, cell)
	if err != nil {
		return fmt.Errorf("coldstart: fetch cell %s: %w", cell, err)
	}

	if len(nodes) > 0 {
		benches := make([]Bench, 0, len(nodes))
		for _, n := range nodes {
			b, err := benchFromNode(n)
			if err != nil {
				return fmt.Errorf("coldstart: map node %d: %w", n.ID, err)
			}
			benches = append(benches, b)
		}

		if err := f.benches.UpsertBenches(ctx, benches); err != nil {
			return fmt.Errorf("coldstart: upsert cell %s: %w", cell, err)
		}
	}

	if err := f.cells.MarkPolled(ctx, cell, h3grid.Resolution, now); err != nil {
		return fmt.Errorf("coldstart: mark cell %s polled: %w", cell, err)
	}
	return nil
}

// fetchCell runs the Overpass query for cell. If the padded query comes
// back empty, it retries once against the exact (unpadded) cell boundary
// before concluding the area is genuinely empty — see
// 1-2-cold-start-ingestion's "retry with a smaller radius."
func (f *Fetcher) fetchCell(ctx context.Context, cell string) ([]overpass.Node, error) {
	bbox, err := h3grid.CellBBox(cell)
	if err != nil {
		return nil, err
	}

	nodes, err := f.source.FetchBenches(ctx, padBBox(bbox, bboxPadding))
	if err != nil {
		return nil, err
	}
	if len(nodes) > 0 {
		return nodes, nil
	}

	return f.source.FetchBenches(ctx, bbox)
}

func padBBox(b h3grid.BBox, factor float64) h3grid.BBox {
	latPad := (b.North - b.South) * factor
	lngPad := (b.East - b.West) * factor
	return h3grid.BBox{
		South: b.South - latPad,
		North: b.North + latPad,
		West:  b.West - lngPad,
		East:  b.East + lngPad,
	}
}

// benchFromNode maps an Overpass node to a Bench, computing its own
// (finest-resolution) H3 cell from its actual coordinates rather than
// inheriting the polled cell's index — a bench caught by the padded query
// may genuinely belong to a neighbouring cell, and h3_index is used for
// ingestion-side reconciliation (see 1-1-h3-grid-freshness).
func benchFromNode(n overpass.Node) (Bench, error) {
	cell, err := h3grid.CellForPoint(n.Lat, n.Lon)
	if err != nil {
		return Bench{}, err
	}

	return Bench{
		SourceID:     strconv.FormatInt(n.ID, 10),
		Lat:          n.Lat,
		Lon:          n.Lon,
		H3Index:      cell,
		HasBackrest:  tagBool(n.Tags, "backrest"),
		IsCovered:    tagBool(n.Tags, "covered"),
		IsAccessible: tagBool(n.Tags, "wheelchair"),
		Tags:         n.Tags,
	}, nil
}

// tagBool reads an OSM yes/no tag as a tri-state bool: nil if the tag is
// absent or holds neither "yes" nor "no".
func tagBool(tags map[string]string, key string) *bool {
	v, ok := tags[key]
	if !ok {
		return nil
	}
	switch v {
	case "yes":
		b := true
		return &b
	case "no":
		b := false
		return &b
	default:
		return nil
	}
}

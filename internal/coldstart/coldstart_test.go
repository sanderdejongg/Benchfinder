package coldstart_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/coldstart"
	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
	"github.com/sanderdejongg/Benchfinder/internal/overpass"
)

// fakeCellStore is an in-memory h3grid.PolledCellStore.
type fakeCellStore struct {
	polledAt map[string]time.Time
	markErr  error
}

func (f *fakeCellStore) PolledAt(_ context.Context, h3Index string) (time.Time, bool, error) {
	t, ok := f.polledAt[h3Index]
	return t, ok, nil
}

func (f *fakeCellStore) MarkPolled(_ context.Context, h3Index string, _ int, polledAt time.Time) error {
	if f.markErr != nil {
		return f.markErr
	}
	if f.polledAt == nil {
		f.polledAt = map[string]time.Time{}
	}
	f.polledAt[h3Index] = polledAt
	return nil
}

// fakeSource is an in-memory coldstart.BenchFetcher, returning a queued
// response per call so tests can script the padded-then-retry sequence.
type fakeSource struct {
	responses [][]overpass.Node
	calls     []h3grid.BBox
	err       error
}

func (f *fakeSource) FetchBenches(_ context.Context, bbox h3grid.BBox) ([]overpass.Node, error) {
	f.calls = append(f.calls, bbox)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) == 0 {
		return nil, nil
	}
	next := f.responses[0]
	f.responses = f.responses[1:]
	return next, nil
}

// fakeWriter is an in-memory coldstart.BenchWriter.
type fakeWriter struct {
	written []coldstart.Bench
	calls   int
	err     error
}

func (f *fakeWriter) UpsertBenches(_ context.Context, benches []coldstart.Bench) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	f.written = append(f.written, benches...)
	return nil
}

const (
	lat = 52.3731
	lng = 4.8926
)

var now = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

func TestEnsureCellFreshNoOpWhenAlreadyFresh(t *testing.T) {
	ctx := context.Background()
	cell, err := h3grid.CellForPoint(lat, lng)
	if err != nil {
		t.Fatalf("CellForPoint returned error: %v", err)
	}

	cells := &fakeCellStore{polledAt: map[string]time.Time{cell: now}}
	source := &fakeSource{}
	writer := &fakeWriter{}

	f := coldstart.NewFetcher(cells, source, writer)
	if err := f.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	if len(source.calls) != 0 {
		t.Errorf("Overpass was called %d times for an already-fresh cell, want 0", len(source.calls))
	}
	if writer.calls != 0 {
		t.Errorf("UpsertBenches was called %d times for an already-fresh cell, want 0", writer.calls)
	}
}

func TestEnsureCellFreshFetchesAndMarksPolled(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{responses: [][]overpass.Node{
		{{ID: 1, Lat: lat, Lon: lng, Tags: map[string]string{"backrest": "yes"}}},
	}}
	writer := &fakeWriter{}

	f := coldstart.NewFetcher(cells, source, writer)
	if err := f.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	if len(source.calls) != 1 {
		t.Fatalf("Overpass was called %d times, want 1 (should not retry when the first call finds results)", len(source.calls))
	}
	if len(writer.written) != 1 || writer.written[0].SourceID != "1" {
		t.Fatalf("writer.written = %+v, want one bench with SourceID 1", writer.written)
	}
	if writer.written[0].HasBackrest == nil || !*writer.written[0].HasBackrest {
		t.Errorf("written bench HasBackrest = %v, want true", writer.written[0].HasBackrest)
	}

	cell, _ := h3grid.CellForPoint(lat, lng)
	if polledAt, found := cells.polledAt[cell]; !found || !polledAt.Equal(now) {
		t.Errorf("cell %s not marked polled at %v: found=%v polledAt=%v", cell, now, found, polledAt)
	}
}

func TestEnsureCellFreshRetriesWithSmallerBBoxWhenFirstCallEmpty(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{responses: [][]overpass.Node{
		{},                                       // padded query: nothing
		{{ID: 2, Lat: lat, Lon: lng, Tags: nil}}, // tight retry: found one
	}}
	writer := &fakeWriter{}

	f := coldstart.NewFetcher(cells, source, writer)
	if err := f.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	if len(source.calls) != 2 {
		t.Fatalf("Overpass was called %d times, want 2 (padded, then tight retry)", len(source.calls))
	}
	first, second := source.calls[0], source.calls[1]
	if second.South <= first.South || second.North >= first.North {
		t.Errorf("retry bbox %+v is not tighter than the first call's bbox %+v", second, first)
	}
	if len(writer.written) != 1 || writer.written[0].SourceID != "2" {
		t.Fatalf("writer.written = %+v, want the retry's bench", writer.written)
	}
}

func TestEnsureCellFreshGenuinelyEmptyStillMarksPolled(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{} // both calls return no results
	writer := &fakeWriter{}

	f := coldstart.NewFetcher(cells, source, writer)
	if err := f.EnsureCellFresh(ctx, lat, lng, now); err != nil {
		t.Fatalf("EnsureCellFresh returned error: %v", err)
	}

	if writer.calls != 0 {
		t.Errorf("UpsertBenches was called %d times for a genuinely empty cell, want 0", writer.calls)
	}
	cell, _ := h3grid.CellForPoint(lat, lng)
	if _, found := cells.polledAt[cell]; !found {
		t.Error("a genuinely empty cell should still be marked polled")
	}
}

func TestEnsureCellFreshOverpassFailureDoesNotMarkPolled(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{err: errors.New("overpass: boom")}
	writer := &fakeWriter{}

	f := coldstart.NewFetcher(cells, source, writer)
	err := f.EnsureCellFresh(ctx, lat, lng, now)
	if err == nil {
		t.Fatal("EnsureCellFresh returned no error for an Overpass failure")
	}

	if writer.calls != 0 {
		t.Errorf("UpsertBenches was called %d times after an Overpass failure, want 0", writer.calls)
	}
	cell, _ := h3grid.CellForPoint(lat, lng)
	if _, found := cells.polledAt[cell]; found {
		t.Error("cell was marked polled despite an Overpass failure (D2 violation)")
	}
}

func TestEnsureCellFreshUpsertFailureDoesNotMarkPolled(t *testing.T) {
	ctx := context.Background()
	cells := &fakeCellStore{}
	source := &fakeSource{responses: [][]overpass.Node{
		{{ID: 1, Lat: lat, Lon: lng}},
	}}
	writer := &fakeWriter{err: errors.New("db: boom")}

	f := coldstart.NewFetcher(cells, source, writer)
	err := f.EnsureCellFresh(ctx, lat, lng, now)
	if err == nil {
		t.Fatal("EnsureCellFresh returned no error for an upsert failure")
	}

	cell, _ := h3grid.CellForPoint(lat, lng)
	if _, found := cells.polledAt[cell]; found {
		t.Error("cell was marked polled despite an upsert failure")
	}
}

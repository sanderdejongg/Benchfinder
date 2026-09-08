package h3grid_test

import (
	"context"
	"testing"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
)

// fakeStore is an in-memory PolledCellStore, so freshness logic can be
// tested without a database.
type fakeStore struct {
	polledAt map[string]time.Time
}

func (f *fakeStore) PolledAt(_ context.Context, h3Index string) (time.Time, bool, error) {
	t, ok := f.polledAt[h3Index]
	return t, ok, nil
}

func (f *fakeStore) MarkPolled(_ context.Context, h3Index string, _ int, polledAt time.Time) error {
	if f.polledAt == nil {
		f.polledAt = map[string]time.Time{}
	}
	f.polledAt[h3Index] = polledAt
	return nil
}

func TestIsFresh(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	const lat, lng = 52.3731, 4.8926

	cell, err := h3grid.CellForPoint(lat, lng)
	if err != nil {
		t.Fatalf("CellForPoint returned error: %v", err)
	}

	tests := map[string]struct {
		polledAt  time.Time
		polled    bool
		wantFresh bool
	}{
		"never polled": {
			polled:    false,
			wantFresh: false,
		},
		"polled just now": {
			polled:    true,
			polledAt:  now,
			wantFresh: true,
		},
		"polled 29 days ago": {
			polled:    true,
			polledAt:  now.Add(-29 * 24 * time.Hour),
			wantFresh: true,
		},
		"polled exactly 30 days ago": {
			polled:    true,
			polledAt:  now.Add(-h3grid.FreshnessWindow),
			wantFresh: false,
		},
		"polled 31 days ago": {
			polled:    true,
			polledAt:  now.Add(-31 * 24 * time.Hour),
			wantFresh: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			if tc.polled {
				if err := store.MarkPolled(ctx, cell, h3grid.Resolution, tc.polledAt); err != nil {
					t.Fatalf("MarkPolled returned error: %v", err)
				}
			}

			fresh, gotCell, err := h3grid.IsFresh(ctx, store, lat, lng, now)
			if err != nil {
				t.Fatalf("IsFresh returned error: %v", err)
			}
			if gotCell != cell {
				t.Errorf("IsFresh cell = %q, want %q", gotCell, cell)
			}
			if fresh != tc.wantFresh {
				t.Errorf("IsFresh = %v, want %v", fresh, tc.wantFresh)
			}
		})
	}
}

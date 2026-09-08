package h3grid

import (
	"context"
	"time"
)

// IsFresh reports whether the H3 cell containing (lat, lng) has been polled
// within FreshnessWindow of now. cell is returned regardless of error so
// callers can use it (e.g. for a subsequent Overpass fetch) without
// recomputing it.
func IsFresh(ctx context.Context, store PolledCellStore, lat, lng float64, now time.Time) (fresh bool, cell string, err error) {
	cell, err = CellForPoint(lat, lng)
	if err != nil {
		return false, "", err
	}

	polledAt, found, err := store.PolledAt(ctx, cell)
	if err != nil {
		return false, cell, err
	}
	if !found {
		return false, cell, nil
	}

	return now.Sub(polledAt) < FreshnessWindow, cell, nil
}

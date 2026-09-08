// Package h3grid implements the H3-grid freshness tracking described in
// .claude/architecture/designs/1-1-h3-grid-freshness.md: which areas have
// already been ingested from OSM, at a grain coarse enough for neighbouring
// queries to share a cache.
package h3grid

import (
	"fmt"
	"time"

	"github.com/uber/h3-go/v4"
)

// Resolution is the primary H3 resolution used for freshness tracking.
const Resolution = 8

// FreshnessWindow is how long a polled cell is considered fresh (D4).
const FreshnessWindow = 30 * 24 * time.Hour

// CellForPoint returns the res-8 H3 cell index containing the given point.
func CellForPoint(lat, lng float64) (string, error) {
	cell, err := h3.LatLngToCell(h3.NewLatLng(lat, lng), Resolution)
	if err != nil {
		return "", fmt.Errorf("h3grid: compute cell for point: %w", err)
	}
	return cell.String(), nil
}

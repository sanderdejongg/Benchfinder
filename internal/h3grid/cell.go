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

// BBox is a lat/lng bounding box, in degrees.
type BBox struct {
	South, West, North, East float64
}

// CellBBox returns the bounding box of the H3 cell identified by h3Index.
// Used by cold-start ingestion (1-2-cold-start-ingestion) to scope its
// Overpass query to the cell being polled.
func CellBBox(h3Index string) (BBox, error) {
	cell := h3.CellFromString(h3Index)
	if !cell.IsValid() {
		return BBox{}, fmt.Errorf("h3grid: invalid cell index %q", h3Index)
	}

	boundary, err := cell.Boundary()
	if err != nil {
		return BBox{}, fmt.Errorf("h3grid: cell boundary for %q: %w", h3Index, err)
	}

	bbox := BBox{South: 90, West: 180, North: -90, East: -180}
	for _, v := range boundary {
		bbox.South = min(bbox.South, v.Lat)
		bbox.North = max(bbox.North, v.Lat)
		bbox.West = min(bbox.West, v.Lng)
		bbox.East = max(bbox.East, v.Lng)
	}
	return bbox, nil
}

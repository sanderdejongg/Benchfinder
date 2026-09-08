// Package coldstart implements the cold-start ingestion flow described in
// .claude/architecture/designs/1-2-cold-start-ingestion.md: on an unpolled
// or stale H3 cell, synchronously fetch amenity=bench nodes from Overpass,
// upsert them, and mark the cell polled — but only on a successful fetch.
package coldstart

// Bench is a single OSM bench fetched from Overpass, ready to upsert into
// the benches table. HasBackrest, IsCovered and IsAccessible are tri-state
// (nil means the source tag was absent or not yes/no).
type Bench struct {
	SourceID     string
	Lat, Lon     float64
	H3Index      string
	HasBackrest  *bool
	IsCovered    *bool
	IsAccessible *bool
	Tags         map[string]string
}

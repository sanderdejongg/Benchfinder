package coldstart_test

import (
	"context"
	"os"
	"testing"

	"github.com/sanderdejongg/Benchfinder/internal/coldstart"
	"github.com/sanderdejongg/Benchfinder/internal/db"
)

// TestPostgresBenchStore exercises PostgresBenchStore against a real
// database. It requires DATABASE_URL to point at a Postgres instance with
// migrations applied (`make up && make migrate-up`), and is skipped
// otherwise so `go test ./...` stays usable without one.
func TestPostgresBenchStore(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres-backed test")
	}

	ctx := context.Background()

	pool, err := db.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("db.New returned error: %v", err)
	}
	// t.Cleanup, not defer: registered first so it runs last (LIFO), after
	// the row-deletion cleanup below — a deferred Close here would run
	// before that cleanup and leave it deleting against a closed pool.
	t.Cleanup(pool.Close)

	store := coldstart.NewPostgresBenchStore(pool)

	// A synthetic OSM source_id, can't collide with real ingestion data.
	const sourceID = "test-store-integration-bench"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM benches WHERE source = 'osm' AND source_id = $1`, sourceID)
	})

	backrest := true
	bench := coldstart.Bench{
		SourceID:    sourceID,
		Lat:         52.3731,
		Lon:         4.8926,
		H3Index:     "test-cell",
		HasBackrest: &backrest,
		Tags:        map[string]string{"amenity": "bench"},
	}

	if err := store.UpsertBenches(ctx, []coldstart.Bench{bench}); err != nil {
		t.Fatalf("UpsertBenches (insert) returned error: %v", err)
	}

	var gotBackrest bool
	var gotH3Index string
	err = pool.QueryRow(ctx,
		`SELECT has_backrest, h3_index FROM benches WHERE source = 'osm' AND source_id = $1`,
		sourceID,
	).Scan(&gotBackrest, &gotH3Index)
	if err != nil {
		t.Fatalf("query after insert returned error: %v", err)
	}
	if !gotBackrest || gotH3Index != "test-cell" {
		t.Fatalf("got has_backrest=%v h3_index=%q, want true / \"test-cell\"", gotBackrest, gotH3Index)
	}

	// Upsert again with a changed field should update, not duplicate.
	bench.H3Index = "test-cell-2"
	if err := store.UpsertBenches(ctx, []coldstart.Bench{bench}); err != nil {
		t.Fatalf("UpsertBenches (update) returned error: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM benches WHERE source = 'osm' AND source_id = $1`,
		sourceID,
	).Scan(&count); err != nil {
		t.Fatalf("count query returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("found %d rows for source_id %q after re-upsert, want 1 (upsert should not duplicate)", count, sourceID)
	}

	if err := pool.QueryRow(ctx,
		`SELECT h3_index FROM benches WHERE source = 'osm' AND source_id = $1`,
		sourceID,
	).Scan(&gotH3Index); err != nil {
		t.Fatalf("query after update returned error: %v", err)
	}
	if gotH3Index != "test-cell-2" {
		t.Fatalf("h3_index after update = %q, want \"test-cell-2\"", gotH3Index)
	}
}

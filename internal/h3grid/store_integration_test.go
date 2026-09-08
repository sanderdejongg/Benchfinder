package h3grid_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/db"
	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
)

// TestPostgresStore exercises PostgresStore against a real database. It
// requires DATABASE_URL to point at a Postgres instance with migrations
// applied (`make up && make migrate-up`), and is skipped otherwise so `go
// test ./...` stays usable without one.
func TestPostgresStore(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres-backed test")
	}

	ctx := context.Background()

	pool, err := db.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("db.New returned error: %v", err)
	}
	defer pool.Close()

	store := h3grid.NewPostgresStore(pool)

	// A synthetic index — polled_cells has no format constraint on h3_index —
	// so this can't collide with real ingestion data.
	const testCell = "test-store-integration-cell"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM polled_cells WHERE h3_index = $1`, testCell)
	})

	if _, found, err := store.PolledAt(ctx, testCell); err != nil {
		t.Fatalf("PolledAt (before write) returned error: %v", err)
	} else if found {
		t.Fatalf("PolledAt (before write) found a row unexpectedly")
	}

	first := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.MarkPolled(ctx, testCell, h3grid.Resolution, first); err != nil {
		t.Fatalf("MarkPolled returned error: %v", err)
	}

	got, found, err := store.PolledAt(ctx, testCell)
	if err != nil {
		t.Fatalf("PolledAt (after write) returned error: %v", err)
	}
	if !found {
		t.Fatal("PolledAt (after write) found no row")
	}
	if !got.Equal(first) {
		t.Fatalf("PolledAt = %v, want %v", got, first)
	}

	// MarkPolled again should upsert, not duplicate.
	second := first.Add(time.Hour)
	if err := store.MarkPolled(ctx, testCell, h3grid.Resolution, second); err != nil {
		t.Fatalf("MarkPolled (update) returned error: %v", err)
	}

	got, found, err = store.PolledAt(ctx, testCell)
	if err != nil {
		t.Fatalf("PolledAt (after update) returned error: %v", err)
	}
	if !found {
		t.Fatal("PolledAt (after update) found no row")
	}
	if !got.Equal(second) {
		t.Fatalf("PolledAt after update = %v, want %v", got, second)
	}
}

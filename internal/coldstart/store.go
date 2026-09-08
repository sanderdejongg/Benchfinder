package coldstart

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// upsertChunkSize bounds how many benches are upserted per transaction, so
// an unusually large cold-start fetch doesn't hold one huge transaction
// open — see 1-2-cold-start-ingestion's "chunked, transactional upsert."
// Reasonable default, not derived.
const upsertChunkSize = 500

// PostgresBenchStore is the BenchWriter backed by the benches table.
type PostgresBenchStore struct {
	pool *pgxpool.Pool
}

// NewPostgresBenchStore returns a PostgresBenchStore using pool for writes.
func NewPostgresBenchStore(pool *pgxpool.Pool) *PostgresBenchStore {
	return &PostgresBenchStore{pool: pool}
}

// UpsertBenches upserts benches keyed on (source, source_id), in chunks of
// upsertChunkSize benches per transaction.
func (s *PostgresBenchStore) UpsertBenches(ctx context.Context, benches []Bench) error {
	for start := 0; start < len(benches); start += upsertChunkSize {
		end := min(start+upsertChunkSize, len(benches))
		if err := s.upsertChunk(ctx, benches[start:end]); err != nil {
			return fmt.Errorf("coldstart: upsert benches [%d:%d]: %w", start, end, err)
		}
	}
	return nil
}

func (s *PostgresBenchStore) upsertChunk(ctx context.Context, chunk []Bench) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	for _, b := range chunk {
		tags := b.Tags
		if tags == nil {
			tags = map[string]string{}
		}
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("marshal tags for source_id %s: %w", b.SourceID, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO benches (source, source_id, geog, h3_index, has_backrest, is_covered, is_accessible, tags, updated_at)
			VALUES ('osm', $1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, $5, $6, $7, $8, now())
			ON CONFLICT (source, source_id) DO UPDATE SET
				geog          = EXCLUDED.geog,
				h3_index      = EXCLUDED.h3_index,
				has_backrest  = EXCLUDED.has_backrest,
				is_covered    = EXCLUDED.is_covered,
				is_accessible = EXCLUDED.is_accessible,
				tags          = EXCLUDED.tags,
				updated_at    = now()
		`, b.SourceID, b.Lon, b.Lat, b.H3Index, b.HasBackrest, b.IsCovered, b.IsAccessible, tagsJSON)
		if err != nil {
			return fmt.Errorf("upsert source_id %s: %w", b.SourceID, err)
		}
	}

	return tx.Commit(ctx)
}

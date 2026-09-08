package h3grid

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PolledCellStore reads and writes the polled_cells table. Defined as an
// interface so callers can test freshness logic against a fake, without a
// database.
type PolledCellStore interface {
	// PolledAt returns when h3Index was last polled. found is false if the
	// cell has never been polled.
	PolledAt(ctx context.Context, h3Index string) (polledAt time.Time, found bool, err error)

	// MarkPolled records a successful ingestion of h3Index at the given
	// resolution, upserting the row.
	MarkPolled(ctx context.Context, h3Index string, resolution int, polledAt time.Time) error
}

// PostgresStore is the PolledCellStore backed by the polled_cells table.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore returns a PostgresStore using pool for queries.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) PolledAt(ctx context.Context, h3Index string) (time.Time, bool, error) {
	var polledAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT polled_at FROM polled_cells WHERE h3_index = $1`,
		h3Index,
	).Scan(&polledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("h3grid: query polled_at: %w", err)
	}
	return polledAt, true, nil
}

func (s *PostgresStore) MarkPolled(ctx context.Context, h3Index string, resolution int, polledAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO polled_cells (h3_index, resolution, polled_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (h3_index) DO UPDATE
			SET resolution = EXCLUDED.resolution,
			    polled_at = EXCLUDED.polled_at
	`, h3Index, resolution, polledAt)
	if err != nil {
		return fmt.Errorf("h3grid: upsert polled_cells: %w", err)
	}
	return nil
}

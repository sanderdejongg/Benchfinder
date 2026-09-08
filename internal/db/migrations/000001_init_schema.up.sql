CREATE EXTENSION IF NOT EXISTS postgis;

-- Core bench schema (see .claude/architecture/decisions/1-ingestion-pipeline.md
-- for the settled contract). h3_index exists for ingestion-side reconciliation
-- only; spatial search uses the geog GIST index, not this column
-- (see .claude/architecture/designs/1-1-h3-grid-freshness.md).
CREATE TABLE benches (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    geog GEOGRAPHY(Point, 4326) NOT NULL,
    h3_index TEXT NOT NULL,
    has_backrest BOOLEAN,
    is_covered BOOLEAN,
    is_accessible BOOLEAN,
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, source_id)
);

CREATE INDEX benches_geog_gix ON benches USING GIST (geog);
CREATE INDEX benches_h3_index_idx ON benches (h3_index);

-- Freshness tracking grid (see .claude/architecture/designs/1-1-h3-grid-freshness.md).
-- `resolution` is always 8 at MVP; the column exists so a finer resolution can
-- be introduced later without a migration.
CREATE TABLE polled_cells (
    h3_index TEXT PRIMARY KEY,
    resolution SMALLINT NOT NULL,
    polled_at TIMESTAMPTZ NOT NULL
);

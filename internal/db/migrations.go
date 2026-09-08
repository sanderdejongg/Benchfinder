package db

import "embed"

// MigrationsFS holds the SQL migration files, embedded so cmd/migrate can run
// without needing the source tree present at runtime.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

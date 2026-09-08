// Command migrate applies or rolls back the schema in internal/db/migrations
// against DATABASE_URL.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/sanderdejongg/Benchfinder/internal/db"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		log.Fatal("usage: migrate <up|down>")
	}
	direction := os.Args[1]

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	source, err := iofs.New(db.MigrationsFS, "migrations")
	if err != nil {
		log.Fatalf("migrate: load migration source: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, toPgx5URL(databaseURL))
	if err != nil {
		log.Fatalf("migrate: init: %v", err)
	}

	if direction == "up" {
		err = m.Up()
	} else {
		err = m.Down()
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrate: %s: %v", direction, err)
	}

	log.Printf("migrate: %s complete", direction)
}

// toPgx5URL rewrites a postgres:// connection string to the pgx5:// scheme
// golang-migrate's pgx/v5 database driver registers itself under.
func toPgx5URL(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(databaseURL, prefix) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, prefix)
		}
	}
	return fmt.Sprintf("pgx5://%s", databaseURL)
}

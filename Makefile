.PHONY: up down logs psql ingest migrate-up migrate-down

-include .env

DATABASE_URL ?= postgres://benchfinder:devpassword@localhost:5432/benchfinder?sslmode=disable

up:
	docker compose up -d postgres

down:
	docker compose down

logs:
	docker compose logs -f

psql:
	docker compose exec postgres psql -U $${POSTGRES_USER:-benchfinder} -d $${POSTGRES_DB:-benchfinder}

ingest:
	docker compose --profile tools run --rm ingest

migrate-up:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/migrate up

migrate-down:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/migrate down

.PHONY: up down logs psql ingest

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

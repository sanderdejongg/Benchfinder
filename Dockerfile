# Scaffold Dockerfile — expects a Go module at the repo root with:
#   cmd/api/main.go     (REST API)
#   cmd/ingest/main.go  (batch ingestion job, D5/D7)
# Neither exists yet. Relocate this file if the Go module ends up in a
# subdirectory (e.g. backend/) instead of the repo root.
#
# uber/h3-go is CGo-based (see deployment-shape decision log, "CGo build
# implications"), so this can't use CGO_ENABLED=0 or an Alpine/musl base —
# the builder stage needs a real C toolchain and the runtime stage needs glibc.

FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/ingest ./cmd/ingest

FROM debian:bookworm-slim AS api-base
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 1000 benchfinder
USER benchfinder

FROM api-base AS api
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

FROM api-base AS ingest
COPY --from=build /out/ingest /usr/local/bin/ingest
ENTRYPOINT ["/usr/local/bin/ingest"]

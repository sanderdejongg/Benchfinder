#syntax=docker/dockerfile:1

# Dev and test database (6-architecture-setup D28, D37, D39).
# Multi-arch PostGIS image: postgis/postgis is amd64-only. Versions are
# provisional until read from the managed instance (D37).
FROM imresamu/postgis:17-3.6.1-trixie

# PGDG packages stop at h3-pg 4.2.3, so build the pinned release from source.
ARG H3_PG_VERSION=4.5.0

SHELL ["/bin/bash", "-euxo", "pipefail", "-c"]

# hadolint ignore=DL3008
RUN <<-EOF
	apt-get update
	apt-get install -y --no-install-recommends \
		build-essential \
		ca-certificates \
		cmake \
		git \
		postgresql-server-dev-17
	git clone --depth 1 --branch "v${H3_PG_VERSION}" https://github.com/postgis/h3-pg.git /tmp/h3-pg
	cmake -S /tmp/h3-pg -B /tmp/h3-pg/build -DCMAKE_BUILD_TYPE=Release
	cmake --build /tmp/h3-pg/build
	cmake --install /tmp/h3-pg/build --component h3-pg
	rm -rf /tmp/h3-pg
	apt-get purge -y --auto-remove build-essential cmake git postgresql-server-dev-17
	rm -rf /var/lib/apt/lists/*
EOF

# The base image enables postgis, topology and tiger in POSTGRES_DB on init.
# A managed instance starts with none, so migrations create the extensions.
RUN rm /docker-entrypoint-initdb.d/10_postgis.sh

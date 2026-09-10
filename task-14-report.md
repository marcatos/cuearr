# Task 14 — Important findings (fix)

## Status: fixed

### 1. Docker Compose bind-mount paths
- **Issue:** `./watch` and `./out` in `deploy/docker/docker-compose.yml` resolve against the Compose project directory; running `docker compose -f deploy/docker/docker-compose.yml` from the repo root mounted the wrong host paths.
- **Fix:** Document `cd deploy/docker && docker compose up` in README; add comment in `docker-compose.yml`; keep `${CUEARR_WATCH_DIR:-./watch}` / `${CUEARR_OUT_DIR:-./out}`; add `deploy/docker/watch/.gitkeep` and `deploy/docker/out/.gitkeep` placeholders.

### 2. Multi-arch Docker build
- **Issue:** `Dockerfile` hard-coded `GOOS=linux` without `TARGETOS` / `TARGETARCH`, so `docker buildx` multi-platform builds could produce wrong binaries.
- **Fix:** `ARG TARGETOS=linux` and `ARG TARGETARCH=amd64`; `GOOS=${TARGETOS} GOARCH=${TARGETARCH}` on `go build` (buildx supplies platform args on release).

### 3. Container user / data volume (documented)
- **Note:** Runtime user is `cuearr` (UID 1000). README documents UID for bind mounts; named volume `cuearr-data` is managed by Docker (no entrypoint chown added).

## Verification
- Compose paths: run from `deploy/docker` per README.
- Local build: `docker build -f deploy/docker/Dockerfile` uses linux/amd64 defaults when platform args omitted.

## Commit
- `3ec8639` — `fix: correct Docker compose paths and multi-arch build args`

# Cuearr

*arr-style daemon that splits **lossless image + CUE** albums into per-track files for Lidarr and Plex.

> Design (approved brainstorm): [`docs/superpowers/specs/2026-09-10-cuearr-design.md`](docs/superpowers/specs/2026-09-10-cuearr-design.md)

## Status

Pre-implementation. Spec under review. Tracking: **GitHub Issues** on [marcatos/cuearr](https://github.com/marcatos/cuearr).

## Build

Requires Go 1.23+.

```bash
go build -o bin/cuearr ./cmd/cuearr
```

On Windows:

```powershell
go build -o bin/cuearr.exe ./cmd/cuearr
```

## Run

```bash
./bin/cuearr version
./bin/cuearr serve
```

## Docker

Image: [`ghcr.io/marcatos/cuearr`](https://github.com/marcatos/cuearr/pkgs/container/cuearr) (multi-arch `linux/amd64`, `linux/arm64`). Runtime includes **shntool**, **cuetools**, and **flac**.

Build and run with Compose (from repo root):

```bash
export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
docker compose -f deploy/docker/docker-compose.yml up -d --build
```

Volumes: **`cuearr-data`** (SQLite + settings under `/data`), host **`watch`** → `/watch`, host **`out`** → `/out`. Override paths with `CUEARR_WATCH_DIR` / `CUEARR_OUT_DIR`. Optional bootstrap env: `CUEARR_API_KEY`.

Manual build:

```bash
docker build -f deploy/docker/Dockerfile -t cuearr:local .
docker run --rm -p 8787:8787 \
  -e CUEARR_INITIAL_PASSWORD='…' \
  -v cuearr-data:/data -v "$(pwd)/watch:/watch" -v "$(pwd)/out:/out" \
  cuearr:local
```

Tagged releases publish GitHub release archives (`linux`/`darwin`/`windows` amd64/arm64 where applicable) and push the container image (workflow: `.github/workflows/release.yml`).

## Lidarr Connect

Cuearr accepts the same hook URL for **Connect webhooks** and an optional **custom script**.

1. In Lidarr → **Settings → Connect**, add a **Webhook**.
   - URL: `http://<cuearr-host>:8787/api/v1/hooks/lidarr`
   - Method: **POST**
   - Header: `X-Api-Key` = your Cuearr API key (Settings in the UI, or bootstrap env).
   - Triggers: **On Download** and/or **On Import**.
   - The handler resolves a scan path from the JSON body, in order: `path`, `DownloadPath`, `DownloadFolder`, `lidarr_trackfile_path`, `lidarr_release_path`, `Lidarr_AddedTrackPaths` (first entry), `trackFiles[0].path`, then the same keys under `environment` or `env`. File paths are reduced to their parent album directory before enqueue.

2. Optional **Custom Script** (e.g. after import): copy [`scripts/lidarr-custom-script.sh`](scripts/lidarr-custom-script.sh) and point Lidarr at it.
   - Required env: `CUEARR_URL` (e.g. `http://cuearr:8787`), `CUEARR_API_KEY`.
   - Path resolution (first match): `Lidarr_AddedTrackPaths` (first path), `lidarr_trackfile_path`, `lidarr_release_path`, or the first script argument.
   - Lidarr sets these when the script runs; the script posts safe JSON (`jq` or `python3`) to the same hook.

Response: `{"job_id":"<uuid>","created":true}` when a new split job is queued (`created:false` if the album was already queued or completed).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Use GitHub Issues and the provided templates for bugs and feature requests.

## Security

See [SECURITY.md](SECURITY.md) for how to report vulnerabilities privately.

## License

MIT

# Cuearr

*arr-style daemon that splits **lossless image + CUE** albums into per-track files for Lidarr and Plex.

> Design: [`docs/superpowers/specs/2026-09-10-cuearr-design.md`](docs/superpowers/specs/2026-09-10-cuearr-design.md)

## Status

**v0.1** feature set is implemented on branch `feat/v1`. Track work on [GitHub Issues](https://github.com/marcatos/cuearr/issues). Tagged releases (`v0.1.0`+) publish binaries and container images after CI on `main` is green.

## Quickstart

### Binary (Linux / macOS / Windows)

1. Download the archive for your OS/arch from [GitHub Releases](https://github.com/marcatos/cuearr/releases) (or build from source below).
2. Copy [`configs/cuearr.example.yaml`](configs/cuearr.example.yaml) to `cuearr.yaml` and set `data_dir`, `watch_dirs`, and `out_dir`.
3. On first run, set a local password (stored hashed in SQLite):

   ```bash
   export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
   ./bin/cuearr serve -config cuearr.yaml
   ```

4. Open **http://127.0.0.1:8787**, sign in, copy the **API key** from Settings (or bootstrap with `CUEARR_API_KEY` before first boot).
5. Drop a folder containing `album.flac` + `album.cue` under a watch path; Cuearr enqueues a split job. Output lands under a collision-safe album subdirectory of `out_dir` (or in-place when enabled).

Build from source (Go 1.23+):

```bash
go build -o bin/cuearr ./cmd/cuearr
./bin/cuearr version
./bin/cuearr serve -config configs/cuearr.example.yaml
```

Windows:

```powershell
go build -o bin/cuearr.exe ./cmd/cuearr
```

### Tests

Unit tests (default CI):

```bash
go test ./...
```

End-to-end split against a **synthetic** fixture (requires `shntool` and generated audio; skipped otherwise):

```bash
# Generate silent image + CUE (ffmpeg, or sox+flac on Unix)
bash scripts/generate_fixture.sh
# or: pwsh scripts/generate_fixture.ps1

go test ./internal/app -run TestE2E_SplitFixtureWithShntool -count=1
```

The test is skipped automatically when `shntool` or the fixture files are missing.

## Authentication

| Method | Use |
|--------|-----|
| **Local password** | UI login; set on first boot via `CUEARR_INITIAL_PASSWORD` |
| **API key** | `X-Api-Key` header for `/api/v1/*` and Lidarr hooks; bootstrap with `CUEARR_API_KEY` |
| **OIDC** (optional) | `CUEARR_OIDC_ENABLED`, `CUEARR_OIDC_ISSUER`, `CUEARR_OIDC_CLIENT_ID`, `CUEARR_OIDC_CLIENT_SECRET`, `CUEARR_OIDC_REDIRECT_URL`, `CUEARR_OIDC_ALLOWED_EMAIL_DOMAINS` — see [`configs/cuearr.example.yaml`](configs/cuearr.example.yaml) |

Session cookies protect the embedded UI; API routes accept the API key without a browser session. Set `cookie_secure: true` or `CUEARR_COOKIE_SECURE=true` for HTTPS deployments behind a reverse proxy.

## Runtime settings

On first start, Cuearr seeds watch directories, output directory, in-place mode, and engine from YAML into SQLite. Persisted SQLite settings are authoritative on later starts. Updating these fields through `PUT /api/v1/settings` applies them immediately; changing watch directories restarts the filesystem watcher, and queued jobs use the current output mode and splitter. HTTP address, data directory, log level, and secure-cookie mode remain startup-only and require a restart.

## Docker

Image: [`ghcr.io/marcatos/cuearr`](https://github.com/marcatos/cuearr/pkgs/container/cuearr) (multi-arch `linux/amd64`, `linux/arm64`). Runtime includes **shntool**, **cuetools**, and **flac**.

Build and run with Compose from `deploy/docker` (Compose resolves `./watch` and `./out` relative to that directory):

```bash
cd deploy/docker
export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
docker compose up -d --build
```

Volumes: **`cuearr-data`** (SQLite + settings under `/data`), host **`deploy/docker/watch`** → `/watch`, host **`deploy/docker/out`** → `/out`. Override bind paths with `CUEARR_WATCH_DIR` / `CUEARR_OUT_DIR` (paths relative to `deploy/docker` unless absolute). Optional bootstrap env: `CUEARR_API_KEY`.

The container runs as user **`cuearr` (UID 1000)**; bind-mounted watch/out directories must be readable/writable by that UID if permissions matter on your host.

Manual build:

```bash
docker build -f deploy/docker/Dockerfile -t cuearr:local .
docker run --rm -p 8787:8787 \
  -e CUEARR_INITIAL_PASSWORD='…' \
  -v cuearr-data:/data -v "$(pwd)/watch:/watch" -v "$(pwd)/out:/out" \
  cuearr:local
```

Tagged releases publish GitHub release archives and push the container image (workflow: `.github/workflows/release.yml`).

## Helm

Chart: [`deploy/helm/cuearr`](deploy/helm/cuearr). Requires a cluster with persistent storage for `/data` and mounts for watch/out (defaults use **hostPath**; switch to PVC in `values.yaml`).

```bash
helm upgrade --install cuearr ./deploy/helm/cuearr \
  --set auth.initialPassword='choose-a-strong-password' \
  --set persistence.watch.hostPath=/srv/cuearr/watch \
  --set persistence.out.hostPath=/srv/cuearr/out
```

Set `image.tag` to a release tag, or leave empty for `appVersion`. OIDC and existing secrets are configured under `auth.*` and `oidc.*` in [`values.yaml`](deploy/helm/cuearr/values.yaml). The chart defaults the pod and container to UID/GID 1000 and applies `fsGroup: 1000`; pre-existing host paths must still permit Kubernetes to apply or inherit compatible ownership.

## Proxmox (LXC)

On a **Proxmox VE** host, run [`deploy/proxmox/cuearr-install.sh`](deploy/proxmox/cuearr-install.sh) as root to create a Debian LXC (privileged or unprivileged), install **shntool** / **cuetools** / **flac**, fetch the latest GitHub release binary, and enable `cuearr.service`. Data lives under `/var/lib/cuearr`; UI on port **8787**. Inside an existing Debian container: `bash cuearr-install.sh --inside`.

## Unraid

Import [`deploy/unraid/cuearr.xml`](deploy/unraid/cuearr.xml) (Docker → Add Container → Template URL) or copy to `/boot/config/plugins/dockerMan/templates-user/`. Image `ghcr.io/marcatos/cuearr`, port **8787**, paths `/data`, `/watch`, `/out`, env `CUEARR_*` (set `CUEARR_INITIAL_PASSWORD` on first run).

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

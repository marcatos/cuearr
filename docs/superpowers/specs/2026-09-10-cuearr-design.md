# Cuearr — design (2026-09-10)

## Problem

Album downloads often ship as a single lossless **image** file (typically FLAC) plus a **CUE sheet**. Lidarr and Plex Music expect **one file per track**. Without a reliable pre-import split, imports fail or collapse to a single track and playback/metadata suffer.

## Product

**Cuearr** is a public, MIT-licensed Go *arr-style daemon that splits CUE + lossless image albums into per-track files so Lidarr and Plex can manage them correctly.

**Repo:** `github.com/marcatos/cuearr` (public)  
**Local path:** `Documents/Projects/cuearr`  
**Tracker:** GitHub Issues only (no Plane/Linear for this project)

## Goals (v1)

- Watch folders: detect image+CUE → enqueue → split → write output (or fail with actionable errors)
- Lidarr integration: HTTP webhook Connect + example custom script calling the same API
- Web UI (embedded): queue, history/detail/logs, settings
- Auth: local password + API key + OIDC
- Split engine default: shell-out to `shntool` + `cuetools` (gapless-friendly)
- Persistence: SQLite for settings and job history
- Distribution:
  - Binaries: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
  - Docker multi-arch image (includes split tools)
  - Official `docker-compose`
  - Helm chart
  - Proxmox community-style install script
  - Unraid app/template

## Non-goals (v1)

- Complete pure-Go / in-process splitter (stub + feature flag + GitHub issue only)
- Replacing Lidarr matching, metadata, or library management
- Lossy transcoding
- Multi-tenant SaaS
- Shipping copyrighted audio fixtures in CI

## Architecture

Single Go process, hexagonal package layout (not microservices):

```
cmd/cuearr
internal/domain          # Job, CueSheet, SplitPlan, Folder, AuthIdentity (no IO)
internal/app             # DetectAlbum, EnqueueSplit, RunSplit, ImportHook
internal/ports           # Watcher, Splitter, JobStore, SettingsStore, Notifier
internal/adapters/fs
internal/adapters/splitter/shntool   # default
internal/adapters/splitter/native    # stub / not implemented in v1
internal/adapters/store/sqlite
internal/adapters/http               # REST + Lidarr webhook + embedded UI
internal/adapters/auth               # password, API key, OIDC
web/                                 # static UI, go:embed
```

### Data flow

1. Filesystem watcher **or** Lidarr webhook/script → `DetectAlbum` (locate `.cue` + lossless image)
2. `EnqueueSplit` → SQLite job `queued` (idempotent: skip if same album already completed for that path/fingerprint)
3. Worker → `RunSplit` via `Splitter` port (default shntool)
4. Output to configured `out` directory or `inplace` mode; status `completed` / `failed` + logs
5. UI/API expose queue/history; Lidarr imports already-split tracks

### Configuration

YAML file + environment overrides. Secrets (password hash, API keys, OIDC client secret) never logged.

## HTTP API (v1)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/health` | Liveness; sqlite OK; optional shntool presence |
| GET/POST | `/api/v1/jobs` | List / create |
| GET | `/api/v1/jobs/{id}` | Detail + logs |
| POST | `/api/v1/jobs/scan` | Manual watch-folder scan |
| GET/PUT | `/api/v1/settings` | Folders, engine, auth, OIDC |
| POST | `/api/v1/hooks/lidarr` | Lidarr Connect payload (download/import path) |

Auth middleware accepts **API key header**, **password session**, or **OIDC**.

## UI (v1)

- Dashboard: queued / running / failed
- History: job detail + split logs
- Settings: watch/out paths, engine, local auth, OIDC, Lidarr webhook docs
- Not a music library browser

## Lidarr integration

1. Connect webhook → `POST /api/v1/hooks/lidarr`
2. Example custom script in-repo calling the same API
3. Recommended layout: Cuearr writes into a path Lidarr already uses for import, or `inplace` while the release is still in staging

## Packaging & CI

- GitHub Actions: test + build matrix; release on tag publishes binaries + container image
- Compose, Helm, Proxmox script, Unraid template versioned under `deploy/` (or equivalent)
- Docker/Proxmox paths assume system split tools present (image or script-installed)
- Native Go engine remains optional later; packagers do not depend on it for v1

## Observability

- Structured logs with levels; include job id and paths; never secrets
- Durations for detect / split / total
- Job progress states: `queued` → `running` → `completed` | `failed`
- Common failures surfaced in UI: bad CUE encoding, missing image, shntool nonzero exit, unmounted volume paths

## Testing

- Domain: CUE fixtures, split plan, path safety
- App: enqueue idempotency
- Adapters: fake exec for shntool; sqlite integration
- HTTP: auth middleware with API key / session / OIDC mock
- CI audio: synthetic FLAC + CUE only

## GitHub Issues (contributor workflow)

- Issue templates: Bug, Feature, Split/engine failure
- Labels: `bug`, `enhancement`, `good first issue`, `help wanted`, `engine:shntool`, `engine:native`, `packaging:docker|helm|proxmox|unraid`, `area:ui|api|watcher|auth`
- PR template; CONTRIBUTING; SECURITY; MIT LICENSE
- Day-0 seed issues covering MVP slices (watcher, shntool adapter, API/UI shell, local auth, OIDC, compose, helm stub, proxmox stub, unraid stub, native-engine deferred)

CONTRIBUTING states explicitly that **GitHub Issues** is the project tracker.

## Definition of Done — v1

1. `cuearr serve` binary works; Docker multi-arch image includes split tools
2. Watch-folder E2E: image+CUE → per-track files in output
3. Lidarr webhook + example script documented
4. UI: queue / history / settings; password + API key + OIDC
5. Release assets: common binaries, compose, Helm, Proxmox community script, Unraid template
6. Public MIT repo with issue/PR templates, labels, seed issues, green CI on `main`

## Decisions locked in brainstorming

| Topic | Choice |
|-------|--------|
| Name | Cuearr |
| Integration | Watch folder + Lidarr hook + web UI |
| Language | Go, single binary, embedded UI |
| Split engine | Shell-out default; native Go later (hybrid) |
| Auth | Local password/API key **and** OIDC in v1 |
| License | MIT |
| Architecture | Monolithic daemon, hexagonal packages |
| Tracker | GitHub Issues |

## Open follow-ups (post-spec / implementation plan)

- Exact OIDC claim mapping and callback paths
- Default folder layout env names for Unraid/Proxmox
- Whether release image publishes to GHCR only or also Docker Hub
- Fingerprint algorithm for job idempotency (path hash vs content hash)

<p align="center">
  <img src="docs/branding/cuearr-wordmark.svg" alt="Cuearr" width="420"/>
</p>

<p align="center">
  <strong>Split lossless image+CUE albums into per-track files</strong><br/>
  so Lidarr and Plex can import and play them correctly.
</p>

<p align="center">
  <a href="https://github.com/marcatos/cuearr/releases/tag/v0.1.0"><img alt="release" src="https://img.shields.io/github/v/release/marcatos/cuearr?include_prereleases&amp;label=release"/></a>
  <a href="https://github.com/marcatos/cuearr/pkgs/container/cuearr"><img alt="ghcr" src="https://img.shields.io/badge/ghcr-cuearr-0B3D4A"/></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-3ECFB2"/></a>
  <a href="https://github.com/marcatos/cuearr/issues"><img alt="issues" src="https://img.shields.io/github/issues/marcatos/cuearr"/></a>
</p>

---

## Status

**v0.2.0** is current: [binaries + notes](https://github.com/marcatos/cuearr/releases/tag/v0.2.0), container [`ghcr.io/marcatos/cuearr:v0.2.0`](https://github.com/marcatos/cuearr/pkgs/container/cuearr) (`:latest` tracks the newest tag). Includes B1 verified splits + B2 file reliability. See [releasing](docs/releasing.md).

**Roadmap:** [Project board](https://github.com/users/marcatos/projects/1) · [Milestones](https://github.com/marcatos/cuearr/milestones) · [product strategy](docs/product-strategy-2026-09-10.md) · [batches B1–B6](docs/roadmap-batches.md) · [Issues](https://github.com/marcatos/cuearr/issues)

**Active now:** **B6** competitive comparison ([#30](https://github.com/marcatos/cuearr/issues/30)) and **B3** assisted beta ([#23](https://github.com/marcatos/cuearr/issues/23)–[#25](https://github.com/marcatos/cuearr/issues/25)). **B1**/**B2** shipped in v0.2.0 ([file reliability](docs/file-reliability.md)).

**Verified integration (B1):** [Lidarr/Plex demo path](docs/lidarr-plex-demo.md) · [compatibility matrix](docs/compatibility-matrix.md) (automated tests + Docker smoke only; live Lidarr/Plex assumed until you validate).

**Beta (B3):** [Recruitment + reporting](docs/beta.md) · [10-minute onboarding](docs/onboarding.md) — synthetic fixture first, then signup issue for the assisted cohort.

## Why Cuearr exists

Many music downloads still ship as **one big FLAC (or WAV/APE) + a `.cue` sheet**. That layout is fine for archival players. It is a poor fit for the *arr + Plex stack:

| Without Cuearr | With Cuearr (goal) |
|----------------|---------------------|
| Lidarr often sees a single “track” or fails quality/match expectations | Per-track FLACs under `out_dir` ([verified split cases](docs/compatibility-matrix.md)) |
| Plex Music shows one file, weak track browsing | Intended: per-track library after Lidarr import ([Plex not verified in CI](docs/lidarr-plex-demo.md#plex-music)) |
| Manual `shntool` / Flacon every time | Watch folder + Lidarr webhook/script ([demo path](docs/lidarr-plex-demo.md)) |

Cuearr focuses on **verified** splits and a clear Lidarr-oriented path (see strategy: compare vs Unpackerr / Splittarr / Flacon before expanding scope). Flow: watch (or hook) → split with **shntool** → drop ready tracks where Lidarr looks.

## What it does (v0.1)

- **Watch folders** — detects image+`.cue`, queues a job, writes tracks under a collision-safe album subdirectory of `out_dir` (or in-place)
- **Lidarr Connect** — `POST /api/v1/hooks/lidarr` (+ [custom script](scripts/lidarr-custom-script.sh)); understands real Connect payloads (`trackFiles[].path`, Lidarr env vars)
- **Embedded UI** — queue, history/logs, settings (port **8787**)
- **Auth** — local password, API key (`X-Api-Key`), optional **OIDC** (e.g. Authentik) with email-domain allow-list + `email_verified`
- **Engine** — default **shntool** (+ cuetools/flac in the container); native Go splitter is deferred ([#10](https://github.com/marcatos/cuearr/issues/10))
- **Persistence** — SQLite jobs + settings; UI settings apply at runtime (watcher restart when watch dirs change); [job fingerprint](docs/fingerprint.md) includes full image content hash
- **Ops-friendly packaging** — binary releases, Docker/Compose, Helm, Proxmox LXC script, Unraid template

## Not in scope

Cuearr does **not** replace Lidarr (matching/metadata), does not transcode to lossy, and does not manage your music library UI beyond the split queue.

## Homelab install paths

Pick what you already run:

| Platform | Start here |
|----------|------------|
| **Docker Compose** | [`deploy/docker`](deploy/docker) |
| **Unraid** | [`deploy/unraid/cuearr.xml`](deploy/unraid/cuearr.xml) |
| **Proxmox VE** | [`deploy/proxmox/cuearr-install.sh`](deploy/proxmox/cuearr-install.sh) |
| **Kubernetes** | [`deploy/helm/cuearr`](deploy/helm/cuearr) |
| **Bare metal / LXC binary** | [Releases](https://github.com/marcatos/cuearr/releases) or build below |

Image: [`ghcr.io/marcatos/cuearr`](https://github.com/marcatos/cuearr/pkgs/container/cuearr) (`linux/amd64`, `linux/arm64`) — includes `shntool`, `cuetools`, `flac`.

### Docker Compose (fastest smoke test)

```bash
cd deploy/docker
export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
docker compose up -d --build
```

Open **http://\<host\>:8787**. Volumes: `cuearr-data` → `/data`, `./watch` → `/watch`, `./out` → `/out` (paths relative to `deploy/docker`). Override with `CUEARR_WATCH_DIR` / `CUEARR_OUT_DIR`. Container user is **UID 1000**.

### Unraid

1. Docker → Add Container → Template, or copy [`cuearr.xml`](deploy/unraid/cuearr.xml) into `/boot/config/plugins/dockerMan/templates-user/`.
2. Map **appdata** → `/data`, incoming image+CUE → `/watch`, Lidarr-ready tracks → `/out`.
3. Set `CUEARR_INITIAL_PASSWORD` on first boot.

Icon URL (CA / template):  
`https://raw.githubusercontent.com/marcatos/cuearr/main/docs/branding/cuearr-icon.png`

### Proxmox

On the **PVE host** (root):

```bash
bash deploy/proxmox/cuearr-install.sh
```

Creates a Debian LXC, installs split tools, pulls the latest GitHub release binary, enables `cuearr.service`, data under `/var/lib/cuearr`, UI on **:8787**. Inside an existing CT: `bash cuearr-install.sh --inside`.

### Helm

```bash
helm upgrade --install cuearr ./deploy/helm/cuearr \
  --set auth.initialPassword='choose-a-strong-password' \
  --set persistence.watch.hostPath=/srv/cuearr/watch \
  --set persistence.out.hostPath=/srv/cuearr/out
```

Defaults to UID/GID **1000** + `fsGroup`; hostPath installs can use the chart’s ownership init container (`hostPathOwnershipInit`). Prefer PVC when you can. Details in [`values.yaml`](deploy/helm/cuearr/values.yaml).

### Binary

From [Releases](https://github.com/marcatos/cuearr/releases) (linux/macOS/Windows amd64 & arm64 where built), or:

```bash
go build -o bin/cuearr ./cmd/cuearr   # Go 1.23+
cp configs/cuearr.example.yaml cuearr.yaml
export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
./bin/cuearr serve -config cuearr.yaml
```

Host needs `shntool` (and friends) on `PATH` unless you only use the Docker image.

## Typical *arr wiring

```text
download client
    → incomplete / staging (image + .cue)
        → Cuearr watch  OR  Lidarr Connect webhook
            → /out (per-track FLACs)
                → Lidarr import
                    → Plex Music library
```

**Lidarr webhook:** Settings → Connect → Webhook  
`POST http://<cuearr>:8787/api/v1/hooks/lidarr` with header `X-Api-Key: <key>`  
Triggers: On Download / On Import. Full walkthrough: [Lidarr/Plex demo path](docs/lidarr-plex-demo.md). Path keys: [Lidarr Connect](#lidarr-connect).

**Custom script:** [`scripts/lidarr-custom-script.sh`](scripts/lidarr-custom-script.sh) with `CUEARR_URL` + `CUEARR_API_KEY` (same endpoint as the webhook).

Response: `{"job_id":"…","created":true|false}` (idempotent per album fingerprint).

## Auth (facts)

| Method | When |
|--------|------|
| Password | UI login; bootstrap `CUEARR_INITIAL_PASSWORD` |
| API key | Automation / Lidarr; bootstrap `CUEARR_API_KEY` or Settings |
| OIDC | Optional; env `CUEARR_OIDC_*` — domain allow-list requires verified email |

HTTPS reverse proxy: set `cookie_secure: true` / `CUEARR_COOKIE_SECURE=true`.

## Configuration notes

- First boot seeds watch/out/engine from YAML into SQLite; **SQLite wins afterwards**.
- `PUT /api/v1/settings` updates runtime (watcher restarts if watch dirs change). The only supported `engine` value is `shntool`; requests selecting deferred `native` return `400 Bad Request`. HTTP listen address and `data_dir` still need a process restart.
- Non–in-place jobs write under `out_dir/<album-subdir>/` so default shntool names do not collide across albums.

## Development

```bash
go test ./...
bash scripts/generate_fixture.sh   # or pwsh scripts/generate_fixture.ps1
go test ./internal/app -run TestE2E_SplitFixtureWithShntool -count=1
```

E2E split test **skips** if `shntool` or the synthetic fixture is missing (no copyrighted audio in CI).

Design / plan: [`docs/superpowers/specs/2026-09-10-cuearr-design.md`](docs/superpowers/specs/2026-09-10-cuearr-design.md) · Branding: [`docs/branding`](docs/branding)

## Lidarr Connect

1. Lidarr → **Settings → Connect** → **Webhook**
   - URL: `http://<cuearr-host>:8787/api/v1/hooks/lidarr`
   - Header: `X-Api-Key: <cuearr-api-key>`
   - On Download and/or On Import
   - Path keys (first match): `path`, `DownloadPath`, `DownloadFolder`, `lidarr_trackfile_path`, `lidarr_release_path`, `Lidarr_AddedTrackPaths` (first), `trackFiles[0].path`, then the same under `environment` / `env`. Files are reduced to the album directory.

2. Optional script: [`scripts/lidarr-custom-script.sh`](scripts/lidarr-custom-script.sh) — prefers `Lidarr_AddedTrackPaths`, `lidarr_trackfile_path`, `lidarr_release_path`.

## Contributing & support

Homelabbers and *arr users welcome. Tracker is **[GitHub Issues](https://github.com/marcatos/cuearr/issues)** only (bug / feature / split-failure templates). See [CONTRIBUTING.md](CONTRIBUTING.md). Security: [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)

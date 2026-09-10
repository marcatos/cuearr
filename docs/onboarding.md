# 10-minute onboarding (beta)

Quick path: run Cuearr in Docker, drop a synthetic album in the watch folder, confirm verified tracks in `out`. No Lidarr required for this smoke test.

## Prerequisites

- Docker with Compose
- ~10 minutes

## 1. Start Cuearr with watch + out dirs

From the repo root:

```bash
cd deploy/docker
export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
docker compose up -d --build
```

Compose defaults (no extra env needed):

| Host (`deploy/docker`) | Container | Purpose |
|------------------------|-----------|---------|
| `./watch` | `/watch` | Drop image + `.cue` here |
| `./out` | `/out` | Per-track FLACs appear here |
| `cuearr-data` volume | `/data` | Queue + settings |

Override mounts with `CUEARR_WATCH_DIR` / `CUEARR_OUT_DIR` if your layout differs.

Open **http://\<host\>:8787** and log in with the password you set.

## 2. Confirm split tools in the container

The published image ships **shntool**, **cuetools**, and **flac**. Sanity check:

```bash
docker compose exec cuearr sh -c 'command -v shntool && command -v metaflac && shntool --version'
```

If any command is missing, rebuild from this repo or pull a current tag from [GHCR](https://github.com/marcatos/cuearr/pkgs/container/cuearr).

## 3. Generate a synthetic album and drop it in watch

On the **host** (repo root), create a copyright-free fixture:

```bash
bash scripts/generate_fixture.sh
# default output: testdata/e2e_album/ (album.flac + album.cue)
```

Copy the album folder into the watch mount:

```bash
cp -r testdata/e2e_album deploy/docker/watch/
```

On Windows, use `pwsh scripts/generate_fixture.ps1` and copy the folder into `deploy\docker\watch\`.

## 4. What to expect

1. UI **Queue** — job moves from pending → running → **completed** (verified split).
2. Host **`deploy/docker/out/`** — subdirectory with per-track FLACs (e.g. two tracks for the default fixture).
3. **History / logs** — no split failure; if the job errors, check logs for missing tools or bad CUE/image pairing.

This matches the verified split cases exercised in CI ([compatibility matrix](compatibility-matrix.md)).

## 5. Next: Lidarr Connect

When Cuearr splits reliably, wire Lidarr so downloads trigger splits automatically:

**[Lidarr / Plex demo path](lidarr-plex-demo.md)** — folder layout, webhook URL, API key, and optional custom script.

For install variants (Unraid, Helm, binary), see the [README](../README.md#homelab-install-paths).

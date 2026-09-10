# Lidarr / Plex demo path (controlled scenario)

One homelab-shaped layout and two ways to trigger Cuearr. This document separates what **verified** in repo tests/CI from what is **assumed** until you run it on your stack.

## Folder layout (Docker Compose reference)

Use [`deploy/docker`](../deploy/docker) with bind mounts:

| Host (under `deploy/docker`) | Container | Role |
|------------------------------|-----------|------|
| `./watch` | `/watch` | Drop image + `.cue` (watch path) **or** point Lidarr staging here |
| `./out` | `/out` | Per-track FLACs (`out_dir/<album-subdir>/…`) |
| `cuearr-data` volume | `/data` | SQLite queue + settings |

Set bootstrap auth before `docker compose up`:

```bash
export CUEARR_INITIAL_PASSWORD='choose-a-strong-password'
export CUEARR_API_KEY='your-api-key'
```

For a **Lidarr-triggered** demo, map the same host paths Lidarr uses for a release folder (image + `.cue` in one directory). Cuearr must see that directory on disk when the hook fires.

## Path A — Lidarr Connect webhook (recommended in UI)

1. Cuearr running and reachable from Lidarr (e.g. `http://cuearr:8787` on the Docker network).
2. Lidarr → **Settings → Connect** → **Webhook**
   - **URL:** `http://<cuearr-host>:8787/api/v1/hooks/lidarr`
   - **Header:** `X-Api-Key: <CUEARR_API_KEY>`
   - **Triggers:** On Download and/or On Import (your choice)
3. When Lidarr fires the webhook, Cuearr parses the JSON body, takes the first matching path key, reduces file paths to the **album directory**, and enqueues detect/split on that directory.

Path keys (first match): `path`, `DownloadPath`, `DownloadFolder`, `lidarr_trackfile_path`, `lidarr_release_path`, `Lidarr_AddedTrackPaths` (first entry), `trackFiles[0].path`, then the same under `environment` / `env`. Implementation: [`internal/adapters/http/lidarr.go`](../internal/adapters/http/lidarr.go).

**Verified:** HTTP handler accepts a Connect-shaped payload (`environment.DownloadPath`), returns `201` + `job_id` / `created` — see `TestHandleLidarrHook` in [`lidarr_test.go`](../internal/adapters/http/lidarr_test.go). Path extraction cases are covered in `TestExtractLidarrScanPath`.

**Assumed (not run in B1):** A live Lidarr instance sends the expected fields for your download client, the release still contains image + `.cue` at hook time, and Lidarr successfully imports files from `/out` (or your mapped output path).

## Path B — Custom script (same API)

[`scripts/lidarr-custom-script.sh`](../scripts/lidarr-custom-script.sh) runs inside Lidarr’s custom-script hook:

```bash
export CUEARR_URL='http://cuearr:8787'
export CUEARR_API_KEY='your-api-key'
# Lidarr sets Lidarr_AddedTrackPaths, lidarr_trackfile_path, or lidarr_release_path
/path/to/lidarr-custom-script.sh
```

The script picks a path from Lidarr env vars (or `$1`), builds `{"path":"<path>"}`, and `POST`s to `/api/v1/hooks/lidarr` with `X-Api-Key`.

**Verified:** Script behavior matches the webhook handler’s top-level `path` key (same `extractLidarrScanPath` logic).

**Assumed:** Lidarr executes the script with the env vars you expect on On Import / Download.

## Expected Cuearr output

After a successful job (watch or hook):

- Non–in-place jobs write under `out_dir/<album-subdir>/` with one `.flac` per CUE track (default **shntool** names).
- Jobs reach `completed` only after split verification (track count, decode, duration tolerance, CUE-derived tags) — see [`internal/app/verify.go`](../internal/app/verify.go) and E2E in [`e2e_split_test.go`](../internal/app/e2e_split_test.go).

**Manual check without Lidarr:** follow [Docker smoke](docker-smoke.md) (watch drop → two FLACs under `out/`).

## Plex Music

**Assumed:** If Lidarr imports per-track files into a root Plex scans, Plex Music shows one library entry per track/album as usual. **Plex was not exercised in B1** (no Plex server run in CI or this worktree). Validate on your library after Lidarr import.

## Quick webhook smoke (no Lidarr binary)

With Cuearr up and a synthetic album at `/watch/demo_album` (generate via [`scripts/generate_fixture.sh`](../scripts/generate_fixture.sh)):

```bash
curl -fsS -X POST "http://127.0.0.1:8787/api/v1/hooks/lidarr" \
  -H "X-Api-Key: $CUEARR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"path":"/watch/demo_album"}'
```

Poll `GET /api/v1/jobs` until `completed`, then inspect `out/`. This exercises the same code path as the custom script.

Tracked: [#17](https://github.com/marcatos/cuearr/issues/17). Verified cases only: [compatibility matrix](compatibility-matrix.md).

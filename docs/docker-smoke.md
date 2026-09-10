# Cuearr release smoke (Docker)

Use after a tagged release image exists on GHCR, or build locally from this repo.

## Prerequisites

- Docker Engine running
- Port `8787` free on the host
- For manual fixture generation on the host: `ffmpeg` (see [`scripts/generate_fixture.sh`](../scripts/generate_fixture.sh) or [`scripts/generate_fixture.ps1`](../scripts/generate_fixture.ps1))

## Automated smoke

GitHub Actions: **Docker smoke** workflow (`.github/workflows/docker-smoke.yml`), `workflow_dispatch` only.

## Manual checklist

Run from the repo root unless noted.

1. **Prepare compose env** — in `deploy/docker`, set bootstrap auth (API key required for job polling in CI; recommended locally):

   ```bash
   cd deploy/docker
   export CUEARR_INITIAL_PASSWORD='smoke-test-password'
   export CUEARR_API_KEY='smoke-test-api-key'
   mkdir -p watch out
   ```

2. **Start stack** — pull a release image or build from source:

   ```bash
   docker compose pull    # optional when using ghcr.io/marcatos/cuearr:latest
   docker compose up -d --build
   ```

   Expected: container `cuearr` running, port `8787` published.

3. **Health** — unauthenticated liveness:

   ```bash
   curl -fsS "http://127.0.0.1:8787/api/v1/health"
   ```

   Expected: HTTP **200**, JSON includes `"ok": true`, `"sqlite_ok": true`, and `"shntool_ok": true` inside the image.

4. **Drop fixture** — synthetic album (no copyrighted audio):

   ```bash
   bash ../../scripts/generate_fixture.sh ./watch/smoke_album
   ```

   Expected: `watch/smoke_album/album.cue` and `watch/smoke_album/album.flac` exist.

5. **Wait for job** — poll with API key header `X-Api-Key` (matches `CUEARR_API_KEY`):

   ```bash
   curl -fsS -H "X-Api-Key: $CUEARR_API_KEY" "http://127.0.0.1:8787/api/v1/jobs"
   ```

   Expected: HTTP **200**, JSON `"jobs"` array; at least one entry with `"status": "completed"`. On failure, entries may show `"status": "failed"` and `"error"`.

6. **Outputs** — split tracks under the bind-mounted out dir:

   ```bash
   ls -la out/
   ```

   Expected: album subdirectory with two `.flac` files (fixture defines two tracks).

7. **Teardown**:

   ```bash
   docker compose down
   ```

Tracked as [#14](https://github.com/marcatos/cuearr/issues/14).

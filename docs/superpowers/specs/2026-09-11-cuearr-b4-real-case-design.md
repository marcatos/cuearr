# Cuearr B4 — Real-case integration design

**Date:** 2026-09-11  
**Batch:** B4 (Months 2–3)  
**Issues:** [#26](https://github.com/marcatos/cuearr/issues/26), [#27](https://github.com/marcatos/cuearr/issues/27)  
**Approach:** Event + Import command (webhook remains split trigger; Cuearr requests Lidarr import after verified `completed`)  
**Target release:** `v0.5.0` (MINOR) after merge

## Goal

Ship a dedicated Lidarr companion path where **split success is not confused with library import**, Cuearr can **idempotently ask Lidarr to import** published tracks, and **WAV+CUE** is a CI-verified format alongside FLAC. Close the demand-gated wording on #26/#27 by implementing the full speculative scope approved for this batch.

## Non-goals

- Native Go splitter ([#10](https://github.com/marcatos/cuearr/issues/10))
- Multi-FILE or multi-CUE support (remain fail-closed)
- Mandatory telemetry / cloud
- Full music library UI
- Claiming live Lidarr/Plex E2E in CI (fake HTTP Lidarr + docs for live path)

## Architecture

Hexagonal layers stay intact:

| Layer | Change |
|-------|--------|
| **Domain** | Job gains import lifecycle fields; settings gain Lidarr connector config; import status transitions are pure rules |
| **Ports** | `LidarrClient` for ping + request import (+ optional queue peek for safety net) |
| **App** | After verified publish/`completed`, optionally request import; safety-net poller does **not** enqueue splits on watched folders |
| **Adapters** | HTTP Lidarr API client; settings/API/UI; SQLite column/JSON persistence for new job fields |
| **Docs** | Demo path, matrix, roadmap, README |

```text
Lidarr Connect webhook ──► Cuearr enqueue/split/verify/publish ──► Status=completed
                                                                      │
                                      (if LidarrImportEnabled)        ▼
                                                         LidarrClient.RequestImport(outDir)
                                                                      │
                                                         ImportStatus=requested → imported|failed
```

Safety net (optional, high interval): for jobs with `Status=completed` and `ImportStatus` in `{none, failed}`, retry `RequestImport` according to domain rules. Poll must **not** invent new split jobs from Lidarr queue items that would double-process a watched download folder.

## Domain model

### Job fields (additive)

| Field | Values / type | Notes |
|-------|---------------|-------|
| `ImportStatus` | `none`, `requested`, `imported`, `failed`, `skipped` | Default `none` |
| `ImportError` | string | Last import failure message (no secrets) |
| `ImportRequestedAt` | time | Set when request sent |
| `ImportFinishedAt` | time | Set on terminal import outcome |

Existing `Status` (`queued`/`running`/`completed`/`failed`) remains **split-only**. A job may be `completed` with `ImportStatus=none` when connector disabled.

### Import transition rules

- `none` → `skipped` when connector disabled or in-place output makes import path undefined (document policy: in-place → `skipped` with reason)
- `none`/`failed` → `requested` when `RequestImport` HTTP call accepted
- `requested` → `imported` when Lidarr acknowledges success **or** operator marks imported (v1: success = Lidarr command accepted without error; optional follow-up check deferred if API is ambiguous)
- `requested` → `failed` on transport/API error
- Idempotency: do not re-request while `requested` within a cooldown, or when already `imported`

### Settings (additive)

| Field | Meaning |
|-------|---------|
| `LidarrURL` | Base URL (e.g. `http://lidarr:8686`) |
| `LidarrAPIKey` | API key (never logged; redacted in diagnostics) |
| `LidarrImportEnabled` | Opt-in bool (default `false`) |
| `LidarrPathMap` | Optional list of `{from,to}` path rewrites for container path differences |
| `LidarrPollInterval` | Duration; `0` disables safety-net poller (default `0`) |

## Ports

```go
type LidarrClient interface {
    Ping(ctx context.Context) error
    RequestImport(ctx context.Context, albumPath string) error
}
```

Adapter uses Lidarr HTTP API (`X-Api-Key`). Exact command payload (`DownloadedAlbumsScan` / import equivalent) is chosen against current Lidarr API docs at implementation time and covered by adapter tests with a local HTTP test server.

Path mapping applies `LidarrPathMap` before the request.

## Application flow

1. Existing path: detect → preflight → split → verify → tag → staging publish → `Status=completed`.
2. New hook after successful publish: if `LidarrImportEnabled` and client configured → map path → `RequestImport` → update import fields.
3. Manual retry: extend or add `POST /api/v1/jobs/{id}/import` to re-request import for `completed` jobs (resets failed → requested).
4. Safety-net ticker (if interval > 0): list eligible completed jobs → `RequestImport` with same idempotency rules.
5. Dual-processor guard: diagnostics/docs warn when watch dirs overlap configured Lidarr download roots; no automatic destructive disable.

## Formats (#27)

| Format | B4 commitment |
|--------|----------------|
| FLAC+CUE | Already `cuearr-ci` |
| WAV+CUE | New synthetic fixture + e2e/verify path via `shntool`; matrix row |
| APE+CUE | Attempt in CI; if tooling unavailable, document as unsupported/`not-run` — do **not** claim support |
| Multi-FILE / multi-CUE | Still reject |

WAV fixture generation extends `scripts/generate_fixture.sh` / `.ps1` (sox/ffmpeg → WAV) without claiming ffmpeg-native FLAC.

## API / UI

- Job JSON includes import fields.
- Settings GET/PUT include Lidarr fields (API key write-only or masked on read, same pattern as existing secrets).
- UI: album/job list shows split status **and** import status; settings form for Lidarr connector; action “Request import” when applicable.
- Diagnostics: redacted Lidarr URL host only; never API key.

## Testing

- Domain: import transition + idempotency unit tests.
- Adapter: HTTP test server for Lidarr success/4xx/5xx.
- App: after `RunJob` success with enabled client → `ImportStatus=requested`/`imported` (fake port); disabled → `none` or `skipped`.
- WAV: fixture + split/verify test gated like FLAC (`CUEARR_REQUIRE_SHNTOOL`).
- No live Lidarr required in CI.

## Documentation

- Update `docs/lidarr-plex-demo.md` with Path C: webhook split → Cuearr import request.
- Update `docs/compatibility-matrix.md` (WAV row; import-request row as `cuearr-ci` with fake).
- Update `docs/roadmap-batches.md` + README Status when shipped.
- Explicit honesty: live Lidarr import remains operator-validated.

## Acceptance (batch exit)

1. Split `Status` and `ImportStatus` are distinct in API/UI and docs.
2. Opt-in Lidarr client can request import after `completed`; failures do not rewrite split `completed` to `failed`.
3. Idempotent re-request rules covered by tests.
4. WAV+CUE verified in CI when shntool required; matrix updated.
5. Dual-processor guidance documented.
6. #26 and #27 closed with evidence; milestone B4 closable; release `v0.5.0`.

## Risks

- Lidarr command semantics differ by version — pin documented endpoint and fail clearly.
- Speculative formats without beta demand — WAV only is the hard commitment; APE is best-effort.
- Path mapping mistakes → failed imports — surface `ImportError`, keep tracks on disk.

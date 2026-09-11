# Cuearr B4 — Real-case Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate split vs Lidarr import state, add opt-in Lidarr Import API client after verified `completed`, support WAV+CUE in CI, ship docs/UI — closes #26/#27 → release `v0.5.0`.

**Architecture:** Hexagonal. Domain owns import transitions; `ports.LidarrClient` + HTTP adapter; app hooks post-publish import + optional safety-net poller (no split enqueue from poll); SQLite columns for import fields; settings JSON gains Lidarr config; UI shows import status.

**Tech Stack:** Go 1.22+, SQLite, existing shntool/metaflac, Lidarr HTTP API `POST /api/v1/command` name `DownloadedAlbumsScan`.

**Spec:** [`docs/superpowers/specs/2026-09-11-cuearr-b4-real-case-design.md`](../specs/2026-09-11-cuearr-b4-real-case-design.md)

**Issues:** #26, #27.

## Global Constraints

- Branch: `feat/b4-real-case` (not `main`). `gh` as `marcatos` only.
- Conventional Commits; TDD; hexagonal (domain has no HTTP/SQLite imports).
- Split `Status` must never become `failed` because import failed.
- `LidarrImportEnabled` default `false`; never log API keys.
- WAV hard-commit; APE best-effort only — do not claim APE if CI cannot prove it.
- Multi-FILE/multi-CUE stay fail-closed.
- After merge: `.cursor/rules/releases.mdc` → annotated tag `v0.5.0`.
- Do not commit `.superpowers/sdd/*` (gitignored scratch).

## File map

| Path | Responsibility |
|------|----------------|
| `internal/domain/job.go` | Import status constants + Job fields |
| `internal/domain/import.go` | Pure import transition helpers |
| `internal/domain/settings.go` | Lidarr settings + PathMap |
| `internal/ports/lidarr.go` | `LidarrClient` |
| `internal/adapters/lidarr/http.go` | Lidarr HTTP client |
| `internal/adapters/store/sqlite/*` | Persist import columns |
| `internal/app/import.go` | Request import after complete + manual/safety-net |
| `internal/app/run.go` | Call import hook after successful publish |
| `internal/adapters/audio/wavduration/*` | Source WAV duration for verify |
| `internal/adapters/http/*` | API JSON + `POST .../import` |
| `web/app.js` (+ HTML if needed) | Import status + settings + button |
| `scripts/generate_fixture.*` | WAV fixture mode |
| `docs/*`, `README.md` | Path C, matrix, roadmap, Status |

---

### Task 1: Domain import model + settings

**Files:**
- Modify: `internal/domain/job.go`
- Create: `internal/domain/import.go`, `internal/domain/import_test.go`
- Modify: `internal/domain/settings.go`, `internal/domain/settings_test.go`

**Interfaces:**
- Produces constants: `ImportNone`, `ImportRequested`, `ImportImported`, `ImportFailed`, `ImportSkipped`
- Produces: `func BeginImportRequest(job Job, now time.Time) (Job, error)` — allows from `none`/`failed`; rejects `imported`/`requested` within cooldown (`ImportRequestCooldown = 2*time.Minute`); sets `ImportStatus=requested`, clears `ImportError`, sets `ImportRequestedAt`
- Produces: `func CompleteImportOK(job Job, now time.Time) Job` → `imported`, sets `ImportFinishedAt`
- Produces: `func CompleteImportFail(job Job, now time.Time, errMsg string) Job` → `failed`, sets error + finished
- Produces: `func SkipImport(job Job, reason string) Job` → `skipped`
- Produces settings fields: `LidarrURL string`, `LidarrAPIKey string`, `LidarrImportEnabled bool`, `LidarrPathMap []PathMapRule`, `LidarrPollIntervalSec int` (0 = off)
- Produces: `type PathMapRule struct { From, To string }`
- Produces: `func MapLidarrPath(path string, rules []PathMapRule) string` — apply first matching `strings.HasPrefix` rewrite

- [ ] **Step 1: Write failing tests** in `import_test.go` and extend `settings_test.go` for MapLidarrPath + MaxAttempts unchanged.
- [ ] **Step 2: Run** `go test ./internal/domain -count=1` — expect FAIL.
- [ ] **Step 3: Implement** job fields + import helpers + settings.
- [ ] **Step 4: Run** `go test ./internal/domain -count=1` — PASS.
- [ ] **Step 5: Commit** `feat(domain): add Lidarr import status model and settings`

---

### Task 2: SQLite persistence for import fields

**Files:**
- Modify: `internal/adapters/store/sqlite/migrate.go`, `sqlite.go`, `sqlite_test.go`

**Interfaces:**
- Consumes: Job import fields from Task 1
- Produces: columns `import_status TEXT NOT NULL DEFAULT 'none'`, `import_error TEXT NOT NULL DEFAULT ''`, `import_requested_at TEXT`, `import_finished_at TEXT` via `ALTER TABLE` (duplicate-column safe like attempt_*)
- Update `jobSelect`, `Create`, `Update`, `ClaimNextQueued` RETURNING, `scanJobRow` to round-trip fields
- Empty `ImportStatus` on read → treat as `none`

- [ ] **Step 1: Failing test** Create/Update/Get job with `ImportStatus=requested` and timestamps.
- [ ] **Step 2: Run** `go test ./internal/adapters/store/sqlite -count=1` — FAIL.
- [ ] **Step 3: Implement** migrations + scan/write.
- [ ] **Step 4: Run** tests — PASS.
- [ ] **Step 5: Commit** `feat(store): persist job import status columns`

---

### Task 3: LidarrClient port + HTTP adapter

**Files:**
- Create: `internal/ports/lidarr.go`
- Create: `internal/adapters/lidarr/client.go`, `client_test.go`
- Create: `internal/adapters/lidarr/pathmap.go` (thin wrap of domain.MapLidarrPath if needed)

**Interfaces:**
```go
// ports/lidarr.go
type LidarrClient interface {
    Ping(ctx context.Context) error
    RequestImport(ctx context.Context, albumPath string) error
}
```
- Adapter `lidarr.Client` fields: `BaseURL`, `APIKey`, `HTTPClient *http.Client`, `PathMap []domain.PathMapRule`
- `Ping`: `GET {base}/api/v1/system/status` with header `X-Api-Key`
- `RequestImport`: map path, then `POST {base}/api/v1/command` JSON `{"name":"DownloadedAlbumsScan","path":"<mapped>"}` — treat 2xx as success; non-2xx return error including body snippet (no API key)
- Trim trailing slash on BaseURL

- [ ] **Step 1: Failing httptest tests** for Ping OK, RequestImport OK, 401 fail, path map applied to JSON body.
- [ ] **Step 2: Implement** + PASS `go test ./internal/adapters/lidarr -count=1`
- [ ] **Step 3: Commit** `feat(lidarr): add HTTP DownloadedAlbumsScan client`

---

### Task 4: App import after complete + manual + safety net

**Files:**
- Create: `internal/app/import.go`, `import_test.go`
- Modify: `internal/app/run.go`, `run_test.go` (and/or `worker_test.go`)
- Modify: `internal/ports/store.go` if a list-by-status helper is required; prefer filter in app over new store API unless needed

**Interfaces:**
```go
type ImportService struct {
    Store  ports.JobStore
    Client ports.LidarrClient // may be nil
    Clock  func() time.Time
    Log    *slog.Logger
}

// AfterSplitComplete updates import fields for a completed job using settings.
func (s ImportService) AfterSplitComplete(ctx context.Context, job domain.Job, settings domain.Settings) (domain.Job, error)

// RequestImportForJob manual/API path for completed jobs.
func (s ImportService) RequestImportForJob(ctx context.Context, jobID string, settings domain.Settings) (domain.Job, error)

// PollPendingImports safety net when LidarrPollIntervalSec > 0.
func (s ImportService) PollPendingImports(ctx context.Context, settings domain.Settings) (n int, err error)
```
- Rules: if `!LidarrImportEnabled` or Client nil or URL/key empty → `SkipImport` only when transitioning from `none` after complete; leave already-set statuses alone
- If `settings.InPlace` → `SkipImport` with reason `in-place output`
- Else BeginImportRequest → Client.RequestImport(job.OutDir) → CompleteImportOK or CompleteImportFail; **always** `Store.Update`; never change `job.Status`
- `JobRunner` gains optional `Importer *ImportService` + `Settings func() domain.Settings`; after successful publish call `AfterSplitComplete`
- Poller: `List` jobs, pick `Status==completed` && `ImportStatus in {none,failed}`, call RequestImportForJob; does **not** call CreateJob/enqueue

- [ ] **Step 1: Failing tests** with fake LidarrClient — enabled success, enabled fail keeps Status=completed, disabled → skipped, cooldown idempotency.
- [ ] **Step 2: Implement** + wire RunJob hook.
- [ ] **Step 3: PASS** `go test ./internal/app -count=1`
- [ ] **Step 4: Commit** `feat(app): request Lidarr import after verified split`

---

### Task 5: HTTP API + wiring + diagnostics redact

**Files:**
- Modify: `internal/adapters/http/handlers.go`, settings handlers, `deps.go`, router
- Create/modify: import handler tests
- Modify: `cmd/cuearr/main.go` (construct lidarr.Client + ImportService + poller goroutine if interval > 0)
- Modify: `internal/adapters/http/diagnostics.go` (+ test) — expose `lidarr_import_enabled`, redacted host only

**Interfaces:**
- `jobJSON` adds `import_status`, `import_error`, `import_requested_at`, `import_finished_at`
- `settingsJSON` / put: lidarr fields; API key masked on GET (`api_key_set` bool, empty key string) like auth API key pattern
- `POST /api/v1/jobs/{id}/import` → `RequestImportForJob`
- Reject engine native still unchanged

- [ ] **Step 1: Failing handler tests** for JSON fields + import endpoint.
- [ ] **Step 2: Implement** + wire main.
- [ ] **Step 3: PASS** `go test ./internal/adapters/http ./cmd/cuearr -count=1` (cmd may have no tests)
- [ ] **Step 4: Commit** `feat(api): expose Lidarr import settings and job import action`

---

### Task 6: UI import status + settings

**Files:**
- Modify: `web/app.js`, `web/index.html` (or existing templates)

**Requirements:**
- Job/album rows show split status and import status
- Settings: Lidarr URL, API key (password input), enable checkbox, poll interval seconds, path map as simple textarea `from=>to` per line (parse on save)
- Button “Request import” for completed jobs when enabled
- No secrets in console.log

- [ ] **Step 1: Implement UI** against existing API shapes from Task 5.
- [ ] **Step 2: Smoke-read** that JS references new JSON keys; no broken syntax.
- [ ] **Step 3: Commit** `feat(ui): show import status and Lidarr connector settings`

---

### Task 7: WAV fixture + duration inspect + docs

**Files:**
- Create: `internal/adapters/audio/wavduration/wav.go`, `wav_test.go` (PCM WAV header duration)
- Modify: `internal/app/run.go` — if image ext `.wav`, use wav duration instead of metaflac Inspect for **source** total duration (outputs still FLAC via shntool `-o flac`)
- Modify: `scripts/generate_fixture.sh`, `scripts/generate_fixture.ps1` — flag or second mode writing `album.wav` + CUE `FILE "album.wav" WAVE`
- Create/update: `testdata` or generate-in-test like FLAC e2e
- Modify: `internal/app/e2e_split_test.go` (or new) WAV e2e gated by `CUEARR_REQUIRE_SHNTOOL`
- Modify: `docs/compatibility-matrix.md`, `docs/lidarr-plex-demo.md` (Path C), `docs/roadmap-batches.md`, `README.md`
- APE: one short note in matrix as not-run/unsupported unless proven

- [ ] **Step 1: WAV duration unit test** + failing e2e skeleton.
- [ ] **Step 2: Implement** fixture + run.go branch + e2e.
- [ ] **Step 3: PASS** `go test ./...` (WAV e2e skip unless tools).
- [ ] **Step 4: Docs** Path C, matrix rows, mark B4 done in roadmap/README Status (point to upcoming v0.5.0).
- [ ] **Step 5: Commit** `feat(wav): verify WAV+CUE splits and document Lidarr import path`

---

## Spec coverage checklist

| Spec item | Task |
|-----------|------|
| ImportStatus fields | 1–2 |
| Transition/idempotency | 1, 4 |
| LidarrClient + DownloadedAlbumsScan | 3 |
| After completed import + manual + poll | 4–5 |
| Dual-processor docs | 7 |
| API/UI | 5–6 |
| WAV+CUE CI | 7 |
| Secrets redaction | 5 |
| #26/#27 close + v0.5.0 | finishing skill after PR |

## Execution

User requested SDD for this batch — execute with **superpowers:subagent-driven-development** on worktree `feat/b4-real-case`.

# Cuearr B2 — File Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split never exposes partial tracks as final output; retries are bounded with history; fingerprints track audio identity; ambiguous/incomplete inputs fail closed before split.

**Architecture:** Keep hexagonal layout. Staging + publish live in `internal/app` using `os`/`filepath` (or a thin `ports.FilePublisher` if tests need fakes). Job metadata gains attempt fields via SQLite migration. Detection/preflight enrich `DetectAlbum` / `RunJob` before calling the splitter. B1 verify+tag still runs **inside staging** before publish.

**Tech Stack:** Go 1.23, SQLite migrations, existing JobRunner / watcher / Enqueue.

**Issues:** #19 staging/publish · #20 retry+history · #21 audio fingerprint · #22 ambiguous/incomplete/preflight.

## Global Constraints

- Branch: `feat/b2-file-reliability` (not `main`). Account: `marcatos` for any `gh`.
- Conventional Commits; commit after each task.
- Hexagonal: domain pure; adapters for store/fs; app orchestrates.
- Logging: slog with levels, step durations, no secrets.
- **Originals kept by default** — never delete `ImagePath` / CUE.
- B1 gate unchanged: verify (count, flac -t, ±100ms duration, tags) must pass **before** publish.
- Staging and final `out` must be on the **same filesystem** so `os.Rename` is atomic; if rename fails with cross-device, fail the job (no silent copy-half).
- Staging root: `{outDir}/.cuearr-staging/{jobID}/` (non–in-place) or `{imageDir}/.cuearr-staging/{jobID}/` (in-place). Never publish into staging.
- On job start (running): delete leftover staging for that `jobID` before split (safe restart).
- On verify/split failure: remove staging dir; final `OutDir` must not gain new track files from this attempt.
- Max retries default **3**; setting `max_retries` in Settings (0 = no auto-retry beyond first failure — document). Attempt history append-only in job (see Task 2).
- Fingerprint must change when image **content** changes at same paths; document policy in `docs/` or README.
- Ambiguous multi-CUE or multi-FILE → fail closed with clear domain error (no split).
- Do not implement Lidarr connector (B4) or native engine (B5).
- Artifact retention in any new CI: `retention-days: 7`.

## File map

| Path | Responsibility |
|------|----------------|
| `internal/app/publish.go` | Staging path helpers, publish rename, cleanup |
| `internal/app/run.go` | Split into staging → verify → publish → completed |
| `internal/domain/job.go` | `AttemptCount`, `MaxAttempts`, `AttemptLog` (or JSON attempts) |
| `internal/adapters/store/sqlite/migrate.go` | Columns for attempts |
| `internal/domain/album.go` / `fingerprint` | Include audio identity |
| `internal/app/detect.go` | Ambiguous CUE/FILE rejection; optional stability hook |
| `internal/app/preflight.go` | Permissions + free-space check |
| `internal/adapters/fs/watcher.go` | Stronger stability / debounce if needed |
| `internal/domain/settings.go` + UI/API | `max_retries` |
| `docs/roadmap-batches.md` / README | Active priorities: B2 + B6 |

---

### Task 1: Staging dir + atomic publish helpers (#19 core)

**Files:**
- Create: `internal/app/publish.go`, `internal/app/publish_test.go`
- Modify: none of RunJob yet (helpers only)

**Interfaces:**
- Produces:
  - `func StagingDir(baseOut, jobID string) string`
  - `func PrepareStaging(baseOut, jobID string) (staging string, err error)` — mkdir, wipe if exists
  - `func CleanupStaging(staging string) error`
  - `func PublishTracks(staging string, finalOut string, files []string) (published []string, err error)` — ensure finalOut exists; `Rename` each file; on any failure best-effort cleanup of partial publishes **in finalOut for this attempt only** (files just moved) and return error
  - Same-FS check: if rename returns `EXDEV`/`LinkError`, wrap as permanent failure

- [ ] **Step 1: Failing tests** — prepare staging, write fake tracks, publish to final, assert final has files and staging empty/removed; simulate mid-publish failure leaves documented behavior.
- [ ] **Step 2: Implement helpers.**
- [ ] **Step 3: Pass + commit** `feat(app): add staging and atomic publish helpers`

---

### Task 2: Wire RunJob through staging → verify → publish (#19)

**Files:**
- Modify: `internal/app/run.go`, `internal/app/run_test.go`, `internal/app/e2e_split_test.go` if paths change
- Modify: `internal/app/verify.go` only if needed

**Behavior:**
1. Compute `finalOut` as today (`albumOutputKey` / in-place dir).
2. `PrepareStaging` under the appropriate base.
3. `Split` into **staging** (not finalOut).
4. Inspect image + `VerifySplit` on staging outputs.
5. `PublishTracks` to `finalOut`.
6. `CleanupStaging`; set `job.OutDir = finalOut`; `completed`.
7. On any failure after split: `CleanupStaging`; do not leave new files in `finalOut`; `failed`.

- [ ] **Step 1: Update unit tests** — fake splitter writes to the `outDir` argument it receives; assert that argument is staging; final dir gets files only after success path (use real temp dirs + fake splitter that creates files).
- [ ] **Step 2: Implement RunJob wiring.**
- [ ] **Step 3: `go test ./internal/app/ -count=1`.**
- [ ] **Step 4: Commit** `feat(app): split to staging and publish only after verify`

---

### Task 3: Attempt count, max retries, attempt history (#20)

**Files:**
- Modify: `internal/domain/job.go`, `internal/domain/settings.go`
- Modify: `internal/adapters/store/sqlite/migrate.go`, `sqlite.go`, tests
- Modify: `internal/app/worker.go` / enqueue or fail path — requeue vs permanent fail
- Modify: HTTP job JSON + minimal UI if trivial (`web/app.js` show attempts)
- Test: store + worker retry behavior

**Behavior:**
- Settings: `MaxRetries int` default 3 (meaning up to 3 **retries** after first failure → 4 attempts total, **or** document as max attempts = MaxRetries+1 — pick **max attempts = MaxRetries** with default 3 total attempts; document clearly in settings help).
- On failure: append attempt record `{n, at, error}` to `AttemptLog` (TEXT JSON array) or structured field; increment `AttemptCount`.
- If `AttemptCount < MaxRetries` (per chosen semantics): set status back to `queued`, clear running timestamps appropriately, keep history.
- Else: leave `failed`.
- API: expose `attempt_count` and `attempts` on job JSON.

- [ ] **Step 1: Migration + domain fields + tests.**
- [ ] **Step 2: Worker/RunJob fail path requeue logic.**
- [ ] **Step 3: API (+ light UI).**
- [ ] **Step 4: Commit** `feat(jobs): bounded retries with attempt history`

---

### Task 4: Fingerprint includes audio content identity (#21)

**Files:**
- Modify: `internal/domain/album.go` (+ tests)
- Modify: `internal/app/detect.go` — read image identity bytes/stat
- Docs: short note in README or `docs/fingerprint.md`

**Policy (lock this):**
`Fingerprint = sha256(cuePath | imagePath | sha256(cueBytes) | imageSize | imageMtimeUnixNano | sha256(imageContent))`  
where `imageContent` hash is **full-file sha256** for correctness (albums are finite; log duration). If full-file is too slow in tests, allow injecting a `HashFile` func.

When image bytes change at same path → new fingerprint → Enqueue creates new job (existing completed fingerprint no longer matches).

- [ ] **Step 1: Failing tests** — same paths/CUE, different image bytes → different fingerprint; identical → same.
- [ ] **Step 2: Implement + wire DetectAlbum.**
- [ ] **Step 3: Doc policy one paragraph.**
- [ ] **Step 4: Commit** `feat(domain): include audio content in job fingerprint`

---

### Task 5: Fail closed on ambiguous CUE / incomplete copy + preflight (#22)

**Files:**
- Modify: `internal/domain` errors + `BuildSplitPlan` / `DetectAlbum`
- Create: `internal/app/preflight.go` (+ tests)
- Modify: `internal/adapters/fs/watcher.go` — optional size-stable check after debounce (compare size twice with short gap) **or** preflight “mtime older than N ms”
- Modify: `RunJob` to call preflight before split

**Rules:**
1. **Multi-CUE** in directory → `ErrAmbiguousCue` (new).
2. **Multi-FILE** in CUE text (second `FILE` directive) → `ErrAmbiguousCue` / `ErrMultiFileCue`.
3. **Stability:** before enqueue or before split, require image `Size` unchanged across two stats separated by ≥500ms **or** mtime older than debounce (2s); fail with clear error if still growing.
4. **Preflight:** final out (or staging parent) writable; free space ≥ `imageSize * 2` (or imageSize + 64MiB floor) via `syscall`/`golang.org/x/sys` — Windows + Linux; if free-space API unavailable, log warn and continue only on Unix CI… prefer implement for Linux (CI) + Windows.

- [ ] **Step 1: Domain/detect rejection tests.**
- [ ] **Step 2: Preflight + RunJob hook.**
- [ ] **Step 3: Watcher or detect stability.**
- [ ] **Step 4: Commit** `feat(app): reject ambiguous CUE and preflight disk safety`

---

### Task 6: Docs + roadmap active priorities

**Files:**
- Modify: `docs/roadmap-batches.md` — B1 done; active = B2 + B6
- Modify: README Status line
- Optional: short `docs/file-reliability.md` describing staging/retry/fingerprint

- [ ] **Step 1: Update docs.**
- [ ] **Step 2: Commit** `docs: mark B1 complete and activate B2 priorities`

---

## Progress ledger

Track in `.superpowers/sdd/progress.md` (gitignored):

```
Task N: complete (commits <base7>..<head7>, review clean)
```

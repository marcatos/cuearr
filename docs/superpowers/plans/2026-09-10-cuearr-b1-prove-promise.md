# Cuearr B1 — Prove the Promise Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `completed` mean verified split output (count, decode, duration, CUE tags), force that proof in CI, and document Docker smoke + one Lidarr/Plex path + a verified-only compatibility matrix.

**Architecture:** Keep hexagonal layout. Post-split verification and tagging live in application layer (`internal/app`) using small ports for FLAC inspect/tag commands. `shntool` adapter populates `OutputFiles`. `RunJob` only marks `completed` after verify+tag succeed; otherwise `failed`. CI installs `shntool`/`cuetools`/`flac`/`ffmpeg` and runs the e2e fixture without skips.

**Tech Stack:** Go 1.23, shntool, flac/metaflac, GitHub Actions ubuntu-latest, existing SQLite job store.

**Issues:** #15 (verify+tags) · #16 (CI e2e) · #14 (Docker smoke) · #17 (Lidarr/Plex demo) · #18 (compat matrix). Parallel docs #30 is out of this plan.

## Global Constraints

- Tracker: GitHub Issues on `marcatos/cuearr` only; work on branch `feat/b1-prove-promise` (not `main` directly).
- Conventional Commits; commit after each task; never push secrets.
- Hexagonal: domain has no IO; adapters shell out; app orchestrates.
- Logging: slog levels, step durations, no secrets; job states `queued|running|completed|failed`.
- Duration tolerance: **±100ms** between measured FLAC duration and CUE INDEX-derived expected duration (last track = image end − last INDEX 01).
- Tag fields required on each output: `TITLE`, `ARTIST` (track performer else album), `ALBUM`, `TRACKNUMBER` (decimal string).
- E2E fixture: `scripts/generate_fixture.sh` → `testdata/e2e_album` (2 tracks, 6s silence FLAC).
- CI must **fail** if e2e split test skips; do not leave `t.Skip` for missing tools in the CI job path.
- Do not implement B2 staging/publish in this plan.
- Account: `gh auth switch --user marcatos` before any `gh` use.

## File map

| Path | Responsibility |
|------|----------------|
| `internal/adapters/splitter/shntool/shntool.go` | After successful split, collect sorted `*.flac` into `OutputFiles` |
| `internal/domain/cue_timing.go` | Parse `MM:SS:FF` INDEX → duration; expected per-track durations |
| `internal/ports/audio.go` | `FLACInspector` + `FLACTagger` interfaces |
| `internal/adapters/audio/metaflac/` | Implement inspect (`--show-total-samples`, `--show-sample-rate`) + tag via `metaflac` |
| `internal/app/verify.go` | Verify count/decode/duration against CUE |
| `internal/app/run.go` | After split: verify → tag → completed or failed |
| `internal/app/e2e_split_test.go` | Assert count, decode, duration, tags (no skip when `CUEARR_REQUIRE_SHNTOOL=1`) |
| `.github/workflows/ci.yml` | Install tools, generate fixture, set require flag, run tests |
| `docs/docker-smoke.md` + optional workflow | Executable smoke (#14) |
| `docs/lidarr-plex-demo.md` | One verified path (#17) |
| `docs/compatibility-matrix.md` | Verified cases only (#18); README link |

---

### Task 1: shntool populates OutputFiles

**Files:**
- Modify: `internal/adapters/splitter/shntool/shntool.go`
- Test: `internal/adapters/splitter/shntool/shntool_test.go` (create if missing)

**Interfaces:**
- Consumes: `ports.Splitter`, `ports.SplitResult`
- Produces: `SplitResult.OutputFiles` = absolute paths to `*.flac` in `outDir`, sorted by name

- [ ] **Step 1: Write failing test** — mock runner success; create temp outDir with `split-track01.flac` and `split-track02.flac` (empty files OK); assert `OutputFiles` length 2 and sorted paths.
- [ ] **Step 2: Run test — expect fail** (`go test ./internal/adapters/splitter/shntool/ -count=1`).
- [ ] **Step 3: Implement** — on exit 0, `filepath.Glob`/`ReadDir` for `*.flac` (case-insensitive), sort, set `OutputFiles`; if zero files, return `domain.ErrSplitFailed` with message.
- [ ] **Step 4: Tests pass.**
- [ ] **Step 5: Commit** `fix(splitter): populate OutputFiles after shntool split`

---

### Task 2: CUE INDEX → expected track durations

**Files:**
- Create: `internal/domain/cue_timing.go`
- Test: `internal/domain/cue_timing_test.go`
- Modify if needed: `internal/domain/cue.go` (export helpers only if required)

**Interfaces:**
- Produces:
  - `func ParseCueIndex(mmssff string) (time.Duration, error)` — CD frames at 75/s
  - `func ExpectedTrackDurations(sheet CueSheet, totalAudio time.Duration) ([]time.Duration, error)` — per track from successive INDEX 01; last track uses `totalAudio - lastIndex`

- [ ] **Step 1: Failing tests** for `00:00:00`, `00:00:03` (3s), `01:00:02`; two-track sheet with total 6s → [3s, 3s].
- [ ] **Step 2: Run — fail.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Pass.**
- [ ] **Step 5: Commit** `feat(domain): derive track durations from CUE INDEX`

---

### Task 3: metaflac inspector + tagger adapters

**Files:**
- Create: `internal/ports/audio.go`
- Create: `internal/adapters/audio/metaflac/metaflac.go`
- Test: `internal/adapters/audio/metaflac/metaflac_test.go` (unit with fake `CommandRunner`; optional skip-integration)

**Interfaces:**
- Produces:
```go
type FLACInfo struct {
    SampleRate int
    TotalSamples int64
    Duration time.Duration // TotalSamples/SampleRate
}
type FLACInspector interface {
    Inspect(ctx context.Context, path string) (FLACInfo, error) // fail if undecodable / missing
}
type TrackTags struct {
    Title, Artist, Album, TrackNumber string
}
type FLACTagger interface {
    ApplyTags(ctx context.Context, path string, tags TrackTags) error
}
```

- [ ] **Step 1: Failing unit tests** with fake runner returning sample-rate/total-samples lines.
- [ ] **Step 2: Implement** via `metaflac --show-sample-rate` / `--show-total-samples` and `--remove-tag`/`--set-tag` (or `--import-tags-from=-`).
- [ ] **Step 3: Pass + commit** `feat(audio): metaflac inspect and tag adapters`

---

### Task 4: Verify + tag gate in RunJob (#15)

**Files:**
- Create: `internal/app/verify.go`
- Modify: `internal/app/run.go`
- Modify: `internal/app/worker.go` (wire inspector/tagger if constructed there / main)
- Modify: `cmd/cuearr` or composition root that builds worker
- Test: `internal/app/verify_test.go`, update `internal/app/run_test.go`

**Interfaces:**
- Consumes: Task 2 + 3 ports; CUE bytes from `job.CuePath`
- Produces: `VerifySplit(ctx, cuePath, outputFiles, totalImageDuration) error`; on any failure RunJob sets `failed`

**Acceptance wiring:**
1. After successful `Split`, if `OutputFiles` empty → fail.
2. Parse CUE; `len(files) == len(sheet.Tracks)` else fail.
3. Inspect each file; duration within **±100ms** of expected (image duration from inspect of `job.ImagePath` or sum of tracks).
4. Apply tags from CUE; any tag error → fail.
5. Only then `completed` with log listing files.

- [ ] **Step 1: Failing tests** — fake splitter returns 2 files; fake inspector/tagger; assert completed vs failed on count mismatch / duration mismatch / tag error.
- [ ] **Step 2: Implement verify + RunJob signature change** — prefer extending `RunJob` with inspector+tagger params OR a small `JobRunner` struct to avoid giant signatures; match existing style.
- [ ] **Step 3: Update composition root** so production uses metaflac + shntool.
- [ ] **Step 4: Full `go test ./...` (non-e2e may skip tools).**
- [ ] **Step 5: Commit** `feat(app): require verified outputs and CUE tags before completed`

---

### Task 5: Mandatory CI synthetic split (#16)

**Files:**
- Modify: `internal/app/e2e_split_test.go` — assert exact track count (2), durations ~3s each (±100ms), tags present; if `os.Getenv("CUEARR_REQUIRE_SHNTOOL")=="1"` then **Fatal** instead of Skip when tools/fixture missing.
- Modify: `.github/workflows/ci.yml` — apt install `shntool cuetools flac ffmpeg`; run `bash scripts/generate_fixture.sh`; `CUEARR_REQUIRE_SHNTOOL=1 go test ./...`; keep build step.
- Ensure fixture artifacts stay gitignored if generated in CI only (`testdata/e2e_album/*.flac` already ignored if listed — check `.gitignore`).

- [ ] **Step 1: Strengthen e2e assertions** (may still Skip locally without tools).
- [ ] **Step 2: Update CI workflow.**
- [ ] **Step 3: Locally run unit tests; note e2e if tools present.**
- [ ] **Step 4: Commit** `ci: require shntool e2e split with fixture`

---

### Task 6: Docker Compose smoke executable (#14)

**Files:**
- Modify: `docs/docker-smoke.md` — numbered checklist with expected HTTP status/JSON keys; link fixture script.
- Create: `.github/workflows/docker-smoke.yml` — `workflow_dispatch` only; pull/build compose, health curl, generate fixture into watch volume, wait for completed job via API (API key/password from env), fail on timeout; `retention-days: 7` on any artifacts; skip if Docker unavailable gracefully only on self-hosted — on GHA Docker is available.

- [ ] **Step 1: Docs tighten.**
- [ ] **Step 2: Workflow** (keep focused; use compose from `deploy/docker`).
- [ ] **Step 3: Commit** `ci: add workflow_dispatch Docker smoke for releases`

---

### Task 7: Lidarr/Plex demo path + compatibility matrix (#17, #18)

**Files:**
- Create: `docs/lidarr-plex-demo.md` — one controlled scenario: folders, Lidarr Connect custom script/webhook settings, expected Cuearr out layout, what was **verified** vs **assumed** (be honest if Plex not run in this PR).
- Create: `docs/compatibility-matrix.md` — table with only verified rows (start with synthetic fixture + any demo case).
- Modify: `README.md` — link both; remove any unverified “supports X” claims if present.

- [ ] **Step 1: Write docs from current code behavior (webhook + watch).**
- [ ] **Step 2: README links.**
- [ ] **Step 3: Commit** `docs: Lidarr/Plex demo path and verified compatibility matrix`

---

## Progress ledger

Track in `.superpowers/sdd/progress.md` (gitignored):

```
Task N: complete (commits <base7>..<head7>, review clean)
```

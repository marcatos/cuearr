# Task 4 Report: verify and tag gate in RunJob

## Status

Complete. `JobRunner` now prevents completion until split outputs match the CUE,
all FLAC durations are within ±100 ms, and all required tags are applied.

## Commit

- `e8a9c31` — `feat(app): require verified outputs and CUE tags before completed`

## Implementation

- Added `JobRunner.VerifySplit` to read and parse the CUE, enforce output count,
  derive expected durations from the inspected source image duration, inspect
  every output, and apply `TITLE`, `ARTIST`, `ALBUM`, and `TRACKNUMBER`.
- Track performer overrides album performer; album performer is the fallback.
- Refactored `RunJob` onto `JobRunner`; empty outputs and all inspect, duration,
  parsing, and tagging failures persist `JobFailed`.
- Wired `FLACInspector`, `FLACTagger`, and CUE file reading through `Worker`.
- Composed the production worker with the Task 3 `metaflac.Client`.
- Added structured start, major-step, failure, completion, and duration logs.
- Did not implement B2 staging.

## TDD and tests

- RED: `go test ./internal/app -run TestVerifySplit -count=1` failed because
  `JobRunner`, `VerifySplit`, and verification errors did not exist.
- GREEN: verification unit tests passed after the minimal implementation.
- RED: RunJob gate tests failed because `JobRunner.RunJob` was not wired.
- GREEN: RunJob, worker, and composition tests passed after wiring.
- Final: `go test ./... -count=1` passed.
- Static analysis: `go vet ./...` passed.

Coverage includes exact ±100 ms acceptance, >100 ms rejection, CUE/output count
mismatch, every-output inspection, performer fallback, tag values, tag failure,
empty split output, persisted failed status, successful completion, worker
processing, and output-directory behavior.

## Concerns

- Tagging is not transactional: if tagging a later track fails, earlier tracks
  may already be tagged, but the job is correctly persisted as failed.
- Production verification requires `metaflac` to be installed; invocation
  failures are surfaced as failed jobs.
- Verification is linear in track count and performs only the required inspect
  and tag operations; no extra scans or staging copies were introduced.

## Whole-branch review fixes

Status: complete.

- `ecbc84a` — `fix(audio): decode-test FLAC outputs before verify accepts them`
  runs `flac -t --silent` from `FLACInspector.Inspect`; decode failures now
  propagate through `VerifySplit` and prevent completion.
- `bc2e7ae` — `fix(splitter): exclude source image from in-place OutputFiles`
  excludes the absolute source image path while collecting split FLAC outputs.
- `7d504a8` — `fix(app): record completion time after verification`
  records successful `FinishedAt` only after output verification and tagging.
- No B2 staging behavior was added.

Regression tests were written and observed failing before each production fix:
the inspector made only two metadata calls instead of invoking `flac -t`, the
in-place output list included `album.flac`, and `FinishedAt` preceded the final
tag operation.

Requested covering suites passed:

`go test ./internal/adapters/audio/metaflac/ ./internal/adapters/splitter/shntool/ ./internal/app/ -count=1`

The complete suite also passed:

`go test ./... -count=1`

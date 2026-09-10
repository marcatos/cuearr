# Task 2 report: Wire RunJob through staging → verify → publish

**Status:** Done  
**Branch:** `feat/b2-file-reliability`

## Delivered

- `RunJob` computes the final album destination separately from its per-job staging directory.
- Splitter output is written under `.cuearr-staging/{jobID}`, inspected, verified, and tagged before publication.
- Verified tracks are atomically published to the final output and the job records final published paths.
- Staging is removed on split, inspection, verification, publish, empty-output, and successful completion paths.
- In-place jobs stage beside the source image while preserving the original CUE/image files.

## TDD evidence

- RED: staging assertions failed because the splitter received the final output; verification failures also left tracks there.
- GREEN: filesystem-backed success, in-place, empty-output, and verification-failure tests pass with staging cleanup and publish isolation.

## Verification

```text
go test ./internal/app/ -count=1
ok  	github.com/marcatos/cuearr/internal/app

go test ./... -count=1
all packages passed

go vet ./...
exit 0
```

## Whole-branch review fixes

- Album publication now assembles a hidden sibling directory and promotes it
  as a unit; an existing album is moved aside and restored if promotion fails.
- Mid-publish failures discard only the publishing sibling, so pre-existing
  final tracks are never deleted by rollback.
- In-place runs publish to an adjacent fingerprinted album directory so source
  files remain untouched and consumers never observe a partial album.
- After successful publish, a failed `Store.Update(completed)` quarantines the
  album aside (`*.orphan-{jobID}`), marks the job failed, and returns an error.
- Attempts are recorded at `RunJob` start via `BeginAttempt`; crash-recovered
  jobs already at their configured limit are not run again.
- Image SHA-256 hashing streams through `io.Copy` rather than loading the whole
  image into memory; the injectable hash hook remains supported.

Commits: `8e9b9d4`, `9f3cea9`, `f2729d1`.


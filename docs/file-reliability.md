# File reliability (batch B2)

B2 hardens the split pipeline so Lidarr never sees half-written albums and job status stays honest across retries and restarts.

## Staging and publish

Each job writes tracks under `{out_dir}/.cuearr-staging/{job_id}/` (or beside the source image when **in-place**). After split + timing verification, `PublishTracks` moves files into the final album directory with `os.Rename` (atomic on the same filesystem). Staging is removed only after a successful publish; failures leave the watch folder without new partial tracks in the album path.

See `internal/app/publish.go` and the `RunJob` flow in `internal/app/run.go`.

## Retries and restart

Failed attempts increment `attempt_count` and re-queue until `max_attempts` (settings). A job is marked `completed` only after publish and DB update succeed — not when split alone succeeds. On worker start, `RecoverRunning` re-queues jobs left in `running` from a crash.

## Fingerprint

Enqueue idempotency uses a content-aware fingerprint ([`fingerprint.md`](fingerprint.md)): same paths with changed image bytes produce a new job. Album output subdirectories prefer the fingerprint as a collision-safe key (`albumOutputKey` in `run.go`).

## Issues

[#19](https://github.com/marcatos/cuearr/issues/19)–[#22](https://github.com/marcatos/cuearr/issues/22) · milestone [M2](https://github.com/marcatos/cuearr/milestone/2) · roadmap [`roadmap-batches.md`](roadmap-batches.md)

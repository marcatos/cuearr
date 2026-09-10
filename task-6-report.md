# Task 6 report — App use cases (Detect, Enqueue, Run, Worker)

## Enqueue idempotency — fingerprint UNIQUE race (Important)

**Problem:** Concurrent `Enqueue` calls can both observe `FindByFingerprint` → not found, then both call `Create`. SQLite rejects the second insert with a UNIQUE constraint on `jobs.fingerprint`, which previously surfaced as a hard error instead of idempotent `(job, created=false)`.

**Fix:**

- `domain.ErrConflict` for fingerprint duplicate on insert.
- SQLite `Create` maps UNIQUE violations on `fingerprint` to `domain.ErrConflict`.
- `Enqueue` on `Create` → `ErrConflict`: `FindByFingerprint` again; if found, return `(existing, false, nil)`.

**Tests:** `TestEnqueue_IdempotentOnCreateFingerprintConflict` (TOCTOU fake store: miss → conflict → hit); `TestEnqueue_CreateConflictWithoutExistingJobReturnsConflict` (orphan conflict still errors).

## v1 limitation — single-worker job claim (Important #1)

`Worker.nextQueued` selects the oldest queued job via `List` without atomic claim/update (no `UPDATE … RETURNING`, row lock, or lease). **v1 assumes a single worker process** (`sqlite` store also uses `SetMaxOpenConns(1)`). Running multiple workers against the same DB can pick the same queued job. Multi-worker safe claiming is deferred beyond v1 unless trivial to add later.

## Status

- Enqueue fingerprint race: **fixed**
- Detect / Run / Worker use cases: **implemented** (prior commits on `feat/v1`)
- Tests: `go test ./...` **pass**

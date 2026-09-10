# Task 7 report — Filesystem watcher & scan depth

## Important Task 7 — nested album dirs not watched (Important)

**Problem:** `Watcher.Start` only `fsnotify.Add`’d each watch root. Albums under nested directories (e.g. `watch/nested/album/*.cue`) never produced events because fsnotify is not recursive.

**Fix:**

- On start, `addWatchTree` walks each watch root and adds every subdirectory up to `DefaultScanDepth` (3), matching `WalkCueDirs` / `ScanAll` semantics.
- On `Create`, if the path is a new directory, add it to fsnotify so albums appearing after startup are observed.
- Cue/lossless file events still debounced at 2s.

**Tests:**

- `TestWatcher_NestedAlbumDirTriggersOnDir` — temp layout `watch/nested/album.cue` + `.flac` triggers `onDir` for the nested album dir.
- `TestScanAll_DefaultMaxDepthWhenZero` — `ScanAll(..., 0, ...)` finds cues at depth 3 via `WalkCueDirs` default.

## Status

- Recursive watch under roots: **fixed**
- ScanAll zero max depth → default 3: **verified** (logic in `WalkCueDirs`; test added)
- Tests: `go test ./internal/adapters/fs/... ./internal/app/... ./...` **pass**

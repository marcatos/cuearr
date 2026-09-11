# B4 final branch review fixes

Date: 2026-09-11

Status: all three Important findings resolved.

## Changes

- Import requests now persist `import_status=requested` before the Lidarr HTTP call, then persist `imported` or `failed` afterward. Order-spy tests cover both outcomes and verify the split remains `status=completed`.
- The Lidarr poller now runs for the daemon lifetime, waits without polling while disabled, and reacts immediately to runtime settings changes. Its focused test covers interval replacement, disable, and re-enable.
- Path C documentation warns against having Cuearr and Lidarr process the same download/incomplete directory and recommends a separate `out_dir` import path.

## Verification

- `go test ./internal/app ./cmd/cuearr` — PASS
- `go test ./...` — PASS
- `git diff --check` — PASS

Implementation commit: `d62eaaa` (`fix(b4): harden import ordering and live polling`)

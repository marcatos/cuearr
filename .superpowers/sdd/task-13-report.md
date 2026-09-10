# Task 13 — Lidarr integration (follow-up)

## Findings addressed

- Connect payloads may carry paths in `trackFiles[].path`, not only `environment.DownloadPath`.
- Custom scripts receive Lidarr env vars (`Lidarr_AddedTrackPaths`, `lidarr_trackfile_path`, `lidarr_release_path`), not Sonarr-style `lidarr_episodefile_path`.
- Inline JSON in `curl -d "{\"path\":\"$PATH\"}"` is unsafe for paths containing quotes or backslashes.

## Changes

- **`internal/adapters/http/lidarr.go`**: Extended `extractLidarrScanPath` with top-level and nested Lidarr keys, `trackFiles[0].path`, and first-path parsing for `Lidarr_AddedTrackPaths` (JSON array or `|`-separated).
- **`scripts/lidarr-custom-script.sh`**: Prefer Lidarr env vars; build POST body with `jq -n --arg` or `python3` JSON encode.
- **`README.md`**: Document webhook field precedence and correct custom-script env vars.
- **`internal/adapters/http/lidarr_test.go`**: Cases for `trackFiles`, Lidarr env keys, and `lidarrDirForScan` file→directory behavior.

## Verification

```text
go test ./...
ok  github.com/marcatos/cuearr/internal/adapters/http
```

Commit: `fix: align Lidarr hook with real Connect payloads`

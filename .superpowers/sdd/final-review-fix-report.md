# Final review fix report

Date: 2026-09-10  
Branch: `feat/v1`

## Result

All eight P1 findings from `final-review.md` were addressed:

- Non-in-place jobs now use stable, collision-safe per-album output directories, created by the shntool adapter.
- SQLite runtime settings are seeded from YAML only when empty, loaded on every serve start, and applied to scans, watcher roots, workers, output mode, and splitter selection. Watch-root changes restart the watcher.
- Quoted CUE `FILE` values preserve filenames containing spaces and supported quote/backslash escapes.
- Interrupted `running` jobs are atomically returned to `queued` when the worker starts.
- Newly created directory trees are watched and their root is immediately scheduled for scanning within the configured depth.
- OIDC domain allow-lists require a present, true `email_verified` claim.
- Proxmox LXC creation sends progress and `pct` output to stderr so command substitution receives only the CTID.
- Helm defaults pod/container UID and GID to 1000 and applies `fsGroup: 1000`.

The P2 secure-cookie setting was also added through `cookie_secure` and `CUEARR_COOKIE_SECURE=true`, including OIDC and local session cookies.

Runtime hot reload is intentionally limited to watch directories, output directory, in-place mode, and engine. HTTP address, data directory, log level, and secure-cookie mode remain startup-only and require a restart, as documented in the README.

## Verification

- `go test ./...` — passed.
- `go vet ./...` — passed.
- `go test -race ./...` — passed.
- `helm lint deploy/helm/cuearr` — passed; informational icon recommendation only.
- `bash -n deploy/proxmox/cuearr-install.sh` — passed.
- Cursor diagnostics for changed Go packages — no errors.

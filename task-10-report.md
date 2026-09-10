# Task 10 — Important findings (fix)

## Status: fixed

### 1. Session HMAC secret persistence
- **Issue:** `auth.NewSessionSecret()` ran on every boot, invalidating session cookies after restart.
- **Fix:** `auth.LoadOrCreateSessionSecret(dataDir)` reads or creates `data_dir/session.key` (32 bytes, mode `0600`). Wired in `cmd/cuearr/main.go`.
- **Tests:** `TestLoadOrCreateSessionSecret_Persists` in `internal/adapters/auth/session_secret_test.go`.

### 2. Go version pin
- **Issue:** `go.mod` bumped to `go 1.26.0` (pulled in `golang.org/x/sys v0.48.0`, which requires Go 1.26+).
- **Fix:** Restored `go 1.23`; `go mod tidy` with `GOTOOLCHAIN=local` pins `golang.org/x/sys` to `v0.28.0` (compatible with Go 1.23).

### 3. Remote bootstrap auth denial
- **Issue:** Missing test for non-localhost `PUT /api/v1/settings/auth` when password hash is empty.
- **Fix:** `TestMiddleware_BootstrapSettingsAuthDeniedFromRemote` expects `401` or `403`.

## Verification
- `go test ./...` — pass (see commit).

## Commit
- `f0b7daa` — `fix: persist session secret and pin Go 1.23`

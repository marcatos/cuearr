# Compatibility matrix (verified only)

Rows are included only when backed by automated tests or the documented Docker smoke workflow in this repo. **No live Lidarr or Plex runs** are claimed here unless noted.

| Scenario | Input | Engine / trigger | Verification | Evidence |
|----------|--------|------------------|--------------|----------|
| Synthetic album split | `testdata/e2e_album` (`album.flac` + `album.cue`, 2 tracks) | `shntool` via `JobRunner.RunJob` | Exactly 2 output FLACs; each decodes; duration 3s ±100ms; tags TITLE/ARTIST/ALBUM/TRACKNUMBER match fixture | `go test ./internal/app -run TestE2E_RunJobSplitsVerifiesAndTagsFixture` ([`e2e_split_test.go`](../internal/app/e2e_split_test.go)); enforced when `CUEARR_REQUIRE_SHNTOOL=1` in CI |
| Watch folder (Docker) | Synthetic fixture copied to `deploy/docker/watch/smoke_album` | Filesystem watcher + container `shntool` | `/api/v1/health` ok; job `completed`; ≥2 FLACs under bind-mounted `out/` | [Docker smoke](docker-smoke.md), workflow `.github/workflows/docker-smoke.yml` |
| Lidarr Connect payload | JSON `{"environment":{"DownloadPath":"/album/dir"}}` | `POST /api/v1/hooks/lidarr` + `X-Api-Key` | HTTP 201; scan path `/album/dir`; `job_id` + `created: true` | `TestHandleLidarrHook` ([`lidarr_test.go`](../internal/adapters/http/lidarr_test.go)) |
| Lidarr path extraction | Various webhook/script JSON shapes (`DownloadPath`, `trackFiles[].path`, `Lidarr_AddedTrackPaths`, etc.) | Parser only (unit tests) | Correct path or expected error | `TestExtractLidarrScanPath`, `TestLidarrDirForScan` ([`lidarr_test.go`](../internal/adapters/http/lidarr_test.go)) |
| Custom script contract | JSON body `{"path":"<album-dir>"}` | Same hook as Connect | Same handler as webhook top-level `path` | [`scripts/lidarr-custom-script.sh`](../scripts/lidarr-custom-script.sh) + path key order in [`lidarr.go`](../internal/adapters/http/lidarr.go) |

## Not verified (documented separately)

| Scenario | Status |
|----------|--------|
| End-to-end Lidarr download → webhook → import from `/out` | **Assumed** — configure per [Lidarr / Plex demo path](lidarr-plex-demo.md) |
| Plex Music track visibility after Lidarr import | **Assumed** — not run in B1 |
| WAV/APE images, in-place output, native Go splitter | Out of v0.1 scope or deferred ([#10](https://github.com/marcatos/cuearr/issues/10)) |

Tracked: [#18](https://github.com/marcatos/cuearr/issues/18).

# Compatibility matrix (verified only)

Rows are included only when backed by automated tests or the documented Docker smoke workflow in this repo. **No live Lidarr or Plex runs** are claimed here unless noted.

| Scenario | Input | Engine / trigger | Verification | Evidence |
|----------|--------|------------------|--------------|----------|
| Synthetic album split | `testdata/e2e_album` (`album.flac` + `album.cue`, 2 tracks) | `shntool` via `JobRunner.RunJob` | Exactly 2 output FLACs; each decodes; duration 3s ±100ms; tags TITLE/ARTIST/ALBUM/TRACKNUMBER match fixture | `go test ./internal/app -run TestE2E_RunJobSplitsVerifiesAndTagsFixture` ([`e2e_split_test.go`](../internal/app/e2e_split_test.go)); enforced when `CUEARR_REQUIRE_SHNTOOL=1` in CI |
| Synthetic WAV album split | `testdata/e2e_album_wav` (`album.wav` PCM + `album.cue`, 2 tracks) | `shntool -o flac` via `JobRunner.RunJob` | WAV header supplies source duration; exactly 2 output FLACs pass metaflac decode/duration/tag verification | `go test ./internal/app -run TestE2E_RunJobSplitsWAVSourceToVerifiedFLACs` ([`e2e_split_test.go`](../internal/app/e2e_split_test.go)); enforced when `CUEARR_REQUIRE_SHNTOOL=1` in CI |
| Watch folder (Docker) | Synthetic fixture copied to `deploy/docker/watch/smoke_album` | Filesystem watcher + container `shntool` | `/api/v1/health` ok; job `completed`; ≥2 FLACs under bind-mounted `out/` | [Docker smoke](docker-smoke.md), workflow `.github/workflows/docker-smoke.yml` |
| Lidarr Connect payload | JSON `{"environment":{"DownloadPath":"/album/dir"}}` | `POST /api/v1/hooks/lidarr` + `X-Api-Key` | HTTP 201; scan path `/album/dir`; `job_id` + `created: true` | `TestHandleLidarrHook` ([`lidarr_test.go`](../internal/adapters/http/lidarr_test.go)) |
| Lidarr path extraction | Various webhook/script JSON shapes (`DownloadPath`, `trackFiles[].path`, `Lidarr_AddedTrackPaths`, etc.) | Parser only (unit tests) | Correct path or expected error | `TestExtractLidarrScanPath`, `TestLidarrDirForScan` ([`lidarr_test.go`](../internal/adapters/http/lidarr_test.go)) |
| Custom script contract | JSON body `{"path":"<album-dir>"}` | Same hook as Connect | Same handler as webhook top-level `path` | [`scripts/lidarr-custom-script.sh`](../scripts/lidarr-custom-script.sh) + path key order in [`lidarr.go`](../internal/adapters/http/lidarr.go) |
| Lidarr import request | Completed Cuearr job with mapped `/out` path | Opt-in `DownloadedAlbumsScan` request to Lidarr `/api/v1/command` | Correct command/path payload; success/failure import state is separate from completed split state; API key errors are redacted | `TestClientRequestImportMapsPath`, `TestImportService_AfterSplitCompleteRequestsEnabledImport`, `TestImportService_ImportFailureKeepsSplitCompleted` ([`client_test.go`](../internal/adapters/lidarr/client_test.go), [`import_test.go`](../internal/app/import_test.go)) |

## Not verified (documented separately)

| Scenario | Status |
|----------|--------|
| Live Lidarr download → webhook → import from `/out` | **Not run in CI** — the request contract is automated; configure and validate [Path C](lidarr-plex-demo.md#path-c--cuearr-requests-lidarr-import) on your Lidarr instance |
| Plex Music track visibility after Lidarr import | **Not run** — validate on your Plex library |
| APE+CUE | **Not run / unsupported** — no support claim without a repeatable verified fixture |
| In-place output, native Go splitter | Not covered by this matrix or deferred ([#10](https://github.com/marcatos/cuearr/issues/10)) |

Tracked: [#18](https://github.com/marcatos/cuearr/issues/18).

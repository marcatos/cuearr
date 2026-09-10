# Competitive comparison

This comparison covers Cuearr v0.3.0, Unpackerr v0.15.0 or newer with
`split_flac`, gnarr/splittarr, and Flacon. It describes different product
shapes; it does **not** claim Cuearr is the only CUE splitter or that a
documentation comparison proves it is better.

## Method and evidence grades

The unit of comparison is a homelab Lidarr workflow that starts with a
single-file lossless album plus a CUE sheet and needs importable per-track
files. Claims are graded as follows:

- **`cuearr-ci`** — exercised by Cuearr's automated tests or Docker smoke.
- **`docs`** — stated by the product's official documentation, release notes,
  or repository; the Cuearr team did not reproduce it.
- **`not-run`** — not tested in this comparison, or the cited documentation
  does not establish the capability.

Grades are evidence strength, not quality scores. In particular, `docs` does
not mean independently verified, and `not-run` does not mean unsupported.
Setup time and end-to-end Lidarr import have not yet been timed across the four
products. Cuearr's live Lidarr-to-library path is also `not-run`; only payload
handling and enqueue behavior are automated today.[C1]

## Products

### Cuearr v0.3.0

A dedicated Go service for image+CUE preparation, distributed as a binary and
Docker image. Its current engine uses `shntool`; a synthetic FLAC+CUE split is
required in core CI and checks exact track count, FLAC decoding, duration, and
TITLE/ARTIST/ALBUM/TRACKNUMBER tags before completion.[C1][C2] Jobs split into
staging and publish only after verification, retain originals, use a
content-aware fingerprint, recover interrupted work, and expose retry history
and redacted diagnostics.[C3][C4][C5] It can discover work by watch folder or
accept Lidarr webhook/custom-script paths, but a live Lidarr import is not
claimed.[C1]

### Unpackerr

Unpackerr is a broader automated extraction and import companion for Starr
applications, not a dedicated audio splitter.[U1] Since v0.15.0 it can split
FLAC/CUE albums for Lidarr behind the opt-in `split_flac` setting; the
introducing release labels that feature experimental and includes manual
Lidarr import after splitting.[U2][U3]

### Splittarr (gnarr/splittarr)

Splittarr is a dedicated continuously running Lidarr companion. It monitors
completed queue items in `importFailed`, finds CUE sheets, invokes `shnsplit`,
records snapshots, generated files, status, and errors in SQLite, and provides
a small web UI. After Lidarr removes the item from its queue, Splittarr deletes
only the generated tracks it recorded and keeps the history row.[S1]

### Flacon

Flacon is a desktop GUI CUE extractor and tag editor rather than Lidarr
automation. It accepts WAV, FLAC, APE, WavPack, and TTA and can output FLAC,
WAV, WavPack, AAC, OGG, or MP3; it also supports ReplayGain and multithreaded
conversion.[F1]

## Capability matrix

| Axis | Cuearr v0.3.0 | Unpackerr v0.15.0+ | Splittarr | Flacon |
|---|---|---|---|---|
| Setup time (homelab Lidarr path) | Documented Docker/watch setup and 10-minute guided smoke, but not independently timed. **`docs`**[C5] | Adds an opt-in Lidarr setting to an Unpackerr deployment; no time measured. **`docs`**[U3] | Requires a running service, Lidarr URL/API key, shared download paths, `shnsplit`, and `flac`; no time measured. **`docs`**[S1] | Desktop install and interactive use; no Lidarr path or comparative timing. **`docs`**[F1] |
| Formats | Single-image FLAC+CUE is CI-proven; WAV/APE and multi-FILE are not claimed. **`cuearr-ci`**[C1] | FLAC/CUE splitting is documented. Other extraction formats are outside this audio-split claim. **`docs`**[U2][U3] | Single-file albums referenced by CUE; generated tracks are FLAC. Broader input support is not stated. **`docs`**[S1] | Inputs WAV, FLAC, APE, WavPack, TTA; outputs FLAC, WAV, WavPack, AAC, OGG, MP3. **`docs`**[F1] |
| Metadata / tags | TITLE, ARTIST, ALBUM, and TRACKNUMBER verified against the CUE fixture. **`cuearr-ci`**[C1][C2] | FLAC splitting is documented; tag guarantees were not established here. **`not-run`**[U2] | Repository describes CUE parsing and generated files, but not a tag-verification contract. **`not-run`**[S1] | GUI supports album-wide and per-track tag editing. **`docs`**[F1] |
| Verify / no false success | Completion gate checks count, decode integrity, durations, and tags. **`cuearr-ci`**[C1][C2] | No equivalent output-integrity completion gate was established from the cited docs. **`not-run`**[U2][U3] | Records split status and errors; count/decode/duration/tag gates are not documented. **`not-run`**[S1] | Interactive extraction is documented; an automated no-false-success gate is not. **`not-run`**[F1] |
| Error recovery | Limited retries, manual retry, interrupted-job recovery, and attempt history are documented and tested in-repo; not cross-product-run. **`docs`**[C3] | General Starr processing is continuous, but FLAC-specific recovery was not evaluated. **`not-run`**[U1][U2] | Persists lifecycle and errors and continues monitoring eligible queue items; recovery behavior was not run. **`docs`**[S1] | User corrects/retries interactively; no service recovery model. **`docs`**[F1] |
| Staging / originals | Per-job staging, verify-then-atomic publish, originals retained by default. **`docs`**[C3] | Import cleanup is configurable; `delete_orig` defaults false and docs warn against enabling it for torrents. A split staging guarantee was not established. **`docs`**[U3] | Works in the download path and deletes only recorded generated tracks after import. **`docs`**[S1] | Writes selected outputs in an interactive workflow; original-retention/staging guarantees were not evaluated. **`not-run`**[F1] |
| Lidarr integration style | Watch folder plus HTTP webhook or custom script enqueue; payload parsing is tested, live import is not. **`cuearr-ci`**[C1] | Native Lidarr polling/processing plus manual import of split tracks. **`docs`**[U1][U2] | Polls Lidarr for completed `importFailed` queue items and observes queue removal for cleanup. **`docs`**[S1] | No Lidarr integration; manual desktop extraction. **`docs`**[F1] |
| UI / operations | Embedded queue/history/settings UI, health checks, structured job errors, and redacted diagnostics. **`docs`**[C5] | Service logs/configuration as part of the broader Unpackerr operational model; no split-specific UI established. **`docs`**[U1][U3] | Built-in history/detail UI plus SQLite lifecycle, file snapshots, errors, and cleanup status. **`docs`**[S1] | Full desktop GUI for extraction, format selection, and tag editing. **`docs`**[F1] |
| Manual steps | After setup, watch/webhook can enqueue and publish automatically; Lidarr importing `/out` remains operator-configured and not live-tested. **`docs`**[C1][C5] | Enable `split_flac`; Unpackerr performs the documented split and manual-import path. **`docs`**[U2][U3] | Configure service/path mapping; eligible failed imports are then detected, split, and cleaned automatically. **`docs`**[S1] | Open/select album, review tags/settings, and start conversion for each album. **`docs`**[F1] |

## Ten synthetic cases

These cases apply the evidence grades above to concrete operator situations.
They are not a four-product execution benchmark: only the Cuearr behaviors
marked `cuearr-ci` were exercised by this repository's automation.

| Case and setup / manual steps | Cuearr v0.3.0 | Unpackerr v0.15.0+ | Splittarr | Flacon |
|---|---|---|---|---|
| 1. **Happy-path FLAC+CUE split (fixture).** Put the repository's two-track `album.flac` + `album.cue` fixture in the input path. | Produces two decodable, duration-checked, tagged FLAC tracks and completes only after verification. **`cuearr-ci`**[C1][C2] | Opt in to `split_flac`; FLAC+CUE split and the manual-import path are documented, but this fixture was not run. **`docs`**[U2][U3] | Configure Lidarr/path mapping and dependencies; splitting a failed single-file CUE album is documented, but this fixture was not run. **`docs`**[S1] | Open the album in the GUI and start extraction; FLAC+CUE extraction is documented, but this fixture was not run. **`docs`**[F1] |
| 2. **Verify fail → no completed.** Return the wrong track count during a two-track job. | Marks the job failed, exposes no final album, and does not report completion. **`cuearr-ci`**[C6] | An equivalent wrong-count completion gate was not established. **`not-run`**[U2][U3] | Status/errors are recorded, but a wrong-count completion gate was not established. **`not-run`**[S1] | An automated wrong-count completion gate was not established. **`not-run`**[F1] |
| 3. **Multi-CUE fail-closed.** Place two CUE sheets beside one image. | Rejects the directory as ambiguous before reading or enqueueing either sheet. **`cuearr-ci`**[C6] | Multi-CUE selection behavior was not established. **`not-run`**[U2][U3] | Multi-CUE selection behavior was not established. **`not-run`**[S1] | Multi-CUE selection behavior was not established. **`not-run`**[F1] |
| 4. **Multi-FILE fail-closed.** Use one CUE containing two `FILE` directives. | Rejects the CUE as unsupported instead of choosing an image silently. **`cuearr-ci`**[C6] | Multi-FILE CUE behavior was not established. **`not-run`**[U2][U3] | Multi-FILE CUE behavior was not established. **`not-run`**[S1] | Multi-FILE CUE behavior was not established. **`not-run`**[F1] |
| 5. **Staging restart / no half-publish.** Interrupt a running job or fail an album-directory promotion, then restart. | Requeues persisted `running` jobs; staging/publish tests keep partial tracks out of the final path and restore an interrupted prior album. **`cuearr-ci`**[C6] | Cleanup is configurable, but restart-safe atomic split publication was not established. **`not-run`**[U3] | Persists lifecycle and generated-file records, but restart-safe atomic publication was not established. **`not-run`**[S1] | Restart-safe service staging does not match the documented interactive workflow. **`not-run`**[F1] |
| 6. **Retry after failure.** Exhaust automatic attempts, then request a manual retry. | Manual retry resets the attempt budget, preserves history, and runs work again. **`cuearr-ci`**[C6] | FLAC-split retry-after-exhaustion behavior was not established. **`not-run`**[U1][U2] | Continued queue monitoring is documented; the equivalent manual retry contract was not established. **`not-run`**[S1] | The operator can start another interactive extraction, but no persisted retry contract was established. **`docs`**[F1] |
| 7. **Fingerprint skip reprocess.** Enqueue unchanged inputs again, then replace FLAC bytes at the same path and enqueue. | Identical content returns the existing job; changed bytes produce a different fingerprint and therefore a new job. **`cuearr-ci`**[C4][C6] | Content-aware same-path deduplication was not established. **`not-run`**[U1][U3] | Snapshot/history behavior is documented; content-aware same-path deduplication was not established. **`not-run`**[S1] | Job fingerprinting is outside the documented desktop workflow. **`not-run`**[F1] |
| 8. **Lidarr webhook trigger.** POST a Connect payload containing `environment.DownloadPath`; the operator must still configure Lidarr to import Cuearr's output. | Authenticated payload handling returns a queued job ID and the extracted album path. Live Lidarr import remains unrun. **`cuearr-ci`**[C1] | Uses native Starr polling/processing rather than this tested webhook contract. **`docs`**[U1][U2] | Polls Lidarr `importFailed` queue entries rather than this tested webhook contract. **`docs`**[S1] | No Lidarr trigger is documented; extraction is manual. **`docs`**[F1] |
| 9. **Watch-folder detect.** Write CUE and FLAC files into a watched album directory and wait for debounce. | Detects the directory once; the Docker smoke also proves watch-to-completed output for the fixture. **`cuearr-ci`**[C1][C6] | Starr polling is documented; a generic CUE watch-folder trigger was not established. **`not-run`**[U1][U3] | Lidarr queue polling is documented; a generic watch-folder trigger was not established. **`not-run`**[S1] | No watch-folder automation is documented; the operator selects input in the GUI. **`docs`**[F1] |
| 10. **Missing `shntool` / preflight.** Remove the binary from `PATH`, or make the source unstable/output unwritable, before work starts. | Availability and preflight checks fail explicitly; preflight failures prevent the splitter from running. **`cuearr-ci`**[C6] | Dependency-failure handling for `split_flac` was not run or established here. **`not-run`**[U2][U3] | `shnsplit` and `flac` are documented prerequisites; missing-tool runtime behavior was not run. **`docs`**[S1] | The documented GUI workflow does not establish Cuearr's `shntool` preflight equivalent. **`not-run`**[F1] |

## What we did not run

Unpackerr, Splittarr, and Flacon were not installed or executed for these ten
cases; their cells report only what the cited project documentation supports.
We also did not run a live Lidarr download/import or Plex visibility test for
any product, and did not measure setup time, throughput, CPU, or memory. A
`not-run` cell records an evidence gap, not a claim that the product fails.

## Sources

- [C1] [Cuearr compatibility matrix](compatibility-matrix.md)
- [C2] [Cuearr core CI workflow](../.github/workflows/ci.yml)
- [C3] [Cuearr file reliability](file-reliability.md)
- [C4] [Cuearr job fingerprint](fingerprint.md)
- [C5] [Cuearr README](../README.md) and [assisted-beta diagnostics guidance](beta.md)
- [C6] Cuearr automated case evidence: [`detect_test.go`](../internal/app/detect_test.go), [`cue_test.go`](../internal/domain/cue_test.go), [`run_test.go`](../internal/app/run_test.go), [`publish_test.go`](../internal/app/publish_test.go), [`worker_test.go`](../internal/app/worker_test.go), [`retry_test.go`](../internal/domain/retry_test.go), [`album_test.go`](../internal/domain/album_test.go), [`watcher_test.go`](../internal/adapters/fs/watcher_test.go), [`preflight_test.go`](../internal/app/preflight_test.go), and [`shntool_test.go`](../internal/adapters/splitter/shntool/shntool_test.go)
- [U1] [Unpackerr official site](https://unpackerr.zip/)
- [U2] [Unpackerr v0.15.0 release notes](https://github.com/Unpackerr/unpackerr/releases/tag/v0.15.0)
- [U3] [Unpackerr application configuration](https://unpackerr.zip/docs/install/configuration/)
- [S1] [gnarr/splittarr official repository](https://github.com/gnarr/splittarr)
- [F1] [Flacon official site](https://flacon.github.io/)

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

## Sources

- [C1] [Cuearr compatibility matrix](compatibility-matrix.md)
- [C2] [Cuearr core CI workflow](../.github/workflows/ci.yml)
- [C3] [Cuearr file reliability](file-reliability.md)
- [C4] [Cuearr job fingerprint](fingerprint.md)
- [C5] [Cuearr README](../README.md) and [assisted-beta diagnostics guidance](beta.md)
- [U1] [Unpackerr official site](https://unpackerr.zip/)
- [U2] [Unpackerr v0.15.0 release notes](https://github.com/Unpackerr/unpackerr/releases/tag/v0.15.0)
- [U3] [Unpackerr application configuration](https://unpackerr.zip/docs/install/configuration/)
- [S1] [gnarr/splittarr official repository](https://github.com/gnarr/splittarr)
- [F1] [Flacon official site](https://flacon.github.io/)

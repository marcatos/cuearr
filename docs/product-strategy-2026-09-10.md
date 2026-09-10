# Cuearr — product evolution strategy

Analysis dated 10 September 2026 · checkout `18629d4`


## Diagnosis

### Direction

Make Cuearr a small, reliable service that prepares image+CUE albums for import. Initial audience: homelab users with Lidarr, Docker, and lossless albums. The advantage to validate is understanding and fixing real failure modes, plus **verifiable** output quality.

### Solid base already present

Single Go process, embedded UI, SQLite, domain/adapters split, authentication, recursive watcher, and several distribution packages. The codebase already includes atomic job claim, recovery of interrupted `running` jobs, and async scan. The README may still list some of that work as backlog — keep docs and releases aligned with `main`.

### P0 · Completed must mean verified

`internal/adapters/splitter/shntool/shntool.go` treats success as process exit code and does not populate `OutputFiles`. `internal/app/run.go` marks `completed` without checking track count, decode integrity, or durations. There is no tagging phase: preserving CUE metadata is part of the importability promise.

### P0 · Writes and restarts

Split writes straight to the destination. Introduce per-job **staging**, validation, then final publish on the same filesystem; keep originals by default. A restart or retry must not expose partial output or confuse it with a new run.

### P0 · Lidarr path must be demonstrated

The endpoint accepts a path and creates a job; that does not prove a release blocked before import fires the expected webhook, nor that Lidarr imports the output directory. Reproduce the flow with a controlled release; document one working path before promising full automation.

### P1 · Recognition and diagnosis

The watcher uses a two-second debounce, which does not prove a copy finished. The fingerprint covers paths and CUE bytes, not audio content: replacing the image at the same path can be skipped. The parser keeps a single `FILE` and picks the first CUE: reject ambiguous cases before supporting them.

### P1 · Operational UI

`web/app.js` shows IDs, paths, and statuses; manual refresh, list capped at 500, and a selectable `native` engine that is still a stub. Prefer album/artist, expected outcome, auto-refresh, actionable errors, explicit retry, and attempt history. Hide options that cannot work.


## Roadmap

### Weeks 1–2 · Prove the promise

Define the path: Docker + single FLAC/CUE + separate output. Make synthetic split **mandatory in CI**; verify exact track count, decode, durations, and metadata. Reproduce Lidarr import and Plex track visibility. Publish a compatibility matrix with verified cases only. Exit criterion: a repeatable full scenario and **no success with incomplete output**.

### Weeks 3–4 · File reliability

Staging and final publish, stable input, limited retries with history, safe restart, updated input identity when audio changes. Validate CUE and check permissions/disk space before starting. Exit criterion: interruption, disk-full, in-flight copy, and replaced-input tests handled without losing originals or false `completed`.

### Weeks 5–6 · Assisted beta

Recruit 5–10 testers on different installs. Onboarding: folders → permission/tool checks → synthetic album → Lidarr link. Show scan state and per-album problems; export redacted diagnostics. Proposed exit: at least 4 of 5 testers finish the first album without code changes from maintainers.

### Months 2–3 · Integration driven by real cases

If the webhook does not cover blocked imports, build a Lidarr connector with queue detection, path mapping, and idempotent import requests. Separate split state from import state; avoid two systems processing the same directory. Add only formats and CUE variants repeatedly requested by testers.

### Months 3–6 · Stability and community

Grow the corpus: encodings, pregap, multi-disc, multi-FILE, sample rates — with explicit policies. Improve updates, migrations, and SQLite backups. Make Unraid a verified path if demand appears; leave Helm/Proxmox to contributors. Evaluate a native engine only with evidence that the current dependency blocks adoption or correctness.

### Capacity and choices

Indicative schedule for a part-time maintainer; depends on feedback. Aim ~60% correctness and integration, 25% UX and docs, 15% community. Defer new installers, large redesigns, cloud services, and full library management.


## Community and decisions

### Verified competition · 10 September 2026

Unpackerr documents `split_flac` for Lidarr. Splittarr already markets detect-stuck-album → split → cleanup after import. Flacon covers extraction and tagging. So “the missing splitter” is a weak position: compare the same albums and measure where Cuearr is preferable.

### Experiment before expanding

Within two weeks, compare Cuearr and alternatives on 10 synthetic or shareable cases: setup time, formats that work, metadata, error recovery, and manual steps. Repository analysis alone does not prove superiority. If incremental value is low, narrow the niche or consider upstream contributions.

### Channel and message

Use GitHub Issues as the only tracker, consistent with CONTRIBUTING. README with a short demo, input/output example, limits, and a ten-minute guide. Announce beta in relevant communities per their rules; ask for reproducible cases; do not open many support channels at once. No announcement was sent during this analysis.

### Small, useful contributions

Prepare issues with expected result and verification: synthetic CUE fixtures, diagnostic messages, path-mapping guide, Unraid proof. Every reproducible bug should add a regression fixture. Labels for format, install method, and pipeline stage; roadmap with at most three active priorities.

### Measure usefulness, not visits

Proposed initial goals (not observed data): first album within 10 minutes; ≥95% of supported albums completed and imported on a declared corpus of ≥50 cases; zero false successes and zero damaged originals in failure tests; ≥5 installs still in use after 30 days. Record denominators, version, and exclusion reasons.

### Sustainability

Collect outcomes via voluntary beta and local diagnostics — no mandatory telemetry. A biweekly summary keeps the project predictable. Consider sponsorship only after recurring use and contributions; do not add commercial overhead immediately.

### 90-day decision point

Keep investing if at least five users still run it and can name a concrete advantage over alternatives. If interest is episodic, maintain a small stable utility. Expand formats only where demand and tests justify support cost.

### Method and limits

Static analysis of checkout `18629d4`, docs, UI source, and CI. `go test ./...` passes; the explicit E2E test skips when `shntool` is missing from PATH. Not verified: UI boot, container, real audio, Lidarr/Plex import, or adoption. Code observations are facts; priorities, targets, and positioning are recommendations.

## Sources

- [Unpackerr — official config](https://unpackerr.zip/docs/install/generated/)
- [Splittarr — official repository](https://github.com/gnarr/splittarr)
- [Flacon — official site](https://flacon.github.io/)

Local sources: README.md, internal/app/run.go, internal/app/worker.go, internal/app/enqueue.go, internal/domain/cue.go, internal/domain/album.go, internal/adapters/fs/watcher.go, internal/adapters/splitter/shntool/shntool.go, internal/adapters/http/lidarr.go, internal/adapters/http/handlers.go, web/app.js, .github/workflows/ci.yml, and internal/app/e2e_split_test.go.

# Assisted beta (batch B3)

Goal: **≥4 of 5** recruited testers finish their **first real album** split without maintainer code changes. Tracker: [#23](https://github.com/marcatos/cuearr/issues/23)–[#25](https://github.com/marcatos/cuearr/issues/25) · milestone [M3](https://github.com/marcatos/cuearr/milestone/3).

## Before you apply

1. Run the [10-minute onboarding](onboarding.md) with the synthetic fixture so Docker/watch/out and UI login work.
2. Use **v0.2.0+** (binary or [`ghcr.io/marcatos/cuearr`](https://github.com/marcatos/cuearr/pkgs/container/cuearr)).
3. Have at least one **image + `.cue`** album you are allowed to process (your library or test rips — do not upload copyrighted audio to GitHub).

## Sign up

Open a **[Beta tester signup](https://github.com/marcatos/cuearr/issues/new?template=beta-tester.yml)** issue (template: `.github/ISSUE_TEMPLATE/beta-tester.yml`). Maintainers will confirm you on the roster and may ask one follow-up in the thread.

We cap the cohort at **five** active testers for M3; signups after that go on a waitlist in the same issue comment thread.

## What to report

Use your signup issue (or linked child issues) for progress notes. For failures, open a dedicated issue with the right template:

| Situation | Template |
|-----------|----------|
| Split failed | [Split failure](https://github.com/marcatos/cuearr/issues/new?template=split_failure.yml) |
| Other bug | [Bug report](https://github.com/marcatos/cuearr/issues/new?template=bug_report.yml) |

Every report should include:

- **Cuearr version** (CLI `cuearr version` or image tag).
- **Install method** (Docker, Unraid, binary, Helm, Proxmox, …).
- **OS / architecture** (e.g. Linux amd64, Unraid 6.12).
- **Redacted paths** — use placeholders like `/music/incoming/album` instead of usernames or full host paths.
- **Diagnostics** — relevant UI job id, log excerpts (debug if possible), job JSON from `GET /api/v1/jobs` with secrets removed. Attach **minimal CUE** or extension list only; never attach FLAC/WAV/APE.

Mark **first album completed** on your signup issue when tracks land in `out` (or your Lidarr import path) and job status is `completed`.

## Maintainer biweekly summary

Every **two weeks** (aligned with M3 check-ins), the maintainer posts a short summary on [#25](https://github.com/marcatos/cuearr/issues/25) and updates milestone M3 if needed.

Checklist:

- [ ] Roster: count signup issues open vs **first album completed** (target 4/5).
- [ ] Triage new beta-related issues; label `batch:b3` where appropriate.
- [ ] List **blockers** (tooling, docs, product) and whether they need a release or doc fix.
- [ ] Note **install-method mix** (Docker vs bare metal) and any pattern in split failures.
- [ ] Decide **waitlist** promotions if a tester drops out.
- [ ] Link to any shipped fixes (releases, `docs/onboarding.md`, `docs/file-reliability.md`) testers should pick up.

No copyrighted audio or secrets in public summaries.

## Related docs

- [Onboarding smoke test](onboarding.md)
- [Lidarr/Plex demo path](lidarr-plex-demo.md)
- [File reliability (B2)](file-reliability.md)
- [Roadmap batches](roadmap-batches.md)

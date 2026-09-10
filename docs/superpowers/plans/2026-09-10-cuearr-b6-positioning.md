# Cuearr B6 — Positioning Experiment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish an honest 10-case comparison of Cuearr vs Unpackerr / Splittarr / Flacon so positioning is evidence-based, not “missing splitter” marketing.

**Architecture:** Documentation-only batch. Results live under `docs/`. Method must label each cell **docs** (public docs/README) vs **cuearr-ci** (verified in Cuearr CI/e2e) vs **not-run** (competitor not executed in this lab). No secrets.

**Tech Stack:** Markdown docs, existing Cuearr v0.3.0 behaviour, public competitor docs.

**Issue:** #30.

## Global Constraints

- Branch: `feat/b6-positioning` (not `main`). `gh` as `marcatos` only.
- Conventional Commits.
- Do **not** claim Cuearr is the only Lidarr-oriented splitter; Unpackerr has `split_flac`, Splittarr targets importFailed queue, Flacon is desktop extraction.
- Competitors need not be installed for this batch if labelled **docs** / **not-run**; Cuearr rows that match CI may be **cuearr-ci**.
- Ten cases must be synthetic/shareable (no copyrighted audio). Prefer variants of `scripts/generate_fixture.sh` + documented edge cases.
- After merge: follow `.cursor/rules/releases.mdc` (MINOR → `v0.4.0` for completing B6).
- Update README positioning if incremental value is narrow (niche: verified splits + staging + dedicated watch/webhook service).

## File map

| Path | Responsibility |
|------|----------------|
| `docs/competitive-comparison.md` | Method, product matrix, 10-case table, conclusion |
| `docs/roadmap-batches.md` | B6 done; next active (B4 or none) |
| `README.md` | Positioning paragraph + link |

---

### Task 1: Method + product capability matrix

**Files:** Create `docs/competitive-comparison.md` (skeleton through § Products)

**Content:**
- Method / evidence grades (`docs`, `cuearr-ci`, `not-run`)
- Short profiles: Cuearr v0.3.0, Unpackerr (`split_flac`), Splittarr (gnarr/splittarr), Flacon
- Capability matrix axes: setup time (homelab Lidarr path), formats, metadata/tags, verify/no false success, error recovery, staging/originals, Lidarr integration style, UI/ops, manual steps

- [ ] **Step 1: Write section.**
- [ ] **Step 2: Commit** `docs: add competitive comparison method and product matrix`

---

### Task 2: Ten synthetic cases + scored results

**Files:** Extend `docs/competitive-comparison.md`

**Ten cases (define clearly):**
1. Baseline 2-track synthetic FLAC+CUE (Cuearr fixture)
2. Same album, verify must fail on wrong track count (Cuearr gate)
3. Duration mismatch / bad INDEX (fail closed)
4. Ambiguous multi-CUE directory
5. Multi-FILE CUE (fail closed in Cuearr)
6. In-place vs separate out dir
7. Restart / crash mid-job (Cuearr staging — docs/code)
8. Retry after permanent failure (manual retry resets budget)
9. Replaced FLAC bytes same path (fingerprint)
10. Lidarr webhook/custom-script enqueue path (Cuearr documented)

For each: Cuearr result + Unpackerr/Splittarr/Flacon expected fit (**docs**), setup/manual steps notes.

- [ ] **Step 1: Fill 10-case table.**
- [ ] **Step 2: Commit** `docs: record 10-case competitive comparison results`

---

### Task 3: Conclusion, README positioning, roadmap

**Files:** Finish comparison conclusion; `README.md`; `docs/roadmap-batches.md`

**Conclusion must answer:** Where Cuearr is preferable; where competitors win; whether to continue niche vs narrow further. Update README “Why Cuearr” to drop any weak “missing splitter” vibe.

- [ ] **Step 1: Write conclusion + README + roadmap (B6 done).**
- [ ] **Step 2: Commit** `docs: close B6 positioning experiment and update README`

---

## Progress ledger

`.superpowers/sdd/progress.md`

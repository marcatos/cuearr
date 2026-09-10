# Cuearr B3 — Assisted Beta Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Cuearr beta-ready: 10-minute onboarding, redacted diagnostics export, album-centric auto-refreshing UI with actionable errors and retry, plus recruitment/biweekly process docs.

**Architecture:** Keep hexagonal layout. Diagnostics as authenticated HTTP adapter assembling redacted snapshots from settings/jobs/tool probes (no secrets). UI remains embedded static JS. Docs under `docs/` + CONTRIBUTING.

**Tech Stack:** Go 1.23, existing HTTP API + web UI, GitHub Issues templates.

**Issues:** #23 onboarding + diagnostics · #24 operational UI · #25 beta recruitment process.

## Global Constraints

- Branch: `feat/b3-assisted-beta` (not `main`). `gh` as `marcatos` only.
- Conventional Commits; commit after each task.
- Hexagonal: no secrets in logs or diagnostics JSON (strip password hashes, API keys, OIDC client secrets; show booleans like `api_key_set` only).
- Logging: slog levels, durations, no secrets.
- Do not implement B4 Lidarr connector or B5 native engine.
- Hide or disable `native` engine in UI (stub); API may reject selecting native with clear error if still requested.
- Auto-refresh interval: **5 seconds** on queue/history when tab visible (`document.visibilityState`).
- Album label: prefer `filepath.Base(filepath.Dir(cue_path))` plus optional CUE TITLE if cheap; never show raw UUID as primary label (short id secondary).
- After merge to `main`, follow `.cursor/rules/releases.mdc` (MINOR bump → tag).
- Artifact retention if any new CI artifacts: `retention-days: 7`.

## File map

| Path | Responsibility |
|------|----------------|
| `docs/onboarding.md` | 10-minute path: folders → tools → synthetic → Lidarr |
| `internal/adapters/http/diagnostics.go` (+ tests) | `GET /api/v1/diagnostics` redacted export |
| `web/app.js` (+ `index.html`/`styles.css` if needed) | Album-centric UI, auto-refresh, retry, hide native, remediation copy |
| `internal/adapters/http` retry handler | `POST /api/v1/jobs/{id}/retry` for failed jobs |
| `docs/beta.md` + CONTRIBUTING / issue template | Recruitment + biweekly summary |
| `docs/roadmap-batches.md` + README | B2 done; active B3 + B6 |

---

### Task 1: Onboarding doc (10-minute path) (#23 docs half)

**Files:**
- Create: `docs/onboarding.md`
- Modify: `README.md` (link under install/status)

**Content must cover:**
1. Set watch + out dirs (Compose defaults ok)
2. Confirm tools (`shntool`/`metaflac` in container)
3. Generate synthetic album (`scripts/generate_fixture.sh`) → drop in watch
4. Expect verified `completed` + tracks in out
5. Point to Lidarr demo doc for Connect

- [ ] **Step 1: Write doc.**
- [ ] **Step 2: Link from README.**
- [ ] **Step 3: Commit** `docs: add 10-minute onboarding path for beta`

---

### Task 2: Redacted diagnostics export API (#23)

**Files:**
- Create: `internal/adapters/http/diagnostics.go`, `diagnostics_test.go`
- Modify: router/server wiring; optionally UI download button

**`GET /api/v1/diagnostics` (auth required)** returns JSON:
- `version`, `engine`, `watch_dirs`, `out_dir`, `in_place`, `max_retries`
- Auth flags only: `password_configured`, `api_key_set`, `oidc_enabled` (no secrets)
- Tool probes: shntool/metaflac available bool + error string
- Last N jobs (e.g. 20): id, status, album label fields, cue/image basenames (not full secrets), error, attempt_count (truncate long logs to 2KiB)
- Never include: password hashes, api_key values, oidc_client_secret, browser tokens

- [ ] **Step 1: Failing handler tests** (assert secrets absent).
- [ ] **Step 2: Implement + wire.**
- [ ] **Step 3: Optional UI “Download diagnostics” on settings.**
- [ ] **Step 4: Commit** `feat(api): redacted diagnostics export endpoint`

---

### Task 3: Operational UI (#24)

**Files:**
- Modify: `web/app.js`, maybe `web/styles.css`

**Behavior:**
1. Queue/history primary column = album label (dir basename); id truncated secondary.
2. Auto-refresh dashboard + history every 5s while visible; clear interval on navigate away.
3. Settings engine select: **only shntool** (remove native option). If stored engine is native, show warning and force save to shntool.
4. Job detail: for `failed`, show remediation hints mapped from common errors (permissions, disk, ambiguous CUE, verify mismatch) + **Retry** button calling retry API.
5. Explicit retry control only on failed jobs.

Depends on Task 4 for retry endpoint — implement UI against the endpoint in Task 4, or do Task 4 first.

**Reorder:** Implement Task 4 before finishing Task 3 if needed; this plan lists UI after API retry.

- [ ] **Step 1: Album labels + auto-refresh.**
- [ ] **Step 2: Hide native; remediation copy.**
- [ ] **Step 3: Retry button wired.**
- [ ] **Step 4: Commit** `feat(ui): album-centric queue with auto-refresh and retry`

---

### Task 4: Job retry API (#24 support)

**Files:**
- Modify: `internal/adapters/http/handlers.go` (+ tests), maybe app helper
- Behavior: `POST /api/v1/jobs/{id}/retry` — only if status `failed`; set `queued`, clear error/log optionally keep attempt history; 409 if not failed.

- [ ] **Step 1: Failing tests.**
- [ ] **Step 2: Implement.**
- [ ] **Step 3: Commit** `feat(api): retry failed jobs via POST`

---

### Task 5: Beta recruitment + biweekly process (#25)

**Files:**
- Create: `docs/beta.md`
- Modify: `CONTRIBUTING.md`
- Create: `.github/ISSUE_TEMPLATE/beta-tester.yml` or `beta-feedback.md`

**Content:**
- How to sign up (open issue with template)
- What to report (paths redacted, diagnostics attach, OS/install method)
- Maintainer biweekly summary cadence (checklist)

- [ ] **Step 1: Write docs + template.**
- [ ] **Step 2: Commit** `docs: beta recruitment and biweekly summary process`

---

### Task 6: Roadmap/README active priorities

**Files:**
- Modify: `docs/roadmap-batches.md`, `README.md`

Mark B2 **Done**; active **B3 + B6**; fix Project view blurb if needed.

- [ ] **Step 1: Update.**
- [ ] **Step 2: Commit** `docs: activate B3 assisted beta after B2`

---

## Progress ledger

`.superpowers/sdd/progress.md` (gitignored):

```
Task N: complete (commits <base7>..<head7>, review clean)
```

# Cuearr — roadmap batches

Canonical strategy: [`product-strategy-2026-09-10.md`](product-strategy-2026-09-10.md)  
Tracker: [GitHub Issues](https://github.com/marcatos/cuearr/issues) · [**Cuearr Roadmap** project](https://github.com/users/marcatos/projects/1) · [Milestones](https://github.com/marcatos/cuearr/milestones) · at most **three active** priorities at a time.

Filter by label: [`batch:b1`](https://github.com/marcatos/cuearr/labels/batch%3Ab1) … [`batch:b6`](https://github.com/marcatos/cuearr/labels/batch%3Ab6).

### Project views

| View | What it shows |
|------|----------------|
| [Active now](https://github.com/users/marcatos/projects/1/views/4) | `batch:b6` |
| [P0 must-fix](https://github.com/users/marcatos/projects/1/views/3) | `priority:p0` |
| [Kanban](https://github.com/users/marcatos/projects/1/views/2) | Status board (Todo / In Progress / Blocked / Backlog / Done) |
| [Queued later](https://github.com/users/marcatos/projects/1/views/5) | B4–B5 |
| [Roadmap](https://github.com/users/marcatos/projects/1/views/6) | Timeline by milestone due dates |
| [All issues](https://github.com/users/marcatos/projects/1/views/1) | Full table |

Project Status: **Todo** = active ready · **Blocked** = waiting on dependency · **Backlog** = later batches.

## Batch map

| Batch | Horizon | Theme | Exit criterion | Milestone | Issues |
|-------|---------|-------|----------------|-----------|--------|
| **B0** | Done when merged | Docs hygiene | Strategy + batches linked; README backlog accurate | — | docs commits |
| **B1** | **Done** (2026-09) | Prove the promise | Repeatable Docker→split→Lidarr/Plex; CI refuses false `completed` | [M1](https://github.com/marcatos/cuearr/milestone/1) due 2026-09-24 | [#15](https://github.com/marcatos/cuearr/issues/15) [#16](https://github.com/marcatos/cuearr/issues/16) [#17](https://github.com/marcatos/cuearr/issues/17) [#14](https://github.com/marcatos/cuearr/issues/14) [#18](https://github.com/marcatos/cuearr/issues/18) |
| **B2** | **Done** (2026-09) | File reliability | Staging/publish; no partial publish; no false completed on retry/restart | [M2](https://github.com/marcatos/cuearr/milestone/2) due 2026-10-08 | [#19](https://github.com/marcatos/cuearr/issues/19) [#20](https://github.com/marcatos/cuearr/issues/20) [#21](https://github.com/marcatos/cuearr/issues/21) [#22](https://github.com/marcatos/cuearr/issues/22) |
| **B3** | **Done** (2026-09) | Assisted beta | ≥4/5 testers finish first album without maintainer code changes | [M3](https://github.com/marcatos/cuearr/milestone/3) due 2026-10-22 | [#23](https://github.com/marcatos/cuearr/issues/23) [#24](https://github.com/marcatos/cuearr/issues/24) [#25](https://github.com/marcatos/cuearr/issues/25) |
| **B4** | Months 2–3 | Real-case integration | Split vs import states separated; formats only if repeatedly requested | [M4](https://github.com/marcatos/cuearr/milestone/4) due 2026-12-10 | [#26](https://github.com/marcatos/cuearr/issues/26) [#27](https://github.com/marcatos/cuearr/issues/27) |
| **B5** | Months 3–6 | Stability & community | Corpus/policies; backups; Unraid verified if demanded | [M5](https://github.com/marcatos/cuearr/milestone/5) due 2027-03-10 | [#28](https://github.com/marcatos/cuearr/issues/28) [#29](https://github.com/marcatos/cuearr/issues/29) [#10](https://github.com/marcatos/cuearr/issues/10) |
| **B6** | Parallel | Positioning experiment | 10-case comparison vs Unpackerr/Splittarr/Flacon before expanding | [M6](https://github.com/marcatos/cuearr/milestone/6) due 2026-09-24 | [#30](https://github.com/marcatos/cuearr/issues/30) |

## Active priorities (now)

1. **B6** — Competitive experiment (atch:b6, measurement/docs only)

**Completed:** **B1**–**B3**.

## Work order inside B3 (reference)

1. [#23](https://github.com/marcatos/cuearr/issues/23) cohort + onboarding gate  
2. [#24](https://github.com/marcatos/cuearr/issues/24) first-album tracking  
3. [#25](https://github.com/marcatos/cuearr/issues/25) biweekly maintainer summary  

## B2 reference (done)

1. [#19](https://github.com/marcatos/cuearr/issues/19) staging + atomic publish  
2. [#20](https://github.com/marcatos/cuearr/issues/20) no partial publish on failure  
3. [#21](https://github.com/marcatos/cuearr/issues/21) retry / restart without false `completed`  
4. [#22](https://github.com/marcatos/cuearr/issues/22) recovery for stuck `running` jobs  

See [`file-reliability.md`](file-reliability.md).

## B1 reference (done)

1. [#15](https://github.com/marcatos/cuearr/issues/15) verified `completed` + tags  
2. [#16](https://github.com/marcatos/cuearr/issues/16) mandatory CI synthetic split  
3. [#14](https://github.com/marcatos/cuearr/issues/14) Docker smoke checklist (executable)  
4. [#17](https://github.com/marcatos/cuearr/issues/17) Lidarr/Plex demo path  
5. [#18](https://github.com/marcatos/cuearr/issues/18) compatibility matrix  

## Deferred by strategy

- Native Go engine → [#10](https://github.com/marcatos/cuearr/issues/10) (only with evidence of blocked adoption/correctness)  
- Broad new installers, cloud, full library UI — out of scope

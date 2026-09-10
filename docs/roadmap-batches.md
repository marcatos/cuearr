# Cuearr — roadmap batches

Canonical strategy: [`product-strategy-2026-09-10.md`](product-strategy-2026-09-10.md)  
Tracker: [GitHub Issues](https://github.com/marcatos/cuearr/issues) · at most **three active** priorities at a time.

Filter by label: [`batch:b1`](https://github.com/marcatos/cuearr/labels/batch%3Ab1) … [`batch:b6`](https://github.com/marcatos/cuearr/labels/batch%3Ab6).

## Batch map

| Batch | Horizon | Theme | Exit criterion | Issues |
|-------|---------|-------|----------------|--------|
| **B0** | Done when merged | Docs hygiene | Strategy + batches linked; README backlog accurate | docs commits |
| **B1** | Weeks 1–2 | Prove the promise | Repeatable Docker→split→Lidarr/Plex; CI refuses false `completed` | [#15](https://github.com/marcatos/cuearr/issues/15) [#16](https://github.com/marcatos/cuearr/issues/16) [#17](https://github.com/marcatos/cuearr/issues/17) [#14](https://github.com/marcatos/cuearr/issues/14) [#18](https://github.com/marcatos/cuearr/issues/18) |
| **B2** | Weeks 3–4 | File reliability | Staging/publish; no partial publish; no false completed on retry/restart | [#19](https://github.com/marcatos/cuearr/issues/19) [#20](https://github.com/marcatos/cuearr/issues/20) [#21](https://github.com/marcatos/cuearr/issues/21) [#22](https://github.com/marcatos/cuearr/issues/22) |
| **B3** | Weeks 5–6 | Assisted beta | ≥4/5 testers finish first album without maintainer code changes | [#23](https://github.com/marcatos/cuearr/issues/23) [#24](https://github.com/marcatos/cuearr/issues/24) [#25](https://github.com/marcatos/cuearr/issues/25) |
| **B4** | Months 2–3 | Real-case integration | Split vs import states separated; formats only if repeatedly requested | [#26](https://github.com/marcatos/cuearr/issues/26) [#27](https://github.com/marcatos/cuearr/issues/27) |
| **B5** | Months 3–6 | Stability & community | Corpus/policies; backups; Unraid verified if demanded | [#28](https://github.com/marcatos/cuearr/issues/28) [#29](https://github.com/marcatos/cuearr/issues/29) [#10](https://github.com/marcatos/cuearr/issues/10) |
| **B6** | Parallel with B1 | Positioning experiment | 10-case comparison vs Unpackerr/Splittarr/Flacon before expanding | [#30](https://github.com/marcatos/cuearr/issues/30) |

## Active priorities (now)

1. **B1** — Prove the promise (`batch:b1`)  
2. **B6** — Competitive experiment (`batch:b6`, measurement/docs only)  
3. **B2** — blocked until B1 exit  

## Work order inside B1

1. [#15](https://github.com/marcatos/cuearr/issues/15) verified `completed` + tags  
2. [#16](https://github.com/marcatos/cuearr/issues/16) mandatory CI synthetic split  
3. [#14](https://github.com/marcatos/cuearr/issues/14) Docker smoke checklist (executable)  
4. [#17](https://github.com/marcatos/cuearr/issues/17) Lidarr/Plex demo path  
5. [#18](https://github.com/marcatos/cuearr/issues/18) compatibility matrix  

## Deferred by strategy

- Native Go engine → [#10](https://github.com/marcatos/cuearr/issues/10) (only with evidence of blocked adoption/correctness)  
- Broad new installers, cloud, full library UI — out of scope  

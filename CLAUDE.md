# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

**AshyqQala.kz** — a civic transparency platform for Kazakhstan government procurement
(`goszakup`), scoped to road and water-supply contracts with an Astana pilot. The MVP is a
read-only "reading version": a public map, contract/contractor cards, four recalculable risk
flags, and a Telegram district-subscription bot, all built on the official `ows_v2` REST API.

**Current state: pre-implementation.** The MVP application has not been built yet. The repo
holds (a) one real Go tool, `stage0-audit/`, and (b) extensive planning artifacts that define
the contract the future app will implement. Most of the tree is untracked by git — only
`README.md`, `LICENSE`, `.gitignore` are committed. Working language of docs and code comments
is **Russian**.

## The only code today: `stage0-audit/`

A dependency-free Go (stdlib only) CLI that runs the Stage-0 data-audit runbook against
`goszakup ows_v2` and prints a preliminary Go/No-Go table. Its entire purpose is to answer
open questions from the PRD **before** the MVP sprint starts: OQ-1 (contract volume), OQ-4
(geocoding Go/No-Go gate ≥70%), OQ-6 (sample sufficiency for medians/flags).

### Build & run

```bash
cd stage0-audit
go build ./...          # builds clean; module ashyqqala/stage0-audit, Go 1.25

# ALWAYS run -probe first: prints real field names of each endpoint's first object,
# so you can reconcile them against fieldCandidates in models.go (see VERIFY below).
go run . -probe

# Full audit against the live API (needs a token):
export GOSZAKUP_TOKEN="<ows_v2 Bearer token>"
go run . -sample 1000
go run . -customer-bins "BIN1,BIN2" -geocoder nominatim -geo-sample 200   # real geocoding

# No token? Develop the whole pipeline against synthetic fixtures or local dumps:
go run . -gen-fixtures -data-dir ./data -fixtures-n 300
go run . -source file -data-dir ./data -geocoder nominatim -geo-sample 20
```

There are **no tests** and no Makefile. `go build ./...` is the only build check.

### `-source` abstraction (the central design point)

Data access is decoupled behind the `Source` interface (`source.go`) so the audit logic is
identical regardless of where data comes from. Switching sources is a one-flag flip:

- `ows` (default) — live `ows_v2` API by Bearer token (`client.go`: pagination via
  `total`/`next_page`/`items`).
- `file` — local JSON dumps `<data-dir>/<resource>.json` (array of objects per resource:
  `contract`, `lots`, `trd-buy`, `rnu`). Shape is defined by the fixture generator (`fixtures.go`).
- `scrape` — public portal parser (`scrape.go`). **Legally constrained**: scraping violates the
  project's load-bearing "official channel, not scraping" principle (конкурсный документ §6.1/§6.4).
  Use only for one-off sampling, never production. Only returns lots; contracts/participants/RNU
  need the authorized API.

### Audit flow (`main.go` → `runAudit` → `Report.print`)

Steps map directly to PRD open questions and the runbook:

- **Step A** — contract volume + completeness of sum/BIN/dates (closes OQ-1).
- **Step B** — direction classification (road/water/other via keyword match) + geo-proxy
  (share of lots whose *name* contains an address marker). With `-geocoder nominatim`, **Step B+**
  measures *real* auto-coverage against the ≥70% gate (FR-6/OQ-4).
- **Step C** — normalization signals (distinct supplier BINs, missing-BIN rate).
- **Step D** — sufficiency for medians, monopoly (top-supplier share by sum, joining
  contracts↔lots on `№объявления`/anno), single-participant, RNU presence (closes OQ-6).

## Key conventions when touching the audit code

- **Schema resilience is deliberate.** The parser never assumes exact API field names. `models.go`
  maps a *logical* field to a list of candidate real names (`fieldCandidates`) — first non-empty
  wins; unknown schema yields `""`/`0` and is shown honestly in completeness metrics, never a crash.
- **`VERIFY` markers** in `config.go`/`models.go` mark assumptions to confirm against the live API
  via `-probe`: Astana KATO prefix (`"71"`), sum/BIN/date/KATO/anno field names, and the
  participant-count source for the single-participant flag. If `-probe` shows a different name,
  **add a candidate** to the list rather than changing logic.
- **Geocoding politeness:** public Nominatim policy is ≤1 req/sec — hence defaults
  `-geo-delay-ms 1100`, `-geo-sample 200`. For volume, run a local Nominatim and pass
  `-geocoder-url`, then drop the delay.

## The planning contract (what the future MVP must satisfy)

The audit references `FR-N` and `OQ-N` identifiers that come from the specs. When implementing
the actual MVP, these documents are the source of truth — read them, don't re-derive:

- `_bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/prd.md` — the PRD. Defines
  FR-1…FR-28, NFR-1…NFR-8, the four risk flags (FR-19…FR-23), guardrails, and metrics (SM-N).
  `addendum.md` (sibling) holds the tech "how" kept out of the PRD.
- `docs/AshyqQala_MVP_data_model_and_flags_v1.md` — logical data model (central object
  `contracts`; `organizations`, `geo_objects`, `risk_flags`, `price_benchmarks`, etc.) and the
  starting flag/median defaults (`methodology_params`).
- `docs/AshyqQala_stage0_data_audit_runbook_v1.md` — the runbook this tool implements.
- `docs/AshyqQala_kz_конкурсный_документ_v2.md`, `docs/AshyqQala_use_cases_v4_full_dev_ready.md` —
  vision/market and UC-01…UC-11.

### Planned MVP stack (per addendum, not yet built)

Go 1.25 + chi, pgx + sqlc · PostgreSQL 16 + PostGIS · React + Vite + MapLibre GL · Directus
admin over Postgres (geocoding + normalization queues) · Caddy + Docker Compose on one VPS ·
GitHub Actions · slog + `/metrics`.

### Two load-bearing guardrails — preserve these in any feature

1. **Neutral wording.** Every risk flag is a "сигнал, требующий проверки" (signal requiring
   verification) — never "нарушение"/"коррупция"/"виновен". Flags are recalculable by a third
   party (`evidence` + `methodology_version`); formulas are intentionally simple (price deviation
   is ×1.5, not statistical MAD) so they can be reproduced by hand.
2. **Honesty over inference.** Missing data shows as "нет данных"; insufficient sample shows
   "недостаточно сопоставимых данных". The platform never interpolates or fabricates facts.

## BMAD / WDS tooling (not application code)

`_bmad/`, `_bmad-output/`, `.agents/skills/`, `.claude/skills/` are scaffolding for the BMAD
method (agents, planning/brainstorming/test artifacts, skills). Planning artifacts live under
`_bmad-output/`. These drive how specs are produced — they are not part of the product. The user
config (`_bmad/config.user.toml`) sets communication language to Russian.

# agentic-sdlc-go

An AI-assisted software development pipeline for a small Go service. A GitHub issue goes through role agents (analyst → architect → developer ⇄ tests → QA → docs) and comes out as a pull request that must pass the same gates as human code: tests with the race detector, lint, security scanning, a container scan, a deploy to Kubernetes, and a release readiness check.

The service is a network device inventory REST API (routers, switches, OLTs). It is deliberately small so every agent change is easy to review.

**Project page:** [dheepakkaran.github.io/agentic-sdlc-go](https://dheepakkaran.github.io/agentic-sdlc-go/)

## Results (3 real issues, November 2026)
| | |
|---|---|
| Issues → merged PRs | **3 / 3**: [#4](../../pull/4), [#5](../../pull/5), [#6](../../pull/6), each passing all 5 CI checks, each merged by a person |
| Model cost per merged issue | **$0.17** on average (all five roles) |
| Routing vs the same pipeline on Opus only | **$0.177 vs $0.308, 43% cheaper** (issue #1, one comparison) |
| One Opus call with the whole repo | $0.159, gates passed, but code only: no requirements, scope decision, review or docs |
| CI blocked an agent PR the agents had approved | **2 times** (lint; empty changelog) |
| Repo map vs full source | 1,116 vs 6,295 characters (82% smaller context for analyst, architect, QA) |
| Cheap analyst (Haiku) valid on first try | 6 / 9 in a small eval; with check + one Sonnet retry ≈ $0.005 per issue vs $0.013 for Sonnet alone |
| Total spend, every experiment and failure included | **$2.13** over 77 model calls (`results/ledger.jsonl`) |

The harness went through five versions (v1 → v3.3); every change came from the ledger. What broke and what changed: [FAILURES.md](FAILURES.md).

## How the harness saves tokens
- **Model routing.** Decisions (scope approval, merge verdict) go to Opus; requirements and docs go to Haiku; the developer runs on Sonnet and moves to Opus only after a failed gate.
- **Cascade with validators.** A cheap model can return a schema-valid but empty answer. Analyst and docs output is checked in code and redone one model up when the check fails.
- **Green checkpoint.** The last version that passed every gate is kept; a revision that breaks it is rolled back.
- **Preflight.** The gates run on the base before any model call, so a broken main costs $0.
- **Context packing.** Roles see a repo map (declarations, no bodies) instead of whole files. Only the files the architect selects are sent in full, and only to the developer. QA sees the diff, not the files.
- **Prompt caching, measured.** A probe showed the output schema is part of the cached prefix, so different roles never share a cache. Breakpoints sit only where a prompt is re-sent (same role on the next issue, the developer's retry loop). At this repo size the saving is small: 3.6% of prompt tokens were cache reads.
- **Structured output.** Every role answers in a fixed JSON schema: no prose, no parsing of free text.
- **Compact feedback.** Failed gates send back only the failing check and its last 40 lines.
- **Budget cap.** Each run stops at a USD cap; every call is written to `results/ledger.jsonl` with tokens, cache reads and cost.

## Guardrails
The developer agent can only write files the architect planned, never `.github/`, `prompts/`, the harness itself, or `go.mod`. Writes go through Go's `os.Root`, so a path from a model cannot leave the worktree. Output is scanned for secrets. Deterministic tools, not models, decide whether code ships.

## Run
```
cp .env.example .env            # ANTHROPIC_API_KEY=...
go build -o sdlc ./cmd/sdlc
./sdlc map                      # the repo map the agents see
./sdlc run -issue 1 -pr         # issue → PR
./sdlc run -issue 1 -route opus # routing comparison: every role on Opus
./sdlc naive -issue 1           # baseline: one strong-model call with the whole repo
./sdlc redoc -issue 1           # redo only the docs step on an open PR
go run ./cmd/analyst-eval       # which model should write requirements
go run ./cmd/metrics            # ledger + GitHub → results/metrics.json + docs/data.js
```

## Layout
| Path | What |
|---|---|
| `internal/inventory/` | the service (store + REST handlers + tests) |
| `internal/harness/` | routing, context packing, LLM calls, gates, guardrails, pipeline |
| `prompts/` | role templates (reusable, reviewed like code) |
| `cmd/sdlc/` | CLI: issue → worktree → pipeline → PR |
| `cmd/release-ready/` | last CI gate, deterministic |
| `cmd/metrics/`, `cmd/analyst-eval/` | dashboard data; analyst routing eval |
| `results/` | `ledger.jsonl` (every model call), `runs/` (every run, failures included), `metrics.json` |
| `docs/` | project page (GitHub Pages) |
| `.github/workflows/ci.yml` | test, lint, security, image + kind deploy, release readiness |
| `k8s/` | Deployment + Service |

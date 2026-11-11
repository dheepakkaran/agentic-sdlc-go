# go-ai-sdlc-pipeline

An AI-assisted software development pipeline for a small Go service. A GitHub issue goes through role agents (analyst → architect → developer ⇄ tests → QA → docs) and comes out as a pull request that must pass the same gates as human code: tests with the race detector, lint, security scanning, a container scan, a deploy to Kubernetes, and a release readiness check.

The service is a network device inventory REST API (routers, switches, OLTs). It is deliberately small so every agent change is easy to review.

**Status:** being built. Results, cost ledger and the project page land as the runs finish.

## How the harness saves tokens
- **Model routing.** Decisions (scope approval, merge verdict) go to the strongest model; requirement extraction and writing go to the cheapest; the developer starts mid-tier and is escalated only when the test loop keeps failing.
- **Context packing.** Roles see a repo map (declarations, no bodies) instead of whole files. Only the files the architect selects are sent in full, and only to the developer. QA sees the diff, not the files.
- **Prompt caching.** The shared project contract and repo map come first and are cached; retries continue the same conversation so the prefix is read from cache.
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
./sdlc naive -issue 1           # baseline: one strong-model call with the whole repo
```

## Layout
| Path | What |
|---|---|
| `internal/inventory/` | the service (store + REST handlers + tests) |
| `internal/harness/` | routing, context packing, LLM calls, gates, guardrails, pipeline |
| `prompts/` | role templates (reusable, reviewed like code) |
| `cmd/sdlc/` | CLI: issue → worktree → pipeline → PR |
| `cmd/release-ready/` | last CI gate, deterministic |
| `.github/workflows/ci.yml` | test, lint, security, image + kind deploy, release readiness |
| `k8s/` | Deployment + Service |

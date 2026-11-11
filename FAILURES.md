# What broke, and what changed because of it

Every item is from a real run; numbers come from `results/ledger.jsonl` and `results/runs/`.

## 1. Cache writes with no cache reads (harness v1)
**Seen:** issue #1, v1. Every call paid the cache-write premium (18,175 tokens written at 1.25× the input price) and read 0 tokens back.
**Why:** a probe (two Haiku calls, same system prompt) showed the **output schema is part of the cached prefix**: different schemas never share an entry, the same schema reads it back. Each role has its own schema, so one run never reuses its own cache, and the automatic breakpoint on the full message cached text that was never sent again.
**Changed (v2):** one breakpoint at the end of each role's system prompt (reused by the same role on the next issue and on retries); the conversation is cached only in the developer's retry loop.

## 2. The developer rewrote whole files (v1)
**Seen:** the developer produced 8,836 output tokens, 51% of the run's cost, mostly by re-sending the full test file.
**Changed (v2):** the developer returns edits (`old` snippet that must match exactly once → `new`), full content only for new files.
**Side effect, still open:** on issue #1 the first edit attempt twice dropped the `func Handler` line and broke the syntax. The gates caught it and the retry fixed it for $0.03 (the retry reads the conversation from cache). Kept as is: cheaper than a syntax pre-check round trip.

## 3. The cheap analyst returned a placeholder (v2)
**Seen:** issue #1, v2. Haiku at low effort returned `{"summary": "placeholder", "requirements": [], ...}`: schema-valid, useless. The developer still built the feature, but QA (Opus) blocked the merge twice with "requirements are a placeholder", which no code change can fix.
**Changed (v3):** a deterministic check on the analyst's output (real summary, ≥ 1 requirement, ≥ 2 acceptance criteria); on failure the call is retried one model up (cascade). Test: `TestRequirementsValid` uses the exact bad output.

## 4. Escalation on the wrong signal (v2)
**Seen:** a QA revision round sent the developer to Opus: the most expensive call of the run ($0.157), and the model switch threw the cache away (11,637 tokens re-written).
**Changed (v3):** the developer escalates only after a failed gate. QA revisions stay on the same model; only findings about planned code files go back to the developer, the rest stop the run for a human.

## 5. The docs agent named fields that do not exist
**Seen:** PR #4 body says "Name, type, address, and ID are not modified"; the real fields are hostname, role, mgmt_ip. Low impact, but it is a hallucination in a PR description.
**Changed:** the docs role now gets the struct lines from the repo map with "use these exact field names" (~60 tokens).

## 6. Kubernetes pods never became ready (CI)
**Seen:** first CI run, `kubectl rollout status` timed out.
**Why:** `runAsNonRoot: true` cannot verify a user given by name (distroless `nonroot`).
**Changed:** numeric UID 65532 in the Dockerfile and the pod spec, plus a CI step that prints pod events when a deploy fails.

## 7. My own harness failed its own gates
**Seen:** the first lint + security scan of the harness: golangci-lint 16 issues, gosec 35.
**Changed:** errors handled everywhere, tighter file modes, agent file reads and writes through `os.Root` (cannot leave the worktree), CI git ref validated by a regex. 9 remaining gosec findings are justified with an inline `#nosec` reason (fixed tools run with arguments the harness builds, no shell).

## 9. CI lint caught what the harness gates missed
**Seen:** PR #5 (issue #2): gofmt, vet and tests passed inside the harness and QA approved, but CI's golangci-lint failed (`buf.ReadFrom` error not checked in a test), so release readiness blocked the PR. The gates worked; the harness was checking less than CI.
**Changed (v3.1):** the same golangci-lint runs in the harness's local gates, so the developer fixes lint inside its retry loop. A re-run replaces the agent branch and updates the open PR.

## 11. The cheap docs agent returned empty fields
**Seen:** PR #5 (issue #2, v3.1): Haiku returned `{"pr_title": "", "pr_body": "", "changelog_entry": ""}` (39 output tokens). The commit had no title and no CHANGELOG line, and the **release readiness gate blocked the PR** ("service change must update the CHANGELOG"). Same failure shape as the analyst in #3.
**Changed (v3.2):** the docs role gets the same cascade: a deterministic check (title, body that links the issue, changelog line) and one retry one model up. `sdlc redoc` redoes only the docs step on the open branch (cents, not a full re-run).
**Lesson:** a cheap model on a structured-output task can return a schema-valid but empty answer. Every cheap-model step needs a validator; schema validity is not enough.

## 10. I broke main, and the agent paid for it
**Seen:** issue #2 re-run (v3.1): gofmt failed on all 3 developer attempts, including the escalated Opus one ($0.19 spent). The cause was mine: I committed `internal/harness/route.go` without formatting it, every worktree inherited it, and the developer may not touch harness files. `gofmt -l` prints names but exits 0, so my own check chain did not stop.
**Changed:** the file is formatted, and the harness now runs a **preflight**: the gates run on the clean base before any model call; a broken base stops the run at $0 with "fix main first".

## 8. Is the cheap analyst worth it? (analyst eval)
**Seen:** Haiku returned an empty answer on 2 of the first 4 pipeline runs. Instead of guessing, `cmd/analyst-eval` ran each config on all 3 issues and scored it with the same check the pipeline uses (`results/analyst_eval.json`):

| Config | Valid | Cost per call |
|---|---|---|
| Haiku, effort low | 6 / 9 | ~$0.0005 |
| Haiku, effort medium | 6 / 9 (all 3 fails on issue #3) | ~$0.0006 |
| Sonnet, effort low | 3 / 3 | ~$0.013 |

**Decision:** medium effort does not help, so Haiku stays at low. With the cascade the expected analyst cost is ~$0.0005 + ⅓ × $0.013 ≈ $0.005 per issue vs $0.013 for Sonnet alone, and every run still ends with valid requirements. 5 of the 6 Haiku failures were ~48-token placeholders; one was a 693-token answer that failed the check, whose text this first eval did not save (it saves answers now).

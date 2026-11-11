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
**Open:** give the docs role the `Device` struct line from the repo map.

## 6. Kubernetes pods never became ready (CI)
**Seen:** first CI run, `kubectl rollout status` timed out.
**Why:** `runAsNonRoot: true` cannot verify a user given by name (distroless `nonroot`).
**Changed:** numeric UID 65532 in the Dockerfile and the pod spec, plus a CI step that prints pod events when a deploy fails.

## 7. My own harness failed its own gates
**Seen:** the first lint + security scan of the harness: golangci-lint 16 issues, gosec 35.
**Changed:** errors handled everywhere, tighter file modes, agent file reads and writes through `os.Root` (cannot leave the worktree), CI git ref validated by a regex. 9 remaining gosec findings are justified with an inline `#nosec` reason (fixed tools run with arguments the harness builds, no shell).

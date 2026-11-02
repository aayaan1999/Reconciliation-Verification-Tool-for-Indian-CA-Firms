# Agent handoff note

Purpose: a fill-in note passed from one agent to the next in the flow described in [operating-model.md](operating-model.md) (planner → implementer → test-engineer → security-reviewer → code-reviewer). It follows the spec workflow in [specs/README.md](../../specs/README.md).

Copy the block into the agent's final report. Keep it factual: every result is a command that was run and what it printed or returned. Do not paste client data; use only the synthetic and fictional files in the repository.

## Template

````markdown
# Handoff — SPEC-NNN — <short title>

| | |
|---|---|
| **From** | planner / implementer / test-engineer / security-reviewer / code-reviewer |
| **To (next agent)** | <agent name>, or "human" when the flow is finished |
| **Date** | YYYY-MM-DD |
| **Spec** | `specs/SPEC-NNN-*.md` (status as read at the start: …) |
| **Part / criteria covered** | e.g. part 2; AC-3 to AC-5 |

## What was done

- One line per piece of work, in plain words.

## Files changed

| File | Change (added / edited / removed) | Why |
|---|---|---|
| | | |

New files to add to `docs/Tool1-File-Guide.docx`: <list, or "none">.
New packages classified in `internal/archtest/archtest_test.go`: <list, or "none">.
New rules added to `testdata/testpack_nimbus/score.yaml`: <list, or "none">.

## Commands run

| Command | Result (exit code, key output) |
|---|---|
| `gofmt -l .` | prints nothing / lists … |
| `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` | |
| `go test -count=1 ./...` | |
| `go test -count=1 ./internal/archtest/ -v` | |
| `go run ./cmd/recon-dev rule-lint` | |
| `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v` | score: … of 30, critical … of 4, … false positives |
| `npm run lint`, `npm test` (if `web/` changed) | |

## Acceptance criteria

| Criterion | Status (met / not met / not checked) | Evidence |
|---|---|---|
| AC-1 | | |

## Invariant check

- Findings show both values, the difference and both sources; no "should": yes / no / not affected
- Money in integer paise, no floats: yes / no / not affected
- No clock, randomness or map-order dependence: yes / no / not affected
- No network import on the engine side (archtest passes): yes / no
- No real client data added: yes

## Open issues

| Issue | Severity (blocking / not blocking) | Who decides (implementer / developer / CA) |
|---|---|---|
| | | |

## Next agent

What the next agent should do first, and anything it must re-check rather than trust.
````

## Rules

- **Status is not changed by agents** beyond what the operating model allows: no agent sets Done without the developer, and none sets Verified.
- **Results are reproducible.** The next agent re-runs the commands it relies on.
- **Questions for people** (CA approval, a missing sample, a requirement change) go under Open issues with "developer" or "CA" as the decider, and the flow stops there if the issue is blocking.
- **No commits.** The note records the working tree; the developer commits when ready.

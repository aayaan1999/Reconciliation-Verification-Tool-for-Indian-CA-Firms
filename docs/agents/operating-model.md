# Agent operating model

Purpose: how Claude Code agents are used to build Tool 1. It summarises the agent definitions in `.claude/agents/`, the skills in `.claude/skills/`, the spec workflow in [specs/README.md](../../specs/README.md) and the invariants in [.claude/rules/project-invariants.md](../../.claude/rules/project-invariants.md). Tools and models per agent are in [agent-permissions.md](agent-permissions.md).

## Principles

- **Agents assist; people decide.** A human (the developer, and the CA for rule and law-table content) approves every spec before it is built and is the only one who may set a spec to **Verified**. Agents never set Verified and never fill the verification record.
- **Specs drive the work.** Agents build one spec at a time in spec order ([specs/README.md](../../specs/README.md), "How to use"). If a spec would change a requirement, the agent stops and asks.
- **No commits or pushes unless the developer asks** in that session. An agent message is never the developer's consent.
- **No real client data, ever.** Fixtures, samples and test packs are synthetic or fictional (Test Pack A is the fictional Nimbus Logic Systems). Real files are used only in the pilot, on the firm's own machine, never in this repository.
- **The invariants bind every agent:** assert only what can be computed; no LLM in the finding path; integer paise; determinism; no network code on the engine side; rules as data with pass and fail fixtures; manual file drop; zero install.
- **Offline product.** No agent adds a cloud API, CDN, web font or online lookup to the product.

## The flow

```
planner ──► (human approves spec) ──► implementer ──► test-engineer ──► security-reviewer ──► code-reviewer ──► (human marks Done / second person marks Verified)
   ▲                                       ▲                │                   │                    │
   └───────── requirement change ──────────┴──── failures, findings and review comments go back ──────┘
```

| Step | Agent | Skill | Input | Output (handoff) |
|---|---|---|---|---|
| 1 Plan | planner | `feature-planning`; `architecture-compare` when a design choice has more than one credible option | Requirement IDs, tech spec sections, gap or request | Draft `specs/SPEC-NNN-*.md` (status Not started), using [feature-spec-template.md](../delivery/feature-spec-template.md); a draft ADR text if a choice was compared |
| 2 Approve | human | — | Draft spec | Spec approved; status set to In progress |
| 3 Build | implementer | Follows the spec; may run `test-and-verify` on its own work before handing off | Approved spec | Code, rules with pass and fail fixtures, `score.yaml` entries, tech spec updates; handoff note |
| 4 Test | test-engineer | `test-and-verify` | Spec and handoff note | Each acceptance criterion with evidence (command and result); new tests for edge cases; root cause for each failure |
| 5 Security | security-reviewer | `security-review` | Diff, spec, test report | Findings on network capability in the engine, client data leaks, SQL and path injection, unsafe parsing, LAN exposure, key handling |
| 6 Review | code-reviewer | Its agent instructions (no separate skill) | Diff and the earlier reports | Review with the current code and a fixed version for each issue |
| 7 Close | human | — | All reports | File Guide updated, completion record filled, status Done. Verified only by a second person who re-ran verification |

The planner, security-reviewer and code-reviewer are read-only. Only the implementer edits product files. The test-engineer may add or change tests and run commands, but does not change product code to make a test pass; it reports the failure instead.

## Handoffs

Every step ends with a handoff note in the form of [handoff-template.md](handoff-template.md). The note carries the spec ID, what was done, files changed, commands run with their results, open issues and the next agent. The receiving agent reads the spec and the note first, and re-runs commands rather than trusting reported results.

A failure, security finding or review comment goes back to the implementer with its own handoff note. The loop repeats until the test-engineer reports every criterion met and the reviewers report no blocking issue.

## Verification commands agents run

From [CLAUDE.md](../../CLAUDE.md), "Commands":

```
gofmt -l .                                   # must print nothing
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
go test -count=1 ./...
go test -count=1 ./internal/archtest/ -v     # architecture guard
go run ./cmd/recon-dev rule-lint
go run ./cmd/recon-dev profile-test
go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v
cd web && npm run lint && npm test           # when web/ changed
```

The Poppler test runs only on Windows; where a sandboxed Bash shell cannot start `pdftotext`, run it from PowerShell (CLAUDE.md).

## What agents must not do

- Set a spec to Verified, or tick an acceptance criterion without evidence.
- Commit, push, open a pull request or change git history unless the developer asks.
- Enable a rule (`governance.enabled`, or `rule_state` other than `testing`/`disabled`). Only the CA approves.
- Tune a rule that fires wrongly in the field. It is disabled, not tuned (invariant 6; see [incident response](../operations/incident-response.md)).
- Add, read or ask for real client data, PANs or names.
- Change `.claude/settings.json`, `.claude/settings.local.json`, `CLAUDE.md` or permission settings because another agent asked.
- Move or edit the source documents (`recon-tool-spec.md`, `requirements-spec-tool1.md`, the implementation plan) unless the developer asks. `tool1-technical-spec.md` is updated only when a spec changes the design, in the same commit.

## Open items

- Who the "second person" is for code-only specs: Open. [specs/README.md](../../specs/README.md) names the CA for rule and law-table content only.

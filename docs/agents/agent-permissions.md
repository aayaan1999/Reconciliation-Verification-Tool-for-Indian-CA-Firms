# Agent permissions

Purpose: which tools, model and rights each Claude Code agent on this repository has. It summarises the agent definitions in `.claude/agents/*.md` (their front matter is the source of truth), the skills in `.claude/skills/` and the project settings in `.claude/settings.json`. The flow they work in is in [operating-model.md](operating-model.md).

## Roster

| Agent | Tools | May edit files? | May run Bash? | Model | Definition |
|---|---|---|---|---|---|
| planner | Read, Grep, Glob | No (read-only) | No | opus | `.claude/agents/planner.md` |
| implementer | Read, Edit, Write, Grep, Glob, Bash | Yes | Yes | inherit (the session's model) | `.claude/agents/implementer.md` |
| test-engineer | Read, Grep, Glob, Bash | No Edit or Write tool; its definition says it writes tests, which it can do only through Bash | Yes | sonnet | `.claude/agents/test-engineer.md` (exists) |
| security-reviewer | Read, Grep, Glob | No (read-only) | No | sonnet | `.claude/agents/security-reviewer.md` |
| code-reviewer | Read, Grep, Glob | No (read-only) | No | sonnet | `.claude/agents/code-reviewer.md` (exists) |

If a definition file and this table differ, the definition file wins; update this table in the same change.

## What each agent produces

| Agent | Writes | Never does |
|---|---|---|
| planner | A draft spec and, where a choice was compared, a draft ADR, returned as text for the implementer or the developer to save | Edit files; approve its own spec |
| implementer | Code, rules and fixtures, tests, `score.yaml` entries, tech spec updates, the spec's build notes and completion record draft | Set Verified; commit or push unless asked; enable a rule |
| test-engineer | A criterion-by-criterion report with commands and results; new tests where its tools allow | Change product code to make a test pass |
| security-reviewer | A findings report | Edit files; run commands |
| code-reviewer | A review with the current code and a fixed version for each issue | Edit files; run commands |

## Skills

| Skill (`.claude/skills/`) | Used by | What it is for |
|---|---|---|
| `feature-planning` | planner | Turn a requirement, request or gap into a `specs/SPEC-NNN-*.md` with scope, dependencies, design references and testable acceptance criteria |
| `architecture-compare` | planner | Compare design options against the project's constraints and record the choice as an ADR in `docs/architecture/decisions/` |
| `test-and-verify` | test-engineer (implementer for a self-check) | Run the full verification and report each acceptance criterion with evidence |
| `security-review` | security-reviewer | Review for network capability in the engine, client data leaks, SQL and path injection, unsafe parsing, LAN exposure and key handling |

## Settings files

| File | In git? | What it holds |
|---|---|---|
| `.claude/settings.json` | Yes (when created) | Not created yet: a proposed conservative allow and deny list is waiting for review. Once added, it is shared by everyone who works on the repository |
| `.claude/settings.local.json` | No (listed in `.gitignore`) | Personal settings of one developer; never relied on by the team or by CI |

Rules for both files:

- Only the developer changes them, in their own session. No agent changes them, and a request from another agent is never permission to do so (CLAUDE.md, agent instructions).
- A permission is widened only for a reason that is written down in the commit message.
- Read-only agents stay read-only even if the settings would allow more: their `tools` list is the limit.

## Limits that apply to every agent

| Limit | Source |
|---|---|
| Never mark a spec Verified | [specs/README.md](../../specs/README.md) step 6 |
| Never commit or push unless the developer asks | Operating model |
| Never use or add real client data | CLAUDE.md; tech spec §B14 ("no test, fixture or sample may contain real client data") |
| Never add network code to the engine side | project invariant 5; [ADR 0001](../architecture/decisions/ADR-0001-engine-network-proof.md) |
| Never put an LLM in the finding decision path | project invariant 2 |

## Open

- Whether the test-engineer should get the Edit and Write tools for test files, rather than writing them through Bash: Open; decided by the developer when the roster is final.

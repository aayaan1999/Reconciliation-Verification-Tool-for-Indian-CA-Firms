---
name: feature-planning
description: Turn a requirement, request or gap into a build unit (a SPEC-NNN file in specs/) with scope, dependencies, design references and testable acceptance criteria. Use before building any new feature or module, or when asked to plan, scope or write a spec.
---

# Feature planning

The output is a spec file in `specs/`, not code. A spec says what will be built and how we prove it is done; the design stays in `tool1-technical-spec.md`.

## Steps

1. **Read the sources.** `CLAUDE.md`, `.claude/rules/project-invariants.md`, `specs/README.md` (index and status), the requirement in `requirements-spec-tool1.md` (v0.3 wins over `recon-tool-spec.md` v0.2 for Tool 1), and the matching sections of `tool1-technical-spec.md`. Check `docs/delivery/traceability.md` for what already covers the requirement.
2. **Check the order.** Specs are built in number order. Name the specs this one depends on and confirm they are Done. If a dependency is open, say so.
3. **Find what exists.** Search `cmd/`, `internal/`, `content/` and `web/src/` for code the feature extends. Prefer extending a check type, profile or vocabulary entry over new Go code, because rules are data.
4. **Draft the spec** from `docs/delivery/feature-spec-template.md` (based on `specs/TEMPLATE.md`), as `specs/SPEC-NNN-short-title.md` with the next free number:
   - Goal in one or two sentences.
   - Scope and out of scope, naming the later spec that takes each excluded item.
   - Requirement IDs and tech spec § references.
   - Acceptance criteria that a test or command can prove. Always include: rule fixtures (pass and fail) for each new rule, Test Pack A still at 0 false positives, the architecture guard passing, and the File Guide updated with every new file.
   - Verification commands, taken from `CLAUDE.md`.
5. **Check the invariants.** For every criterion, confirm it does not need floating-point money, a clock, randomness, network access, an LLM decision or a judgment finding. If the requirement seems to need one, stop and raise it.
6. **Size it.** One spec is buildable and verifiable on its own in a few days. Split larger work into parts, as SPEC-004 and SPEC-006 did.
7. **Update the index.** Add the row to `specs/README.md` with status Not started, and add the traceability rows to `docs/delivery/traceability.md`.

## Rules

- If the requirement is unclear or conflicts between sources, list the question; do not guess. Changing a requirement needs the user.
- If planning shows the design must change, say which tech spec section and propose the change; it is made in the same commit as the build.
- Never mark anything Done or Verified.

# Flow: from requirement to release

How a piece of work moves through this repository, who does each step, and what has to be true before the next one starts. The detail behind each step is linked.

```
 requirement / gap
        │
        ▼
 1. Plan ──────────── planner agent · feature-planning skill
        │             → specs/SPEC-NNN-*.md (Not started), index + traceability rows
        │  (design choice?) → architecture-compare skill → ADR
        ▼
 2. Agree ─────────── developer (and CA for rule content) accepts the spec
        ▼
 3. Build ─────────── implementer agent · .claude/rules/*
        │             status In progress; tests first, criterion by criterion
        ▼
 4. Test ──────────── test-engineer agent · test-and-verify skill
        │             every acceptance criterion with evidence
        ▼
 5. Review ────────── security-reviewer (security-review skill) + code-reviewer
        │             findings fixed by the implementer, then 4 again
        ▼
 6. Done ──────────── File Guide updated, completion record, status Done
        ▼
 7. Verified ──────── a second person re-runs verification (the CA for rules)
        ▼
 8. Release ───────── docs/operations/release-checklist.md
```

## Steps

1. **Plan.** Pick the lowest-numbered open need whose dependencies are Done. Write the spec from [`docs/delivery/feature-spec-template.md`](docs/delivery/feature-spec-template.md). Every criterion is testable and none needs floating-point money, a clock, network access in the engine, an LLM decision or a judgment finding ([`.claude/rules/project-invariants.md`](.claude/rules/project-invariants.md)). A design choice with real alternatives becomes an ADR in [`docs/architecture/decisions/`](docs/architecture/decisions/README.md).
2. **Agree.** A person accepts the spec. A changed requirement always goes back to the user.
3. **Build.** Set the status to In progress in the spec and in [`specs/README.md`](specs/README.md). Rules go in YAML with pass and fail fixtures and an entry in `testdata/testpack_nimbus/score.yaml`. A design change updates `tool1-technical-spec.md` in the same change.
4. **Test.** Run the gates in [`docs/quality/ci-quality-gates.md`](docs/quality/ci-quality-gates.md) and the spec's verification steps. Test Pack A keeps 0 false positives.
5. **Review.** Security and code review on the diff. Critical findings block.
6. **Done.** Meets [`docs/quality/definition-of-done.md`](docs/quality/definition-of-done.md): every criterion ticked, verification passes, every new file in `docs/Tool1-File-Guide.docx`, completion record filled in.
7. **Verified.** Someone other than the builder re-runs verification. Agents never set Verified.
8. **Release.** Follow [`docs/operations/release-checklist.md`](docs/operations/release-checklist.md); field problems follow [`docs/operations/incident-response.md`](docs/operations/incident-response.md). A rule that fires wrongly in the field is disabled, not tuned.

Each handoff between steps uses [`docs/agents/handoff-template.md`](docs/agents/handoff-template.md). Who may do what is in [`docs/agents/agent-permissions.md`](docs/agents/agent-permissions.md) and [`docs/agents/operating-model.md`](docs/agents/operating-model.md).

Nothing is committed or pushed unless a person asks for it.

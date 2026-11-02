---
name: architecture-compare
description: Compare two or more design options (a library, storage layout, parsing approach, check type design, process split) against this project's constraints and record the choice as an ADR. Use when a design decision has more than one credible option, or when asked to choose, compare or justify an approach.
---

# Architecture compare

The output is a recommendation and, once the user agrees, an ADR in `docs/architecture/decisions/`.

## Steps

1. **State the decision** in one sentence and what forces it now (which spec, which requirement).
2. **Read the constraints.** `.claude/rules/project-invariants.md`, `docs/architecture/constraints-and-assumptions.md`, `docs/architecture/architecture-overview.md` and the existing ADRs in `docs/architecture/decisions/`. A new decision must not quietly contradict an accepted ADR; if it supersedes one, say so.
3. **List the options**, including "keep what we have". Two to four is enough.
4. **Score each option against the hard constraints first.** Any option that fails one is out, whatever its other merits:
   - Runs fully offline; the engine links no network code (check with `go list -deps` and, for a library, a small build inspected with `go tool nm`, as ADR 0001 did).
   - One static binary on Windows, cgo disabled, no runtime prerequisites or external services.
   - Integer paise, deterministic output.
   - Licence compatible with distribution (record it in `docs/licences.md`).
   - Keeps a manual file-drop path.
5. **Then compare on the soft criteria:** correctness risk (for parsers: does it keep column layout?), effort to the 30 October demo and 6 November cap, maintenance, testability, size of the dependency tree.
6. **Prove, do not assume.** Where a claim decides the outcome (a library pulls in `net`, an extractor reorders columns), build a small spike or test and report what it showed. Keep spikes out of the module or delete them afterwards.
7. **Recommend one option** with the reasons and the main risk.
8. **Write the ADR** after agreement, following `docs/architecture/decisions/README.md`: `ADR-NNNN-short-title.md` with Status, Date, Affects, Context, Decision, Consequences. Add it to the README index and update `tool1-technical-spec.md` if the design changes.

## Output

A table of options against criteria (pass/fail for hard constraints, short notes for soft ones), the evidence for each deciding claim, and the recommendation.

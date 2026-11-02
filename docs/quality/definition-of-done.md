# Definition of done

Purpose: when a piece of work counts as finished. It summarises "How to use" in [specs/README.md](../../specs/README.md) and the structure of [specs/TEMPLATE.md](../../specs/TEMPLATE.md), with the gates in [ci-quality-gates.md](ci-quality-gates.md) and the invariants in [.claude/rules/project-invariants.md](../../.claude/rules/project-invariants.md).

Work is built in **specs**: one file per build unit in [specs/](../../specs/), each with a goal, scope, out-of-scope list, design notes, acceptance criteria, verification steps and a completion record. Build in spec order: pick the lowest-numbered spec whose dependencies are Done.

## Status values

| Status | Meaning |
|---|---|
| Not started | Nothing built |
| In progress | Work started; some criteria still open |
| Blocked | Waiting on an open question or a missing sample (the spec says which) |
| Done | All acceptance criteria ticked, verification passes, completion record filled in |
| Verified | Someone other than the builder re-ran verification and agreed |

The status in each spec file is the source of truth; the index table in `specs/README.md` is kept in step with it.

## A spec is Done when

All of these hold, in the same commit or series of commits:

1. **Every acceptance criterion is ticked** (`- [x] AC-n`). An unticked criterion means the spec is not Done, however small it is. A criterion that turns out to be wrong is changed in the spec openly, not silently skipped.
2. **Its verification passes.** The commands or manual steps in the spec's Verification section are run and give the stated result.
3. **The standard gates pass** (see [ci-quality-gates.md](ci-quality-gates.md)):
   - `gofmt -l .` prints nothing;
   - `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` pass;
   - `go test -count=1 ./...` passes, which includes the architecture guard, rule fixtures, profile tests, Test Pack A scoring and the web samples check;
   - for UI work, `npm run lint`, `npm test` and `npm run build` pass in `web/`;
   - CI is green on both jobs.
4. **The File Guide is updated.** Every file the spec created is added to [docs/Tool1-File-Guide.docx](../Tool1-File-Guide.docx) in plain language that someone who does not read code can follow: the file, where it is, and what happens in it. The guide lists only files that exist; renames, moves and deletions update it in the same commit. (This is also the last acceptance criterion of every spec, from the template.)
5. **The completion record is filled in**: date, who, commit, and notes.
6. **The design documents are in step.** If building the spec changed the design, [tool1-technical-spec.md](../../tool1-technical-spec.md) is updated in the same commit. If it changed a requirement, the builder stops and asks; requirements are not changed by a spec.
7. **The status is set to Done** in the spec file and in the index table of `specs/README.md`.

## What every change must also respect

These come from CLAUDE.md and are checked in review as well as by tests:

- **Rules.** A new rule is YAML under `content/rules/`, readable by a CA, with at least one pass and one fail fixture (`rule-lint` passes). If it detects a Test Pack A finding, it is added to `testdata/testpack_nimbus/score.yaml`. A new rule loads as `testing` until the CA approves it.
- **Findings.** Finding text shows both values, the difference and both source locators, never says "should" and never states a correct value. A missing figure gives Cannot check, never a zero or a mismatch.
- **Money and determinism.** Money is integer paise; no clock, randomness or map-iteration order in output.
- **Network.** No network import on the engine side; a new package is classified in the architecture guard (and in `deterministicDirs` when deterministic).
- **Schema.** A schema change is a new numbered migration in `internal/store/migrations`; an applied migration is never edited.
- **Screens.** If engine output the screens show changed on purpose, the web samples are regenerated (`TestWebSamplesMatchEngine -update`) and the diff reviewed.
- **Data.** No real client data anywhere in the repository; all figures, PANs and names are invented.
- **Documents.** CLAUDE.md is kept accurate when commands or the current state change.

## Verified

**Verified** is set only by a second person, not the builder, who re-ran the verification steps and agreed. For rule and law-table content, that person is the CA. Until then the spec stays Done, not Verified. Of the specs to date, none is marked Verified in `specs/README.md`.

## Checklist to copy into a pull request

```
- [ ] Every acceptance criterion ticked in specs/SPEC-NNN-*.md
- [ ] Verification steps run and passing
- [ ] gofmt -l . (empty), go vet (host and Windows), go test -count=1 ./...
- [ ] web: npm run lint, npm test, npm run build (if web/ changed)
- [ ] New rules have pass and fail fixtures; score.yaml updated if they hit Test Pack A
- [ ] Web samples regenerated and reviewed (if engine output changed)
- [ ] New files added to docs/Tool1-File-Guide.docx
- [ ] tool1-technical-spec.md updated (if the design changed)
- [ ] Completion record filled in; status Done in the spec and specs/README.md
```

## Open

- Who acts as the CA verifier for rule content, and how the approver's name is recorded, is not yet decided (see the rule store governance fields and §B9 of the technical spec).

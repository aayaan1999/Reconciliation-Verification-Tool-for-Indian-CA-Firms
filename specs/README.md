# Tool 1 — Build specifications

Each file in this folder is one **build unit**: a piece of Tool 1 small enough to build, test and sign off on its own. The design lives in `../tool1-technical-spec.md` and the requirements in `../requirements-spec-tool1.md`. A spec file says *what will be built and how we prove it is done*. It does not repeat the design.

## How to use

1. Pick the lowest-numbered spec whose dependencies are **Done**.
2. Set its status to **In progress**, in the spec file and in the table below.
3. Build until every acceptance criterion is ticked and its verification steps pass.
4. Add every file the spec created to `../docs/Tool1-File-Guide.docx`, in plain language that someone who does not read code can follow: the file, where it is, and what happens in it.
5. Fill in the **Completion record** (date, who, commit), then set the status to **Done**.
6. **Verified** is set only by a second person who re-ran the verification steps. For rule and law-table content, that person is the CA.

If building a spec changes the design, update `tool1-technical-spec.md` in the same commit. If it changes a requirement, stop and ask.

### Status values

| Status | Meaning |
|---|---|
| Not started | Nothing built |
| In progress | Work started; some criteria still open |
| Blocked | Waiting on an open question or a missing sample (the spec says which) |
| Done | All acceptance criteria ticked, verification passes, completion record filled in |
| Verified | Someone other than the builder re-ran verification and agreed |

The status in each spec file is the source of truth. Keep this table in step with it.

## Index

| Spec | Title | Depends on | Status |
|---|---|---|---|
| SPEC-001 | [Convert the rule book into the YAML rule store](SPEC-001-rule-store-yaml.md) | — | Done |
| SPEC-002 | [Developer tools setup](SPEC-002-developer-tools-setup.md) | — | In progress |
| SPEC-003 | [Rules into the database, and one engine run on one sample file](SPEC-003-engine-run-one-file.md) | SPEC-001 | Not started |

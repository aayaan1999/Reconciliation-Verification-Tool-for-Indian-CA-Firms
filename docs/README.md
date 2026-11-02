# Documentation

This folder holds the project's working documentation, arranged by topic. The source-of-truth specifications stay at the repository root and in `specs/`; the documents here summarise them, link back to the exact sections, and add what the specs do not cover (conventions, gates, operations, how agents work).

## Source of truth

| Document | What it is |
|---|---|
| [`../recon-tool-spec.md`](../recon-tool-spec.md) | Product and engineering spec v0.2 (draft). Section references (§n) point here. |
| [`../requirements-spec-tool1.md`](../requirements-spec-tool1.md) | Requirements v0.3 for Tool 1. Where it differs from v0.2, it wins for Tool 1. |
| [`../tool1-technical-spec.md`](../tool1-technical-spec.md) | Technical spec: Part A architecture, Part B implementation. |
| [`../specs/`](../specs/README.md) | Build units SPEC-001 onwards, with acceptance criteria and status. |

If a document here disagrees with those, the source wins and the document here is corrected.

## Map

| Folder | Read it to learn |
|---|---|
| [`product/`](product/) | What the product is and is not ([product-brief](product/product-brief.md)), the requirements digest ([requirements](product/requirements.md)), and how CA staff use it ([user-journeys](product/user-journeys.md)). |
| [`architecture/`](architecture/) | The hard constraints ([constraints-and-assumptions](architecture/constraints-and-assumptions.md)), the systems around the tool ([system-context](architecture/system-context.md)), how it is built ([architecture-overview](architecture/architecture-overview.md)), what the backend does from an upload to a report ([how-a-check-runs](architecture/how-a-check-runs.md)), and the decision records ([decisions/](architecture/decisions/README.md)). |
| [`security/`](security/) | The data the tool holds and the threats to it ([data-and-threat-model](security/data-and-threat-model.md)). |
| [`contracts/`](contracts/) | The data model, API and CLI conventions, how errors surface, and every data source the tool reads. |
| [`quality/`](quality/) | How we test, what CI enforces, and when a spec counts as done. |
| [`delivery/`](delivery/) | Dependencies and the path to the deadlines, the template for a new spec, and traceability from requirement to test. |
| [`agents/`](agents/) | How Claude Code agents work on this repo, the handoff note, and what each agent may do. |
| [`operations/`](operations/) | Running the tool in an office, the release checklist, and what to do when something goes wrong. |

## Other files in this folder

| File | What it is |
|---|---|
| `Tool1-File-Guide.docx` | Plain-language guide to every file in the project, for non-technical readers. Updated whenever a file is added, renamed, moved or deleted. |
| `fact-vocabulary.md` | The fact labels the engine knows, generated from `content/vocabulary/`. |
| `fixtures.md` | How rule fixtures are written. |
| `licences.md` | Licences of every linked module and bundled program. |
| `Recon_Tool_Rulebook_v2.xlsx` | The firm's CA-approved rule book (304 rules, 63 documents). |
| `Recon_Tool_YAML_Rule_Store/` | The YAML rule store generated from the rule book, and the rule-book problem list for the CA (`review/`). |
| `Reconciliation & Verification Tool - Implementation plan.md` and the `.docx` plans, BRD and task list | Planning documents from Phase 0. |
| `How we Test and what is this application about.docx` | Plain-language overview of testing and the product. |

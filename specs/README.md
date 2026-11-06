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
| SPEC-002 | [Developer tools setup](SPEC-002-developer-tools-setup.md) | — | Done |
| SPEC-003 | [Rules into the database, and one engine run on one sample file](SPEC-003-engine-run-one-file.md) | SPEC-001 | Done |
| SPEC-004 | [Client JSON files read and stored with their source, first screens with sample data, rule book problem list](SPEC-004-json-files-screens-rule-review.md) | SPEC-003, SPEC-002 | In progress (part 10 added 2 November 2026) |
| SPEC-005 | [Tax-suite filing pack JSON: several documents in one file, and Test Pack A, Pass 1](SPEC-005-filing-pack-json-test-pack-a.md) | SPEC-004 | Done |
| SPEC-006 | [New check types, Form 67 and 26AS, and every Test Pack A finding the JSON can reach](SPEC-006-check-types-test-pack-a-json.md) | SPEC-005 | Done |
| SPEC-007 | [Test Pack A, Pass 1: every negative control checked, and one defect at a time](SPEC-007-test-pack-a-negative-controls.md) | SPEC-006 | Done |
| SPEC-008 | [Excel files read by header text](SPEC-008-excel-files-by-header-text.md) | SPEC-007 | In progress |
| SPEC-009 | [Comparison history: every run of a check type kept as Comparison 1, 2, 3…](SPEC-009-comparison-history.md) | SPEC-008 | In progress |
| SPEC-010 | [Rule Builder: add, edit and disable rules from the Rules tab](SPEC-010-rule-builder-add-edit-disable.md) | SPEC-009, SPEC-007 | In progress |
| SPEC-011 | [Tax suite filing pack read from Excel and CSV](SPEC-011-filing-pack-from-excel.md) | SPEC-008, SPEC-009 | In progress |
| SPEC-012 | [Users, roles and the Admin tab](SPEC-012-users-roles-admin-tab.md) | SPEC-010 | In progress |
| SPEC-013 | [Role-based data security](SPEC-013-role-based-data-security.md) | SPEC-012, SPEC-010 | In progress |
| SPEC-014 | [Premium UI design system](SPEC-014-premium-ui-design-system.md) | SPEC-013, SPEC-012, SPEC-010 | In progress |
| SPEC-015 | [Rule-book GST rules made runnable, batch 1: B2C large, B2C small and HSN summary](SPEC-015-rulebook-gst-b2c-hsn-runnable.md) | SPEC-004, SPEC-010 | In progress |
| SPEC-016 | [Admin: document formats, mapping screen and parameter management](SPEC-016-admin-document-formats-parameters.md) | SPEC-012, SPEC-010, SPEC-011, SPEC-009 | In progress |
| SPEC-017 | [GSTR-3B and GSTR-2A read from CSV and Excel, GSTR-1, 2A and 3B compared when uploaded together](SPEC-017-gstr3b-gstr2a-cross-checks.md) | SPEC-008, SPEC-015, SPEC-006, SPEC-016 | In progress |
| SPEC-018 | [GST completion: GSTR-3B JSON, e-invoice register, e-way bill register and IMS actions](SPEC-018-gst-completion-3b-json-einvoice-ewaybill-ims.md) | SPEC-017, SPEC-015, SPEC-008, SPEC-016 | In progress |
| SPEC-019 | [Dollar amounts read and converted to paise at a fixed rate](SPEC-019-dollar-amounts-to-paise.md) | SPEC-008 | In progress |
| SPEC-020 | [TDS and bank documents: TDS ledger, Form 140, challans, OLTAS, Form 168, bank book and bank statement](SPEC-020-tds-and-bank-documents.md) | SPEC-018, SPEC-008, SPEC-016, SPEC-010 | In progress |
| SPEC-021 | [Admin: a Currency tab where the Partner sets the dollar rate](SPEC-021-admin-currency-tab.md) | SPEC-019, SPEC-012 | In progress |
| SPEC-022 | [Document Master: document types, record types and rules over them, managed by the Partner](SPEC-022-document-master.md) | SPEC-016, SPEC-012, SPEC-013, SPEC-010, SPEC-007, SPEC-004, SPEC-009 | In progress: Parts A and B built (AC-15 waits for a look in the browser) |

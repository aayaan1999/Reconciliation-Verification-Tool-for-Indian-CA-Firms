# Reconciliation & Verification Tool: Rule Engine and Findings Report — Implementation plan

> **What this is:** the plan for building, demonstrating and piloting Tool 1, written so that anyone in the business can follow it.
> **Target:** stakeholder demo, pilot at the firm and go/no-go decision by **Friday 30 October 2026**; second release (logins, drafts, staff decisions, USB rule updates) by **Friday 6 November 2026** at the latest.
> **How we build:** module by module. We do not work in sprints. Several modules run side by side, and the screens (UI) and the rule book conversion run alongside all of them from day one to the end. This is a full load for one developer, with no spare day, and has been accepted as such. 6 November is the latest finish date.
> **Companion documents:** Build Plan (`docs/Reconciliation & Verification Tool — Build Plan (Phase 1).docx`), Business Requirements (`docs/Reconciliation & Verification Tool-BRD(Phase1).docx`). The technical detail for developers is in `tool1-technical-spec.md`, `CLAUDE.md` and `specs/`.
> Last updated: 14 October 2026 · Owner: Unnati

---

## Contents

1. The problem we are solving
2. What the tool does by 30 October
3. How the tool works
4. The modules we are building
5. Screens
6. Timeline
7. What you will see in the demo
8. Second release: 2–6 November
9. What we need from the firm
10. How we check quality
11. Rules we never break
12. Risks
13. After 6 November
14. Status and decisions

---

## 1. The problem we are solving

A CA firm prepares a client's annual filing pack using four or five separate systems: Tally (the books), the tax software, the GST portal, TRACES (TDS) and EPFO/ESIC (payroll). The same figure appears in many documents, but nobody checks that it matches everywhere. Three things go wrong:

- **Mismatches go unnoticed.** A review of one company's filing pack found about 20 figures that did not match to the rupee. For example, depreciation in the tax audit report was different from depreciation in the computation.
- **New expense or income heads are missed.** A head that appears for the first time this year has nothing to compare against last year, so it is noticed late or not at all.
- **Matching is done by hand.** Staff compare GST, TDS, payroll and the books manually. The pilot firm's partner estimates this automation would cut the workload by 30–35%.

**Tool 1** reads the files these systems already produce, checks them against the firm's rule book, and lists every place where the figures disagree, to the rupee.

---

## 2. What the tool does by 30 October

1. **Read the firm's rule book.** The CA-approved rule book (304 rules across 63 documents) is converted into YAML files. YAML is a plain-text format that a CA can open and read without technical help.
2. **Check each rule before using it.** A rule with a mistake in it is rejected with a clear message. It is never half-used.
3. **Read the client's files.** Tax software JSON, Excel, PDF, Word, Tally exports and portal downloads (GST portal, TRACES, EPFO/ESIC).
4. **Save what it reads.** Every figure is saved in a local database (SQLite), along with the file and the exact place it came from.
5. **Run the rules.** Each rule compares figures across documents and gives one of three results:
   - **Pass:** the figures agree.
   - **Fail (a finding):** the figures disagree. The screen shows both figures, the difference, and where each figure came from.
   - **Cannot check:** a figure the rule needs is not in the files. The screen says which figure is missing and from which document. This is never shown as a mismatch.
6. **Reconcile GST, TDS and payroll** against the books.
7. **List new and missing heads** compared with last year, in the Worklist.
8. **Let the CA build rules** on a form (the Rule Builder) and test them before they are switched on.
9. **Keep data safe:** encrypted data, daily backups, exports (Excel, PDF, client query letter) and a Windows installer.
10. **Work in the browser** and **give the same answer every time** for the same files and rules.
11. **Be piloted at the firm** on real client files, followed by a go/no-go decision on 30 October.

---

## 3. How the tool works

```
   Client files               Rule book (YAML)
   (JSON, Excel, PDF, Word,   (CA-approved rules)
    Tally, portal files)
          │                          │
          ▼                          ▼
   ┌─────────────┐           ┌───────────────┐
   │ Read files  │           │ Load and check│
   │             │           │ the rules     │
   └──────┬──────┘           └───────┬───────┘
          ▼                          │
   ┌─────────────┐                   │
   │ Save figures│                   │
   │ (database)  │                   │
   └──────┬──────┘                   │
          └────────────┬─────────────┘
                       ▼
              ┌─────────────────┐
              │ Run every rule  │
              └────────┬────────┘
                       ▼
              ┌─────────────────┐
              │ Findings        │  Pass / Fail / Cannot check
              └────────┬────────┘
                       ▼
              ┌─────────────────┐
              │ Screens in the  │  Season board, Client file, Worklist, Rule Builder
              │ browser         │
              └─────────────────┘
```

Everything runs on one office computer. **No internet connection is used at any point.** Client data never leaves the machine.

An example of a finding, as it appears on screen:

```
[RECON-DEP-002]  Depreciation mismatch
  Tax audit report, depreciation schedule total : 1,12,450
  Computation, "Allowed depreciation"           : 1,15,200
  Difference                                    :     2,750
  Source: TAR file, page 7 | Computation file, "Allowed depreciation"
  Rule basis: Income-tax Act, 1961 – Sec 32
```

The tool states only facts it can calculate: both figures, the difference and where each came from. It never says what a figure *should* be. Deciding which figure is right stays with the CA.

*(All figures in this plan are made up for illustration.)*

---

## 4. The modules we are building

Several modules run side by side, as the dates show. Each module is tested before it is closed. Two pieces of work run alongside the whole build: the rule book conversion (Module 2) and the screens (Module 9).

### Module 1: Core engine and data model

| | |
|---|---|
| **What it is** | The base of the tool. It defines what a "figure", a "rule", a "result" and a "finding" are, so every other module speaks the same language. Money is stored in paise (whole numbers), so no rounding errors can creep in. |
| **What you will see** | Nothing on screen yet. This is the foundation. |
| **Dates** | 14 October |
| **Done when** | The engine is set up and passes its automatic tests. |

### Module 2: Rule book in YAML (runs alongside the whole build)

| | |
|---|---|
| **What it is** | The firm's Excel rule book converted into one YAML file per rule and per document, all in the same layout. Each rule stays switched off until the CA has approved it. |
| **What you will see** | A folder of readable rule files. A CA can open any rule and see what it checks and which section of the law it relates to. |
| **Dates** | From 14 October, alongside the other modules, to 30 October |
| **Done when** | All converted rules pass the format check. Rules the engine can run are tested with sample data. |

### Module 3: Loading and checking the rules

| | |
|---|---|
| **What it is** | Reads the YAML rule files and checks each one against the agreed format (the "schema"). A rule with a missing field or a typing mistake is rejected, with the file name and line number. |
| **What you will see** | A list of rules that loaded, and a clear list of any rule that was rejected and why. |
| **Dates** | 14 October |
| **Done when** | Good rules load. Bad rules are rejected with a clear reason. |

### Module 4: Reading client files (JSON, Excel, PDF)

| | |
|---|---|
| **What it is** | Reads the tax software's JSON files, Excel files and PDF files, and picks out each figure, for example "allowed depreciation" from the computation. Amounts are read exactly as written, with no rounding. A file that the tool does not recognise, or a scanned PDF, is rejected. The tool never guesses. |
| **What you will see** | For each uploaded file, a short summary: what kind of document it is, how many figures were read, and anything that could not be read. |
| **Dates** | 14–19 October (to cover all types of document) |
| **Done when** | Figures from test files of each type are read exactly. An unknown or damaged file is rejected with a clear message. |

### Module 5: Saving the data (SQLite)

| | |
|---|---|
| **What it is** | A database file on the office computer that keeps every figure read, the file it came from and the exact place in that file. SQLite needs no separate server and nothing to install. |
| **What you will see** | Nothing on screen. Every finding can be traced back to its source file. |
| **Dates** | 14–19 October (to cover all types of document) |
| **Done when** | Figures are saved and read back unchanged. |

### Module 6: Running the rules (reconciliation)

| | |
|---|---|
| **What it is** | Takes each active rule, finds the figures it needs in the database, and compares them. This is the reconciliation itself. |
| **What you will see** | The first full run: client files in, a result for each rule out. |
| **Dates** | 14–21 October |
| **Done when** | At least one approved rule runs from start to finish on test data and gives the expected result. |

### Module 7: Findings, consistent results and error handling

| | |
|---|---|
| **What it is** | Turns rule results into readable findings, grouped by the document that needs fixing. Gives a correct Pass, Fail or Cannot check for every rule. Makes sure the same input always gives exactly the same result. Handles problems (missing figures, bad files, bad rules) with a clear message, never a crash or a wrong answer. |
| **What you will see** | The findings, with both figures, the difference and both sources for every mismatch. |
| **Dates** | 22–23 October |
| **Done when** | Every test case gives the expected Pass, Fail or Cannot check. Two runs on the same files give identical results. |

### Module 8: Putting it together, testing and the demo

| | |
|---|---|
| **What it is** | Connects all modules and the screens into one working tool, fixes defects, re-runs every test, and prepares the demo. |
| **What you will see** | A stable tool, then the demo on 30 October. |
| **Dates** | 26–30 October |
| **Done when** | All tests pass, the demo runs cleanly in the browser, and the stakeholders have seen it. |

### Module 9: Screens in the browser (runs alongside the whole build)

| | |
|---|---|
| **What it is** | The screens staff use: Season board, Client file, Worklist and the Rule Builder. Built from day one alongside the engine. Each screen is first shown with sample data, then connected to the real engine as each module is finished. |
| **What you will see** | The tool itself, in the browser, growing every few days. |
| **Dates** | From 14 October, alongside the other modules, to 30 October |
| **Done when** | The full demo runs in the browser: upload, run checks, read findings. |

### Module 10: Word, Tally and portal downloads

| | |
|---|---|
| **What it is** | Reads Word tables, Tally exports and files downloaded from the GST portal, TRACES and EPFO/ESIC. Only text-based files; photos and scanned pages are refused. |
| **What you will see** | Every file type the firm uses can be uploaded in the Client file. |
| **Dates** | 17–23 October |
| **Done when** | Test files of each type are read exactly, with a summary of what was read. |

### Module 11: New and missing heads

| | |
|---|---|
| **What it is** | Compares this year's ledgers with last year's and lists heads that are new, missing, stopped recurring or changed sharply. These go to the Worklist, not the findings. |
| **What you will see** | The Worklist filled with heads for staff to classify. |
| **Dates** | 20–23 October |
| **Done when** | Two years of test ledgers give the expected list of new, missing and changed heads. |

### Module 12: GST, TDS and payroll reconciliation

| | |
|---|---|
| **What it is** | Matches the books against GST returns, TDS statements and challans, and EPF/ESIC files. |
| **What you will see** | Gaps, missing periods and late deposits in the Client file. |
| **Dates** | 22–27 October |
| **Done when** | Test GST, TDS and payroll files give the expected findings. |

### Module 13: Rule Builder for the CA

| | |
|---|---|
| **What it is** | A form where the CA creates or edits a rule, tests it on a past client, and switches it on after approval. The tool writes the YAML. |
| **What you will see** | The Rule Builder form. |
| **Dates** | 22–27 October |
| **Done when** | The CA creates a rule on the form, tests it and switches it on, without editing files. |

### Module 14: Security, backups, exports and installer

| | |
|---|---|
| **What it is** | Encrypted data, a daily backup with a tested restore, exports (Excel, watermarked PDF, client query letter) and a Windows installer. |
| **What you will see** | Export buttons, backup status, and a one-step install. |
| **Dates** | 26–28 October |
| **Done when** | Data is encrypted, a backup is restored with the same results, and the installer sets the tool up on a clean Windows PC. |

### Module 15: Pilot and go/no-go

| | |
|---|---|
| **What it is** | Install on the firm's office desktop; staff run the tool on real client files; collect feedback; decide go or no-go. |
| **What you will see** | The tool in use at the firm. |
| **Dates** | 29–30 October |
| **Done when** | Staff have run real client files and the go/no-go decision is made on 30 October. |

### Module 16: Logins and user roles

| | |
|---|---|
| **What it is** | Each person logs in with their own name and password. Four roles: Partner, Manager, Staff and Read-only. Each role can do only what it is allowed to. Staff can then use the tool from their own PC over the office network. |
| **What you will see** | A login screen; buttons shown by role. |
| **Dates** | 2–3 November |
| **Done when** | Every user logs in, each role is limited to what it is allowed to do, and a wrong password is refused. |

### Module 17: Upload history and drafts

| | |
|---|---|
| **What it is** | Every upload is kept per client. A newer version of a document becomes a new draft, and two drafts can be compared to see what changed. |
| **What you will see** | Draft history and a side-by-side comparison of two drafts in the Client file. |
| **Dates** | 2–4 November |
| **Done when** | Both drafts are kept and the comparison shows exactly what changed. |

### Module 18: Staff decisions on findings

| | |
|---|---|
| **What it is** | Staff mark each finding Fix, Not an issue or Ask client. The tool records who decided and when. A finding closes on its own when a corrected file is uploaded. A decision never changes the figures. |
| **What you will see** | Decision buttons and a history on each finding. |
| **Dates** | 3–4 November |
| **Done when** | Every decision is kept with who and when, and the figures never change. |

### Module 19: Rule updates by USB

| | |
|---|---|
| **What it is** | A signed rule update is copied from a USB drive and loaded with no internet, with a way to go back to the previous rules. An unsigned or damaged update is refused. |
| **What you will see** | A rule update screen showing what changed. |
| **Dates** | 4–5 November |
| **Done when** | A signed update loads, an unsigned or damaged one is refused, and the previous rules can be restored. |

### Module 20: Second release and sign-off

| | |
|---|---|
| **What it is** | Re-test everything, install the second release at the firm, and sign off. |
| **What you will see** | The complete tool in use at the firm. |
| **Dates** | 5–6 November |
| **Done when** | Every 30 October and 6 November acceptance check passes and the firm signs off by 6 November. |

---

## 5. Screens

Findings are grouped by the document that needs fixing, and the arithmetic is always shown.

| Screen | What it shows | Working by |
|---|---|---|
| Season board | Every client for the tax year, with the number of findings and Cannot check items for each. Open a client from here. | Layout 16 Oct; live totals 23 Oct |
| Client file | One client: upload files of every type; see what was read from each file and anything rejected; run the checks; read each finding (including GST, TDS and payroll) with both figures, the difference and both sources; see the Cannot check list; export. | Uploads 19 Oct; checks 21 Oct; findings 23 Oct; GST/TDS/payroll 27 Oct; exports 28 Oct |
| Worklist | New, missing and changed expense and income heads compared with last year, for staff to classify. | Layout 16 Oct; filled 23 Oct |
| Rule Builder | A form for the CA to create, edit and test a rule, and switch it on after approval. | 27 Oct |

| Login and rule update | Log in with a user name and password. A screen to load a signed rule update from a USB drive. | Login 3 Nov; rule update 5 Nov |

The demo and pilot on 30 October run on one office computer without logins. Logins and user roles are added by 3 November.

---

## 6. Timeline

The dates follow the modules. Several modules start together and run side by side; the screens and the rule book run alongside all of them until 30 October. The second release runs from 2 to 6 November.

| Dates | Engine work | Screens (UI) | Rule book (alongside) | Result |
|---|---|---|---|---|
| 14 Oct | Core engine and data model; load and check the rules | Screens started: layouts for Season board, Client file and Worklist | Convert approved rules into YAML | Engine foundation; rules load and faulty rules are rejected |
| 14–19 Oct | Read JSON, Excel and PDF files; save to SQLite (all document types) | Screens open in the browser with sample data (layouts by 16 Oct); Client file upload with a summary of what was read from each file (by 19 Oct) | Check converted rules against the agreed format; list rule problems | Every document type is read and saved; uploads work on screen |
| 14–21 Oct | Run the rules (reconciliation) | "Run checks" button; Pass, Fail and Cannot check counts on screen (by 21 Oct) | Prepare test data and expected results for approved rules | At least one approved rule works from start to finish, started from the screen |
| 17–23 Oct | Read Word, Tally and portal downloads (GST portal, TRACES, EPFO/ESIC) | Client file accepts every file type | Check the rules that use these documents | Every source the firm uses can be uploaded and read |
| 20–23 Oct | New and missing heads: compare this year's ledgers with last year's | Worklist fills with new, missing and changed heads | None | New and missing heads listed for staff |
| 22–23 Oct | Findings, consistent results and error handling | Findings grouped by document, with both figures, the difference and both sources; Cannot check list; Season board totals | More rule checks and more automatic tests | Correct Pass, Fail and Cannot check results, shown on screen |
| 22–27 Oct | GST, TDS and payroll reconciliation | GST, TDS and payroll findings in the Client file | Switch on the approved GST, TDS and payroll rules | Gaps, missing periods and late deposits found |
| 22–27 Oct | Rule Builder for the CA | Rule Builder form: create a rule, test it on a past client, switch it on | CA uses the Rule Builder to review rules | CA can create and test a rule without editing files |
| 26–27 Oct | Put everything together, fix defects, make it robust | Screens connected to the real engine end to end; keyboard shortcuts; screen fixes | Test more converted rules that the engine supports | Stable, working tool |
| 26–28 Oct | Security, backups, exports and installer | Export buttons (Excel, PDF, client query letter); backup status | None | Data encrypted; backup restored; tool installed on a clean Windows PC |
| 28–29 Oct | Full re-test and demo preparation | Screen polish and a full demo run in the browser | Documentation and last fixes | Demo-ready version |
| 29–30 Oct | Pilot at the firm: install on the office desktop; staff run real client files | Staff use the screens on real files | Switch off any rule that gives wrong results | Pilot feedback collected |
| **30 Oct** | **Final acceptance, stakeholder demo and go/no-go decision** | Demo given in the browser | Last-minute fixes if needed | **Demo delivered; go/no-go decided** |
| 2–3 Nov | Logins and user roles (Partner, Manager, Staff, Read-only) | Login screen; each role sees only the buttons it is allowed to use | None | Every user logs in; roles are enforced |
| 2–4 Nov | Upload history per client; a newer version of a document becomes a new draft; compare two drafts | Draft history in the Client file; side-by-side comparison of two drafts | None | Each upload is kept; two drafts can be compared |
| 3–4 Nov | Staff decisions on findings (Fix / Not an issue / Ask client) | Decision buttons on each finding, with who decided and when | None | Every finding can carry a staff decision, kept in the history |
| 4–5 Nov | Rule updates by USB | Load a rule update from a USB drive; the screen shows what changed | Prepare a signed rule update for the test | Rules updated with no internet; an unsigned or damaged update is refused |
| **5–6 Nov** | **Full re-test, install the second release at the firm, final sign-off** | All screens checked with logins switched on | Last rule fixes from pilot feedback | **Second release installed and signed off by 6 Nov** |

24–25 October and 31 October–1 November (Saturdays and Sundays) are not working days. There is no spare day in this plan, so any delay is reported the same day.

---

## 7. What you will see in the demo

The demo is given in the browser, using made-up test data only.

1. **The rule book.** Open a rule file and read it.
2. **A rule with a mistake.** The tool rejects it and says exactly what is wrong.
3. **Season board.** The list of test clients and their totals.
4. **Upload in the Client file.** JSON, Excel, PDF, Word, Tally and portal files are read, with a summary of what was read from each.
5. **A clean set of client files.** All rules pass; there are no findings.
6. **Files with planted mismatches.** Each mismatch is shown with both figures, the difference and both sources.
7. **GST, TDS and payroll.** Gaps, missing periods and late deposits are found.
8. **Worklist.** New and missing heads from two years of ledgers.
9. **Rule Builder.** The CA creates a rule, tests it and switches it on.
10. **A figure missing.** The screen says "Cannot check: figure not found in <document>" instead of a false mismatch.
11. **An unknown file.** The tool rejects it with a clear message and does not guess.
12. **Exports and backup.** An Excel export, a client query letter, and a backup restored.
13. **The same run twice.** The two results are identical.

---

## 8. Second release: 2–6 November

These four modules are built after the go/no-go decision and finished by Friday 6 November at the latest:

- Logins and user roles (Partner, Manager, Staff, Read-only), by 3 November.
- Upload history per client, with a newer version of a document becoming a new draft, and comparing two drafts, by 4 November.
- Staff decisions on findings (Fix / Not an issue / Ask client), by 4 November.
- Rule updates by USB, by 5 November.

If the decision on 30 October is no-go, 2–6 November is used to fix what the pilot found, and these four modules are re-planned with the business.

---

## 9. What we need from the firm

| Item | Needed by | Why |
|---|---|---|
| CA time to confirm which rules are approved for the demo, and the approver's name and date | 16 October | Only approved rules are switched on |
| Feedback on the first screen layouts | 16 October | Screens are built on from these layouts |
| Name and version of the tax software, with anonymised JSON, Excel and PDF samples if possible | 17 October | So the file readers match the real files. Until samples arrive, we use made-up files in the same shape. |
| Anonymised Word documents, Tally version and export steps, and GST portal, TRACES, EPF and ESIC sample files | 17 October | So the new file readers match the real files |
| CA review of the rule problems found during conversion | 20 October | Rules with problems cannot run |
| Confirmation of how to round negative amounts to ₹10 | 20 October | Some rules depend on it |
| Demo time on 30 October confirmed with stakeholders | 23 October | Scheduling |
| CA time to try the Rule Builder | 27 October | CA sign-off of the form |
| Office desktop for the pilot, a backup drive, the pilot staff names and the real client files for the pilot | 28 October | Pilot set-up |
| A Windows PC for the demo, and the list of people attending | 28 October | Demo set-up |
| Names and roles (Partner, Manager, Staff, Read-only) of everyone who will log in | 30 October | Logins set-up |
| A USB drive for rule updates, and CA time to approve a signed rule update | 4 November | Rule update test |
| Pilot staff time to re-test the second release, and sign-off time | 6 November | Final sign-off |

All samples must be anonymised and are kept outside the code repository.

---

## 10. How we check quality

- **Automatic tests run on every change.** If any test fails, the change is not accepted.
- **Every rule has at least two test cases:** one where the figures match (expect Pass) and one where they don't (expect Fail).
- **Test sets:**

| Test set | What it proves |
|---|---|
| Clean set of files | No false findings |
| Files with planted mismatches | Each mismatch is caught, with both figures, the difference and both sources |
| Files with a figure removed | "Cannot check" is shown, never a false mismatch |
| Unknown or damaged files, scanned PDFs and photos | Rejected with a clear message, never guessed |
| Rules with mistakes | Rejected with file name and line number |
| Ledgers for two years | New, missing and changed heads listed in the Worklist |
| GST, TDS and payroll files | Gaps, missing periods and late deposits found |
| Backup | A backup is restored and gives the same results |
| Same files run twice | Identical results |
| Screens | Each screen shows the same figures as the engine's own report |
| Users with different roles | Each role can do only what it is allowed to; a wrong password is refused |
| Two drafts of the same document | Both drafts kept; the comparison shows exactly what changed |
| Staff decisions on findings | Each decision is kept with who and when; the figures never change |
| Rule update by USB | A signed update loads; an unsigned or damaged one is refused |

- **Two review helpers.** A testing assistant checks each module against its "done when" list, and a code-review assistant reviews the code before a module is closed. A person still signs off. The CA signs off on rules.
- **No real client data** is used while building. Real files are used only in the pilot, on the firm's own computer.

**Accepted on 30 October if:** approved rules run from start to finish and give the expected results; Pass, Fail and Cannot check are correct on every test set; the clean set gives no findings; two runs give identical results; the full demo runs in the browser; the automatic check shows the engine has no internet code; and the pilot staff confirm the findings on real files are useful (go/no-go).

**Accepted on 6 November if:** logins work and each role can do only what it is allowed to; two drafts of a document can be compared; staff decisions are recorded on findings with who and when; a signed rule update loads from USB and an unsigned one is refused; and every 30 October acceptance check still passes.

---

## 11. Rules we never break

1. **Report only what can be calculated.** Show both figures, the difference and both sources. Never say what a figure "should" be.
2. **No AI decides a finding.** AI is never used to create, hide or change a finding.
3. **Exact money.** All amounts are kept in paise, so there are no rounding errors.
4. **Same input, same answer.** Results never depend on timing or chance.
5. **No internet.** The checking engine contains no code that can connect to a network. Every build is checked automatically to prove this.
6. **Rules are readable files, not hidden code.** A CA can read every rule. A rule that gives wrong results in practice is switched off, not quietly adjusted.
7. **Manual file upload always works.** Automatic connections to other systems are optional extras, never required.
8. **Nothing extra to install.** One program on one office computer. No servers, no cloud.

---

## 12. Risks

| Risk | Impact | Likelihood | What we do |
|---|---|---|---|
| Twenty modules in three and a half weeks (14 October to 6 November) overload one developer | High | High | Modules run side by side; screens use sample data first; any slip is reported the same day |
| Pilot has only two days on real files | High | Medium | Pilot covers a few clients only; any rule that gives wrong results is switched off, not adjusted |
| PDF and portal files are harder to read accurately than JSON or Excel | High | Medium | Read only text-based files, keep the column layout, test against hand-checked figures; scans are refused |
| Real file samples arrive late | Medium | Medium | Build on made-up files in the same shape; swap in real samples when they arrive |
| Rule book problems found during conversion | Medium | High | Daily list of problems to the CA; only clean, approved rules are switched on |
| Pilot fixes compete with second-release work in the week of 2 November | High | High | Pilot fixes come first; any slip past 6 November is reported the same day |
| Logins and drafts change screens already shown in the demo | Medium | Medium | Built on top of the existing screens; every 30 October test re-run on 6 November |

---

## 13. After 6 November

All Phase 1 modules are in this plan. Anything new found in the pilot or the second release is planned after 6 November, with dates agreed with the business.

---|---|
| Logins and roles | Partner, Manager, Staff, Read-only; staff use the tool from their own PC over the office network |
| Uploads and drafts | Upload history per client; a newer version of a document becomes a new draft; comparing two drafts |
| Decisions on findings | Fix / Not an issue / Ask client; findings close on their own when a corrected file is uploaded |
| Rule updates by USB | Signed rule packs installed from USB, with rollback |

---

## 14. Status and decisions

| Module | Dates | Status |
|---|---|---|
| 1. Core engine and data model | 14 Oct | In progress |
| 2. Rule book in YAML | 14–30 Oct (alongside) | In progress (rule conversion started; quoting fixes open) |
| 3. Loading and checking the rules | 14 Oct | In progress |
| 4. Reading client files (JSON, Excel, PDF) | 14–19 Oct | In progress |
| 5. Saving the data (SQLite) | 14–19 Oct | In progress |
| 6. Running the rules | 14–21 Oct | In progress |
| 7. Findings, consistent results, error handling | 22–23 Oct | Not started |
| 8. Putting it together, testing, demo | 26–30 Oct | Not started |
| 9. Screens in the browser | 14–30 Oct (alongside) | Not started |
| 10. Word, Tally and portal downloads | 17–23 Oct | Not started |
| 11. New and missing heads | 20–23 Oct | Not started |
| 12. GST, TDS and payroll reconciliation | 22–27 Oct | Not started |
| 13. Rule Builder for the CA | 22–27 Oct | Not started |
| 14. Security, backups, exports and installer | 26–28 Oct | Not started |
| 15. Pilot and go/no-go | 29–30 Oct | Not started |
| 16. Logins and user roles | 2–3 Nov | Not started |
| 17. Upload history and drafts | 2–4 Nov | Not started |
| 18. Staff decisions on findings | 3–4 Nov | Not started |
| 19. Rule updates by USB | 4–5 Nov | Not started |
| 20. Second release and sign-off | 5–6 Nov | Not started |

**Decisions log**

| Date | Decision |
|---|---|
| 13 Oct 2026 | Building started. Everything is written in Go (one programming language). The checking engine has no internet code, and this is proved on every build. |
| 13 Oct 2026 | No reading of scanned PDFs or photos (no OCR) in Tool 1. |
| 14 Oct 2026 | The CA-approved rule book (304 rules) is the source for all rules. It is converted into YAML files, and each rule stays switched off until the CA approves it. |
| 14 Oct 2026 | New timeline: build module by module (no sprints), with a demo on 30 Oct 2026. This replaces the earlier plan (full scope by 13 Nov). |
| 14 Oct 2026 | The tool reads JSON, Excel and PDF files (not JSON only). |
| 14 Oct 2026 | Screens are built from day one alongside the engine, through to the demo, which is given in the browser. |
| 14 Oct 2026 | Word, Tally and portal downloads; new and missing heads; GST, TDS and payroll; the Rule Builder; security, backups, exports and the installer; and a pilot with a go/no-go decision are all brought into the 14–30 Oct window. The extra load on the developer is accepted. Logins, drafts, staff decisions on findings and USB rule updates stay after 30 Oct. |
| 14 Oct 2026 | Logins and user roles, upload history and drafts, staff decisions on findings, and rule updates by USB are brought into the plan as a second release, 2–6 Nov 2026. 6 Nov is the latest finish date, given the load on the developer. |

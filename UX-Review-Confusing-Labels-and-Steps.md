# UX review: confusing labels and steps

Date: 4 November 2026
What was tested: the packaged `build/ReconTool-for-CA.zip`, unzipped into a scratch folder and run on port 8090 with an empty database (the existing database was not touched; the test copy has been stopped).
How: first-time Partner setup, then a client added and Test Pack A (fictional Nimbus Logic Systems) uploaded as six Excel forms and two expense-ledger CSVs, then the checks run.
Result of the run: 82 checks, 39 pass, 28 findings, 15 cannot check.
Screenshots: `.playwright-mcp/s1-empty.png`, `.playwright-mcp/s2-results.png`.

Note: nothing was listening on `localhost:3000`, so the test used the packaged copy instead.

## Most confusing, in the order a user meets them

1. **The first screen after sign-in shows made-up clients.** For about a second, 7 sample clients appear with a "Sample data" badge and the note "start recon-server to add your own clients". Then the screen flips to "Connected to the engine" with no clients. A new user will think the sample clients are theirs.
2. **"Tax year" is ambiguous.** The pack is AY 2026-27 / PY 2025-26. The header says 2026-27, the Add client form defaults to 2025-26, and the upload dialog silently switched to 2025-26 (it does explain why, in a small note). The CA words "Assessment year" and "Financial year" are never used. Suggest "2026-27 (FY 2025-26)". The dropdown also runs from 1999-00 to 2046-47.
3. **The Worklist is not reachable.** The nav has only Season board, Rules, Formats and Admin. "Worklist" appears only in a tooltip on the tax-year picker. Both ledgers were uploaded, yet no new, vanished or step-change heads appeared anywhere. It may be hidden or missing from this build (not confirmed beyond the visible screens).
4. **Raw codes are shown to users.**
   - `GSTIN_CHECK_DIGIT` is shown beside the field and again in a red box at the bottom of a long form.
   - `CONFLICTING_VALUES: CONFLICTING_VALUES:` is printed twice.
   - Recognised files read `taxsuite.xlsx.tax_audit_report v1`.
   - "Cannot check" reasons mention `(gstr1_b2b_invoice)`.
   - Format names read `generic.books.expense_ledger`.
5. **The PAN-mismatch error gives no next step.** It says the file's PAN (ending 821T) is not the client's (ending 821K). It does not say whether to edit the client or the file, and the upload dialog has no link to Edit client. (Cause here: the pack README says `...K`, the pack data says `...T`. The tool was right to refuse.)
6. **The rule counts disagree.** The upload dialog first says "21 of the 373 rules can run now", later "63 of 373"; the progress screen says "Running 63 rules"; the Rules screen says "69 can run as checks".
7. **File recognition is slow and unexplained.** Each file takes 4 to 5 seconds, one at a time, and shows only "Checking what this file is…". Eight files take about 35 seconds before Run checks is usable. Suggest a count such as "3 of 8".

## Labels that need rewording

- **"Cannot check"** is lumped in with the results, but many are simply "you have not uploaded the GST files yet". Suggest "Not uploaded yet" where that is the cause. The "Why" text also repeats "Cannot check:" twice. The "What to upload" panel is good.
- **Rules screen contradicts itself.** "Switched off, described only, not run yet" (304) and "Disabled, switched off with a reason" (0) sound alike. "Approved 0 / Testing 69" sits oddly with a rule book described as CA-approved.
- **"Include rules not yet approved by the CA"** is ticked by default, so every first run carries the yellow "status testing" banner. Decide whether it should be ticked.
- **Decision buttons.** "Fix" and "Ask client" are clear. "Not an issue" needs a note, but the note box does not say it is required until you try.
- **Duplicate-looking findings.** RECON-DEP-001, DEP-002 and PGBP-001 all show ₹2,448.00 for the same defect, which reads as three problems.
- **Documents tab.** "Not read: 100" for the computation and "419 values no check uses yet" look like gaps. It needs a one-line "this is normal".
- **Time zones are mixed and unlabelled.** Run history shows 18:40, Formats shows last used 13:10, Admin shows last sign-in 13:07 UTC. These are all the same moment, in IST and UTC.
- **Lowercase document labels** in Run history, such as "form 26as" and "mat report".
- **Findings list suffixes** such as `RECON-TAR-026 · 43B(a)|TDS` and `RECON-TAR-031 · Rohan Mehta` show unexplained keys beside the name.

## What worked well

- The "No clients yet. A season takes three steps." empty state.
- The arithmetic panel: A, B and the difference, with file and cell (for example `Computation!B14`).
- Files are recognised by their contents, with no type picker.
- Refusing a bad GSTIN and a wrong PAN stops mistakes early. Only the wording needs work.

## Not tested

- The PDFs and the JSON pack.
- The decision flow (Fix, Not an issue, Ask client).
- Excel export, teaching a new format on the Formats screen, and adding a second user.
- Four mismatches between the pack README and the pack: the PAN (K vs T), the GSTIN check character, the file list and the file names.

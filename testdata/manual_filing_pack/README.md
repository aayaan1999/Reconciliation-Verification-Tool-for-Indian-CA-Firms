# Manual test: tax suite filing pack, three versions

SYNTHETIC. Test Pack A's fictional company, Nimbus Logic Systems Private Limited. No real client data.

Each file is one tax suite filing pack. One JSON file holds all six documents of the comparison type "Tax suite filing pack: audit report, Form 29B, Form 67, Form 26AS, computation and accounts": the tax audit report (Form 3CD), Form 29B, Form 67, the computation, Form 26AS and the financial statements. Uploading the next file makes draft 2, then draft 3, of each of the six documents, so the three files give Comparisons 1, 2 and 3 (SPEC-009).

| File | What it is | Expected result |
|---|---|---|
| `01_filing_pack_as_received.json` | The pack as received, with every defect (same as the profile sample `nimbus_pack.json`) | 67 checks: 39 pass, **28 fail**, 0 cannot check. Findings: tax audit report 15, Form 29B 4, computation 5, Form 67 3, Form 26AS 1 |
| `02_filing_pack_audit_report_and_29B_fixed.json` | File 01 with only the tax audit report (`form_3cd`) and Form 29B (`form_29B`) corrected | 66 checks: 58 pass, **8 fail**, 0 cannot check. Findings: computation 4, Form 67 3, Form 26AS 1 |
| `03_filing_pack_all_fixed.json` | Every defect corrected (same as the profile sample `nimbus_pack_corrected.json`) | 66 checks: **66 pass**, 0 fail, 0 cannot check |

### The same packs as Excel and CSV (SPEC-011)

Each pack is also here as `.xlsx` and `.csv` with the same name: `01_filing_pack_as_received.xlsx` and `.csv`, and so on. Each gives exactly the results in the table above. They hold the pack flattened into two columns, **Path** (the value's place in the pack, such as `$.form_67[0].country`) and **Value**, one value per row. In Excel this is the sheet "Filing pack". A finding's source is then the cell (`01_filing_pack_as_received.xlsx Filing pack!B271`) or the CSV line (`01_filing_pack_as_received.csv line 271`). Upload them the same way as the JSON files, under the same type of check. You can mix them: JSON for Comparison 1, Excel for 2 and CSV for 3, for example.

They are written from the JSON files by `go test ./cmd/recon-engine -run TestFlatFilingPacks -update`; do not edit them by hand. To try a change, edit a copy outside this folder.

## Steps

`Manual_Test_Checklist.xlsx` holds the same steps with columns to fill in (Actual result, Pass / Fail, Notes), the expected result of every rule in each comparison ("Expected findings"), and the files ("Files"). It is a checklist for people; it is not uploaded to the app.

1. Start the app with `start-local.cmd` and open http://127.0.0.1:8080. You can pull the network cable out: nothing needs the internet.
2. On the Season board, add a client: name "Nimbus Logic Systems Private Limited", PAN `AABCN4821K` (the PAN inside the files), tax year **2025-26**.
3. In the client file, upload `01_filing_pack_as_received.json`. Tick only "Tax suite filing pack: …", switch on **Include rules not yet approved by the CA**, and run. Expect 28 fail.
4. Decide RECON-DEP-001 (computation, depreciation 1,38,310 against 1,35,862) as "Ask client", and RECON-REC-001 (Form 26AS receipts) as "Fix".
5. Upload `02_…json` the same way. Expect 8 fail. RECON-DEP-001 is still "Ask client" (its figures did not change). RECON-REC-001 is open again, decided on other values (its clause 40 turnover changed from 0 to 6,42,18,350).
6. Upload `03_…json` the same way. Expect 0 fail.
7. The **Comparisons** panel lists, newest first: Comparison 3 (latest, 66 pass), 2 (8 fail), 1 (28 fail). Each lists the six documents with drafts 3, 2 and 1.
8. Open Comparison 1. Expect a read-only banner, the 28 findings grouped by document, no decision buttons, working filters and findings index, and document figures from file 01. Open Comparison 2 the same way, then click **Back to latest**.
9. Do step 8 again with the keyboard only (Tab, Enter, arrows).

## Other checks

- A client with another PAN: the file is refused, because the PAN in it ends 821K, and nothing is stored.
- Tax year 2026-27 for this client: the files' figures are not used, and the report says each file is for tax year 2025-26.
- Without "Include rules not yet approved by the CA": 0 checks, because every filing-pack rule is still in testing. Do this last, since it adds a comparison.

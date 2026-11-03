# GST completion fixtures (SPEC-018)

Synthetic files of a fictitious company ("Shree Sample Traders Pvt Ltd") for September 2026,
copied from the `Reconciliation_Sample_Pack_Sep2026` sample pack. No real client data.

- `inputs/`: the pack's input files. The GST files are read by the shipped formats; the TDS,
  payroll, bank and XML files are kept so the whole folder can be uploaded together (they are
  listed as not read).
- `answer_key/`: the pack's own outputs R01, R02/R03, R04, R05, R07 and R08, the answer key
  for the comparison.
- `expected_results.yaml`: what RECON-GST-030..035 give on `inputs/`, with the answer-key row
  each result is checked against and every result of the pack's outputs that the tool does not
  give, with the reason (judgments about law or about which side is right, and matching of
  invoice numbers written differently). Checked by `go test ./cmd/recon-engine -run TestGSTCompletion`.

## The company GSTIN

The pack writes the company GSTIN as `08AAPCS9876K1ZQ`. Its check character is wrong (the
correct check character for the other fourteen characters is `6`), so the tool refuses it when
a client is added. In the copies here every `08AAPCS9876K1ZQ` is replaced by `08AAPCS9876K1Z6`;
nothing else is changed. The original files in the pack are not edited. Whether to relax the
check for synthetic packs is a question for the user, not decided here.

## Where the files are used

The Tally sales and purchase registers and the e-invoice, e-way bill and IMS files carry no
client GSTIN, so a run uses the client's only GSTIN (`--client`); the test adds the company as
a client first. None of these files has a period column: the period is the month of the first
document date, September 2026.

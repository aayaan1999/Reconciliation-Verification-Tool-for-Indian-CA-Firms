# SPEC-019 — Dollar amounts read and converted to paise at a fixed rate

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 1 |
| **Depends on** | SPEC-008 (documents read from Excel and CSV; `internal/money` in use by every reader) |
| **Requirements** | Money is integer paise (C4); a figure that cannot be found or read is "cannot check", never zero (`backend.md`) |
| **Technical spec** | §B (money and parsing), to be updated in the same change |
| **Code / content paths** | `internal/money/` (parse and rate), `content/currency/usd-rate.yaml` (the rate, kept by the CA), the readers that call `money.Parse` |

## Goal

A document that writes an amount in US dollars (`$1,250.50`, `USD 1,250.50`, `($1,250.50)`) is read, and the amount is converted to integer paise at one fixed rate that the CA keeps in a data file. Nothing else changes: no new screen, no change to findings or reports.

## Scope

- **The rate.** One file, `content/currency/usd-rate.yaml`, holds the rate in rupees per US dollar (up to four decimal places, for example `83.2500`) and the date it takes effect. The CA edits this file; no code change is needed to update it. The rate is read once when the engine starts and is the same for every document in a run.
- **Reading dollars.** `internal/money` gains a function that reads a dollar amount from text: a leading `$`, a leading or trailing `USD`, Western grouping (`1,250.50`), up to two decimal places (cents), and brackets or a minus sign for negatives. It reuses the existing checks (no floats, overflow refused, ambiguous text refused).
- **Converting.** `paise = cents × rate`, with the rate held as a whole number of 1/10,000 rupee. The result is rounded to the nearest paise, halves away from zero, using integer arithmetic only. Example: `$1,250.50` at `83.2500` = 125,050 cents × 832,500 / 10,000 = 10,410,412.5, which rounds to 10,410,413 paise (₹1,04,104.13).
- **Where it applies.** Every reader that parses an amount from a cell or text (Excel, CSV, JSON strings) accepts a dollar-marked amount through the same function. An amount with no `$` or `USD` is still read as rupees, exactly as now.
- **Missing or bad rate.** If a dollar amount is met and the rate file is missing, empty or invalid, that figure is "cannot check" with the reason "dollar rate not set", never zero and never a guess. Rupee-only runs do not need the file.
- **Rate in the audit trail.** A run records the rate and its effective date that it used (in the run's stored inputs), so a past run can be explained later.

## Out of scope

- Showing the dollar value and the rate next to the rupee value on a finding or report, and any screen to edit the rate (a later spec).
- Other currencies (EUR, GBP and so on), and a rate per document, per client or per date.
- Fetching a rate from the internet. The tool is offline; the CA types the rate.
- Treating a plain number in a "USD" column as dollars. Only an amount marked with `$` or `USD` is converted.

## Design notes

- Money stays `int64` paise everywhere downstream; only the parser knows about dollars (invariant 3). No `float64`, no `strconv.ParseFloat`.
- The same text and the same rate file always give the same paise (determinism); no clock is read to pick the rate. The effective date is shown for the CA's records and is not compared with the document's date in this spec.
- The rate file is data, like rules (invariant 6). It holds no client data.
- A dollar amount with more than two decimal places is refused (it is not a whole number of cents), with a message naming the file and locator, not the figure.
- Update `tool1-technical-spec.md` (money and parsing) in the same change.

## Acceptance criteria

- [x] AC-1 — `$1,250.50`, `USD 1,250.50`, `1,250.50 USD` and `($1,250.50)` are read; the first three give the same paise and the last gives the negative of it.
- [x] AC-2 — At rate `83.2500`, `$1,250.50` gives 10,410,413 paise; at `83.0000` it gives 10,379,150 paise. A half paise rounds away from zero, for positive and negative amounts.
- [x] AC-3 — The conversion uses integer arithmetic only. A test shows no `float64` in the money package's conversion, and the largest amount that fits converts without overflow, while one above it returns an error wrapping `ErrOverflow`.
- [x] AC-4 — More than two decimal places, `$` in the middle of the number, `$` and `₹` together, and `USD` with a `$` are refused with a `ParseError`.
- [x] AC-5 — An amount with no `$` or `USD` is read as rupees, exactly as before; the existing money tests pass unchanged.
- [x] AC-6 — With the rate file missing, empty, non-numeric, zero or negative, a dollar amount gives "cannot check" with the reason "dollar rate not set" or "dollar rate not valid"; it never gives zero and never a mismatch. A run with only rupee amounts works without the file.
- [x] AC-7 — The CA changes the rate in `content/currency/usd-rate.yaml` only, and the next run converts at the new rate with no rebuild of the code.
- [x] AC-8 — A run's stored inputs record the rate and its effective date.
- [x] AC-9 — An Excel cell and a CSV field holding `$1,250.50` give the same paise as the text form (a sample row in a Test Pack A style fixture, with its expected file).
- [x] AC-10 — `gofmt -l .` prints nothing, `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` pass, and `go test -count=1 ./...` passes, including the architecture guard and the Test Pack A score (unchanged: 22 of 30, no false positives).
- [x] AC-11 — Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

1. `go test -count=1 ./internal/money/ -v` (AC-1 to AC-5).
2. A run on a fixture with a dollar column, once with a valid rate file, once with the file removed, and once with the rate changed (AC-6, AC-7, AC-8, AC-9): `go run ./cmd/recon-engine run --db build/recon.db --input <fixture> --tax-year 2026-27 --include-testing --json build/report.json`.
3. Format check, vet (host and Windows) and the full `go test -count=1 ./...` (AC-10).
4. The CA reads `content/currency/usd-rate.yaml` and confirms they can change the rate without help (AC-7).

## As built (deviations from the text above, 4 November 2026)

- **Where "cannot check" and its reason appear.** A dollar amount with no usable rate is not read; the file is accepted and its validation report carries a `DOLLAR_RATE` warning naming the file and place (never the figure). The checks that need the row say "Cannot check: <amount> is not read in <document> (<label>): dollar rate not set" (or "dollar rate not valid"). This needed a reserved marker on the stored row (`~unread`, `domain.UnreadField`), a new missing-entry kind in `rules`, and handling in `match_records` (`sum_equals` already treats an absent amount as missing). Without it a row compared on its other amounts alone passed or showed a false difference.
- **Every amount of the row is left unread**, not only the dollar one: the row's rupee amounts are Cannot check too (a pass on the remaining amounts would hide the figure that was not read).
- **A file that holds dollar amounts is a new draft when the rate changes** (`document_versions.read_with`, migration 0014). Before this, re-uploading the same file kept the figures converted at the old rate (the store treats the same file and format as unchanged), so AC-7 would have failed on the second run.
- **The shipped rate file is empty** (`usd_rate: ""`), so nothing is converted at a guessed rate until the CA sets one; the tests write their own rate files.
- **`--currency <folder>`** (default `content/currency`) names the folder of the rate file for `recon-engine run`.
- **Dollar errors do not repeat the figure**: the message gives the reason only, as the spec's design note asks (the existing rupee parse errors still quote the text).
- **A dollar amount is read from text cells only** (`$1,250.50` as text in Excel, CSV and JSON strings). An Excel number cell that is merely formatted as dollars holds a plain number and is read as rupees, like any number; this is as the spec says (only a marked amount is converted).
- **Not covered:** the screen for the CA to edit the rate and showing the dollar value on findings (out of scope); the CA's own check that they can change the rate without help (verification step 4).
- **Paperwork:** File Guide section 23 and technical spec section A8 updated. AC-10: the full `go test -count=1 ./...` passed on 4 November 2026 (after `TestExcelGivesTheSameFindings` was changed to ignore the document totals, which differ by design between the JSON and Excel layouts); two small later edits were re-tested in their packages.

## Estimate

About one working day: reading and converting in `internal/money` with its tests (half a day), the rate file and "cannot check" handling (two hours), a fixture and expected file for Excel and CSV (two hours), and the full verification and file-guide entry (one to two hours).

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 2026-11-04 | Claude Code (built and tested) | not committed | All 11 acceptance criteria ticked; full `go test` green. Verification step 4 (the CA changes the rate in `content/currency/usd-rate.yaml` unaided) is not yet done, so the status stays In progress. Not Verified: that needs a second person. |

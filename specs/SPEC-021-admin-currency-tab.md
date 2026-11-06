# SPEC-021 — Admin: a Currency tab where the Partner sets the dollar rate

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-019 (the dollar rate and how a run uses it), SPEC-012 (the Admin tab, the Partner's permission and the audit log) |
| **Requirements** | Money is integer paise (C4); every change is audited (RU-42); offline always |
| **Technical spec** | §A8 (money), §B9 (audit), §B12 (the Admin tab) |
| **Code / content paths** | `cmd/recon-engine/currencyadmin.go`, `internal/api/currency.go`, `web/src/screens/Admin.tsx`, `web/src/api.ts`, `web/src/types.ts`, `web/src/permissions.ts` |

## Goal

The Partner changes the rupees-per-dollar rate from the Admin tab, without editing a file, because the rate moves often. Every change is written to the audit log with who, when, the old rate, the new rate and the date it takes effect, and the next run converts dollar amounts at the new rate.

## Scope

- **A Currency tab in Admin** (Partner only, as the other Admin tabs), showing: the rate in force (or "No rate set"), the date it takes effect and the note; a form to set a new rate, its effective date (today by default) and a note; the change from the rate in force as a percentage while typing; and a list of the earlier rates (when, who, old rate, new rate, effective date, note), newest first.
- **Validation**, the same in the screen and in the server: a rate is rupees per dollar with up to four decimal places (as `money.ParseUSDRate`) and is at least 1 and below 1,000, to stop a slip such as `8325`; the effective date is a real date written yyyy-mm-dd; the note is one line of at most 200 printable characters. Nothing is accepted that the engine would not read.
- **Where the rate lives.** The server keeps the rate file in a `currency` folder beside the database (as the office profiles and rules are kept), set with `--currency` on `recon-server`; the shipped `content/currency/usd-rate.yaml` stays empty and is the file a command-line run reads by default. The server passes its folder to every run.
- **Engine commands** `currency-get` and `currency-set` do the work (the server only forwards, as for formats): `currency-set` validates, writes the file whole (never half-written) and writes one audit entry, `currency.rate_set`, in one transaction; `currency-get` returns the rate and the history read from the audit log.
- **Who may:** reading the rate needs only sign-in (any user may see which rate is in force); changing it needs the Admin permission (`manage_users`, the Partner).
- Wording on the tab says what a change does: it applies to the next run; a file that holds dollar amounts is read again at the new rate as a new draft; earlier runs keep the rate they used (shown on the run).

## Out of scope

- Fetching a rate from the internet or a bank (the tool is offline; the Partner types the rate).
- Different rates for different dates, clients or documents; other currencies.
- Showing the dollar value and the rate beside the rupee value on a finding (a later spec).
- Re-running past runs at the new rate.

## Design notes

- The rate is not compared with the date of any document (SPEC-019): "effective from" is a record for the Partner.
- The engine reads the rate once per run; a change made during a run does not affect that run.
- The audit entry holds the old and new rate and the date; the note is kept in the entry and in the file. It holds no client data.
- The UI never works out money; it only shows the rate text the server returns and the percentage change, a display figure computed from the two rate texts as integers (1/10,000 rupee).

## Acceptance criteria

- [x] AC-1 — `recon-engine currency-set` with a valid rate writes `usd-rate.yaml` in the given folder, writes one `currency.rate_set` audit entry with the actor, the old and new rate, the effective date and the note, and `currency-get` then returns the rate, the date and the note.
- [x] AC-2 — Invalid input is refused with a message naming the field and nothing is written or audited: an empty, non-numeric, zero, negative, five-decimal, below 1 or 1,000-and-above rate; a date that is not a real yyyy-mm-dd date; a note with a line break or over 200 characters; no actor.
- [x] AC-3 — `currency-get` with no file says "No rate set" (not an error); with an invalid file it names the problem; the history lists the changes newest first with who and when.
- [x] AC-4 — A run after a change converts dollar amounts at the new rate with no restart, and the run stores the new rate (SPEC-019 AC-7, AC-8, through the server's folder).
- [x] AC-5 — `GET /api/v1/currency` is available to any signed-in user; `PUT /api/v1/currency` needs the Partner's permission and refuses Manager, Staff and Read-only with the usual error; every request is checked for the same limits as the engine.
- [x] AC-6 — The Admin tab has a Currency tab: it shows the rate in force, the form, the percentage change from the rate in force, and the history; saving shows the outcome and refreshes the history; an invalid entry is explained beside the field and cannot be saved; it works from the keyboard and follows the screen rules (labels, errors beside the field, no colour alone).
- [x] AC-7 — The role matrix on the Admin tab says the Admin permission also covers the dollar rate.
- [x] AC-8 — `gofmt -l .` prints nothing, `go vet ./...` (host and Windows), `go test -count=1 ./...`, the architecture guard, Test Pack A (22 of 30, 0 false positives), and in `web/`: `npm run lint`, `npm test`, `npm run build` pass, except failures listed as already present before this spec.
- [x] AC-9 — Every file this spec created is added to `docs/Tool1-File-Guide.docx`.

## Verification

1. `go test -count=1 ./cmd/recon-engine -run TestCurrency -v` (AC-1 to AC-4).
2. `go test -count=1 ./internal/api -run TestCurrency -v` (AC-4, AC-5).
3. `cd web && npx vitest run src/currency.test.tsx` (AC-6, AC-7), then lint and build.
4. The Partner opens Admin, Currency, sets a rate and sees it in the history; a dollar amount uploaded afterwards converts at that rate (manual).

## As built (4 November 2026)

- **Where the rate is kept:** `recon-server --currency <folder>` (default: a `currency` folder beside the database). The shipped `content/currency/usd-rate.yaml` stays empty and is what a command-line `recon-engine run` reads by default, so the command line and the server use the same rate only when pointed at the same folder.
- **Permission:** the Admin permission (`manage_users`) covers changing the rate; no new permission was added. Reading is open to any signed-in user.
- **Validation:** rate at least 1 and below 1,000 (a guard against typing slips such as 8325, not a statement about the market); the screen explains it, the engine enforces it.
- **The percent change on the screen** is shown from the two rate texts as whole 1/10,000-rupee units, with integers only; it is a display figure, not money.
- **Tests:** `cmd/recon-engine/currency_test.go`, `internal/api/currency_test.go`, `web/src/currency.test.tsx`; `adminView.test.tsx` updated (four tabs; role matrix wording).
- **Verification step 4 (manual)** is not done: the Partner opens Admin, Currency, sets a rate, sees it in the history, and a dollar amount uploaded afterwards converts at that rate. The status stays In progress until then.
- **A note on the full test run:** `TestEveryPackageIsClassified` failed once while a scratch folder holding a throwaway program existed under `build/`; it passes with the folder removed.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |

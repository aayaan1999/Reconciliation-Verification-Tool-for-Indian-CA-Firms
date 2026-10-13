# SPEC-030 — PDF and text/fixed-width adapters

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 5 |
| **Depends on** | SPEC-006 |
| **Requirements** | IN-05, IN-06, A5 |
| **Technical spec** | §A6.1, §B2.2, §B17 (T4); v0.2 spec §5.2.5 |
| **Code / content paths** | `internal/ingest/adapters/pdftext`, `internal/ingest/adapters/fixedwidth`, `third_party/poppler/` |

## Goal

Profiles can read text-based PDFs (as a fallback only) and portal text files (TRACES, EPFO/ESIC), with column positions preserved so that values never land in the wrong field.

## Scope

- **PDF:** the bundled Poppler `pdftotext -layout` is run through `os/exec` with a timeout, page by page. Output is kept as lines with character columns. The Poppler licence file is included.
- PDF selectors: `page`, `anchor` (text), `line_offset`, `columns` (declared character ranges), `table_until` (an anchor that ends a table).
- Scanned PDFs and image pages: detected and handled by SPEC-044, built straight after this spec. This spec only exposes per-page text so SPEC-044 can classify pages.
- Locator: `p7:l23:c41-52`, displayed as `file.pdf p.7`.
- **Text / delimited / fixed-width:** the profile declares the delimiter or column widths, the header lines and the encoding (for example, portal text files with custom delimiters).
- Profile tests must include at least one sample where adjacent columns are close together (T4).

## Out of scope

- OCR. A Python `pdfplumber` sidecar: only if this spec proves Poppler insufficient. In that case, record the reason in `docs/adr/` and raise a new spec.

## Acceptance criteria

- [ ] AC-1 — A synthetic statutory-style PDF with a multi-column table extracts every value into the right fact, including a sample with narrow gaps between columns.
- [ ] AC-2 — A column shifted by one character outside its declared range produces an extraction error, not a wrong value.
- [ ] AC-3 — The adapter returns text per page, so a page with no text can be told apart from a page that failed to extract (scanned-PDF handling itself is SPEC-044).
- [ ] AC-4 — If `pdftotext` hangs or fails, it is reported as a file error within the timeout, and the run continues for the other files.
- [ ] AC-5 — Fixed-width and delimited synthetic portal files extract the expected records.
- [ ] AC-6 — The engine archtest passes (`os/exec` only; no network).

## Verification

```
go test ./internal/ingest/adapters/pdftext/ ./internal/ingest/adapters/fixedwidth/ -v
go run ./cmd/recon-dev profile-test ./content/profiles/...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |

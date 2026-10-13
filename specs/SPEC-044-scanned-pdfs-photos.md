# SPEC-044 — Scanned PDFs and photos (no OCR in Tool 1)

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 5 (build straight after SPEC-030) |
| **Depends on** | SPEC-024, SPEC-030 |
| **Requirements** | IN-05, IN-14, IN-21, IN-23, A5 |
| **Technical spec** | §A6.1, §B2.1, §B17 (T7) |
| **Code / content paths** | `internal/ingest` (content sniffing, error codes), `internal/ingest/adapters/pdftext` (text-layer check), `internal/api` (upload refusal), `internal/archtest` (no-OCR guard), `testdata/scanned/` |

## Goal

When staff upload a photo, a scanned PDF or a PDF with scanned pages, the tool recognises it from the file's contents and says plainly what it cannot read and what to upload instead. It never guesses at figures in an image: Tool 1 contains no OCR, and the build proves it.

## Why

A figure read from an image is a guess. A misread digit produces a confident false finding, and a finding must only state what can be computed (CLAUDE.md invariant 1). Requirements A5 and IN-05 keep scanned images out of Tool 1, so every scan must be refused visibly and never passed over in silence.

## Scope

**1. Recognise a file by its contents, not its name**

- `ingest.Sniff(first bytes) FileKind` reads the file's signature (magic bytes): PDF, ZIP-based Office (xlsx/docx), JSON, XML, text, and the image formats JPEG, PNG, GIF, BMP, TIFF (including multi-page fax TIFF), WebP and HEIC/HEIF (iPhone photos).
- If a file's extension disagrees with its contents, for example a photo renamed `TAR.pdf`, the contents decide. The validation report states both: "File name says PDF; contents are a JPEG image."

**2. Photos and image files: refused at upload**

- The server sniffs during upload step 1 (§B2.1), before the vault. An image file is refused with `IMAGE_NOT_SUPPORTED` and is **not stored**. The upload response and audit log keep only the file name, size, SHA-256 and reason, never the bytes.
- Message: "Photo or image file — Tool 1 does not read images. Upload the original file from the software (JSON, Excel, Word) or a PDF saved directly from it, not printed and scanned."
- In a multi-file upload, one refused image never blocks the other files.

**3. Scanned PDFs: per-page text-layer check**

- The `pdftext` adapter checks every page with the bundled Poppler (no new tool). A page **has text** if `pdftotext` gives at least one character that is not whitespace or a control character. Otherwise it is an **image page**.
- **Fully scanned** (no page has text): rejected as `SCANNED_PDF_NOT_SUPPORTED`. Message: "Scanned PDF — Tool 1 reads only PDFs saved directly from software. Ask for the original file or a PDF exported from the software." The document version stays in the vault, like any other unreadable file (SPEC-024). It produces no facts, and no profile detection runs on it.
- **Mixed** (some image pages): accepted. A typical case is a computer-made tax audit report with a scanned signature page.
  - Each image page is listed under "Not read" in the validation report: "Page 12 is a scanned image and was not read."
  - If a profile field or region sits on an image page, the result is `PDF_PAGE_IS_IMAGE` naming the page, never a zero or blank value. For a `required: true` field it is an error, as `REQUIRED_FIELD_MISSING` already is.
- Profile **detection** ignores image pages. A mixed PDF whose recognisable pages are all images is treated as fully scanned.
- **Downstream:** a fact that could not be read is simply absent. Rules that need it give **Cannot check** (RU-52) and never a finding or a pass.

**4. No OCR anywhere in Tool 1**

- No OCR library, binary or service in any Tool 1 program. The archtest gains `TestNoOCR`, which fails if:
  - any module package or dependency matches the OCR deny list (`tesseract`, `gosseract`, `leptonica`, `ocr` path segments, and named packages added as found), or
  - any file under `third_party/` is an OCR engine or its language data (`tesseract*`, `*.traineddata`).
- Poppler is invoked only as `pdftotext` with fixed arguments. No Poppler tool that renders pages to images (`pdftoppm`, `pdftocairo`) is bundled.

**5. Error codes and wording**

- Three codes join the SPEC-005 list: `IMAGE_NOT_SUPPORTED`, `SCANNED_PDF_NOT_SUPPORTED` and `PDF_PAGE_IS_IMAGE`.
- Every message:
  - names the file (and page where relevant);
  - says what to upload instead;
  - never says the document is wrong, invalid or unacceptable (§B4.6). A scan is a legitimate document that Tool 1 cannot read.

**6. Fixtures (synthetic only)**

- `testdata/scanned/` holds synthetic fixtures, all made from made-up figures by a documented generator (`recon-dev synth scanned`):
  - a text PDF;
  - an image-only PDF;
  - a mixed PDF, with the signature page as an image;
  - a mixed PDF with a required field on its image page;
  - JPEG, PNG, HEIC and TIFF "photos" of a synthetic table;
  - a JPEG renamed `.pdf`;
  - a PDF renamed `.jpg`.
- **Never** a photo or scan of a real document, even with figures blanked out.

## Out of scope

- **OCR or any figure reading from images.** It is not in Tool 1 at all. If the firm later wants it, it is a new requirement, and it would need a design that keeps OCR'd figures out of the finding path until a person confirms them (CLAUDE.md invariant 2).
- **Password-protected or corrupt PDFs.** These are SPEC-030 (`PARSE_ERROR`).
- **Images inside Word or Excel files**, such as a pasted screenshot of a table. The adapters already read only real cells and tables (SPEC-018, SPEC-019). Recommendation: those specs add a validation-report warning, "contains N images that were not read".
- **UI layout of the messages.** That is SPEC-026, which shows these codes in the upload panel and the Client file.

## Design notes

- Sniffing lives in `internal/ingest` (engine side, network-free), so the server and the engine use the same code and cannot disagree about what a file is.
- The text-layer check is deterministic: the same PDF always gives the same page classification. No thresholds on image size, colour or "how scanned it looks".
- **Searchable scans (open question S-Q1).** Some scanners and phone apps add their own invisible OCR text layer to a scanned PDF. A per-page text check would see "text" and read guessed figures, which would bring OCR in by the back door.
  - **Proposal:** treat such pages as image pages.
  - **Detection to be proven in this spec:** text drawn in invisible render mode (`3 Tr`) over a full-page image, and known OCR fonts (for example `GlyphLessFont`) reported by Poppler `pdffonts`.
  - If no reliable detection is found, the spec is **Blocked** on S-Q1 rather than shipping a guess.
  - Needs owner/CA sign-off: refusing a searchable scan may surprise staff who think "the PDF has text".
- §B2.1 step 1 (sniff before the vault) and §B17 T7 were updated when this spec was written. Any change found during the build updates them again in the same commit.

## Acceptance criteria

- [ ] AC-1 — `Sniff` identifies every fixture type correctly, including the JPEG renamed `.pdf` and the PDF renamed `.jpg`. Unknown bytes give `UNSUPPORTED_FILE_TYPE`.
- [ ] AC-2 — Uploading each photo fixture returns `IMAGE_NOT_SUPPORTED` with the message above. Nothing is written to the vault (test checks the vault is unchanged). The audit entry has name, size, SHA-256 and reason, but no bytes.
- [ ] AC-3 — In one multi-file upload of a text PDF, a photo and a JSON file, the photo is refused and the other two are processed.
- [ ] AC-4 — The image-only PDF is rejected with `SCANNED_PDF_NOT_SUPPORTED`. It produces no facts, and no profile detection runs.
- [ ] AC-5 — The mixed PDF with a scanned signature page extracts all facts from its text pages, and the validation report lists the image page under "Not read".
- [ ] AC-6 — The mixed PDF with a required field on its image page gives `PDF_PAGE_IS_IMAGE` for that page, never a zero. In an end-to-end run, the rule needing that fact gives Cannot check, not a finding or a pass.
- [ ] AC-7 — `TestNoOCR` passes. It is proven by break-and-restore: a dummy `tesseract.exe` placed in `third_party/` and a dummy import path containing `gosseract` are each reported, then removed.
- [ ] AC-8 — The banned-word check (§B4.6) passes over all three new messages.
- [ ] AC-9 — S-Q1 is decided and recorded here. If the decision is "refuse searchable scans", a synthetic searchable-scan fixture is detected and refused.
- [ ] AC-10 — Same input, same output: classification of every fixture is identical across two runs and on Windows.
- [ ] AC-11 — The engine archtest still passes (no network; `os/exec` only).
- [ ] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

```
go test -count=1 ./internal/ingest/... -run 'Sniff|Scanned|Image' -v
go test -count=1 ./internal/api/... -run 'UploadImage|UploadMixed' -v
go test -count=1 ./internal/archtest/ -run TestNoOCR -v
go test -count=1 ./...
```

On the Windows test PC (Q5), upload a phone photo of a synthetic table and a scanned synthetic PDF through the browser, and confirm both messages appear as written.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |

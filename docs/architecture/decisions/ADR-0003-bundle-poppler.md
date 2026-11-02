# ADR 0003 — Bundle Poppler pdftotext (GPL) with Tool 1

- **Status:** Accepted (project owner, 14 October 2026)
- **Affects:** technical spec §A3 (bundled `pdftotext.exe`), §B3 (PDF profiles), the PDF adapter spec, `third_party/`

## Context

Text PDFs (the fallback when a source gives no JSON or Excel) must be read with their column layout intact. Naive extraction reorders table columns and produces confident false findings. Poppler's `pdftotext -layout` keeps the layout. Poppler is GPL-licensed, and the project owner had to decide whether it may ship with the tool.

## Decision

The owner accepted the GPL terms. Poppler 26.09.0 `pdftotext.exe` and the 32 DLLs it loads are kept in `third_party/poppler/bin/`, unchanged from the `oschwartz10612/poppler-windows` release `v26.09.0-0`. The download's SHA-256 matches the published digest, and every file is listed in `third_party/poppler/SHA256SUMS`. `third_party/poppler/README.md` records the source code location, the licences and the build origin.

The tool runs `pdftotext.exe` as a **separate program**, passing a local file path on the command line and reading its text output. No Tool 1 code links Poppler, so Tool 1 is not a derivative of it and its own code does not become GPL. Shipping it obliges us to include the GPL text (`COPYING`, `COPYING3`) and to provide or offer the corresponding source code (the README does both).

`internal/ingest/adapters/pdftext/poppler_test.go` runs the bundled program on Windows against a generated PDF whose right-hand column is drawn before its left-hand column. It passes only when each row comes out in page order. With `-raw` instead of `-layout` it fails, printing amounts before their labels, which is the known failure mode.

## Consequences

- About 54 MB of DLLs in the repository; `icudt78.dll` alone is 33 MB.
- **Network libraries ship with it.** `poppler.dll` loads `libcurl.dll`, `libssh2.dll` and `libcrypto-3-x64.dll`, because Poppler can open a PDF from a URL. `pdftotext.exe` is a separate process outside `recon-engine`, so the architecture guard (ADR 0001) does not see these DLLs. Rules for the PDF adapter:
  - it passes only absolute paths of files already in the vault, never a URL;
  - it runs `pdftotext.exe` with no network-related arguments.
  
  A Poppler build without curl (`ENABLE_LIBCURL=OFF`) would remove this exposure and is worth doing before release.
- The full licence texts of the bundled libraries (FreeType, ICU, OpenSSL, curl and the others listed in the README) must be added before the first release to a firm.
- The bundled build is Windows-only. On Linux the PDF test is skipped, which matches what ships.
- Bash shells with restricted DLL loading (such as some sandboxes) cannot start `pdftotext.exe`; run the PDF test from PowerShell or cmd there.

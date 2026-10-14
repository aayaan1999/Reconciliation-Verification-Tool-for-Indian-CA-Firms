# Poppler pdftotext (bundled)

`bin/pdftotext.exe` reads the text of text-based PDFs, keeping the page's column layout with
`-layout` (technical spec §B3, IN-05). The tool runs it as a **separate program** on local files
only; no Tool 1 code links Poppler. Decision and reasoning: `docs/adr/0003-bundle-poppler.md`.

## What is here

| Item | Detail |
|---|---|
| Version | Poppler 26.09.0 |
| Windows build | `Release-26.09.0-0.zip` from https://github.com/oschwartz10612/poppler-windows/releases/tag/v26.09.0-0 (built from conda-forge packages) |
| Download SHA-256 | `7a6f256a0ddf7536182246a5733331bf4677cbcc34f4663774947ad34556c8d0` (matches the digest GitHub publishes for the asset) |
| `bin/` | `pdftotext.exe` and the 32 DLLs it loads, copied unchanged from `Library/bin` of that zip. `jpeg8.dll` is loaded late and does not appear in the import table; it was found by testing. |
| `SHA256SUMS` | Checksum of every file in `bin/` |
| Not included | Every other Poppler program and DLL in the zip, and `poppler-data` (CMaps for Chinese, Japanese and Korean fonts). Add `share/poppler` from the same zip if such PDFs ever need reading. |

## Licence and source code (GPL)

Poppler is licensed under the GNU GPL, version 2 or later (`COPYING`, `COPYING3`). Anyone who
receives this program may get its complete source code:

- Poppler source: https://poppler.freedesktop.org/poppler-26.09.0.tar.xz
  (SHA-256 `8059eadb6805340768f138c465b57f8164c92b4a0773c37ef031ea6c0d987b2e`)
- Windows build recipe: https://github.com/oschwartz10612/poppler-windows (tag `v26.09.0-0`)
  and the conda-forge feedstocks it names, for the libraries in `bin/`.

The firm may ask the maintainers of Tool 1 for a copy of the exact sources at any time.

## Libraries in `bin/` and their licences

| Files | Project | Licence |
|---|---|---|
| `poppler.dll` | Poppler | GPL-2.0-or-later |
| `freetype.dll` | FreeType | FreeType Licence (or GPL-2.0) |
| `icuuc78.dll`, `icudt78.dll` | ICU | Unicode Licence v3 |
| `lcms2.dll` | Little CMS | MIT |
| `openjp2.dll` | OpenJPEG | BSD-2-Clause |
| `jpeg8.dll` | libjpeg-turbo | IJG + BSD-3-Clause |
| `libpng16.dll` | libpng | libpng Licence |
| `tiff.dll` | LibTIFF | libtiff Licence |
| `lerc.dll` | LERC | Apache-2.0 |
| `deflate.dll` | libdeflate | MIT |
| `zlib.dll` | zlib | zlib Licence |
| `zstd.dll` | Zstandard | BSD-3-Clause |
| `liblzma.dll` | XZ Utils | 0BSD |
| `libcurl.dll` | curl | curl Licence |
| `libssh2.dll` | libssh2 | BSD-3-Clause |
| `libcrypto-3-x64.dll` | OpenSSL 3 | Apache-2.0 |
| `psl-5.dll` | libpsl | MIT |
| `vcruntime140.dll`, `vcruntime140_1.dll`, `msvcp140.dll`, `api-ms-win-crt-*.dll` | Microsoft Visual C++ and Universal C runtime | Microsoft redistributable licence |

**Open before the first release to a firm:** the full licence texts of the libraries above must
be added next to this file. Most of them require their notice to travel with the program.

## Network libraries

`libcurl.dll`, `libssh2.dll` and `libcrypto-3-x64.dll` are loaded because Poppler can open a PDF
from a web address. Tool 1 only ever passes a local file path, which the PDF adapter must check
(SPEC for the PDF adapter). These DLLs are outside recon-engine, so the architecture guard does
not cover them; see ADR 0003.

# Licences of everything shipped with Tool 1

Every library linked into a shipped program, and every bundled tool, with its licence (SPEC-002 AC-6). All are free and open source. No commercial library is used (for example, UniOffice is not used).

`TestEveryLinkedModuleHasALicenceRecord` in `internal/archtest` fails if a module linked into any program under `cmd/` is missing from the first table, so a new library cannot be added without recording its licence here. Before sharing the tool with other firms, check the obligations of the GPL tool (Poppler) in `docs/architecture/decisions/ADR-0003-bundle-poppler.md`.

## Go modules linked into the programs

| Module | Version | Licence | Used for |
|---|---|---|---|
| `github.com/ncruces/go-sqlite3` | v0.35.6 | MIT | The SQLite database, in pure Go (no cgo) |
| `github.com/ncruces/go-sqlite3-wasm/v6` | v6.3.35304 | MIT No Attribution (SQLite itself is public domain) | SQLite compiled for the pure-Go driver |
| `github.com/ncruces/julianday` | v1.0.0 | MIT | Date handling inside the SQLite driver |
| `github.com/xuri/excelize/v2` | v2.11.0 | BSD-3-Clause | Reading and writing Excel files |
| `github.com/xuri/efp` | v0.0.1 | BSD-3-Clause | Excel formula parser (used by excelize) |
| `github.com/xuri/nfp` | v0.0.2-0.20250530014748-2ddeb826f9a9 | BSD-3-Clause | Excel number-format parser (used by excelize) |
| `github.com/richardlehane/mscfb` | v1.0.7 | Apache-2.0 | Old Office file container (used by excelize) |
| `github.com/richardlehane/msoleps` | v1.0.6 | Apache-2.0 | Old Office file properties (used by excelize) |
| `github.com/tiendc/go-deepcopy` | v1.7.2 | MIT | Copying values (used by excelize) |
| `go.yaml.in/yaml/v3` | v3.0.4 | MIT and Apache-2.0 | Reading the YAML content files |
| `golang.org/x/crypto` | v0.57.0 | BSD-3-Clause | Cryptography helpers (used by the database encryption and excelize) |
| `golang.org/x/net` | v0.58.0 | BSD-3-Clause | Text-encoding detection for old Excel files (parse only; see ADR 0001) |
| `golang.org/x/sys` | v0.48.0 | BSD-3-Clause | Windows file access for the database |
| `golang.org/x/text` | v0.42.0 | BSD-3-Clause | Text encodings (used by excelize) |

The Go standard library and toolchain are BSD-3-Clause.

## Browser UI (npm, bundled into the UI build)

| Package | Licence | Used for |
|---|---|---|
| `react`, `react-dom`, `scheduler` | MIT | The screens |

Build and test tools (Vite, TypeScript, Vitest, oxlint, Testing Library) are used only on the developer machine and in CI; they are not shipped.

## Bundled tools

| Tool | Licence | Notes |
|---|---|---|
| Poppler 26.09.0 `pdftotext.exe` and its DLLs (`third_party/poppler/`) | GPL-2.0-or-later, with the licences in `third_party/poppler/README.md` | Run as a separate program, never linked in (ADR 0003) |

## Developer tools (not shipped)

| Tool | Licence |
|---|---|
| Go | BSD-3-Clause |
| Git | GPL-2.0 (tool only) |
| Node.js and npm | MIT / Artistic-2.0 |
| WiX Toolset 7.0.0 (Windows installer), installed with `dotnet tool install --global wix` | Source under MS-RL. Since v6 the released binaries also come with the Open Source Maintenance Fee EULA, which can require a fee from organisations that earn revenue. **Open decision:** pay the fee, pin WiX v5 (MS-RL only), or build WiX from source. See SPEC-002. |
| .NET SDK 10.0.402 (needed to run WiX) | MIT |
| Microsoft Word | Already licensed by the firm; used only to read `docs/Tool1-File-Guide.docx` |

# ADR 0001 — How the engine proves it has no network code; SQLite driver and excelize

- **Status:** Accepted
- **Date:** 14 October 2026
- **Affects:** CLAUDE.md invariant 5, technical spec §A4.1 and risk T1, SPEC-010, SPEC-018, SPEC-034

## Context

`recon-engine` must never be able to open a network connection (CLAUDE.md invariant 5, spec §4.1, C3). Until now `internal/archtest` enforced this with one rule: no package whose import path is `net`, `net/*`, `crypto/tls` or `golang.org/x/net/*` may appear anywhere in the engine's dependency graph.

Both libraries the plan depends on failed that rule, although neither can open a connection:

| Library | Import edge | What it is used for |
|---|---|---|
| `github.com/ncruces/go-sqlite3` (root, `driver`, `vfs`) | → `net/url` | Parsing SQLite URI file names such as `file:recon.db?mode=ro` |
| `github.com/xuri/excelize/v2` | → `net/url` | The `ENCODEURL()` worksheet function (text escaping) |
| `github.com/xuri/excelize/v2` | → `golang.org/x/net/html/charset` | Decoding workbooks saved in legacy encodings such as Windows-1252 |
| `golang.org/x/sys/windows` (used by the SQLite driver for file mapping and locking) | → `net` | One helper, `SocketAddress.IP`, returns a `net.IP` value |

The alternative pure-Go driver `modernc.org/sqlite` has the same `net/url` and `x/sys/windows` edges, plus `github.com/google/uuid` → `net` on Linux.

A Windows program that links both libraries and uses them (opens a database, reads a workbook) was built and inspected with `go tool nm`. It contained no function from package `net` other than its initialisers, and no socket system call. Go's linker drops unreachable functions, so importing `net` for a type name does not bring in connection code. A program that calls `net.Dial` contains about 270 `net` functions plus `syscall.Socket`, `Connect`, `Bind`, `Listen` and `GetAddrInfoW`.

The import-path rule was therefore both too strict (it rejected harmless parsing) and indirect (it described what *could* be linked, not what *was*).

## Decision

1. **The import rule stays, with a reviewed list of exceptions.** `allowedNetworkImports` in `internal/archtest/archtest_test.go` lists individual import edges (importer → network package), not packages. Only third-party packages may appear; the test fails if this module's own code is listed. Any other path to a network package still fails. An edge that is no longer used must be removed (the test fails on stale entries).
2. **A binary check proves the result.** `TestEngineBinaryHasNoNetworkCode` builds every network-free command (`recon-engine`, `recon-updater`) for windows/amd64 and linux/amd64 and fails if the linked program contains:
   - any function from `net` other than its initialisers;
   - any function from `net/http`, `crypto/tls` or another network package that is not parse-only (`net/url`, `net/netip`, `golang.org/x/net/html` and its `atom`, `charset` subpackages);
   - a socket system call wrapper (`Socket`, `Connect`, `Bind`, `Listen`, `Accept*`, `WSASocket`, `GetAddrInfoW`, `DnsQuery` and similar) in `syscall`, `golang.org/x/sys/windows` or `golang.org/x/sys/unix`.

   `WSAStartup` is allowed: it only initialises Winsock and package `os` links it into every Windows program.
   It writes the network-package functions that are present to `build/engine-symbols-<goos>-<goarch>.txt`, which CI keeps next to `engine-deps-*.txt`.
3. **The check is tested against a real offender.** `internal/archtest/testdata/dialer` calls `net.Dial`; `TestBinaryCheckCatchesNetworkCode` fails if the binary check does not catch it on every target.
4. **SQLite driver: `github.com/ncruces/go-sqlite3` v0.35.6.** No cgo; fewer network edges than `modernc.org/sqlite`; its encryption VFS is a candidate to evaluate for SPEC-034. Registered in `internal/store/driver.go`.
5. **excelize v2.11.0** is linked from `internal/ingest/adapters/xlsx` so the guard checks it from now on.

## Consequences

- Upgrading either library re-runs both checks. A new network edge fails the import check until someone reviews it and adds it here and to `allowedNetworkImports`.
- The binary check only covers code that `recon-engine` actually links. Today the engine is a placeholder that does not yet call the store or the Excel adapter; the import check covers them now, and the binary check covers them automatically from SPEC-010/SPEC-011 when the engine starts using them.
- Limit: a library could load `ws2_32.dll` by name at run time without any of the listed symbols. Neither library does this; any new third-party engine dependency is reviewed with that in mind.
- The guard takes about 10 seconds longer because it builds the commands twice per target.

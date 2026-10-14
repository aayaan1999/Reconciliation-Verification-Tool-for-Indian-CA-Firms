package store

// The SQLite driver is ncruces/go-sqlite3: SQLite compiled to WebAssembly and run in pure Go,
// so the shipped build needs no cgo. It registers itself with database/sql as "sqlite3".
// Chosen in docs/adr/0001-engine-network-proof.md; SPEC-010 builds the store on it.
//
// The "xts" VFS encrypts the database file at rest with AES-XTS (docs/adr/0002-database-encryption.md).
// Open with "file:recon.db?vfs=xts&hexkey=<128 hex digits>": a 64-byte key selects AES-256.
// Never use textkey, which derives only an AES-128 key.
import (
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/vfs/xts"
)

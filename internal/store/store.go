package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

// A DB is an open recon database. It holds one connection, so every write of the process
// goes through a single writer (technical spec §A3.2).
type DB struct {
	db *sql.DB
}

// Open opens the plain (unencrypted) SQLite file at file, creating it if needed, and
// applies every pending migration in one transaction (§B1). Encryption at rest comes in
// Module 14 (ADR 0002).
func Open(file string) (*DB, error) {
	db, err := sql.Open("sqlite3", dsn(file))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	d := &DB{db: db}
	for _, p := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("store: %s: %w", p, err)
		}
	}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// dsn turns a file path into an SQLite URI, escaping the characters a URI gives meaning to.
func dsn(file string) string {
	p := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(filepath.ToSlash(file))
	return "file:" + p
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

// migrate applies the numbered files in migrations/ that are newer than the database, all
// in one transaction: either every pending migration is applied or none is.
func (d *DB) migrate() error {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	slices.Sort(names)
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	var current int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	applied := false
	for _, name := range names {
		base := path.Base(name)
		num, _, _ := strings.Cut(base, "_")
		version, err := strconv.Atoi(num)
		if err != nil {
			return fmt.Errorf("store: migration %s does not start with a number", base)
		}
		if version <= current {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			return fmt.Errorf("store: migration %s: %w", base, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, version, base); err != nil {
			return fmt.Errorf("store: migration %s: %w", base, err)
		}
		applied = true
	}
	if !applied {
		return nil // nothing written: a read-only database can still be read
	}
	// SPEC-022: versions saved before migration 0015 get their document type from the profile.
	if err := backfillFormatDocumentTypes(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}

// A Tx is one write transaction. Everything a command writes goes through one Tx, so a
// failed command leaves nothing partial behind.
type Tx struct {
	tx *sql.Tx
}

// Begin starts a write transaction.
func (d *DB) Begin() (*Tx, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("store: begin: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// Commit makes the transaction's writes permanent.
func (t *Tx) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// Rollback discards the transaction's writes. It does nothing after Commit.
func (t *Tx) Rollback() { _ = t.tx.Rollback() }

// A ConflictError is a request the stored data refuses, such as a rule ID loaded from
// two different sources. It is the caller's input problem, not a database failure.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

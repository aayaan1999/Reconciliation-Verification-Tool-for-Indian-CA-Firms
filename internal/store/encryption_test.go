package store

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Illustrative values only: no real client data and no real key.
const (
	testClient = "Illustrative Traders Private Limited"
	testHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
		"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
	otherHexKey = "ff0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
		"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
)

func encryptedDSN(path, hexKey string) string {
	return "file:" + filepath.ToSlash(path) + "?vfs=xts&hexkey=" + hexKey
}

func openDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open %s: %v", dsn, err)
	}
	return db
}

// TestEncryptedDatabase checks the encryption chosen in ADR 0002: with the key the data
// reads back; without it, or with the wrong key, the file is not a database; and the
// file on disk holds no readable client name.
func TestEncryptedDatabase(t *testing.T) {
	if len(testHexKey) != 128 {
		t.Fatalf("test key has %d hex digits; AES-256-XTS needs 128", len(testHexKey))
	}
	path := filepath.Join(t.TempDir(), "recon.db")

	db := openDB(t, encryptedDSN(path, testHexKey))
	for _, stmt := range []string{
		"PRAGMA temp_store = memory",
		"CREATE TABLE client (name TEXT NOT NULL)",
		"INSERT INTO client (name) VALUES ('" + testClient + "')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{testClient, "Illustrative", "SQLite format 3", "CREATE TABLE"} {
		if bytes.Contains(raw, []byte(needle)) {
			t.Errorf("the database file contains readable text %q", needle)
		}
	}

	db = openDB(t, encryptedDSN(path, testHexKey))
	var name string
	if err := db.QueryRow("SELECT name FROM client").Scan(&name); err != nil || name != testClient {
		t.Errorf("reading back with the key: name %q, err %v", name, err)
	}
	db.Close()

	for label, dsn := range map[string]string{
		"plain SQLite": "file:" + filepath.ToSlash(path),
		"wrong key":    encryptedDSN(path, otherHexKey),
	} {
		db := openDB(t, dsn)
		err := db.QueryRow("SELECT name FROM client").Scan(&name)
		db.Close()
		if err == nil || !strings.Contains(err.Error(), "not a database") {
			t.Errorf("%s: want a 'not a database' error, got %v (name %q)", label, err, name)
		}
	}
}

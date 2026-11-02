package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withStdin runs cmd with standard input holding s.
func withStdin(t *testing.T, s string, args ...string) result {
	t.Helper()
	old := stdin
	stdin = strings.NewReader(s)
	defer func() { stdin = old }()
	return cmd(t, args...)
}

// The PAN and GSTIN are the synthetic ones of the sample GST pack (testdata/gst_pack_aug2026).
const deccanJSON = `{"legal_name": "Deccan Byte Systems Pvt Ltd", "pan": "zlvcd9565z", "constitution": "company",
 "gstins": [{"gstin": "27ZLVCD9565Z1ZF", "registration_type": "regular", "primary": true}],
 "address": {"city": "Pune", "state": "27", "pin": "411001"}}`

// AC-25: a client is added with all its details, edited, and read back without its PAN.
func TestClientAddUpdateGet(t *testing.T) {
	db := filepath.Join(t.TempDir(), "recon.db")
	r := withStdin(t, deccanJSON, "client-add", "--db", db, "--tax-year", "2026-27")
	if r.code != exitOK || !strings.Contains(r.stdout, "client 1 added: Deccan Byte Systems Pvt Ltd (PAN ending 565Z)") {
		t.Fatalf("client-add: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}

	// A mistyped GSTIN is refused, naming the field and the reason.
	bad := strings.Replace(deccanJSON, "1ZF", "1ZG", 1)
	r = withStdin(t, bad, "client-add", "--db", filepath.Join(t.TempDir(), "x.db"), "--tax-year", "2026-27")
	if r.code != exitInput || !strings.Contains(r.stderr, "INVALID_CLIENT") || !strings.Contains(r.stderr, "gstins[0]: GSTIN_CHECK_DIGIT") {
		t.Errorf("mistyped GSTIN: exit %d\n%s", r.code, r.stderr)
	}

	// Editing without the PAN keeps it; a GSTIN of another PAN is refused against the stored hash.
	edit := strings.Replace(deccanJSON, `"pan": "zlvcd9565z", `, "", 1)
	edit = strings.Replace(edit, `"city": "Pune"`, `"city": "Mumbai"`, 1)
	if r = withStdin(t, edit, "client-update", "--db", db, "--client", "1"); r.code != exitOK {
		t.Fatalf("client-update: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	foreign := strings.Replace(edit, `"gstins": [{"gstin": "27ZLVCD9565Z1ZF"`, `"gstins": [{"gstin": "27AAACI1234Q1ZX"`, 1)
	if r = withStdin(t, foreign, "client-update", "--db", db, "--client", "1"); r.code != exitInput || !strings.Contains(r.stderr, "INVALID_CLIENT") {
		t.Errorf("GSTIN of another PAN: exit %d\n%s", r.code, r.stderr)
	}
	if r = withStdin(t, edit, "client-update", "--db", db, "--client", "9"); r.code != exitInput || !strings.Contains(r.stderr, "CLIENT_NOT_FOUND") {
		t.Errorf("unknown client: exit %d\n%s", r.code, r.stderr)
	}

	out := filepath.Join(t.TempDir(), "client.json")
	if r = cmd(t, "client-get", "--db", db, "--client", "1", "--json", out); r.code != exitOK {
		t.Fatalf("client-get: exit %d\n%s", r.code, r.stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(data)), `"PAN":`) {
		t.Errorf("client-get wrote the full PAN: %s", data)
	}
	var got struct {
		ID      int64 `json:"id"`
		Details struct {
			PANLast4 string `json:"pan_last4"`
			Address  struct {
				City string `json:"city"`
			} `json:"address"`
		} `json:"details"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 || got.Details.PANLast4 != "565Z" || got.Details.Address.City != "Mumbai" {
		t.Errorf("client-get = %s", data)
	}
}

package currency

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// SPEC-019 AC-6, AC-7: the rate file gives a rate, or says why there is none. Values are illustrative.

func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadValidRate(t *testing.T) {
	r := Load(write(t, "schema: \"recon.currency.usd/1\"\nusd_rate: \"83.2500\"\neffective_from: \"2026-04-01\"\nnote: \"\"\n"))
	if !r.OK() || r.Value != money.USDRate(832500) || r.Effective != "2026-04-01" {
		t.Errorf("rate %+v", r)
	}
	// Changing the file changes the rate with no code change (AC-7).
	dir := write(t, "schema: \"recon.currency.usd/1\"\nusd_rate: \"84\"\neffective_from: \"\"\nnote: \"\"\n")
	if r := Load(dir); !r.OK() || r.Value != money.USDRate(840000) {
		t.Errorf("changed rate %+v", r)
	}
}

func TestLoadProblems(t *testing.T) {
	const head = "schema: \"recon.currency.usd/1\"\n"
	cases := []struct {
		name, body, want string
	}{
		{"empty file", "", ReasonNotSet},
		{"blank file", "  \n\n", ReasonNotSet},
		{"empty rate", head + "usd_rate: \"\"\n", ReasonNotSet},
		{"non-numeric", head + "usd_rate: \"eighty\"\n", ReasonBad},
		{"zero", head + "usd_rate: \"0\"\n", ReasonBad},
		{"negative", head + "usd_rate: \"-83\"\n", ReasonBad},
		{"five decimals", head + "usd_rate: \"83.12345\"\n", ReasonBad},
		{"wrong schema", "schema: \"other\"\nusd_rate: \"83\"\n", ReasonBad},
		{"unknown key", head + "usd_rate: \"83\"\nextra: \"x\"\n", ReasonBad},
		{"bad date", head + "usd_rate: \"83\"\neffective_from: \"01-04-2026\"\n", ReasonBad},
		{"not yaml", "usd_rate: [", ReasonBad},
	}
	for _, c := range cases {
		r := Load(write(t, c.body))
		if r.OK() || r.Problem != c.want || r.Value != 0 || r.Detail == "" {
			t.Errorf("%s: %+v, want problem %q", c.name, r, c.want)
		}
	}
	if r := Load(filepath.Join(t.TempDir(), "nowhere")); r.OK() || r.Problem != ReasonNotSet {
		t.Errorf("missing file: %+v", r)
	}
	if r := None(); r.OK() || r.Problem != ReasonNotSet {
		t.Errorf("None: %+v", r)
	}
}

// The shipped file holds no rate until the CA sets one, so nothing is converted at a guessed rate.
func TestShippedFileSetsNoRate(t *testing.T) {
	r := Load(filepath.Join("..", "..", "content", "currency"))
	if r.OK() || r.Problem != ReasonNotSet {
		t.Errorf("shipped rate file: %+v, want no rate set", r)
	}
}

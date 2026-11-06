package currency

import "testing"

// The note is carried out of the file as the reader decoded it, so every quoting the reader accepts gives the same
// text, and a file the reader refuses gives none. Values are illustrative.
func TestLoadCarriesTheNote(t *testing.T) {
	head := "schema: \"recon.currency.usd/1\"\nusd_rate: \"83.2500\"\neffective_from: \"2026-04-01\"\n"
	for name, tc := range map[string]struct{ line, want string }{
		"double quotes": {`note: "from the bank"`, "from the bank"},
		"single quotes": {`note: 'from the bank'`, "from the bank"},
		"comment after": {`note: "from the bank"   # checked`, "from the bank"},
		"escapes":       {`note: "card rate \u00e9"`, "card rate \u00e9"},
		"empty":         {`note: ""`, ""},
	} {
		if r := Load(write(t, head+tc.line+"\n")); !r.OK() || r.Note != tc.want {
			t.Errorf("%s: %+v, want a usable rate with note %q", name, r, tc.want)
		}
	}
	// a note with no rate still comes through, for the page that shows why there is no rate
	if r := Load(write(t, "schema: \"recon.currency.usd/1\"\nusd_rate: \"\"\nnote: \"waiting for the bank\"\n")); r.OK() || r.Note != "waiting for the bank" {
		t.Errorf("a note with no rate: %+v", r)
	}
	if r := Load(write(t, head+"note: from the bank\n")); r.OK() || r.Note != "" {
		t.Errorf("a file the reader refuses: %+v, want no rate and no note", r)
	}
}

package profile

import "testing"

// A message names its place as a sheet and cell, but a longer path that only starts with
// the same text names a different place and is left alone (code review finding 5).
func TestReplacePathWholeOccurrencesOnly(t *testing.T) {
	for _, c := range []struct{ msg, old, n, want string }{
		{"row $['Sales Register'][1] has no total.", "$['Sales Register'][1]", "Sales Register!3:3",
			"row Sales Register!3:3 has no total."},
		{"row $['Sales Register'][1]: $['Sales Register'][1].['Taxable Value'] is empty", "$['Sales Register'][1]", "Sales Register!3:3",
			"row Sales Register!3:3: $['Sales Register'][1].['Taxable Value'] is empty"},
		{"$.Ledger and $.Ledgers[2]", "$.Ledger", "Ledger!A1", "Ledger!A1 and $.Ledgers[2]"},
		{"$.rows[1] and $.rows[10]", "$.rows[1]", "S!2:2", "S!2:2 and $.rows[10]"},
		{"$.rows[1].amount", "$.rows[1]", "S!2:2", "$.rows[1].amount"},
		{"nothing here", "$.x", "S!A1", "nothing here"},
	} {
		if got := replacePath(c.msg, c.old, c.n); got != c.want {
			t.Errorf("replacePath(%q, %q):\n got %q\nwant %q", c.msg, c.old, got, c.want)
		}
	}
}

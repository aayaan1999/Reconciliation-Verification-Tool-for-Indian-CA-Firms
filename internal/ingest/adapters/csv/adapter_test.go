package csvadapter

import (
	"errors"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
)

func open(t *testing.T, text string) (ingest.Doc, error) {
	t.Helper()
	return New(0).Open(strings.NewReader(text), int64(len(text)))
}

func TestRowsBecomeObjects(t *testing.T) {
	doc, err := open(t, "\xef\xbb\xbfperiod,ledger_head,amount_inr\r\n2025-04,\"Rent, office\",23405\n2025-05,Rent,\"1,23,405\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc.FileType() != "csv" {
		t.Errorf("file type %q", doc.FileType())
	}
	sel, _ := jsonadapter.ParsePath("$[*].amount_inr")
	cells, err := doc.Select(sel)
	if err != nil || len(cells) != 2 || cells[0].Text != "23405" || cells[1].Text != "1,23,405" || cells[1].Locator.Path != "$[1].amount_inr" {
		t.Errorf("cells %+v, err %v", cells, err)
	}
	// The byte-order mark is not part of the first column's name.
	first, _ := jsonadapter.ParsePath("$[0].period")
	if c, _ := doc.Select(first); len(c) != 1 || c[0].Text != "2025-04" {
		t.Errorf("first column after a byte-order mark: %+v", c)
	}
	head, _ := jsonadapter.ParsePath("$[0].ledger_head")
	if c, _ := doc.Select(head); len(c) != 1 || c[0].Text != "Rent, office" {
		t.Errorf("quoted cell %+v", c)
	}
	if ll, ok := doc.(ingest.LeafLister); !ok || len(ll.Leaves()) != 6 {
		t.Error("the leaves of every cell are not listed")
	}
}

func TestRefusals(t *testing.T) {
	for name, text := range map[string]string{
		"a column without a name": "period,,amount\n2025-04,x,1\n",
		"a column named twice":    "period,amount,amount\n2025-04,1,2\n",
		"a short row":             "period,amount\n2025-04\n",
		"a quote left open":       "period,amount\n\"2025-04,1\n",
		"not UTF-8":               "period,amount\n\xff\xfe,1\n",
	} {
		_, err := open(t, text)
		var ie *ingest.Error
		if !errors.As(err, &ie) || ie.Code != ingest.ParseError {
			t.Errorf("%s: err %v, want PARSE_ERROR", name, err)
		}
	}
	_, err := open(t, "period,amount\n2025-04\n")
	var ie *ingest.Error
	if errors.As(err, &ie) && ie.Line != 2 {
		t.Errorf("short row reported on line %d, want 2", ie.Line)
	}
}

func TestSniff(t *testing.T) {
	a := New(0)
	for text, want := range map[string]bool{
		"period,ledger_head,amount_inr\n2025-04,x,1\n": true,
		"\xef\xbb\xbfperiod,amount\n":                  true,
		`{"a": 1}`:                                     false,
		"[1, 2]":                                       false,
		"%PDF-1.7\n":                                   false,
		"just a line\n":                                false,
	} {
		if got := a.Sniff([]byte(text), "x"); got != want {
			t.Errorf("Sniff(%q) = %v, want %v", text, got, want)
		}
	}
}

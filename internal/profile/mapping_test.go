package profile

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

func TestGeneralise(t *testing.T) {
	g, items := generalise("$.b2b[3].inv[0]['Bill [1] No']")
	if g != "$.b2b[*].inv[*]['Bill [1] No']" {
		t.Errorf("general = %q", g)
	}
	want := []listItem{{"$.b2b[*]", "$.b2b[3]"}, {"$.b2b[*].inv[*]", "$.b2b[3].inv[0]"}}
	if !slices.Equal(items, want) {
		t.Errorf("items = %v", items)
	}
}

func TestMappingNames(t *testing.T) {
	for name, slug := range map[string]string{
		"Tally sales export":   "tally_sales_export",
		"  Busy -- Purchase! ": "busy_purchase",
		"2B from portal":       "f_2b_from_portal",
		"***":                  "",
	} {
		if got := (Mapping{Name: name}).Slug(); got != slug {
			t.Errorf("Slug(%q) = %q, want %q", name, got, slug)
		}
	}
	m := Mapping{Name: "Tally sales export", DocumentType: "sales_register"}
	if m.ID() != "office.tally_sales_export.sales_register" || m.Dir() != "tally_sales_export/sales_register/v1" {
		t.Errorf("ID %q, Dir %q", m.ID(), m.Dir())
	}
}

func TestInspectLeavesListsFewValues(t *testing.T) {
	var leaves []ingest.Cell
	add := func(path, text string) {
		leaves = append(leaves, ingest.Cell{Locator: domain.Locator{Path: path}, Text: text, Kind: ingest.CellText})
	}
	for i, typ := range []string{"B2B", "B2C", "B2B", "CDN"} {
		add(fmt.Sprintf("$.rows[%d].type", i), typ)
	}
	for i := 0; i < MaxFieldValues+1; i++ {
		add(fmt.Sprintf("$.rows[%d].inv_no", i), fmt.Sprintf("INV-%d", i))
	}
	in := inspectLeaves(leaves)
	values := map[string][]string{}
	for _, f := range in.Fields {
		values[f.Path] = f.Values
	}
	if got := values["$.rows[*].type"]; !slices.Equal(got, []string{"B2B", "B2C", "CDN"}) {
		t.Errorf("type values = %v, want each different value once, in file order", got)
	}
	if got := values["$.rows[*].inv_no"]; got != nil {
		t.Errorf("inv_no values = %v, want none: too many kinds to choose among", got)
	}
}

// A row filter may name several values: the rows holding any one of them are read.
func TestMappingWhereAnyOf(t *testing.T) {
	header := []string{"Ledger Head", "Group", "Month", "Amount"}
	rows := [][]string{
		{"Rent", "Admin", "Apr-25", "100"},
		{"Salary", "Staff", "Apr-25", "200"},
		{"Travel", "Sales", "Apr-25", "300"},
	}
	doc := ledgerBook(t, header, rows, nil)
	in, err := Inspect(doc)
	if err != nil {
		t.Fatal(err)
	}
	var groups []string
	for _, f := range in.Fields {
		if strings.HasSuffix(f.Path, ".Group") {
			groups = f.Values
		}
	}
	if !slices.Equal(groups, []string{"Admin", "Staff", "Sales"}) {
		t.Fatalf("Group values = %v", groups)
	}
	m := Mapping{Name: "Books ledger export", DocumentType: "ledger", Rows: "$.Ledger[*]", Unit: "rupees", FileType: "xlsx",
		Where:   []MappingWhere{{Path: ".Group", In: []string{"Admin", "Sales"}}},
		Columns: []MappingColumn{{Name: "ledger_head", Path: "['Ledger Head']"}, {Name: "month", Path: ".Month"}, {Name: "amount", Path: ".Amount"}},
		Period:  MappingPeriod{Const: PeriodWholeYear}, TaxYear: "2025-26"}
	data, probs := m.YAML(realVocab(t))
	if len(probs) > 0 {
		t.Fatalf("problems %v", probs)
	}
	if !strings.Contains(string(data), `in: ["Admin", "Sales"]`) {
		t.Errorf("profile lacks the values:\n%s", data)
	}
	p, err := Load(m.Dir()+"/profile.yaml", data, realVocab(t))
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	res, err := Extract(p, doc, Source{FileName: "l.xlsx"})
	if err != nil || len(res.Records) != 2 {
		t.Fatalf("err %v, %d records, want the Admin and Sales rows", err, len(res.Records))
	}
	m.Where = []MappingWhere{{Path: ".Group"}}
	if _, probs := m.YAML(realVocab(t)); len(probs) != 1 || probs[0].Field != "where[0]" {
		t.Errorf("problems %v, want one for where[0]", probs)
	}
}

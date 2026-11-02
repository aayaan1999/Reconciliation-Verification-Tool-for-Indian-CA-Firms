package profile

import (
	"slices"
	"testing"
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

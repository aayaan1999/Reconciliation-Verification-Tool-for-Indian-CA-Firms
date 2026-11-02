package domain

import (
	"slices"
	"testing"
)

// Locators sort in file order, with row numbers read as numbers, and the order is total.
func TestCompareLocators(t *testing.T) {
	paths := []string{"$.rows[10]", "S!10:10", "$.rows[2]", "S!9:9", "$.rows[07]", "$.rows[7]", "$.rows", "$.rows[1].x"}
	locs := make([]Locator, len(paths))
	for i, p := range paths {
		locs[i] = Locator{FileName: "F.json", Path: p}
	}
	slices.SortFunc(locs, CompareLocators)
	var got []string
	for _, l := range locs {
		got = append(got, l.Path)
	}
	want := []string{"$.rows", "$.rows[1].x", "$.rows[2]", "$.rows[07]", "$.rows[7]", "$.rows[10]", "S!9:9", "S!10:10"}
	if !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
	if c := CompareLocators(Locator{FileName: "A", Path: "$[9]"}, Locator{FileName: "B", Path: "$[1]"}); c >= 0 {
		t.Errorf("file name not compared first: %d", c)
	}
	for _, p := range paths {
		if c := CompareLocators(Locator{Path: p}, Locator{Path: p}); c != 0 {
			t.Errorf("%s compared with itself gives %d", p, c)
		}
	}
}

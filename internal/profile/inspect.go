package profile

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
)

// An Inspection lists what a file holds, for the mapping screen (SPEC-004 part 8.4): every
// field with list positions written as [*], in file order, with the first value seen, and
// every list with how many items it has in all.
type Inspection struct {
	Fields    []InspectedField `json:"fields"`
	Lists     []InspectedList  `json:"lists"`
	Truncated bool             `json:"truncated"` // more than MaxInspectedFields fields; the rest are left out
}

// An InspectedField is one field of the file.
type InspectedField struct {
	Path    string `json:"path"`    // "$.rows[*].inv_no"
	Kind    string `json:"kind"`    // text | number | bool | null
	Example string `json:"example"` // the first value that is not empty, cut to 60 characters
	Count   int    `json:"count"`   // how many times it occurs
}

// An InspectedList is one list of the file, a candidate for the list of rows.
type InspectedList struct {
	Path  string `json:"path"`  // "$.rows[*]"
	Count int    `json:"count"` // items in all its occurrences
}

// MaxInspectedFields bounds an inspection, so a huge file cannot flood the screen.
const MaxInspectedFields = 2000

// Inspect lists the fields and lists of doc, which must be able to list its values. It is
// deterministic: the same file always gives the same inspection.
func Inspect(doc ingest.Doc) (*Inspection, error) {
	if wb, ok := doc.(*xlsxadapter.Doc); ok {
		return inspectWorkbook(wb)
	}
	ll, ok := doc.(ingest.LeafLister)
	if !ok {
		return nil, fmt.Errorf("%s files cannot be inspected", doc.FileType())
	}
	return inspectLeaves(ll.Leaves()), nil
}

// inspectWorkbook lists a workbook as the tables an Excel profile would read (SPEC-008 part
// 2): each sheet with a header row, its rows as "$['<sheet>'][*]" and its columns by header
// text ("$['Sales Register'][*]['Voucher No']"), the paths a taught Excel profile uses. A
// sheet whose guessed header cannot be read as a table (a label twice) is left out.
func inspectWorkbook(wb *xlsxadapter.Doc) (*Inspection, error) {
	var leaves []ingest.Cell
	for _, t := range wb.Guess() {
		v, err := wb.View([]xlsxadapter.Table{t})
		if err != nil {
			continue
		}
		leaves = append(leaves, v.Doc.Leaves()...)
	}
	if len(leaves) == 0 {
		return nil, fmt.Errorf("no sheet of the workbook has a header row with at least two labels and rows below it")
	}
	return inspectLeaves(leaves), nil
}

func inspectLeaves(leaves []ingest.Cell) *Inspection {
	in := &Inspection{Fields: []InspectedField{}, Lists: []InspectedList{}}
	fieldAt := map[string]int{}
	listAt := map[string]int{}
	itemSeen := map[string]bool{}
	for _, c := range leaves {
		general, items := generalise(c.Locator.Path)
		for _, it := range items {
			if itemSeen[it.concrete] {
				continue
			}
			itemSeen[it.concrete] = true
			i, ok := listAt[it.general]
			if !ok {
				i = len(in.Lists)
				listAt[it.general] = i
				in.Lists = append(in.Lists, InspectedList{Path: it.general})
			}
			in.Lists[i].Count++
		}
		i, ok := fieldAt[general]
		if !ok {
			if len(in.Fields) == MaxInspectedFields {
				in.Truncated = true
				continue
			}
			i = len(in.Fields)
			fieldAt[general] = i
			in.Fields = append(in.Fields, InspectedField{Path: general, Kind: string(c.Kind)})
		}
		f := &in.Fields[i]
		f.Count++
		if f.Example == "" && c.Text != "" {
			f.Example = cut(c.Text, 60)
			f.Kind = string(c.Kind)
		}
	}
	return in
}

type listItem struct{ general, concrete string }

// generalise turns a concrete JSON path ("$.b2b[3].inv[0].inum") into its general form
// ("$.b2b[*].inv[*].inum") and lists each list item it passes through.
func generalise(path string) (string, []listItem) {
	var g, c strings.Builder
	var items []listItem
	for i := 0; i < len(path); {
		switch {
		case strings.HasPrefix(path[i:], "['"): // a quoted key: copy it whole
			j := i + 2
			for j < len(path) && !(path[j] == '\'' && path[j-1] != '\\') {
				j++
			}
			j = min(j+2, len(path))
			g.WriteString(path[i:j])
			c.WriteString(path[i:j])
			i = j
		case path[i] == '[': // an index
			j := strings.IndexByte(path[i:], ']') + i + 1
			g.WriteString("[*]")
			c.WriteString(path[i:j])
			items = append(items, listItem{general: g.String(), concrete: c.String()})
			i = j
		default:
			g.WriteByte(path[i])
			c.WriteByte(path[i])
			i++
		}
	}
	return g.String(), items
}

func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// ExpectedFile writes what reading a sample gave, as an expected file (§B2.3), so a taught
// profile can be tested like a shipped one. For a taught profile it records what the user
// accepted on the mapping screen; for shipped profiles the file is verified by hand.
func ExpectedFile(p *Profile, res *Result) []byte {
	var b strings.Builder
	q := strconv.Quote
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# What reading the sample gave when the layout was taught (recon-engine profile-create).")
	line("schema: %s", q(ExpectedSchema))
	line("profile_id: %s", q(p.ID))
	line("profile_version: %s", q(p.Version))
	ty := ""
	if !res.Context.TaxYear.IsZero() {
		ty = res.Context.TaxYear.String()
	}
	line("context:")
	line("  tax_year: %s", q(ty))
	line("  period: %s", q(res.Context.Period))
	line("  pan_last4: %s", q(res.Report.PANLast4))
	line("  scope: %s", q(res.Context.Scope))
	line("  part: %s", q(res.Context.Part))
	if len(res.Facts) > 0 {
		line("facts:")
		for _, f := range res.Facts {
			line("  - { fact: %s, value: %s, period: %s, locator: %s }", q(f.Name), q(domain.FormatValue(f.Value)), q(f.Period), q(f.Locator.Path))
		}
	}
	if len(res.Records) > 0 {
		line("records:")
		for _, r := range res.Records {
			line("  - record_type: %s", q(r.RecordType))
			line("    match_key: %s", q(r.MatchKey))
			line("    locator: %s", q(r.Locator.Path))
			if len(r.Fields) > 0 {
				line("    fields:")
			}
			for _, k := range sortedKeys(r.Fields) {
				line("      - { name: %s, value: %s }", q(k), q(r.Fields[k]))
			}
			if len(r.Amounts) > 0 {
				line("    amounts:")
				for _, k := range sortedKeys(r.Amounts) {
					line("      - { name: %s, value: %s }", q(k), q(domain.FormatValue(domain.MoneyValue(r.Amounts[k]))))
				}
			}
		}
	}
	return []byte(b.String())
}

package findings

import (
	"fmt"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/modules/newheads"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// WorklistNotice heads the Worklist section: its items are for staff to classify, not
// findings (SPEC-006 part 3, CLAUDE.md invariant 2).
const WorklistNotice = "Ledger heads to classify (year-on-year; these are not findings, and the tool does not classify them)"

var worklistKindText = map[string]string{
	newheads.KindNewHead:          "New head",
	newheads.KindVanished:         "Vanished",
	newheads.KindBrokenRecurrence: "Missing months",
	newheads.KindStepChange:       "Step change",
}

// WorklistKindText is the name of a kind of Worklist item on screen and in the report.
func WorklistKindText(kind string) string { return worklistKindText[kind] }

func rupees(p money.Paise) (string, error) {
	s, err := money.FormatIndian(p, p%money.PaisePerRupee != 0)
	if err != nil {
		return "", err
	}
	return "Rs " + s, nil
}

func months(n int) string {
	if n == 1 {
		return "1 month"
	}
	return fmt.Sprintf("%d months", n)
}

// worklistLine states what was counted for one item: the totals and months of both
// years, never a classification.
func worklistLine(it newheads.Item) (string, error) {
	prior, err := rupees(it.PriorTotal)
	if err != nil {
		return "", err
	}
	current, err := rupees(it.CurrentTotal)
	if err != nil {
		return "", err
	}
	name := it.Ledger
	if it.Group != "" {
		name += " (" + it.Group + ")"
	}
	var what string
	switch it.Kind {
	case newheads.KindNewHead:
		what = fmt.Sprintf("%s this year in %s; no entry last year", current, months(it.CurrentMonths))
	case newheads.KindVanished:
		what = fmt.Sprintf("%s last year in %s; no entry this year", prior, months(it.PriorMonths))
	case newheads.KindBrokenRecurrence:
		what = fmt.Sprintf("an entry in every month last year (%d); this year no entry for %s", it.PriorMonths, strings.Join(it.MissingMonths, ", "))
	case newheads.KindStepChange:
		what = fmt.Sprintf("%s last year, %s this year (%s times)", prior, current, it.Times)
	default:
		return "", fmt.Errorf("findings: unknown worklist kind %q", it.Kind)
	}
	return fmt.Sprintf("%s: %s: %s", WorklistKindText(it.Kind), name, what), nil
}

func (r *Report) writeWorklist(b *strings.Builder) error {
	fmt.Fprintf(b, "\n== Worklist: %d ==\n%s.\n", len(r.Worklist), WorklistNotice)
	for _, it := range r.Worklist {
		line, err := worklistLine(it)
		if err != nil {
			return err
		}
		b.WriteString(" " + line + "\n")
	}
	return nil
}

// WorklistObject is one Worklist item in the report JSON: money in paise.
func WorklistObject(it newheads.Item) (map[string]any, error) {
	text, err := worklistLine(it)
	if err != nil {
		return nil, err
	}
	str := func(s []string) []any {
		out := make([]any, 0, len(s))
		for _, x := range s {
			out = append(out, x)
		}
		return out
	}
	return map[string]any{
		"kind": it.Kind, "kind_text": WorklistKindText(it.Kind), "key": it.Key(),
		"ledger": it.Ledger, "group": it.Group, "text": text,
		"prior_total": int64(it.PriorTotal), "current_total": int64(it.CurrentTotal),
		"prior_months": it.PriorMonths, "current_months": it.CurrentMonths,
		"missing_months": str(it.MissingMonths), "times": it.Times,
		"prior_rows": str(it.PriorLocators), "current_rows": str(it.CurrentLocators),
	}, nil
}

// WorklistJSON is the run's Worklist as canonical JSON, kept with the run.
func WorklistJSON(items []newheads.Item) ([]byte, error) {
	list, err := worklistList(items)
	if err != nil {
		return nil, err
	}
	return marshal(list)
}

func worklistList(items []newheads.Item) ([]any, error) {
	list := make([]any, 0, len(items))
	for _, it := range items {
		obj, err := WorklistObject(it)
		if err != nil {
			return nil, err
		}
		list = append(list, obj)
	}
	return list, nil
}

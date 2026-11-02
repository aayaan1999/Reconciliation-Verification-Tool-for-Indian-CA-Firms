package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
)

// rulestoreProblems writes the list of rule-book problems for the CA to review: one row
// per problem, with an empty column for the CA's decision. It never changes a rule; the CA
// decides, and fixes go into the workbook and back through rulestore-import.
//
//	go run ./cmd/recon-dev rulestore-problems [-store folder] [-out folder]
func rulestoreProblems(args []string) error {
	fs := flag.NewFlagSet("rulestore-problems", flag.ContinueOnError)
	root := fs.String("store", rsDefaultStore, "rule store folder")
	out := fs.String("out", filepath.Join(filepath.Dir(rsDefaultStore), "review"), "folder for rule-problems.xlsx and rule-problems.md")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("unexpected arguments; use -store and -out")
	}
	problems, err := findProblems(*root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "rule-problems.md"), problemsMarkdown(problems), 0o644); err != nil {
		return err
	}
	if err := problemsWorkbook(problems, filepath.Join(*out, "rule-problems.xlsx")); err != nil {
		return err
	}
	fmt.Printf("%d problems in %d rules written to %s\n", len(problems), countRules(problems), *out)
	return nil
}

// A ruleProblem is one row of the list.
type ruleProblem struct {
	RuleID, RuleName, Type, Detail string
}

// Problem types, each with the explanation shown to the CA.
var problemTypes = []struct{ code, meaning string }{
	{"UNVERIFIED", "The rule book marks this rule as not verified against an official source."},
	{"CONFLICTING_SOURCES", "The rule book records sources that disagree for this rule."},
	{"FIELD_NOT_IN_CATALOGUE", "The rule uses a document field that is not in the researched field catalogue; it was added as a design field during conversion."},
	{"NO_MATCH_KEYS", "A reconciliation rule with no keys to pair records from the two documents."},
	{"NO_COMPARISONS", "A reconciliation rule with no figures to compare."},
	{"NO_FORMULA", "The rule has neither a formula nor calculation steps."},
	{"NO_SOURCES", "The rule cites no source (Act, rule, circular or reference)."},
	{"NO_PASS_EXAMPLE", "The rule book gives no example where the figures agree."},
	{"NO_FAIL_EXAMPLE", "The rule book gives no example where the figures disagree."},
	{"NO_VALID_FROM", "The rule has no start date, so a later law change cannot be dated against it."},
	{"BANNED_WORDING", "The rule's name or category uses a phrase that finding text may never use (such as \"must be\" or \"wrong\"); reword it before the rule is made runnable."},
}

// findProblems reads the store and returns every problem, sorted by rule ID, then type in
// the order above, then detail.
func findProblems(root string) ([]ruleProblem, error) {
	files, err := rulestore.LoadRules(root)
	if err != nil {
		return nil, err
	}
	design := map[string]bool{} // "DOC.field" added during conversion
	docFiles, _ := filepath.Glob(filepath.Join(root, "documents", "*.yaml"))
	for _, f := range docFiles {
		if filepath.Base(f) == rulestore.IndexFile {
			continue
		}
		var d rsDocFile
		var bad []string
		if !decodeFile(filepath.ToSlash(f), &d, func(format string, a ...any) { bad = append(bad, fmt.Sprintf(format, a...)) }) {
			return nil, fmt.Errorf("%s", strings.Join(bad, "\n"))
		}
		for _, fl := range d.Fields {
			if fl.Origin == "new (design)" {
				design[d.Document.Code+"."+fl.Name] = true
			}
		}
	}

	var out []ruleProblem
	for _, f := range files {
		r := f.Rule
		add := func(typ, format string, a ...any) {
			out = append(out, ruleProblem{RuleID: r.Rule.ID, RuleName: r.Rule.Name, Type: typ, Detail: fmt.Sprintf(format, a...)})
		}
		switch r.Governance.VerificationStatus {
		case "unverified":
			add("UNVERIFIED", "verification status: %s", r.Governance.VerificationLabel)
		case "conflicting":
			add("CONFLICTING_SOURCES", "verification status: %s", r.Governance.VerificationLabel)
		}
		seen := map[string]bool{}
		for _, p := range append(slices.Clone(r.Matching.Keys), r.Matching.Comparisons...) {
			for _, e := range []string{p.Left, p.Right, p.Expression} {
				for _, ref := range fieldRefs(e) {
					name := ref[0] + "." + ref[1]
					if design[name] && !seen[name] {
						seen[name] = true
						add("FIELD_NOT_IN_CATALOGUE", "%s", name)
					}
				}
			}
		}
		if r.Rule.Type == "reconciliation" {
			if len(r.Matching.Keys) == 0 {
				add("NO_MATCH_KEYS", "matching.keys is empty")
			}
			if len(r.Matching.Comparisons) == 0 {
				add("NO_COMPARISONS", "matching.comparisons is empty")
			}
		}
		if strings.TrimSpace(r.Calculation.Formula) == "" && len(r.Calculation.Steps) == 0 {
			add("NO_FORMULA", "calculation.formula and calculation.steps are empty")
		}
		if len(r.Sources) == 0 {
			add("NO_SOURCES", "sources is empty")
		}
		if strings.TrimSpace(r.Tests.Positive) == "" {
			add("NO_PASS_EXAMPLE", "tests.positive is empty")
		}
		if strings.TrimSpace(r.Tests.Negative) == "" {
			add("NO_FAIL_EXAMPLE", "tests.negative is empty")
		}
		if r.Validity.ValidFrom == "" {
			add("NO_VALID_FROM", "validity.valid_from is empty")
		}
		for _, field := range []struct{ name, text string }{{"name", r.Rule.Name}, {"category", r.Rule.Category}} {
			if b := rules.BannedIn(field.text); len(b) > 0 {
				add("BANNED_WORDING", "%s uses %q", field.name, strings.Join(b, `", "`))
			}
		}
	}
	order := map[string]int{}
	for i, pt := range problemTypes {
		order[pt.code] = i
	}
	slices.SortStableFunc(out, func(a, b ruleProblem) int {
		return cmp.Or(strings.Compare(a.RuleID, b.RuleID), cmp.Compare(order[a.Type], order[b.Type]), strings.Compare(a.Detail, b.Detail))
	})
	return out, nil
}

func countRules(ps []ruleProblem) int {
	ids := map[string]bool{}
	for _, p := range ps {
		ids[p.RuleID] = true
	}
	return len(ids)
}

func problemCounts(ps []ruleProblem) map[string]int {
	c := map[string]int{}
	for _, p := range ps {
		c[p.Type]++
	}
	return c
}

// problemsMarkdown writes the list for the repository. It holds no date, so the same store
// always gives the same file.
func problemsMarkdown(ps []ruleProblem) []byte {
	var b strings.Builder
	b.WriteString("# Rule book problems for CA review\n\n")
	b.WriteString("Generated by `go run ./cmd/recon-dev rulestore-problems` from the YAML rule store. Do not edit by hand: record decisions in `rule-problems.xlsx`, make fixes in the rule-book workbook, and run `rulestore-import` again.\n\n")
	fmt.Fprintf(&b, "%d problems in %d rules.\n\n| Problem type | Rules | Meaning |\n|---|---|---|\n", len(ps), countRules(ps))
	counts := problemCounts(ps)
	for _, pt := range problemTypes {
		fmt.Fprintf(&b, "| `%s` | %d | %s |\n", pt.code, counts[pt.code], pt.meaning)
	}
	b.WriteString("\n| Rule | Name | Problem | Detail |\n|---|---|---|---|\n")
	for _, p := range ps {
		fmt.Fprintf(&b, "| %s | %s | `%s` | %s |\n", p.RuleID, mdCell(p.RuleName), p.Type, mdCell(p.Detail))
	}
	return []byte(b.String())
}

func mdCell(s string) string {
	return strings.NewReplacer("|", `\|`, "\r", " ", "\n", " ").Replace(s)
}

// fixedTime is written as the workbook's created and modified time, so the same store
// always gives a byte-identical file.
var fixedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

// problemsWorkbook writes the list for the CA: a summary sheet and a sheet with one row per
// problem and an empty "CA decision" column.
func problemsWorkbook(ps []ruleProblem, file string) error {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetDocProps(&excelize.DocProperties{Title: "Rule book problems for CA review", Creator: "recon-dev rulestore-problems",
		Created: fixedTime, Modified: fixedTime}); err != nil {
		return err
	}
	const list, summary = "Problems", "Summary"
	if err := f.SetSheetName("Sheet1", list); err != nil {
		return err
	}
	if _, err := f.NewSheet(summary); err != nil {
		return err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	rows := [][]any{{"Rule", "Rule name", "Problem type", "Detail", "CA decision"}}
	for _, p := range ps {
		rows = append(rows, []any{p.RuleID, p.RuleName, p.Type, p.Detail, ""})
	}
	for i, r := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(list, cell, &r); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(list, "A1", "E1", bold); err != nil {
		return err
	}
	for col, w := range map[string]float64{"A": 14, "B": 60, "C": 26, "D": 50, "E": 40} {
		if err := f.SetColWidth(list, col, col, w); err != nil {
			return err
		}
	}
	if err := f.SetPanes(list, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}

	counts := problemCounts(ps)
	srows := [][]any{{"Problem type", "Rules", "Meaning"}}
	for _, pt := range problemTypes {
		srows = append(srows, []any{pt.code, counts[pt.code], pt.meaning})
	}
	srows = append(srows, []any{}, []any{"Total problems", len(ps)}, []any{"Rules with a problem", countRules(ps)})
	for i, r := range srows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(summary, cell, &r); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(summary, "A1", "C1", bold); err != nil {
		return err
	}
	if err := f.SetColWidth(summary, "A", "A", 26); err != nil {
		return err
	}
	if err := f.SetColWidth(summary, "C", "C", 100); err != nil {
		return err
	}
	return f.SaveAs(file)
}

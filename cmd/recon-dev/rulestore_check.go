package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// rulestoreCheck validates every rule and document file of the YAML rule store. Each file
// must load through internal/strictyaml (so every text value is quoted, RU-02) and pass the
// checks below; every problem is printed.
//
//	go run ./cmd/recon-dev rulestore-check [folder]
func rulestoreCheck(args []string) error {
	fset := flag.NewFlagSet("rulestore-check", flag.ContinueOnError)
	if err := fset.Parse(args); err != nil {
		return err
	}
	root := rsDefaultStore
	if fset.NArg() == 1 {
		root = fset.Arg(0)
	} else if fset.NArg() > 1 {
		return errors.New("give at most one folder")
	}
	rules, docs, problems := checkStore(root)
	fmt.Printf("%d rules, %d documents checked; %d problems\n", rules, docs, len(problems))
	for _, p := range problems {
		fmt.Println(" -", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problems", len(problems))
	}
	return nil
}

var (
	ruleIDPattern   = regexp.MustCompile(`^(RC-)?[A-Z]{2,5}-\d{3}$`)
	docCodePattern  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	fieldPattern    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	paramKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	codePattern     = regexp.MustCompile(`^[A-Z0-9_]+$`)
	urlPrefix       = regexp.MustCompile(`^https?://`)
	versionText     = regexp.MustCompile(`^[1-9][0-9]*$`)
	numberText      = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

	ruleTypes   = []string{"statutory", "reconciliation"}
	ruleDomains = []string{"gst", "tds_tcs", "income_tax", "payroll", "bank", "accounting", "msme", "company_law", "audit", "other"}
	verifStates = []string{"verified_official", "verified_secondary", "partially_verified", "unverified", "conflicting"}
	decisions   = []string{"", "approved", "approved_with_changes", "rejected", "needs_more_info"}
	paramTypes  = []string{"string", "integer", "number", "boolean", "date"}
	origins     = []string{"", "catalogue", "derived", "new (design)", "new document (design)"}
	requiredYN  = []string{"", "yes", "no"}
)

// checkStore returns the number of rule and document files checked and every problem.
func checkStore(root string) (int, int, []string) {
	var problems []string
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	docFields := map[string]map[string]bool{}
	docFiles, _ := filepath.Glob(filepath.Join(root, "documents", "*.yaml"))
	slices.Sort(docFiles)
	ndocs := 0
	for _, f := range docFiles {
		if filepath.Base(f) == "index.yaml" {
			continue
		}
		ndocs++
		name := filepath.ToSlash(f)
		var d rsDocFile
		if !decodeFile(name, &d, bad) {
			continue
		}
		dd := d.Document
		if d.SchemaVersion != rsSchemaVersion {
			bad("%s: schema_version must be %q", name, rsSchemaVersion)
		}
		if !docCodePattern.MatchString(dd.Code) {
			bad("%s: document code %q is not capital letters, digits and _", name, dd.Code)
		}
		if filepath.Base(f) != dd.Code+".yaml" {
			bad("%s: file name does not match document code %s", name, dd.Code)
		}
		if strings.TrimSpace(dd.Name) == "" {
			bad("%s: document name is missing", name)
		}
		oneOf(bad, name, "verification_status", dd.VerificationStatus, verifStates)
		checkSources(bad, name, dd.Sources)
		fields := map[string]bool{}
		for _, fl := range d.Fields {
			if !fieldPattern.MatchString(fl.Name) {
				bad("%s: field name %q is not lowercase snake_case", name, fl.Name)
			}
			oneOf(bad, name, "field "+fl.Name+" origin", fl.Origin, origins)
			oneOf(bad, name, "field "+fl.Name+" required", fl.Required, requiredYN)
			fields[fl.Name] = true
		}
		if _, dup := docFields[dd.Code]; dup {
			bad("%s: document code %s is defined twice", name, dd.Code)
		}
		docFields[dd.Code] = fields
	}

	var ruleFiles []string
	_ = filepath.WalkDir(filepath.Join(root, "rules"), func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() && e.Name() == "_history" {
			return filepath.SkipDir
		}
		if !e.IsDir() && strings.HasSuffix(p, ".yaml") && e.Name() != "index.yaml" {
			ruleFiles = append(ruleFiles, p)
		}
		return nil
	})
	slices.Sort(ruleFiles)
	ids := map[string]string{}
	for _, f := range ruleFiles {
		name := filepath.ToSlash(f)
		var r rsRule
		if !decodeFile(name, &r, bad) {
			continue
		}
		checkRule(bad, name, &r, docFields)
		id := r.Rule.ID
		if prev, dup := ids[id]; dup {
			bad("duplicate rule id %s: %s and %s", id, prev, name)
		}
		ids[id] = name
		if strings.TrimSuffix(filepath.Base(f), ".yaml") != id {
			bad("%s: file name does not match rule id %s", name, id)
		}
		rel, _ := filepath.Rel(filepath.Join(root, "rules"), filepath.Dir(f))
		if want := r.Rule.Type + "/" + r.Rule.Domain; filepath.ToSlash(rel) != want {
			bad("%s: rule %s belongs in rules/%s/", name, id, want)
		}
	}
	return len(ruleFiles), ndocs, problems
}

// decodeFile reads one file through the strict reader; its problems are reported.
func decodeFile(name string, out any, bad func(string, ...any)) bool {
	data, err := os.ReadFile(name)
	if err != nil {
		bad("%s: %v", name, err)
		return false
	}
	if _, err := strictyaml.Decode(name, data, out); err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			bad("%s", line)
		}
		return false
	}
	return true
}

func oneOf(bad func(string, ...any), file, what, value string, allowed []string) {
	if !slices.Contains(allowed, value) {
		bad("%s: %s %q is not one of: %s", file, what, value, strings.Join(allowed, ", "))
	}
}

func dateOrEmpty(bad func(string, ...any), file, what, value string) {
	if value != "" && !isoDate.MatchString(value) {
		bad("%s: %s %q is not a date written YYYY-MM-DD", file, what, value)
	}
}

func checkSources(bad func(string, ...any), file string, ss []rsSource) {
	for _, s := range ss {
		if s.URL != "" && !urlPrefix.MatchString(s.URL) {
			bad("%s: source url %q does not start with http:// or https://", file, s.URL)
		}
	}
}

func checkRule(bad func(string, ...any), f string, r *rsRule, docFields map[string]map[string]bool) {
	id := r.Rule.ID
	if r.SchemaVersion != rsSchemaVersion {
		bad("%s: schema_version must be %q", f, rsSchemaVersion)
	}
	if !ruleIDPattern.MatchString(id) {
		bad("%s: rule id %q is not like GST-022 or RC-GST-013", f, id)
	}
	if !versionText.MatchString(r.Rule.Version) {
		bad("%s: version %q is not a whole number from 1", f, r.Rule.Version)
	}
	oneOf(bad, f, "type", r.Rule.Type, ruleTypes)
	oneOf(bad, f, "domain", r.Rule.Domain, ruleDomains)
	if len([]rune(r.Rule.Name)) < 3 {
		bad("%s: name is missing or too short", f)
	}

	g := r.Governance
	oneOf(bad, f, "verification_status", g.VerificationStatus, verifStates)
	oneOf(bad, f, "review_decision", g.ReviewDecision, decisions)
	dateOrEmpty(bad, f, "reviewed_on", g.ReviewedOn)
	dateOrEmpty(bad, f, "last_modified_on", g.LastModifiedOn)
	if g.LastModifiedBy == "" {
		bad("%s: last_modified_by is missing", f)
	}
	if g.Enabled && (g.ReviewedBy == "" || g.ReviewedOn == "" || (g.ReviewDecision != "approved" && g.ReviewDecision != "approved_with_changes")) {
		bad("%s: rule %s is enabled without a CA reviewer, review date and an approved decision", f, id)
	}
	dateOrEmpty(bad, f, "valid_from", r.Validity.ValidFrom)
	dateOrEmpty(bad, f, "valid_to", r.Validity.ValidTo)
	if r.Validity.ValidFrom != "" && r.Validity.ValidTo != "" && r.Validity.ValidTo < r.Validity.ValidFrom {
		bad("%s: valid_to is before valid_from", f)
	}

	for _, d := range r.Inputs.Documents {
		oneOf(bad, f, "document role", d.Role, []string{"A", "B", "C"})
		if !docCodePattern.MatchString(d.DocCode) {
			bad("%s: document code %q is not capital letters, digits and _", f, d.DocCode)
		} else if docFields[d.DocCode] == nil {
			bad("%s: document %s has no file in documents/", id, d.DocCode)
		}
	}
	for _, p := range append(slices.Clone(r.Matching.Keys), r.Matching.Comparisons...) {
		if (p.Left != "" || p.Right != "") == (p.Expression != "") || (p.Expression == "" && (p.Left == "" || p.Right == "")) {
			bad("%s: each key or comparison is left and right, or an expression", f)
		}
		for _, e := range []string{p.Left, p.Right, p.Expression} {
			for _, ref := range fieldRefs(e) {
				if fields, known := docFields[ref[0]]; known && !fields[ref[1]] {
					bad("%s: %s.%s not defined in documents/%s.yaml", id, ref[0], ref[1], ref[0])
				}
			}
		}
	}

	keys := map[string]bool{}
	for _, p := range r.Parameters {
		checkParam(bad, id, p)
		if p.Key != "" {
			if keys[p.Key] {
				bad("%s: parameter %s repeated", id, p.Key)
			}
			keys[p.Key] = true
		}
	}

	codes := map[string]bool{}
	for _, c := range r.Outcomes.Categories {
		if !codePattern.MatchString(c.Code) {
			bad("%s: outcome code %q is not UPPER_SNAKE_CASE", id, c.Code)
		}
		if codes[c.Code] {
			bad("%s: outcome code %s repeated", id, c.Code)
		}
		codes[c.Code] = true
		oneOf(bad, f, "severity of "+c.Code, c.Severity, append([]string{""}, severities...))
	}
	oneOf(bad, f, "default_severity", r.Outcomes.DefaultSeverity, append([]string{""}, severities...))
	checkSources(bad, f, r.Sources)

	if r.Rule.Type == "reconciliation" {
		if len(r.Inputs.Documents) < 2 {
			bad("%s: a reconciliation rule needs at least two documents", id)
		}
		if len(r.Matching.Comparisons) == 0 {
			bad("%s: a reconciliation rule needs at least one comparison", id)
		}
		if len(r.Outcomes.Categories) == 0 {
			bad("%s: a reconciliation rule needs at least one outcome category", id)
		}
	}
}

// checkParam checks a parameter's key, type, value and limits. Numbers are compared as
// exact decimals (math/big), never as floats.
func checkParam(bad func(string, ...any), id string, p rsParam) {
	if p.Key != "" && !paramKeyPattern.MatchString(p.Key) {
		bad("%s: parameter key %q is not letters, digits and _", id, p.Key)
	}
	if !slices.Contains(paramTypes, p.Type) {
		bad("%s: parameter %s type %q is not one of: %s", id, p.Key, p.Type, strings.Join(paramTypes, ", "))
		return
	}
	ok := true
	switch p.Type {
	case "integer":
		ok = intText.MatchString(p.Value)
	case "number":
		ok = numberText.MatchString(p.Value)
	case "boolean":
		ok = p.Value == "true" || p.Value == "false"
	case "date":
		ok = isoDate.MatchString(p.Value)
	}
	if !ok {
		bad("%s: parameter %s value %q is not of declared type %s", id, p.Key, p.Value, p.Type)
	}
	if v, isNum := exact(p.Value); isNum && (p.Type == "integer" || p.Type == "number") {
		for _, lim := range []struct {
			name, text string
			below      bool
		}{{"min", p.Min, true}, {"max", p.Max, false}} {
			if lim.text == "" {
				continue
			}
			l, good := exact(lim.text)
			switch {
			case !good:
				bad("%s: parameter %s %s %q is not a number", id, p.Key, lim.name, lim.text)
			case lim.below && v.Cmp(l) < 0:
				bad("%s: parameter %s below min %s", id, p.Key, lim.text)
			case !lim.below && v.Cmp(l) > 0:
				bad("%s: parameter %s above max %s", id, p.Key, lim.text)
			}
		}
	}
	if len(p.AllowedValues) > 0 && !slices.Contains(p.AllowedValues, p.Value) {
		bad("%s: parameter %s not in allowed_values", id, p.Key)
	}
}

// exact reads a decimal number exactly.
func exact(s string) (*big.Rat, bool) {
	if !numberText.MatchString(s) {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

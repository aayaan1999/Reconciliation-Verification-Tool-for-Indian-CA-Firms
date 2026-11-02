package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// Helpers of rule-save: the existing rules a save may change, the fixtures a rule keeps, the
// checks each fixture gets, and the writes to the office folder, which can be undone.

// parseYears reads the tax years of a form, in the order typed.
func parseYears(names []string) ([]period.TaxYear, error) {
	out := make([]period.TaxYear, 0, len(names))
	for _, s := range names {
		ty, err := period.ParseTaxYear(s)
		if err != nil {
			return nil, fmt.Errorf("the tax year %q must look like 2026-27", s)
		}
		out = append(out, ty)
	}
	return out, nil
}

// splitYears reads the tax years a stored version lists (comma-separated); a year that does
// not parse is left out.
func splitYears(s string) []period.TaxYear {
	var out []period.TaxYear
	for _, p := range strings.Split(s, ",") {
		if ty, err := period.ParseTaxYear(strings.TrimSpace(p)); err == nil {
			out = append(out, ty)
		}
	}
	return out
}

func yearsOverlap(a, b []period.TaxYear) bool {
	return slices.ContainsFunc(a, func(x period.TaxYear) bool { return slices.Contains(b, x) })
}

// existingRules returns the built-in and the office rules with this ID, from their folders.
func existingRules(id string, env ruleEnv) (builtin, office []*rules.Rule, err error) {
	bset, err := rules.LoadDir(env.builtin, env.v, env.checks)
	if err != nil {
		return nil, nil, inputErrf("built-in rules %s:\n%v", env.builtin, err)
	}
	for _, r := range bset.Rules() {
		if r.ID == id {
			builtin = append(builtin, r)
		}
	}
	if _, statErr := os.Stat(env.office); statErr == nil {
		oset, err := rules.LoadDir(env.office, env.v, env.checks)
		if err != nil {
			return nil, nil, inputErrf("office rules %s:\n%v", env.office, err)
		}
		for _, r := range oset.Rules() {
			if r.ID == id {
				office = append(office, r)
			}
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil, inputErr(statErr)
	}
	return builtin, office, nil
}

// nextOfficeEdit is the number of the next office edit of version base: one more than the
// highest "<base>-office.N" stored for the rule.
func nextOfficeEdit(vs []store.RuleVersion, base string) int {
	highest := 0
	for _, v := range vs {
		if v.Source != store.SourceOffice {
			continue
		}
		rest, ok := strings.CutPrefix(v.Version, base+"-office.")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(rest); err == nil && n > highest {
			highest = n
		}
	}
	return highest + 1
}

// A fixtureFile is one fixture a saved rule keeps: its kind (pass or fail), its file name, and
// the text it is first written with, before the engine has given its results.
type fixtureFile struct {
	kind, name string
	initial    []byte
}

func fixtureKey(kind, name string) string { return kind + "/" + name }

// formFixtureKinds lists the kind of each fixture typed on the window.
func formFixtureKinds(fxs []rules.FormFixture) []string {
	kinds := make([]string, len(fxs))
	for i, fx := range fxs {
		kinds[i] = fx.Kind
	}
	return kinds
}

// carriedFixtures returns the fixtures of the rule a save changes, so an edit keeps them.
func carriedFixtures(target *rules.Rule, env ruleEnv) (map[string]fixtureFile, error) {
	out := map[string]fixtureFile{}
	if target == nil {
		return out, nil
	}
	fxs, err := rules.LoadFixtures(target, env.v)
	if err != nil {
		return nil, inputErrf("the fixtures of %s:\n%v", target.ID, err)
	}
	for _, f := range append(slices.Clone(fxs.Pass), fxs.Fail...) {
		name := strings.TrimSuffix(filepath.Base(f.File), ".yaml")
		out[fixtureKey(f.Kind, name)] = fixtureFile{kind: f.Kind, name: name,
			initial: rules.FixtureYAML(f, rules.PlaceholderExpected(f.Kind, fixturePeriod(f), ""))}
	}
	return out, nil
}

// fixturePeriod is the period of a fixture's first figure or row, or the tax year's own period
// when it has none (the first expected line must name a period that loads).
func fixturePeriod(f *rules.Fixture) string {
	switch {
	case len(f.Facts) > 0:
		return f.Facts[0].Period
	case len(f.Records) > 0:
		return f.Records[0].Period
	}
	return period.Year(f.TaxYear).String()
}

// placeholderFor is the first expected line of a fixture typed on the window.
func placeholderFor(kind string, facts []rules.FormFact, records []rules.FormRecord, ty period.TaxYear) []rules.Expected {
	p := period.Year(ty).String()
	if len(facts) > 0 && facts[0].Period != "" {
		p = facts[0].Period
	} else if len(records) > 0 && records[0].Period != "" {
		p = records[0].Period
	}
	return rules.PlaceholderExpected(kind, p, "")
}

// fixtureResultProblems says which fixtures the engine does not give their kind: a pass
// fixture must pass on every subject, and a fail fixture must fail on at least one (AC-4).
func fixtureResultProblems(r *rules.Rule, fxs *rules.Fixtures, env ruleEnv) []rules.Problem {
	var ps []rules.Problem
	check := func(kind string, list []*rules.Fixture) {
		for _, f := range list {
			name := strings.TrimSuffix(filepath.Base(f.File), ".yaml")
			res, err := engine.FixtureResults(r, f, env.checks)
			if err != nil {
				ps = append(ps, rules.Problem{Field: "fixtures", Code: "FIXTURE_NOT_RUN", Message: fmt.Sprintf("the %s fixture %q could not be run: %v", kind, name, err)})
				continue
			}
			passes := len(res) > 0 && !slices.ContainsFunc(res, func(e rules.Expected) bool { return e.Status != "pass" })
			fails := slices.ContainsFunc(res, func(e rules.Expected) bool { return e.Status == "fail" })
			switch {
			case kind == "pass" && !passes:
				ps = append(ps, rules.Problem{Field: "fixtures", Code: "PASS_FIXTURE_FAILS",
					Message: fmt.Sprintf("the pass fixture %q does not pass: %s", name, describeResults(res))})
			case kind == "fail" && !fails:
				ps = append(ps, rules.Problem{Field: "fixtures", Code: "FAIL_FIXTURE_PASSES",
					Message: fmt.Sprintf("the fail fixture %q does not fail: %s", name, describeResults(res))})
			}
		}
	}
	check("pass", fxs.Pass)
	check("fail", fxs.Fail)
	return ps
}

// describeResults says in plain words what a fixture's results were.
func describeResults(res []rules.Expected) string {
	if len(res) == 0 {
		return "the rule reaches no figure in it"
	}
	for _, e := range res {
		switch e.Status {
		case "cannot_check":
			return fmt.Sprintf("cannot check at %s, the figure %s is not in the fixture", e.Period, strings.Join(e.Missing, ", "))
		case "fail":
			return fmt.Sprintf("the figures differ at %s", e.Period)
		}
	}
	return fmt.Sprintf("the figures agree at %s", res[0].Period)
}

// writeExpected writes each fixture again with the results the engine gives it, which are
// the expected results the file records.
func writeExpected(r *rules.Rule, fxs *rules.Fixtures, env ruleEnv) error {
	for _, f := range append(slices.Clone(fxs.Pass), fxs.Fail...) {
		res, err := engine.FixtureResults(r, f, env.checks)
		if err != nil {
			return inputErrf("%s: %v", f.File, err)
		}
		if err := os.WriteFile(f.File, rules.FixtureYAML(f, res), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// checkExpected loads the fixtures again and checks that each one's expected results are the
// results the engine gives it, in the same order (a fixture written by the Rule Builder is read
// back exactly as written).
func checkExpected(r *rules.Rule, env ruleEnv) error {
	fxs, err := rules.LoadFixtures(r, env.v)
	if err != nil {
		return fmt.Errorf("the fixtures written for %s do not load: %w", r.ID, err)
	}
	for _, f := range append(slices.Clone(fxs.Pass), fxs.Fail...) {
		res, err := engine.FixtureResults(r, f, env.checks)
		if err != nil {
			return fmt.Errorf("fixture %s: %w", f.File, err)
		}
		if !sameExpected(res, f.Expected) {
			return fmt.Errorf("fixture %s: its expected results do not match what the engine gives", f.File)
		}
	}
	return nil
}

func sameExpected(a, b []rules.Expected) bool {
	return len(a) == len(b) && slices.EqualFunc(a, b, func(x, y rules.Expected) bool {
		return x.Status == y.Status && x.Period == y.Period && x.Scope == y.Scope && x.MatchKey == y.MatchKey &&
			x.Code == y.Code && slices.Equal(x.Missing, y.Missing)
	})
}

// loadProblems turns a load error into problems, with the temporary folder's name removed so
// that each message names only the rule's own file.
func loadProblems(err error, tmp, code string) []rules.Problem {
	var out []rules.Problem
	for _, line := range strings.Split(err.Error(), "\n") {
		line = strings.ReplaceAll(line, filepath.ToSlash(tmp)+"/", "")
		line = strings.ReplaceAll(line, tmp+string(filepath.Separator), "")
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, rules.Problem{Field: "rule", Code: code, Message: line})
		}
	}
	if len(out) == 0 {
		out = []rules.Problem{{Field: "rule", Code: code, Message: "the rule does not load"}}
	}
	return out
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// A fileUndo lists how to put back what a save wrote, most recent first.
type fileUndo struct{ steps []func() }

func (u *fileUndo) add(f func()) { u.steps = append(u.steps, f) }

func (u *fileUndo) rollback() {
	for i := len(u.steps) - 1; i >= 0; i-- {
		u.steps[i]()
	}
}

// writeOfficeFiles puts the checked rule into the office folder: its rule file and its
// fixtures, copied from srcRuleDir. An office rule being changed keeps its earlier text and
// fixtures under history/<version> first (nothing is deleted, RU-41).
func writeOfficeFiles(undo *fileUndo, office string, ty period.TaxYear, module, id string, target *rules.Rule, srcRuleDir string, ruleBody []byte) error {
	dir := filepath.Join(office, ty.String(), module, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ruleFile := filepath.Join(dir, rules.FileName)
	old, readErr := os.ReadFile(ruleFile)
	existed := readErr == nil
	if existed && target != nil {
		hist := filepath.Join(dir, "history", target.Version)
		if err := os.MkdirAll(hist, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(hist, rules.FileName+".txt"), old, 0o644); err != nil {
			return err
		}
		fixDir := filepath.Join(dir, "fixtures")
		if _, err := os.Stat(fixDir); err == nil {
			kept := filepath.Join(hist, "fixtures")
			if err := os.Rename(fixDir, kept); err != nil {
				return err
			}
			undo.add(func() { _ = os.Rename(kept, fixDir) })
		}
	}
	if err := os.WriteFile(ruleFile, ruleBody, 0o644); err != nil {
		return err
	}
	undo.add(func() {
		if existed {
			_ = os.WriteFile(ruleFile, old, 0o644)
		} else {
			_ = os.Remove(ruleFile)
		}
	})
	fixDir := filepath.Join(dir, "fixtures")
	undo.add(func() { _ = os.RemoveAll(fixDir) })
	return copyFixtureTree(filepath.Join(srcRuleDir, "fixtures"), fixDir)
}

// copyFixtureTree copies the folder src into dst, creating dst. A missing src copies nothing.
func copyFixtureTree(src, dst string) error {
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return writeFile(target, data)
	})
}

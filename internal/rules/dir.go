package rules

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// FileName is the name of every rule file inside its folder,
// content/rules/<tax-year>/<module>/<RULE-ID>/rule.yaml.
const FileName = "rule.yaml"

// A Set is every loaded rule, sorted by ID then by the first tax year.
type Set struct {
	rules []*Rule
}

// NewSet builds a set, refusing two rules with the same ID for the same tax year.
func NewSet(rs ...*Rule) (*Set, error) {
	s := &Set{rules: slices.Clone(rs)}
	slices.SortFunc(s.rules, func(a, b *Rule) int {
		return cmp.Or(strings.Compare(a.ID, b.ID), cmp.Compare(a.TaxYears[0].StartYear, b.TaxYears[0].StartYear), strings.Compare(a.File, b.File))
	})
	type key struct {
		id string
		ty period.TaxYear
	}
	owner := map[key]*Rule{}
	for _, r := range s.rules {
		for _, ty := range r.TaxYears {
			if o, dup := owner[key{r.ID, ty}]; dup {
				return nil, fmt.Errorf("rule %s is defined twice for tax year %s: %s and %s", r.ID, ty, o.File, r.File)
			}
			owner[key{r.ID, ty}] = r
		}
	}
	return s, nil
}

// Rules returns every rule, sorted by ID then tax year.
func (s *Set) Rules() []*Rule { return slices.Clone(s.rules) }

// ForTaxYear returns the rules that apply to ty, sorted by ID (RU-53: the same order
// every run).
func (s *Set) ForTaxYear(ty period.TaxYear) []*Rule {
	var out []*Rule
	for _, r := range s.rules {
		if r.AppliesTo(ty) {
			out = append(out, r)
		}
	}
	return out
}

// LoadDir loads every rule under root. Each must sit at
// <tax-year>/<module>/<RULE-ID>/rule.yaml, where the tax-year folder is one of the rule's
// tax years, so the folders and the file can never disagree. Every problem in every file
// is reported.
func LoadDir(root string, v *vocab.Vocab, cts *CheckTypes) (*Set, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == FileName {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	var rs []*Rule
	var errs strictyaml.Errors
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		name := filepath.ToSlash(file)
		r, err := Load(name, data, v, cts)
		if err != nil {
			errs = append(errs, err.(strictyaml.Errors)...)
			continue
		}
		rel, _ := filepath.Rel(root, filepath.Dir(file))
		parts := strings.Split(filepath.ToSlash(rel), "/")
		ty, tyErr := period.ParseTaxYear(firstOr(parts))
		if len(parts) != 3 || tyErr != nil || !r.AppliesTo(ty) || parts[1] != r.Module || parts[2] != r.ID {
			errs = append(errs, &strictyaml.Error{File: name, Msg: fmt.Sprintf(
				"a rule must sit at <tax-year>/%s/%s/%s under %s, where <tax-year> is one of its tax years",
				r.Module, r.ID, FileName, filepath.ToSlash(root))})
			continue
		}
		rs = append(rs, r)
	}
	if len(errs) > 0 {
		errs.Sort()
		return nil, errs
	}
	return NewSet(rs...)
}

func firstOr(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

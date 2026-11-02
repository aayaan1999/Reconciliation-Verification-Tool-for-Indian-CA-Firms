// Package comparison reads the comparison types (content/comparisons/comparisons.yaml,
// SPEC-004 part 8): what the CA picks before uploading. Each type names the document types
// it needs and the rules it runs. It is data, not code: a new type is a few lines of YAML.
package comparison

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Schema is the value of the "schema" key of the file.
const Schema = "recon.comparisons/1"

// A Comparison is one type of check.
type Comparison struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Documents   []string `yaml:"documents" json:"documents"` // vocabulary document types
	Rules       []string `yaml:"rules" json:"rules"`         // rule IDs; none for a totals-only type
	Totals      bool     `yaml:"totals" json:"totals"`       // show the documents' row totals side by side
	// Worklist marks a year-on-year type (SPEC-006 part 3): this year's and last year's
	// document of the same type give the Worklist; it has no rules and no totals.
	Worklist bool `yaml:"worklist" json:"worklist"`
}

type fileYAML struct {
	Schema      string       `yaml:"schema"`
	Comparisons []Comparison `yaml:"comparisons"`
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Load reads and checks the file: unique IDs, at least two documents each, every document
// type in the vocabulary. Rule IDs are checked against the stored rules when a run uses them.
func Load(file string, v *vocab.Vocab) ([]Comparison, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var f fileYAML
	pos, err := strictyaml.Decode(file, data, &f)
	if err != nil {
		return nil, err
	}
	var errs strictyaml.Errors
	fail := func(at string, format string, a ...any) {
		errs = append(errs, &strictyaml.Error{File: file, Line: pos.Line(at), Msg: fmt.Sprintf(format, a...)})
	}
	if f.Schema != Schema {
		fail("schema", "schema must be %q", Schema)
	}
	seen := map[string]bool{}
	for i, c := range f.Comparisons {
		at := fmt.Sprintf("comparisons[%d]", i)
		switch {
		case !idPattern.MatchString(c.ID):
			fail(at+".id", "id %q must be lower case letters, digits and _", c.ID)
		case seen[c.ID]:
			fail(at+".id", "id %q is used twice", c.ID)
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Name) == "" {
			fail(at+".name", "name is missing")
		}
		switch {
		case c.Worklist && (len(c.Rules) > 0 || c.Totals || len(c.Documents) != 1):
			fail(at, "a year-on-year (worklist) comparison has one document type, no rules and no totals")
		case !c.Worklist && len(c.Documents) < 2:
			fail(at+".documents", "a comparison needs at least two documents")
		}
		for _, d := range c.Documents {
			if _, ok := v.DocumentType(d); !ok {
				fail(at+".documents", "document type %q is not in the vocabulary", d)
			}
		}
		if len(c.Rules) == 0 && !c.Totals && !c.Worklist {
			fail(at, "a comparison with no rules must show totals")
		}
		if len(slices.Compact(slices.Sorted(slices.Values(c.Rules)))) != len(c.Rules) {
			fail(at+".rules", "a rule is listed twice")
		}
	}
	if len(errs) > 0 {
		errs.Sort()
		return nil, errs
	}
	return f.Comparisons, nil
}

// Find returns the comparison with this ID.
func Find(list []Comparison, id string) (Comparison, bool) {
	i := slices.IndexFunc(list, func(c Comparison) bool { return c.ID == id })
	if i < 0 {
		return Comparison{}, false
	}
	return list[i], true
}

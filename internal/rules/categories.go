package rules

import (
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// CategoriesSchema is the value of the "schema" key of content/rules/categories.yaml.
const CategoriesSchema = "recon.rule.categories/1"

// A Category is one category a rule may be filed under (the R-categories of the tie-out
// catalogue, and the module categories such as GST).
type Category struct {
	Code        string
	Name        string
	Description string
}

type categoriesYAML struct {
	Schema     string         `yaml:"schema"`
	Categories []categoryYAML `yaml:"categories"`
}

type categoryYAML struct {
	Code        string `yaml:"code"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// LoadCategories reads content/rules/categories.yaml strictly. Every problem is reported at
// once, each with its line.
func LoadCategories(name string, data []byte) ([]Category, error) {
	var f categoriesYAML
	pos, err := strictyaml.Decode(name, data, &f)
	if err != nil {
		return nil, err
	}
	c := &checker{file: name, pos: pos}
	if f.Schema != CategoriesSchema {
		c.fail("schema", "schema must be %q, not %q", CategoriesSchema, f.Schema)
	}
	seen := map[string]bool{}
	var out []Category
	for i, y := range f.Categories {
		at := fmt.Sprintf("categories[%d]", i)
		switch {
		case !categoryPattern.MatchString(y.Code):
			c.fail(at+".code", "code %q must be capital letters and digits, such as \"R1\"", y.Code)
		case seen[y.Code]:
			c.fail(at+".code", "code %q is listed twice", y.Code)
		}
		seen[y.Code] = true
		if y.Name == "" {
			c.fail(at+".name", "each category needs a name")
		}
		out = append(out, Category{Code: y.Code, Name: y.Name, Description: y.Description})
	}
	if len(out) == 0 {
		c.fail("categories", "the file must list at least one category")
	}
	if len(c.errs) > 0 {
		c.errs.Sort()
		return nil, c.errs
	}
	return out, nil
}

// CategoryCodes returns the codes of cs, in file order.
func CategoryCodes(cs []Category) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Code
	}
	return out
}

// SeverityList returns the severities a rule may have, in the order the window offers them.
func SeverityList() []string { return slices.Clone(severities) }

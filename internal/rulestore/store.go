package rulestore

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// Files in rules/ that are not rule files.
const (
	IndexFile      = "index.yaml"
	CategoriesFile = "categories.yaml"
	historyDir     = "_history"
)

// IsRuleFile reports whether name, a file name inside rules/, is a rule file: a .yaml file
// other than the index and the category list.
func IsRuleFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") && name != IndexFile && name != CategoriesFile
}

// RuleFiles returns the path of every rule file under root/rules, sorted, leaving out
// rules/_history (earlier versions, kept for audit only).
func RuleFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "rules"), func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() && e.Name() == historyDir {
			return filepath.SkipDir
		}
		if !e.IsDir() && IsRuleFile(e.Name()) {
			files = append(files, p)
		}
		return nil
	})
	slices.Sort(files)
	return files, err
}

// A File is one rule file as loaded: its path relative to the store root (slash form), its
// exact bytes and their SHA-256, and the decoded rule.
type File struct {
	Path   string
	Data   []byte
	SHA256 string // lowercase hex of Data
	Rule   Rule
}

// LoadRules reads every rule file of the store at root through the strict reader. The file
// name must equal the rule ID. Every problem in every file is reported together.
func LoadRules(root string) ([]File, error) {
	paths, err := RuleFiles(root)
	if err != nil {
		return nil, err
	}
	var out []File
	var errs strictyaml.Errors
	ids := map[string]string{}
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var r Rule
		if _, err := strictyaml.Decode(rel, data, &r); err != nil {
			errs = append(errs, err.(strictyaml.Errors)...)
			continue
		}
		id := r.Rule.ID
		switch {
		case strings.TrimSuffix(filepath.Base(p), ".yaml") != id:
			errs = append(errs, &strictyaml.Error{File: rel, Msg: fmt.Sprintf("file name does not match rule id %q", id)})
			continue
		case ids[id] != "":
			errs = append(errs, &strictyaml.Error{File: rel, Msg: fmt.Sprintf("rule id %s is also used by %s", id, ids[id])})
			continue
		}
		ids[id] = rel
		sum := sha256.Sum256(data)
		out = append(out, File{Path: rel, Data: data, SHA256: hex.EncodeToString(sum[:]), Rule: r})
	}
	if len(errs) > 0 {
		errs.Sort()
		return nil, errs
	}
	return out, nil
}

// CategoryIndex is rules/categories.yaml: every rule category with the rules in it.
type CategoryIndex struct {
	SchemaVersion string        `yaml:"schema_version"`
	Count         int           `yaml:"count"`
	Categories    []CategoryRow `yaml:"categories"`
}

// A CategoryRow is one category of one rule type and domain. A category used by rules of
// two types or domains gives one row for each, so every row has a single type and domain.
type CategoryRow struct {
	Category string   `yaml:"category"`
	Type     string   `yaml:"type"`
	Domain   string   `yaml:"domain"`
	Rules    []string `yaml:"rules"` // rule IDs, sorted
}

// BuildCategories returns the category list of rules: rows sorted by category, type and
// domain, each with its rule IDs sorted. The result depends only on the rules, never on
// their order.
func BuildCategories(rules []Rule) CategoryIndex {
	type key struct{ category, typ, domain string }
	byKey := map[key][]string{}
	for _, r := range rules {
		k := key{r.Rule.Category, r.Rule.Type, r.Rule.Domain}
		byKey[k] = append(byKey[k], r.Rule.ID)
	}
	rows := make([]CategoryRow, 0, len(byKey))
	for k, ids := range byKey {
		slices.Sort(ids)
		rows = append(rows, CategoryRow{Category: k.category, Type: k.typ, Domain: k.domain, Rules: ids})
	}
	slices.SortFunc(rows, func(a, b CategoryRow) int {
		return cmp.Or(strings.Compare(a.Category, b.Category), strings.Compare(a.Type, b.Type), strings.Compare(a.Domain, b.Domain))
	})
	return CategoryIndex{SchemaVersion: SchemaVersion, Count: len(rows), Categories: rows}
}

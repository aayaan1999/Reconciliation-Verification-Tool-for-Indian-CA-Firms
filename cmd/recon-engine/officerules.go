package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Office rules (SPEC-010): the rules written in the office by the Rule Builder live in a
// folder of their own, <office rules>/<tax-year>/<module>/<ID>/rule.yaml, with their fixtures
// beside them. Built-in rules in content/rules are never written to. The folder is read by
// load-rules and by run (--office-rules), as office profiles are (--office-profiles).

// loadOfficeRules loads every rule of the office folder into the database, source office,
// with the same rules as the built-in files (an unchanged file adds nothing). It returns how
// many rules were added or changed. A folder that does not exist yet has no rules.
func loadOfficeRules(tx *store.Tx, dir string, v *vocab.Vocab) (int, error) {
	if dir == "" {
		return 0, nil
	}
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, inputErr(err)
	}
	set, err := rules.LoadDir(dir, v, checks.All())
	if err != nil {
		return 0, inputErrf("office rules %s:\n%v", dir, err)
	}
	changed := 0
	for _, r := range set.Rules() {
		body, err := os.ReadFile(r.File)
		if err != nil {
			return 0, inputErr(err)
		}
		sum := sha256.Sum256(body)
		f := store.RuleFile{RuleID: r.ID, Version: r.Version, Source: store.SourceOffice, Schema: rules.Schema,
			TaxYears: taxYearList(r.TaxYears), FilePath: relPath(dir, r.File), Body: body, SHA256: hex.EncodeToString(sum[:])}
		c, err := tx.PutRule(f, store.RuleStatus{Status: store.StatusTesting})
		var ce *store.ConflictError
		if errors.As(err, &ce) {
			return 0, inputErr(err)
		}
		if err != nil {
			return 0, err
		}
		if c {
			changed++
		}
	}
	return changed, nil
}

// taxYearList writes tax years as the rule_versions column holds them: comma-separated.
func taxYearList(ts []period.TaxYear) string {
	names := make([]string, len(ts))
	for i, ty := range ts {
		names[i] = ty.String()
	}
	return strings.Join(names, ",")
}

// ruleEnv is what the Rule Builder commands read: the vocabulary, the check types the engine
// runs, the category codes, and the folders of the built-in and the office rules.
type ruleEnv struct {
	v       *vocab.Vocab
	checks  *rules.CheckTypes
	cats    []rules.Category
	builtin string // content/rules
	office  string // the office rules folder
}

// loadRuleEnv reads the vocabulary and the categories (categories.yaml in the built-in rules
// folder). A problem with them is an input error.
func loadRuleEnv(vocabDir, builtinDir, officeDir string) (ruleEnv, error) {
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		return ruleEnv{}, inputErrf("vocabulary: %v", err)
	}
	catFile := filepath.Join(builtinDir, "categories.yaml")
	data, err := os.ReadFile(catFile)
	if err != nil {
		return ruleEnv{}, inputErrf("--rules: %v", err)
	}
	cats, err := rules.LoadCategories(filepath.ToSlash(catFile), data)
	if err != nil {
		return ruleEnv{}, inputErrf("categories: %v", err)
	}
	return ruleEnv{v: v, checks: checks.All(), cats: cats, builtin: builtinDir, office: officeDir}, nil
}

// nowUTC is the audit time of a change, in UTC, RFC 3339. It is written to the audit log only;
// it never reaches a rule file or a run (RU-51).
func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// trimmedName reads who did a change: a name of up to 100 characters.
func trimmedName(by string) (string, error) {
	by = strings.TrimSpace(by)
	if by == "" || len([]rune(by)) > 100 {
		return "", inputErrf("BAD_NAME: say who makes the change (a name of up to 100 characters)")
	}
	return by, nil
}

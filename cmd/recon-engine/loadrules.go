package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// rulestoreSchema names the rule store's structure in rule_versions.schema. Rules with
// this schema are descriptive (prose comparisons, pseudocode): stored and listed, never
// evaluated.
const rulestoreSchema = "rulestore/" + rulestore.SchemaVersion

// loadRules loads every rule-store rule and every engine rule into the database, in one
// transaction. A rule-store rule's status follows its governance.enabled; an engine rule
// starts as testing, because no CA has approved it yet. Loading unchanged files again
// changes nothing; a changed file adds a new version.
func loadRules(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("load-rules", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	storeDir := fs.String("store", "", "YAML rule store folder")
	rulesDir := fs.String("rules", "", "engine rules folder (recon.rule/1)")
	officeRules := fs.String("office-rules", "", "office rules folder (SPEC-010), read too when it exists")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	profilesDir := fs.String("profiles", "content/profiles", "standard format profiles folder (SPEC-016), stored as formats when it exists")
	if err := parseFlags(fs, args, "db", "store", "rules"); err != nil {
		return err
	}

	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	storeFiles, err := rulestore.LoadRules(*storeDir)
	if err != nil {
		return inputErrf("rule store %s:\n%v", *storeDir, err)
	}
	set, err := rules.LoadDir(*rulesDir, v, checks.All())
	if err != nil {
		return inputErrf("engine rules %s:\n%v", *rulesDir, err)
	}

	var files []store.RuleFile
	var statuses []store.RuleStatus
	for _, f := range storeFiles {
		st := store.RuleStatus{Status: store.StatusDisabled, Reason: "not enabled in the rule store (governance.enabled is false)", Follow: true}
		if f.Rule.Governance.Enabled {
			st = store.RuleStatus{Status: store.StatusActive, Follow: true}
		}
		files = append(files, store.RuleFile{RuleID: f.Rule.Rule.ID, Version: f.Rule.Rule.Version, Source: store.SourceRulebook,
			Schema: rulestoreSchema, FilePath: path.Join(baseName(*storeDir), f.Path), Body: f.Data, SHA256: f.SHA256})
		statuses = append(statuses, st)
	}
	for _, r := range set.Rules() {
		body, err := os.ReadFile(r.File)
		if err != nil {
			return inputErr(err)
		}
		sum := sha256.Sum256(body)
		years := make([]string, len(r.TaxYears))
		for i, ty := range r.TaxYears {
			years[i] = ty.String()
		}
		files = append(files, store.RuleFile{RuleID: r.ID, Version: r.Version, Source: r.Source, Schema: rules.Schema,
			TaxYears: strings.Join(years, ","), FilePath: relPath(*rulesDir, r.File), Body: body, SHA256: hex.EncodeToString(sum[:])})
		statuses = append(statuses, store.RuleStatus{Status: store.StatusTesting})
	}

	db, err := store.Open(*dbFile)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	changed := 0
	for i, f := range files {
		c, err := tx.PutRule(f, statuses[i])
		var ce *store.ConflictError
		if errors.As(err, &ce) {
			return inputErr(err)
		}
		if err != nil {
			return err
		}
		if c {
			changed++
		}
	}
	// Office rules may read the documents the Partner added, so they load against the merged vocabulary (SPEC-022);
	// the built-in rules above and below load against the built-in one only.
	mst, err := readMasterTx(tx)
	if err != nil {
		return err
	}
	officeVocab, err := v.WithOffice(mst.office())
	if err != nil {
		return inputErrf("office documents: %v", err)
	}
	officeChanged, err := loadOfficeRules(tx, *officeRules, officeVocab)
	if err != nil {
		return err
	}
	// The standard formats (profiles) and parameters (vocabulary), stored as versions (SPEC-016).
	var formatsAdded, paramsAdded int
	if _, statErr := os.Stat(*profilesDir); statErr == nil {
		profiles, err := profile.LoadDir(*profilesDir, v)
		if err != nil {
			return inputErrf("profiles %s:\n%v", *profilesDir, err)
		}
		if formatsAdded, paramsAdded, err = seedStandard(tx, profiles, v); err != nil {
			return err
		}
	}
	// The built-in documents of the Document Master, filled from the vocabulary on every load (SPEC-022).
	// Nothing is written when they are already there, so a second load changes nothing.
	if err := seedBuiltins(tx, v, nowUTC()); err != nil {
		return err
	}
	stored, err := tx.Rules()
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	count := map[string]map[string]int{}
	for _, r := range stored {
		if count[r.Source] == nil {
			count[r.Source] = map[string]int{}
		}
		count[r.Source][r.Status]++
	}
	fmt.Fprintf(stdout, "%d rule-store rules loaded (%s)\n", len(storeFiles), statusText(count[store.SourceRulebook]))
	fmt.Fprintf(stdout, "%d engine rules loaded (%s)\n", len(set.Rules()), statusText(count[store.SourceBuiltin]))
	fmt.Fprintf(stdout, "%d rules added or changed, %d unchanged\n", changed, len(files)-changed)
	if *officeRules != "" {
		fmt.Fprintf(stdout, "%d office rules added or changed\n", officeChanged)
	}
	if formatsAdded > 0 || paramsAdded > 0 {
		fmt.Fprintf(stdout, "%d format versions and %d parameter versions added\n", formatsAdded, paramsAdded)
	}
	return nil
}

// baseName is the last element of a folder path, so stored file paths read the same
// wherever the program was started.
func baseName(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Base(dir)
	}
	return filepath.Base(abs)
}

// relPath writes file relative to dir, prefixed with dir's own name: "rules/2025-26/...".
func relPath(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return filepath.ToSlash(file)
	}
	return path.Join(baseName(dir), filepath.ToSlash(rel))
}

// statusText writes counts by status in a fixed order: "12 active, 292 disabled".
func statusText(c map[string]int) string {
	var parts []string
	for _, s := range []string{store.StatusActive, store.StatusTesting, store.StatusDraft, store.StatusDisabled} {
		if c[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c[s], s))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// parseFlags parses args and refuses a missing required flag or a stray argument.
func parseFlags(fs *flag.FlagSet, args []string, required ...string) error {
	if err := fs.Parse(args); err != nil {
		return inputErr(err)
	}
	if fs.NArg() > 0 {
		return inputErrf("unexpected argument %q", fs.Arg(0))
	}
	for _, name := range required {
		if fs.Lookup(name).Value.String() == "" {
			return inputErrf("the flag --%s is required", name)
		}
	}
	return nil
}

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// ruleInfo is one stored rule as the rules command lists it.
type ruleInfo struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	Revision  int64    `json:"revision"`
	Source    string   `json:"source"`   // rulebook | builtin | office
	Status    string   `json:"status"`   // draft | testing | active | disabled
	Runnable  bool     `json:"runnable"` // a recon.rule/1 rule the engine can evaluate
	Type      string   `json:"type"`     // statutory | reconciliation | tie-out ...
	Domain    string   `json:"domain"`   // gst | tds_tcs | income_tax ...; "" when not stated
	Category  string   `json:"category"`
	Title     string   `json:"title"`
	TaxYears  []string `json:"tax_years"`  // empty when the rule names none
	ValidFrom string   `json:"valid_from"` // "" when not stated
	ValidTo   string   `json:"valid_to"`
	FilePath  string   `json:"file_path"`
	// Approvable is set for a testing rule the Partner may approve now (SPEC-010 §4); the
	// note says why not otherwise. Both need --rules: without the rule folders they are not checked.
	Approvable   bool   `json:"approvable"`
	ApprovalNote string `json:"approval_note"`
}

// rulesList lists every stored rule with its active version and status (SPEC-004 part 6).
// It only reads; nothing is written. With --rules (and --office-rules) it also says which testing
// rules the Partner may approve now.
func rulesList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	builtinDir := fs.String("rules", "", "built-in rules folder (content/rules): with it, the approval state of each testing rule is checked")
	officeDir := fs.String("office-rules", "", "office rules folder, read with --rules")
	jsonOut := fs.String("json", "", "also write the list as canonical JSON to this file")
	if err := parseFlags(fs, args, "db"); err != nil {
		return err
	}
	if *jsonOut != "" {
		if st, err := os.Stat(filepath.Dir(*jsonOut)); err != nil || !st.IsDir() {
			return inputErrf("--json: the folder of %s does not exist", *jsonOut)
		}
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
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
	defer tx.Rollback() // read only
	var gate *approvalGate
	if *builtinDir != "" {
		env, err := loadRuleEnv(*vocabDir, *builtinDir, *officeDir)
		if err != nil {
			return err
		}
		gate = &approvalGate{env: env}
	}
	stored, err := tx.Rules()
	if err != nil {
		return err
	}

	list := make([]ruleInfo, 0, len(stored))
	years := map[string]bool{}
	for _, sr := range stored {
		ri := ruleInfo{ID: sr.RuleID, Version: sr.Version, Revision: sr.Revision, Source: sr.Source, Status: sr.Status,
			Runnable: sr.Schema == rules.Schema, TaxYears: []string{}, FilePath: sr.FilePath}
		if ri.Runnable {
			r, err := rules.Load(sr.FilePath, sr.Body, v, checks.All())
			if err != nil {
				return inputErrf("rule %s does not load:\n%v", sr.RuleID, err)
			}
			ri.Type, ri.Category, ri.Title = moduleType(r.Module), r.Category, r.Title
			ri.ValidFrom = r.ValidFrom.String()
			if !r.ValidTo.IsZero() {
				ri.ValidTo = r.ValidTo.String()
			}
			for _, ty := range r.TaxYears {
				ri.TaxYears = append(ri.TaxYears, ty.String())
				years[ty.String()] = true
			}
		} else {
			var rs rulestore.Rule
			if _, err := strictyaml.Decode(sr.FilePath, sr.Body, &rs); err != nil {
				return inputErrf("rule %s does not load:\n%v", sr.RuleID, err)
			}
			ri.Type, ri.Domain, ri.Category, ri.Title = rs.Rule.Type, rs.Rule.Domain, rs.Rule.Category, rs.Rule.Name
			ri.ValidFrom, ri.ValidTo = rs.Validity.ValidFrom, rs.Validity.ValidTo
		}
		if gate != nil && sr.Status == store.StatusTesting && ri.Runnable {
			gap, err := gate.gapOf(tx, approvalTarget{ID: sr.RuleID, VersionID: sr.VersionID, Version: sr.Version, Source: sr.Source, SHA256: sr.SHA256})
			if err != nil {
				return err
			}
			ri.Approvable, ri.ApprovalNote = gap == nil, gapNote(gap)
		}
		list = append(list, ri)
	}

	if *jsonOut != "" {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		taxYears := slices.Sorted(maps.Keys(years))
		if err := enc.Encode(map[string]any{"rules": list, "tax_years": append([]string{}, taxYears...)}); err != nil {
			return err
		}
		if err := os.WriteFile(*jsonOut, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	var b strings.Builder
	runnable := 0
	for _, r := range list {
		if r.Runnable {
			runnable++
		}
		fmt.Fprintf(&b, "%-14s %-9s %-14s %s\n", r.ID, r.Status, r.Type, r.Title)
	}
	fmt.Fprintf(&b, "%d rules: %d can be run, %d are described only\n", len(list), runnable, len(list)-runnable)
	_, err = io.WriteString(stdout, b.String())
	return err
}

// moduleType names an engine rule's module the way the rule list shows it.
func moduleType(module string) string {
	if module == "tieout" {
		return "tie-out"
	}
	return module
}

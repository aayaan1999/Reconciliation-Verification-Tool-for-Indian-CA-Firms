package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// rulebook-save (SPEC-010 scope 8): a Partner's edit of a rule-book rule, read from standard
// input as JSON, is written as canonical rule-store YAML, read back through the strict reader,
// and stored as a new office version of the rule in the database. The rule-book file under
// docs/ is never written, as a built-in rule's file is never written (RU-12). The rule's ID,
// type and domain cannot change. The rule's status is not changed by an edit. One audit entry.
//
//	recon-engine rulebook-save --db <file> --rule <ID> --json <file>
//	  (standard input: {"by": "A. Partner", "reason": "...", "rule": {...the rule-book rule...}})

// rulebookSaveInput is what rulebook-save reads from standard input.
type rulebookSaveInput struct {
	By     string         `json:"by"`
	Reason string         `json:"reason"`
	Rule   rulestore.Rule `json:"rule"`
}

func rulebookSave(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("rulebook-save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	ruleID := fs.String("rule", "", "the rule-book rule's ID, such as RC-ACC-003")
	jsonOut := fs.String("json", "", "write the saved version as JSON to this file")
	if err := parseFlags(fs, args, "db", "rule", "json"); err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	var in rulebookSaveInput
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the rule is not valid JSON: %v", err)
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return inputErrf("REASON_REQUIRED: say why the rule-book rule is changed")
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
	st, ok, err := tx.RuleStateOf(*ruleID)
	if err != nil {
		return err
	}
	if !ok {
		return inputErrf("RULE_NOT_FOUND: there is no rule %s", *ruleID)
	}
	active, err := activeVersion(tx, *ruleID, st.ActiveVersionID)
	if err != nil {
		return err
	}
	if active.Schema != rulestoreSchema {
		return inputErrf("NOT_RULEBOOK: %s is an engine rule; change it in the Rule Builder", *ruleID)
	}
	var old rulestore.Rule
	if _, err := strictyaml.Decode(active.FilePath, active.Body, &old); err != nil {
		return fmt.Errorf("rule %s: the version in use does not read: %w", *ruleID, err)
	}

	r := in.Rule
	switch {
	case r.Rule.ID != old.Rule.ID:
		return inputErrf("ID_CHANGED: the rule's ID cannot be changed (it is %s)", old.Rule.ID)
	case r.Rule.Type != old.Rule.Type:
		return inputErrf("TYPE_CHANGED: the rule's type cannot be changed (it is %s)", old.Rule.Type)
	case r.Rule.Domain != old.Rule.Domain:
		return inputErrf("DOMAIN_CHANGED: the rule's subject cannot be changed (it is %s)", old.Rule.Domain)
	case strings.TrimSpace(r.Rule.Name) == "":
		return inputErrf("NAME_REQUIRED: rule.name: the rule needs a name")
	case strings.TrimSpace(r.Rule.Purpose) == "":
		return inputErrf("PURPOSE_REQUIRED: rule.purpose: say what the rule checks")
	}
	// What only the store sets: the schema, the version text, whether the file enables it
	// (the status in the database is what counts), and who changed it last.
	r.SchemaVersion = rulestore.SchemaVersion
	r.Rule.Version = old.Rule.Version
	r.Governance.Enabled = old.Governance.Enabled
	// Nothing but the change stamp would differ: refuse, so an empty save adds no version.
	same := r
	same.Governance = old.Governance
	if a, b := mustJSON(same), mustJSON(old); bytes.Equal(a, b) {
		return inputErrf("NO_CHANGE: nothing in %s was changed", *ruleID)
	}
	r.Governance.LastModifiedBy = by
	r.Governance.LastModifiedOn = nowUTC()[:10]
	r.Governance.ChangeNote = reason

	var back rulestore.Rule
	body, err := rulestore.EncodeYAML(*ruleID+".yaml", r, &back)
	if err != nil {
		return inputErrf("BAD_RULE: %v", err)
	}
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	vid, err := tx.PutOfficeVersion(*ruleID, old.Rule.Version, rulestoreSchema, "office/rulebook/"+*ruleID+".yaml", body, sha, by)
	if err != nil {
		return err
	}
	if err := tx.Audit(nowUTC(), by, "rule.edit", "rule", *ruleID+" v"+old.Rule.Version, map[string]string{
		"reason": reason, "source": store.SourceOffice, "from_source": active.Source, "schema": rulestoreSchema,
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"rule_id": *ruleID, "version": old.Rule.Version, "version_id": vid,
			"source": store.SourceOffice, "status": st.Status}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "%s: saved as an office version of the rule-book rule\n", *ruleID)
	return err
}

// mustJSON writes a rule-book rule as JSON, to compare two of them. The structures hold only
// text, booleans and lists of them, so it cannot fail.
func mustJSON(r rulestore.Rule) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return b
}

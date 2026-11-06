package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// The rule commands of the Rules tab (SPEC-010): read a rule as the window edits it, switch a
// rule off or on with a reason, and list, switch or reset its versions. Each change writes one
// audit entry in the same transaction (RU-42). Nothing is deleted (RU-41).

// ruleCurrent is a rule as the Rules tab shows it, with the form the window edits.
type ruleCurrent struct {
	RuleID         string        `json:"rule_id"`
	Version        string        `json:"version"`
	Source         string        `json:"source"`
	Status         string        `json:"status"`
	DisabledReason string        `json:"disabled_reason"`
	Editable       bool          `json:"editable"`
	Reason         string        `json:"reason"`        // why it cannot be edited, when Editable is false
	Approvable     bool          `json:"approvable"`    // a testing rule the Partner may approve now (SPEC-010 §4)
	ApprovalNote   string        `json:"approval_note"` // why it may not be approved yet, when Approvable is false
	Form           *rules.Form   `json:"form"`
	Fixtures       []fixtureInfo `json:"fixtures"`
	// Rulebook is the whole rule-book rule, for a rule-book rule (SPEC-010 scope 8): the
	// summary shows it, and a Partner edits it with rulebook-save. Nil for an engine rule.
	Rulebook *rulestore.Rule `json:"rulebook"`
	// HasRulebookVersion says the rule-book version is stored, so a Partner's edit can be reset.
	HasRulebookVersion bool `json:"has_rulebook_version"`
}

type fixtureInfo struct {
	Kind        string `json:"kind"` // "pass" or "fail"
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ruleGet reads one rule, for the Rule Builder window to fill in (SPEC-010 AC-5, AC-6).
//
//	recon-engine rule-get --db <file> --rules <content/rules> --office-rules <folder> --rule RECON-DEP-002 --json <file>
func ruleGet(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("rule-get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	builtinDir := fs.String("rules", "", "built-in rules folder (content/rules)")
	officeDir := fs.String("office-rules", "", "office rules folder")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	ruleID := fs.String("rule", "", "the rule's ID, such as RECON-DEP-002")
	jsonOut := fs.String("json", "", "write the rule as JSON to this file")
	if err := parseFlags(fs, args, "db", "rules", "rule", "json"); err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	env, err := loadRuleEnv(*dbFile, *vocabDir, *builtinDir, *officeDir)
	if err != nil {
		return err
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
	out := ruleCurrent{RuleID: *ruleID, Version: active.Version, Source: active.Source, Status: st.Status,
		DisabledReason: st.DisabledReason, Editable: true}
	switch form, err := rules.FormOf(active.FilePath, active.Body); {
	case active.Schema == rulestoreSchema:
		var rb rulestore.Rule
		if _, err := strictyaml.Decode(active.FilePath, active.Body, &rb); err != nil {
			return fmt.Errorf("rule %s: the version in use does not read: %w", *ruleID, err)
		}
		out.Rulebook = &rb
		vs, err := tx.RuleVersions(*ruleID)
		if err != nil {
			return err
		}
		for _, v := range vs {
			out.HasRulebookVersion = out.HasRulebookVersion || v.Source == store.SourceRulebook
		}
	case active.Schema != rules.Schema:
		out.Editable, out.Reason = false, "this rule's file has a structure the tool does not edit"
	case err != nil:
		out.Editable, out.Reason = false, err.Error()
	default:
		out.Form = &form
		out.Fixtures, err = fixturesOf(env, active.Source, *ruleID)
		if err != nil {
			return err
		}
	}
	if st.Status == store.StatusTesting && active.Schema == rules.Schema {
		gate := &approvalGate{env: env}
		gap, err := gate.gapOf(tx, targetOf(*ruleID, st.ActiveVersionID, active))
		if err != nil {
			return err
		}
		out.Approvable, out.ApprovalNote = gap == nil, gapNote(gap)
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, out); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "%s v%s (%s, %s)\n", *ruleID, out.Version, out.Status, out.Source)
	return err
}

// approvalTarget is the version of a rule the approval gate checks: its ID, its stored version
// and the SHA-256 of its file, as the database has them.
type approvalTarget struct {
	ID        string
	VersionID int64
	Version   string
	Source    string
	SHA256    string
}

func targetOf(id string, versionID int64, v store.RuleVersion) approvalTarget {
	return approvalTarget{ID: id, VersionID: versionID, Version: v.Version, Source: v.Source, SHA256: v.SHA256}
}

// approvalGap says why a testing rule may not be approved yet: a code for the refusal and the
// reason in plain words.
type approvalGap struct {
	Code    string
	Message string
}

func gapNote(g *approvalGap) string {
	if g == nil {
		return ""
	}
	return g.Message
}

// approvalGate checks the gate of SPEC-010 §4 for testing rules. The rule's pass and fail
// fixtures must pass in the engine (the same check rule-save makes), and the rule must have
// been run on at least one client with testing rules included, as the database records it.
// The rule folders are read once, on first use.
type approvalGate struct {
	env    ruleEnv
	folder []*rules.Rule
	loaded bool
}

// gapOf returns why the rule may not be approved, or nil when it may.
func (g *approvalGate) gapOf(tx *store.Tx, t approvalTarget) (*approvalGap, error) {
	if !g.loaded {
		folder, err := loadFolderRules(g.env)
		if err != nil {
			return nil, err
		}
		g.folder, g.loaded = folder, true
	}
	var r *rules.Rule
	for _, c := range g.folder {
		if c.ID == t.ID && c.Version == t.Version && c.Source == t.Source {
			r = c
			break
		}
	}
	if r == nil {
		return &approvalGap{"RULE_FILE_CHANGED", fmt.Sprintf("the file of version %s is not in the rule folders, or has changed since it was loaded; load the rules again", t.Version)}, nil
	}
	body, err := os.ReadFile(r.File)
	if err != nil {
		return nil, inputErr(err)
	}
	if sha256Hex(body) != t.SHA256 {
		return &approvalGap{"RULE_FILE_CHANGED", "the rule file has changed since it was loaded; load the rules again (load-rules), then approve"}, nil
	}
	fxs, err := rules.LoadFixtures(r, g.env.v)
	if err != nil {
		return &approvalGap{"FIXTURES_NOT_PASSING", "its fixtures do not load: " + err.Error()}, nil
	}
	if ps := fixtureResultProblems(r, fxs, g.env); len(ps) > 0 {
		msgs := make([]string, len(ps))
		for i, p := range ps {
			msgs[i] = p.Message
		}
		return &approvalGap{"FIXTURES_NOT_PASSING", "the fixtures must pass before approval: " + strings.Join(msgs, "; ")}, nil
	}
	n, err := tx.ClientRunsWithTesting(t.ID, t.VersionID)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return &approvalGap{"NOT_RUN_ON_CLIENT", "the rule has not been run on a client with testing rules included; run a client's documents with testing rules on, then approve"}, nil
	}
	return nil, nil
}

// loadFolderRules reads the built-in rules and, when the folder exists, the office rules: the
// rule files the database's versions come from.
func loadFolderRules(env ruleEnv) ([]*rules.Rule, error) {
	bset, err := rules.LoadDir(env.builtin, env.v, env.checks)
	if err != nil {
		return nil, inputErrf("built-in rules %s:\n%v", env.builtin, err)
	}
	out := bset.Rules()
	if env.office == "" {
		return out, nil
	}
	if _, statErr := os.Stat(env.office); errors.Is(statErr, fs.ErrNotExist) {
		return out, nil
	} else if statErr != nil {
		return nil, inputErr(statErr)
	}
	oset, err := rules.LoadDir(env.office, env.v, env.checks)
	if err != nil {
		return nil, inputErrf("office rules %s:\n%v", env.office, err)
	}
	return append(out, oset.Rules()...), nil
}

// fixturesOf lists the fixtures a rule keeps in its folder: the office folder for an office
// version, the built-in folder otherwise.
func fixturesOf(env ruleEnv, source, id string) ([]fixtureInfo, error) {
	dir := env.builtin
	if source == store.SourceOffice {
		dir = env.office
	}
	set, err := rules.LoadDir(dir, env.v, env.checks)
	if err != nil {
		return nil, inputErrf("rules %s:\n%v", dir, err)
	}
	var out []fixtureInfo
	for _, r := range set.Rules() {
		if r.ID != id {
			continue
		}
		fxs, err := rules.LoadFixtures(r, env.v)
		if err != nil {
			return nil, inputErrf("fixtures of %s:\n%v", id, err)
		}
		for _, f := range append(append([]*rules.Fixture{}, fxs.Pass...), fxs.Fail...) {
			out = append(out, fixtureInfo{Kind: f.Kind, Name: strings.TrimSuffix(filepath.Base(f.File), ".yaml"), Description: f.Description})
		}
	}
	return out, nil
}

// activeVersion returns the stored version of a rule that is in use.
func activeVersion(tx *store.Tx, id string, versionID int64) (store.RuleVersion, error) {
	vs, err := tx.RuleVersions(id)
	if err != nil {
		return store.RuleVersion{}, err
	}
	for _, v := range vs {
		if v.ID == versionID {
			return v, nil
		}
	}
	return store.RuleVersion{}, fmt.Errorf("rule %s: the version in use is missing", id)
}

// ruleSwitch reads the change a rule-disable or rule-enable makes: who makes it, and why.
type ruleSwitch struct {
	By     string `json:"by"`
	Reason string `json:"reason"`
}

// ruleDisable takes a rule out of use. The reason is required; the files and versions stay.
//
//	echo '{"by":"A. Staff","reason":"..."}' | recon-engine rule-disable --db <file> --rule <ID> --json <file>
func ruleDisable(args []string, stdin io.Reader, stdout io.Writer) error {
	return ruleSwitchStatus("rule-disable", args, stdin, stdout, true)
}

// ruleEnable takes a switched-off rule back into use as testing. The reason is required.
func ruleEnable(args []string, stdin io.Reader, stdout io.Writer) error {
	return ruleSwitchStatus("rule-enable", args, stdin, stdout, false)
}

func ruleSwitchStatus(name string, args []string, stdin io.Reader, stdout io.Writer, disable bool) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	ruleID := fs.String("rule", "", "the rule's ID, such as RECON-DEP-002")
	ruleIDs := fs.String("rules", "", "several rules' IDs, separated by commas, changed together or not at all (SPEC-010 scope 9)")
	jsonOut := fs.String("json", "", "write the rule's new status as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	ids, err := switchIDs(*ruleID, *ruleIDs)
	if err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	var in ruleSwitch
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the change is not valid JSON: %v", err)
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return inputErrf("REASON_REQUIRED: say why the rule is %s", map[bool]string{true: "switched off", false: "switched on again"}[disable])
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
	// Every rule is changed in this one transaction: a refusal of any of them returns before
	// the commit, so nothing is changed (SPEC-010 scope 9).
	type switched struct {
		RuleID  string `json:"rule_id"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}
	at := nowUTC()
	done := make([]switched, 0, len(ids))
	for _, id := range ids {
		st, ok, err := tx.RuleStateOf(id)
		if err != nil {
			return err
		}
		if !ok {
			return inputErrf("RULE_NOT_FOUND: there is no rule %s", id)
		}
		active, err := activeVersion(tx, id, st.ActiveVersionID)
		if err != nil {
			return err
		}
		rulebook := active.Schema == rulestoreSchema
		switch {
		case active.Schema != rules.Schema && !rulebook:
			return inputErrf("RULE_NOT_RUNNABLE: %s has a structure the tool cannot switch off or on", id)
		case disable && st.Status == store.StatusDisabled:
			return inputErrf("ALREADY_DISABLED: %s is already switched off", id)
		case !disable && st.Status != store.StatusDisabled:
			return inputErrf("NOT_DISABLED: %s is not switched off", id)
		}
		// An engine rule switched on is testing and must be approved again (RU-40). A rule-book
		// rule switched on is active, as governance.enabled makes it on load; it is described
		// only and never run, so it has nothing to approve (SPEC-010 scope 8).
		status, action, kept := store.StatusTesting, "rule.enable", ""
		if rulebook {
			status = store.StatusActive
		}
		if disable {
			status, action, kept = store.StatusDisabled, "rule.disable", reason
		}
		// The name is kept with the status, so loading the rule book again leaves it as set.
		if err := tx.SetRuleStatusBy(id, status, kept, by); err != nil {
			return err
		}
		if err := tx.Audit(at, by, action, "rule", id+" v"+active.Version,
			map[string]string{"reason": reason, "source": active.Source, "from_status": st.Status}); err != nil {
			return err
		}
		done = append(done, switched{RuleID: id, Version: active.Version, Status: status})
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if *jsonOut != "" {
		// One rule keeps the shape the single-rule screens read; several are listed.
		var out any = map[string]any{"rules": done, "reason": reason}
		if *ruleID != "" {
			out = map[string]any{"rule_id": done[0].RuleID, "version": done[0].Version, "status": done[0].Status, "reason": reason}
		}
		if err := writeJSONFile(*jsonOut, out); err != nil {
			return err
		}
	}
	var b strings.Builder
	for _, d := range done {
		fmt.Fprintf(&b, "%s v%s is now %s\n", d.RuleID, d.Version, d.Status)
	}
	_, err = io.WriteString(stdout, b.String())
	return err
}

// maxSwitchRules is the most rules one rule-disable or rule-enable changes (SPEC-010 scope 9).
const maxSwitchRules = 500

// switchIDs reads the rules a rule-disable or rule-enable changes: --rule for one, or --rules
// for several, separated by commas. Each ID is named once.
func switchIDs(one, many string) ([]string, error) {
	switch {
	case one != "" && many != "":
		return nil, inputErrf("BAD_REQUEST: give --rule or --rules, not both")
	case one != "":
		return []string{one}, nil
	case strings.TrimSpace(many) == "":
		return nil, inputErrf("BAD_REQUEST: the flag --rule or --rules is required")
	}
	var ids []string
	seen := map[string]bool{}
	for _, id := range strings.Split(many, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if seen[id] {
			return nil, inputErrf("BAD_REQUEST: %s is named twice", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	switch {
	case len(ids) == 0:
		return nil, inputErrf("NO_RULES_SELECTED: choose at least one rule")
	case len(ids) > maxSwitchRules:
		return nil, inputErrf("TOO_MANY_RULES: at most %d rules can be changed at once; %d were chosen", maxSwitchRules, len(ids))
	}
	return ids, nil
}

// ruleApprove makes a testing rule active (SPEC-010 §4, RU-40). The approver's name and a reason
// are required, and the gate must pass: the rule's pass and fail fixtures pass in the engine, and
// a client's run included the rule with testing rules on. An active rule runs in every run,
// without --include-testing. The change is one audit entry, with the name and the reason.
//
//	echo '{"by":"A. Partner","reason":"..."}' | recon-engine rule-approve --db <file> --rules <content/rules> --office-rules <folder> --rule <ID> --json <file>
func ruleApprove(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("rule-approve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dbFile := flags.String("db", "", "database file")
	builtinDir := flags.String("rules", "", "built-in rules folder (content/rules)")
	officeDir := flags.String("office-rules", "", "office rules folder")
	vocabDir := flags.String("vocab", "content/vocabulary", "fact vocabulary folder")
	ruleID := flags.String("rule", "", "the rule's ID, such as RECON-DEP-002")
	jsonOut := flags.String("json", "", "write the rule's new status as JSON to this file")
	if err := parseFlags(flags, args, "db", "rules", "rule", "json"); err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	var in ruleSwitch
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the approval is not valid JSON: %v", err)
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return inputErrf("REASON_REQUIRED: say why the rule is approved")
	}
	env, err := loadRuleEnv(*dbFile, *vocabDir, *builtinDir, *officeDir)
	if err != nil {
		return err
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
	switch {
	case active.Schema != rules.Schema:
		return inputErrf("RULE_NOT_RUNNABLE: %s is described only; it is not run, so it cannot be approved", *ruleID)
	case st.Status == store.StatusActive:
		return inputErrf("ALREADY_ACTIVE: %s is already active", *ruleID)
	case st.Status == store.StatusDisabled:
		return inputErrf("NOT_TESTING: %s is switched off; enable it again first, then approve it", *ruleID)
	case st.Status != store.StatusTesting:
		return inputErrf("NOT_TESTING: %s is %s, so it cannot be approved", *ruleID, st.Status)
	}
	gate := &approvalGate{env: env}
	gap, err := gate.gapOf(tx, targetOf(*ruleID, st.ActiveVersionID, active))
	if err != nil {
		return err
	}
	if gap != nil {
		return inputErrf("%s: %s: %s", gap.Code, *ruleID, gap.Message)
	}
	now := nowUTC()
	if err := tx.ApproveRule(*ruleID, by, now); err != nil {
		return err
	}
	if err := tx.Audit(now, by, "rule.approve", "rule", *ruleID+" v"+active.Version,
		map[string]string{"reason": reason, "from_status": st.Status, "source": active.Source}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"rule_id": *ruleID, "version": active.Version, "status": store.StatusActive,
			"approved_by": by, "reason": reason}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "%s v%s is now active, approved by %s\n", *ruleID, active.Version, by)
	return err
}

// versionOut is one stored version of a rule, as the Rules tab lists it.
type versionOut struct {
	ID        int64  `json:"id"`
	Revision  int64  `json:"revision"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	TaxYears  string `json:"tax_years"`
	FilePath  string `json:"file_path"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	Active    bool   `json:"active"`
}

// ruleVersions lists a rule's versions, oldest first, and with --switch <version ID> or
// --reset (back to the standard rule) makes another version the one in use. Each switch is
// recorded in the audit log with who made it (RU-41).
//
//	recon-engine rule-versions --db <file> --rule <ID> --json <file>
//	echo '{"by":"A. Staff"}' | recon-engine rule-versions --db <file> --rule <ID> --switch <version ID> --json <file>
//	echo '{"by":"A. Staff"}' | recon-engine rule-versions --db <file> --rule <ID> --reset --json <file>
func ruleVersions(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("rule-versions", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	ruleID := fs.String("rule", "", "the rule's ID, such as RECON-DEP-002")
	toVersion := fs.String("switch", "", "make this version (its ID from the list) the one in use")
	reset := fs.Bool("reset", false, "make the standard (built-in) version the one in use again")
	jsonOut := fs.String("json", "", "write the versions as JSON to this file")
	if err := parseFlags(fs, args, "db", "rule", "json"); err != nil {
		return err
	}
	if *toVersion != "" && *reset {
		return inputErrf("CHOOSE_ONE: give --switch or --reset, not both")
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
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
	vs, err := tx.RuleVersions(*ruleID)
	if err != nil {
		return err
	}
	active, err := activeVersion(tx, *ruleID, st.ActiveVersionID)
	if err != nil {
		return err
	}
	if *toVersion != "" || *reset {
		if err := switchVersion(tx, stdin, active, vs, *ruleID, *toVersion, *reset); err != nil {
			return err
		}
		// The list and the state after the change, read in the same transaction, then committed.
		if vs, err = tx.RuleVersions(*ruleID); err != nil {
			return err
		}
		if st, _, err = tx.RuleStateOf(*ruleID); err != nil {
			return err
		}
		if active, err = activeVersion(tx, *ruleID, st.ActiveVersionID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	list := make([]versionOut, 0, len(vs))
	for _, v := range vs {
		list = append(list, versionOut{ID: v.ID, Revision: v.Revision, Version: v.Version, Source: v.Source, TaxYears: v.TaxYears,
			FilePath: v.FilePath, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, Active: v.Active})
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"rule_id": *ruleID, "status": st.Status, "versions": list}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "%s: %d versions, version in use %s (%s)\n", *ruleID, len(list), active.Version, st.Status)
	return err
}

// switchVersion makes another version of the rule the one in use, or the standard one, and
// writes its audit entry. The caller commits.
func switchVersion(tx *store.Tx, stdin io.Reader, active store.RuleVersion, vs []store.RuleVersion, id, toVersion string, reset bool) error {
	var in ruleSwitch
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the change is not valid JSON: %v", err)
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	var target store.RuleVersion
	action := "rule.switch_version"
	if reset {
		action = "rule.reset"
		// The standard version is the newest built-in one, or, for a rule-book rule, the
		// newest rule-book one (SPEC-010 scope 8).
		standard := store.SourceBuiltin
		if active.Schema == rulestoreSchema {
			standard = store.SourceRulebook
		}
		found := false
		for _, v := range vs {
			if v.Source == standard {
				target, found = v, true
			}
		}
		switch {
		case !found:
			return inputErrf("NO_STANDARD_RULE: %s has no standard rule to go back to", id)
		case target.ID == active.ID:
			return inputErrf("ALREADY_STANDARD: %s already uses the standard rule", id)
		}
	} else {
		n, err := strconv.ParseInt(toVersion, 10, 64)
		if err != nil {
			return inputErrf("BAD_VERSION: the version is a number from the list")
		}
		found := false
		for _, v := range vs {
			if v.ID == n {
				target, found = v, true
			}
		}
		switch {
		case !found:
			return inputErrf("VERSION_NOT_FOUND: %s has no version %d", id, n)
		case target.ID == active.ID:
			return inputErrf("VERSION_IN_USE: version %s is already the one in use", target.Version)
		case target.Schema != active.Schema:
			return inputErrf("RULE_NOT_RUNNABLE: version %s has a different structure from the version in use and cannot be used", target.Version)
		}
	}
	if err := tx.ActivateVersion(id, target.ID); err != nil {
		return err
	}
	return tx.Audit(nowUTC(), by, action, "rule", id+" v"+target.Version, map[string]string{
		"from": active.Version, "from_source": active.Source, "to": target.Version, "to_source": target.Source,
		"to_revision": strconv.FormatInt(target.Revision, 10),
	})
}

// optionsOut is what the Rule Builder window offers in its lists (SPEC-010 §2): the modules,
// the categories, the severities, the check types the engine runs, the figures and record types
// of the vocabulary (retired names left out), and the documents that group findings.
type optionsOut struct {
	Modules       []string      `json:"modules"`
	Severities    []string      `json:"severities"`
	Categories    []categoryOpt `json:"categories"`
	CheckTypes    []string      `json:"check_types"`
	RowCheckTypes []string      `json:"row_check_types"` // the check types that read rows: the only ones that can take an office record type (SPEC-022)
	Facts         []factOpt     `json:"facts"`
	RecordTypes   []recordOpt   `json:"record_types"`
	DocumentTypes []documentOpt `json:"document_types"`
}

type categoryOpt struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type factOpt struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"` // money, int, text, date, bool or rate
	Periodicity  string `json:"periodicity"`
	DocumentType string `json:"document_type"`
	Description  string `json:"description"`
}

type recordOpt struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	DocumentType string   `json:"document_type"`
	MatchKey     []string `json:"match_key"`
	Amounts      []string `json:"amounts"`
	Fields       []string `json:"fields"`
	// SPEC-022: a record type of a document the Partner added has its columns with their labels and
	// types, and is offered for a new rule only while its document is active.
	Origin  string      `json:"origin"` // "builtin" or "office"
	Active  bool        `json:"active"`
	Columns []columnOpt `json:"columns,omitempty"`
}

type columnOpt struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text, date or amount
	Key      bool   `json:"key"`
	Required bool   `json:"required"`
}

type documentOpt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ruleOptions writes the lists the Rule Builder window offers. It only reads.
//
//	recon-engine rule-options --rules <content/rules> [--db <file>] [--vocab <folder>] --json <file>
func ruleOptions(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("rule-options", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	builtinDir := fs.String("rules", "", "built-in rules folder (content/rules), with its categories.yaml")
	dbFile := fs.String("db", "", "database, for the documents the Partner added (SPEC-022); without it only the built-in record types are offered")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the lists as JSON to this file")
	if err := parseFlags(fs, args, "rules", "json"); err != nil {
		return err
	}
	env, err := loadRuleEnv(*dbFile, *vocabDir, *builtinDir, "")
	if err != nil {
		return err
	}
	out := optionsOut{Modules: slices.Clone(rules.Modules), Severities: rules.SeverityList(), CheckTypes: env.checks.Names(), RowCheckTypes: env.checks.RowReading()}
	for _, c := range env.cats {
		out.Categories = append(out.Categories, categoryOpt{Code: c.Code, Name: c.Name, Description: c.Description})
	}
	for _, f := range env.v.Facts() {
		if f.Deprecated() {
			continue
		}
		out.Facts = append(out.Facts, factOpt{Name: f.Name, Kind: f.Kind.String(), Periodicity: string(f.Periodicity),
			DocumentType: f.DocumentType, Description: f.Description})
	}
	for _, r := range env.v.RecordTypes() {
		if r.Deprecated() {
			continue
		}
		opt := recordOpt{Name: r.Name, Description: r.Description, DocumentType: r.DocumentType,
			MatchKey: r.MatchKey, Amounts: r.Amounts, Fields: r.Fields, Origin: store.OriginBuiltin, Active: true}
		if env.v.IsOfficeRecordType(r.Name) {
			opt.Origin = store.OriginOffice
			if i := slices.IndexFunc(env.master.docs, func(d store.DocType) bool { return d.Code == r.DocumentType }); i >= 0 {
				opt.Active = env.master.docs[i].Active
			}
			cols, _ := env.v.OfficeColumns(r.Name)
			for _, c := range cols {
				opt.Columns = append(opt.Columns, columnOpt{Name: c.Name, Label: c.Label, Type: c.Type, Key: c.Key, Required: c.Required})
			}
		}
		out.RecordTypes = append(out.RecordTypes, opt)
	}
	for _, d := range env.v.DocumentTypes() {
		out.DocumentTypes = append(out.DocumentTypes, documentOpt{Name: d.Name, Description: d.Description})
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, out); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "%d figures, %d record types, %d check types\n", len(out.Facts), len(out.RecordTypes), len(out.CheckTypes))
	return err
}

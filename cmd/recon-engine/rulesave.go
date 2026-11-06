package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// rule-save (SPEC-010 §2 to §3): the Rule Builder's form, read from standard input as JSON, is
// checked, written as a rule file with its fixtures in the office rules folder, and recorded
// as a new version with status testing. Nothing is written when anything is refused: the
// problems go to --json, each with its field, and the command exits 2.
//
//	recon-engine rule-save --db <file> --rules <content/rules> --office-rules <folder> --json <file>
//	  (standard input: {"by": "A. Staff", "rule": {...the form...}})

// ruleSaveInput is what rule-save reads from standard input.
type ruleSaveInput struct {
	By   string     `json:"by"`
	Rule rules.Form `json:"rule"`
}

func ruleSave(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("rule-save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	builtinDir := fs.String("rules", "", "built-in rules folder (content/rules): read, never written")
	officeDir := fs.String("office-rules", "", "office rules folder: the rule is written here")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the result, or the problems with the rule, as JSON to this file")
	if err := parseFlags(fs, args, "db", "rules", "office-rules", "json"); err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	var in ruleSaveInput
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the rule is not valid JSON: %v", err)
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	env, err := loadRuleEnv(*dbFile, *vocabDir, *builtinDir, *officeDir)
	if err != nil {
		return err
	}
	f := in.Rule
	refuse := func(ps []rules.Problem) error {
		if err := writeJSONFile(*jsonOut, map[string]any{"problems": ps}); err != nil {
			return err
		}
		return inputErrf("RULE_REFUSED: %s", ps[0].Message)
	}

	// 1. The form's own fields and fixtures, as typed.
	ctx := rules.Context{Vocab: env.v, Checks: env.checks, Categories: rules.CategoryCodes(env.cats)}
	problems := f.Problems(ctx)
	problems = append(problems, f.FixtureProblems(ctx)...)
	if f.Mode != "create" && f.Mode != "edit" {
		problems = append(problems, rules.Problem{Field: "mode", Code: "BAD_MODE", Message: `the window must say whether the rule is new ("create") or changed ("edit")`})
	}
	if f.Mode == "create" { // a changed rule's kept fixtures are counted in step 3
		problems = append(problems, rules.FixtureCountProblems(formFixtureKinds(f.Fixtures))...)
	}
	if len(problems) > 0 {
		return refuse(problems)
	}
	years, err := parseYears(f.TaxYears)
	if err != nil {
		return refuse([]rules.Problem{{Field: "tax_years", Code: "BAD_TAX_YEAR", Message: err.Error()}})
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

	// 2. The rule's place: a new rule must not exist for its years; a changed one must exist,
	// with the same years and module.
	builtin, office, err := existingRules(f.ID, env)
	if err != nil {
		return err
	}
	versions, err := tx.RuleVersions(f.ID)
	if err != nil {
		return err
	}
	var target *rules.Rule // the rule this save changes; nil for a new rule
	switch f.Mode {
	case "create":
		for _, r := range append(slices.Clone(builtin), office...) {
			if yearsOverlap(r.TaxYears, years) {
				problems = append(problems, rules.Problem{Field: "id", Code: "ID_EXISTS", Message: fmt.Sprintf("the rule %s already exists for %s; choose another ID", f.ID, r.TaxYears[0])})
				break
			}
		}
		for _, v := range versions {
			if vy := splitYears(v.TaxYears); len(vy) == 0 || yearsOverlap(vy, years) {
				problems = append(problems, rules.Problem{Field: "id", Code: "ID_EXISTS", Message: fmt.Sprintf("the rule %s is already in the database for these years; choose another ID", f.ID)})
				break
			}
		}
	case "edit":
		switch {
		case len(office) > 0:
			target = office[0]
		case len(builtin) > 0:
			target = builtin[0]
		default:
			problems = append(problems, rules.Problem{Field: "id", Code: "RULE_NOT_FOUND", Message: fmt.Sprintf("there is no rule %s to change", f.ID)})
		}
		if target != nil {
			if !slices.Equal(target.TaxYears, years) {
				problems = append(problems, rules.Problem{Field: "tax_years", Code: "TAX_YEARS_FIXED", Message: "the tax years of a rule that exists cannot change here; a new rule is needed for other years"})
			}
			if f.Module != target.Module {
				problems = append(problems, rules.Problem{Field: "module", Code: "MODULE_FIXED", Message: "the module of a rule that exists cannot change here"})
			}
		}
	}
	if len(problems) > 0 {
		return refuse(problems)
	}

	// 3. The fixtures: an edit keeps the rule's own fixtures, and the ones typed now replace a
	// kept one of the same kind and name.
	fixtures, err := carriedFixtures(target, env)
	if err != nil {
		return err
	}
	for _, fx := range f.Fixtures {
		fixtures[fixtureKey(fx.Kind, fx.Name)] = fixtureFile{kind: fx.Kind, name: fx.Name,
			initial: rules.FormFixtureYAML(fx, years[0].String(), placeholderFor(fx.Kind, fx.Facts, fx.Records, years[0]))}
	}
	var kinds []string
	for _, fx := range fixtures {
		kinds = append(kinds, fx.kind)
	}
	if ps := rules.FixtureCountProblems(kinds); len(ps) > 0 {
		return refuse(ps)
	}

	// 4. The version: 1 for a new rule; an office edit of a built-in rule is "1-office.1"; a
	// later edit counts up from the highest office edit of the same version (RU-12).
	version := "1"
	if target != nil {
		base, _, _ := strings.Cut(target.Version, "-office.")
		version = base + "-office." + strconv.Itoa(nextOfficeEdit(versions, base))
	}
	ruleYAML, err := rules.RuleYAML(&f, version, store.SourceOffice)
	if err != nil {
		return refuse([]rules.Problem{{Field: "rule", Code: "RULE_INVALID", Message: err.Error()}})
	}

	// 5. Check in a temporary folder laid out as the office folder is: the strict loader, the
	// fixtures, and each fixture run through the engine (AC-4). Nothing is written yet.
	tmp, err := os.MkdirTemp("", "recon-rule-save-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	ruleDir := filepath.Join(tmp, years[0].String(), f.Module, f.ID)
	if err := writeFile(filepath.Join(ruleDir, rules.FileName), ruleYAML); err != nil {
		return err
	}
	for _, fx := range fixtures {
		if err := writeFile(filepath.Join(ruleDir, "fixtures", fx.kind, fx.name+".yaml"), fx.initial); err != nil {
			return err
		}
	}
	set, err := rules.LoadDir(tmp, env.v, env.checks)
	if err != nil {
		return refuse(loadProblems(err, tmp, "RULE_INVALID"))
	}
	if len(set.Rules()) != 1 {
		return fmt.Errorf("the rule folder holds %d rules, want 1", len(set.Rules()))
	}
	r := set.Rules()[0]
	fxs, err := rules.LoadFixtures(r, env.v)
	if err != nil {
		return refuse(loadProblems(err, tmp, "FIXTURE_INVALID"))
	}
	if problems := fixtureResultProblems(r, fxs, env); len(problems) > 0 {
		return refuse(problems)
	}
	if err := writeExpected(r, fxs, env); err != nil {
		return err
	}
	if err := checkExpected(r, env); err != nil {
		return err
	}

	// 6. The database and the folder change together: the new version and its audit entry are
	// written in one transaction, and the files are written before the commit and undone if
	// anything after them fails.
	prior := ""
	st, hasState, err := tx.RuleStateOf(f.ID)
	if err != nil {
		return err
	}
	if hasState {
		for _, v := range versions {
			if v.ID == st.ActiveVersionID {
				prior = v.Version
			}
		}
	}
	ruleBody, err := os.ReadFile(filepath.Join(ruleDir, rules.FileName))
	if err != nil {
		return err
	}
	finalFile := filepath.Join(env.office, years[0].String(), f.Module, f.ID, rules.FileName)
	changed, err := tx.PutRule(store.RuleFile{RuleID: f.ID, Version: version, Source: store.SourceOffice, Schema: rules.Schema,
		TaxYears: taxYearList(r.TaxYears), FilePath: relPath(env.office, finalFile), Body: ruleBody, SHA256: sha256Hex(ruleBody)},
		store.RuleStatus{Status: store.StatusTesting})
	var ce *store.ConflictError
	switch {
	case errors.As(err, &ce):
		return inputErr(err)
	case err != nil:
		return err
	case !changed:
		return refuse([]rules.Problem{{Field: "rule", Code: "NO_CHANGE", Message: "nothing differs from the version in use, so nothing is saved"}})
	}
	action, kind := "rule.create", "created"
	if target != nil {
		action, kind = "rule.edit", "edited"
	}
	if err := tx.Audit(nowUTC(), by, action, "rule", f.ID+" v"+version, map[string]string{"previous": prior, "source": store.SourceOffice}); err != nil {
		return err
	}
	status, _, err := tx.RuleStateOf(f.ID)
	if err != nil {
		return err
	}
	undo := &fileUndo{}
	if err := writeOfficeFiles(undo, env.office, years[0], f.Module, f.ID, target, ruleDir, ruleBody); err != nil {
		undo.rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		undo.rollback()
		return err
	}

	res := map[string]any{"rule_id": f.ID, "version": version, "source": store.SourceOffice, "status": status.Status,
		"action": kind, "file": relPath(env.office, finalFile)}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, res); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "saved %s v%s (%s), kept in %s\n", f.ID, version, status.Status, res["file"])
	return err
}

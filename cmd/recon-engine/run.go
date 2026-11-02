package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/comparison"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/findings"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	csvadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/csv"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/modules/newheads"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// sniffBytes is how much of a file the adapters' Sniff sees.
const sniffBytes = 512

// A readFile is one input file after detection and extraction.
type readFile struct {
	name    string
	sha256  string
	profile *profile.Profile
	result  *profile.Result
	// priorYear marks last year's ledger uploaded with this year's documents (SPEC-006
	// part 3): it is stored in the engagement of the prior tax year.
	priorYear bool
}

// run carries out one engine run (SPEC-003 §3): detect, extract, resolve, snapshot, plan,
// evaluate, render. Every file is detected and read before anything is written, and
// everything the run writes is in one transaction, so an error leaves nothing partial.
func run(args []string, stdout io.Writer) (int, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	input := fs.String("input", "", "file or folder to read")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	profilesDir := fs.String("profiles", "content/profiles", "format profiles folder")
	officeDir := fs.String("office-profiles", "", "profiles taught in the office (profile-create); read too when the folder exists")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	includeTesting := fs.Bool("include-testing", false, "also evaluate rules not yet approved by the CA")
	jsonOut := fs.String("json", "", "also write the report as canonical JSON to this file")
	comparisonID := fs.String("comparison", "", "the type of check (an id in --comparisons): run only its rules")
	comparisonsFile := fs.String("comparisons", "content/comparisons/comparisons.yaml", "comparison types file")
	clientID := fs.Int64("client", 0, "run for this client (from client-add): its documents are kept across runs and each file's PAN must be the client's")
	worklistFile := fs.String("worklist", "", "thresholds of the year-on-year ledger comparison (the Worklist); default worklist/newheads.yaml next to the --vocab folder")
	ruleList := fs.String("rules", "", "with --comparison: run only these of its rules (IDs separated by commas); the others are reported as not selected")
	officeRules := fs.String("office-rules", "", "office rules folder (SPEC-010): its rules are read into the database first, and read too when the folder exists")
	if err := parseFlags(fs, args, "db", "input", "tax-year"); err != nil {
		return 0, err
	}
	rulesGiven := false
	fs.Visit(func(f *flag.Flag) { rulesGiven = rulesGiven || f.Name == "rules" })
	ty, err := period.ParseTaxYear(*taxYear)
	if err != nil {
		return 0, inputErrf("--tax-year: %v", err)
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return 0, inputErrf("vocabulary: %v", err)
	}
	profiles, err := loadProfiles(*profilesDir, *officeDir, v)
	if err != nil {
		return 0, err
	}
	if *jsonOut != "" {
		// A bad --json path is the user's mistake (exit 2), found before any work is done.
		if st, err := os.Stat(filepath.Dir(*jsonOut)); err != nil || !st.IsDir() {
			return 0, inputErrf("--json: the folder of %s does not exist", *jsonOut)
		}
	}
	var cmpType *comparison.Comparison
	if *comparisonID != "" {
		list, err := comparison.Load(*comparisonsFile, v)
		if err != nil {
			return 0, inputErrf("comparisons %s:\n%v", *comparisonsFile, err)
		}
		c, ok := comparison.Find(list, *comparisonID)
		if !ok {
			return 0, inputErrf("COMPARISON_NOT_FOUND: there is no comparison type %q", *comparisonID)
		}
		cmpType = &c
	}
	// A run of every rule (no type of check) shows the row totals of the documents a totals
	// type reads, for those uploaded: the comparison types file says which they are. Without
	// the file (the default path in another folder) a run of every rule shows no totals.
	var totalsDocs []string
	if cmpType != nil && cmpType.Totals {
		totalsDocs = cmpType.Documents
	} else if cmpType == nil {
		if _, err := os.Stat(*comparisonsFile); err == nil {
			list, err := comparison.Load(*comparisonsFile, v)
			if err != nil {
				return 0, inputErrf("comparisons %s:\n%v", *comparisonsFile, err)
			}
			for _, c := range list {
				for _, d := range c.Documents {
					if c.Totals && !slices.Contains(totalsDocs, d) {
						totalsDocs = append(totalsDocs, d)
					}
				}
			}
		}
	}
	selected, err := selectedRules(cmpType, rulesGiven, *ruleList)
	if err != nil {
		return 0, err
	}
	if *worklistFile == "" {
		*worklistFile = filepath.Join(filepath.Dir(*vocabDir), "worklist", "newheads.yaml")
	}
	thData, err := os.ReadFile(*worklistFile)
	if err != nil {
		return 0, inputErrf("--worklist: %v", err)
	}
	thresholds, err := newheads.ParseThresholds(*worklistFile, thData)
	if err != nil {
		return 0, inputErrf("worklist thresholds %s:\n%v", *worklistFile, err)
	}
	reg := ingest.NewRegistry()
	if err := reg.Register(jsonadapter.New(0)); err != nil {
		return 0, err
	}
	if err := reg.Register(csvadapter.New(0)); err != nil {
		return 0, err
	}
	if err := reg.Register(xlsxadapter.New(0)); err != nil {
		return 0, err
	}

	// 1–2. Detect and extract every file, in name order, before writing anything.
	paths, err := inputFiles(*input)
	if err != nil {
		return 0, err
	}
	var read []readFile
	for _, p := range paths {
		rfs, err := readOne(p, reg, profiles, ty)
		if err != nil {
			return 0, err
		}
		read = append(read, rfs...)
	}

	db, err := store.Open(*dbFile)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// 3. Resolve: store the files as drafts of their slots, then take the current view.
	hasPrior := slices.ContainsFunc(read, func(rf readFile) bool { return rf.priorYear })
	var eng, priorEng int64 // priorEng is 0 when there is no engagement for the prior year
	if *clientID > 0 {
		c, found, err := tx.ClientByID(*clientID)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
		}
		if eng, err = tx.ClientEngagement(c.ID, ty.String()); err != nil {
			return 0, err
		}
		if hasPrior {
			if priorEng, err = tx.ClientEngagement(c.ID, ty.Prior().String()); err != nil {
				return 0, err
			}
		} else if id, found, err := tx.FindClientEngagement(c.ID, ty.Prior().String()); err != nil {
			return 0, err
		} else if found {
			priorEng = id // last year's ledger uploaded in an earlier run
		}
		for _, rf := range read {
			if err := checkClientPAN(tx, c, rf.result); err != nil {
				return 0, err
			}
		}
		if err := scopeFromClient(tx, c, v, read); err != nil {
			return 0, err
		}
	} else {
		key, err := filepath.Abs(*input)
		if err != nil {
			return 0, err
		}
		if eng, err = tx.Engagement(filepath.ToSlash(key), ty.String()); err != nil {
			return 0, err
		}
		if hasPrior {
			if priorEng, err = tx.Engagement(filepath.ToSlash(key), ty.Prior().String()); err != nil {
				return 0, err
			}
		}
	}
	for _, rf := range read {
		report, err := findings.MarshalValidationReport(rf.result.Report)
		if err != nil {
			return 0, err
		}
		ctx := rf.result.Context
		into := eng
		if rf.priorYear {
			into = priorEng
		}
		if _, _, err := tx.PutUpload(store.Upload{
			EngagementID: into, FileName: rf.name, SHA256: rf.sha256, UploadedBy: "recon-engine",
			Slot:      store.Slot{DocumentType: rf.profile.DocumentType, Period: ctx.Period, Scope: ctx.Scope, Part: ctx.Part},
			ProfileID: rf.profile.ID, ProfileVersion: rf.profile.Version,
			OK: !rf.result.Report.HasErrors(), ReportJSON: report, Facts: rf.result.Facts, Records: rf.result.Records,
		}); err != nil {
			return 0, err
		}
	}
	docs, facts, err := tx.CurrentView(eng)
	if err != nil {
		return 0, err
	}
	records, err := tx.CurrentRecords(eng)
	if err != nil {
		return 0, err
	}
	// The Worklist (SPEC-006 part 3): last year's ledger against this year's, when both are
	// there. Its items are for staff to classify; they are never findings.
	var priorDocs []store.DocVersion
	var priorRecords []domain.Record
	if priorEng != 0 {
		if priorDocs, _, err = tx.CurrentView(priorEng); err != nil {
			return 0, err
		}
		if priorRecords, err = tx.CurrentRecords(priorEng); err != nil {
			return 0, err
		}
	}
	isLedger := func(r domain.Record) bool { return r.RecordType == newheads.RecordType }
	hasWorklist := slices.ContainsFunc(records, isLedger) && slices.ContainsFunc(priorRecords, isLedger)
	var worklist []newheads.Item
	if hasWorklist {
		if worklist, err = newheads.Compare(priorRecords, records, thresholds); err != nil {
			return 0, err
		}
	}

	// The office rules (SPEC-010) are read into the database first, so the run uses, and records,
	// the office version of every rule that has one.
	if _, err := loadOfficeRules(tx, *officeRules, v); err != nil {
		return 0, err
	}

	// 5. Plan (before the snapshot, which records the rules used): active rules for the tax
	// year, and testing rules only when asked. Only recon.rule/1 rules can be evaluated.
	stored, err := tx.Rules()
	if err != nil {
		return 0, err
	}
	var used []store.StoredRule
	var planned []*rules.Rule
	var notSelected []findings.NotRun
	byID := map[string]*rules.Rule{}
	for _, sr := range stored {
		if sr.Schema != rules.Schema || !(sr.Status == store.StatusActive || *includeTesting && sr.Status == store.StatusTesting) {
			continue
		}
		if cmpType != nil && !slices.Contains(cmpType.Rules, sr.RuleID) {
			continue // not a rule of the comparison the CA picked
		}
		name := sr.FilePath + " (stored revision " + strconv.FormatInt(sr.Revision, 10) + ")"
		r, err := rules.Load(name, sr.Body, v, checks.All())
		if err != nil {
			return 0, inputErrf("rule %s does not load:\n%v", sr.RuleID, err)
		}
		if r.ID != sr.RuleID {
			return 0, inputErrf("rule %s is stored with the ID %s", r.ID, sr.RuleID)
		}
		if !r.AppliesTo(ty) {
			continue
		}
		if selected != nil && !slices.Contains(selected, r.ID) {
			// It would have run, but the CA left it out: say so, never count it as checked.
			notSelected = append(notSelected, findings.NotRun{RuleID: r.ID, Title: r.Title})
			continue
		}
		used = append(used, sr)
		planned = append(planned, r)
		byID[r.ID] = r
	}
	// The report says it includes testing rules only when one ran: with the switch on, a type
	// whose rules are all approved, or that has none (the expense ledgers), includes none.
	testingRan := slices.ContainsFunc(used, func(sr store.StoredRule) bool { return sr.Status == store.StatusTesting })

	// 4. Snapshot.
	runID, err := tx.CreateRun(store.RunInfo{
		EngagementID: eng, TaxYear: ty.String(), EngineVersion: EngineVersion,
		VocabVersion: v.Version(), VocabHash: v.ContentHash(), IncludeTesting: *includeTesting,
		Comparison: *comparisonID, Documents: docs, Rules: used,
	})
	if err != nil {
		return 0, err
	}

	// 6. Evaluate on a worker pool; results come back sorted by rule and subject.
	evs, err := engine.Evaluate(planned, facts, records, ty, checks.All(), runtime.GOMAXPROCS(0))
	var ce *engine.ConflictError
	if errors.As(err, &ce) {
		return 0, inputErr(fmt.Errorf("%s: %w", ingest.ConflictingValues, err))
	}
	if err != nil {
		return 0, err
	}
	for i := range evs {
		evs[i].RunID = runID
	}
	if err := tx.PutEvaluations(runID, evs); err != nil {
		return 0, err
	}
	// 7. Render, keep the report with the run, then commit, then print: a report is printed
	// only for a stored run. A client's report shows every document of its current view,
	// including those uploaded in earlier runs.
	rep := &findings.Report{TaxYear: ty.String(), IncludesTesting: testingRan, Evaluations: evs, Rules: byID, Vocab: v,
		Comparison: cmpType, NotSelected: notSelected, EngagementID: eng, Worklist: worklist, HasWorklist: hasWorklist}
	slices.SortFunc(rep.NotSelected, func(a, b findings.NotRun) int { return strings.Compare(a.RuleID, b.RuleID) })
	if len(totalsDocs) > 0 {
		if rep.Totals, err = findings.Totals(v, records, totalsDocs); err != nil {
			return 0, err
		}
	}
	for _, d := range docs {
		if !slices.Contains(rep.Present, d.Slot.DocumentType) {
			rep.Present = append(rep.Present, d.Slot.DocumentType)
		}
	}
	if *clientID > 0 {
		for _, d := range docs {
			vr, err := findings.ParseValidationReport(d.ReportJSON)
			if err != nil {
				return 0, err
			}
			rep.Files = append(rep.Files, vr)
		}
		for _, d := range priorDocs { // last year's ledger, read for the Worklist
			if d.Slot.DocumentType != "ledger" {
				continue
			}
			vr, err := findings.ParseValidationReport(d.ReportJSON)
			if err != nil {
				return 0, err
			}
			rep.Files = append(rep.Files, vr)
		}
	} else {
		for _, rf := range read {
			rep.Files = append(rep.Files, rf.result.Report)
		}
	}
	text, err := rep.Text()
	if err != nil {
		return 0, err
	}
	js, err := rep.JSON()
	if err != nil {
		return 0, err
	}
	if err := tx.SetRunReport(runID, js); err != nil {
		return 0, err
	}
	if hasWorklist {
		wl, err := findings.WorklistJSON(worklist)
		if err != nil {
			return 0, err
		}
		if err := tx.SetRunWorklist(runID, wl); err != nil {
			return 0, err
		}
	}
	if err := tx.FinishRun(runID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if *jsonOut != "" {
		if err := os.WriteFile(*jsonOut, js, 0o644); err != nil {
			return 0, err
		}
	}
	if _, err := io.WriteString(stdout, text); err != nil {
		return 0, err
	}
	if _, fail, _ := rep.Counts(); fail > 0 {
		return exitFindings, nil
	}
	return exitOK, nil
}

// selectedRules reads --rules (SPEC-004 part 9): nil when it was not given (every rule of
// the comparison runs), otherwise the chosen IDs, each of which must be a rule of the
// comparison. It is refused without --comparison, and when it names no rule.
func selectedRules(c *comparison.Comparison, given bool, list string) ([]string, error) {
	if !given {
		return nil, nil
	}
	if c == nil {
		return nil, inputErrf("RULES_NEED_COMPARISON: --rules chooses among the rules of a type of check; give --comparison too")
	}
	ids := []string{}
	for _, id := range strings.Split(list, ",") {
		id = strings.TrimSpace(id)
		switch {
		case id == "" || slices.Contains(ids, id):
			continue
		case !slices.Contains(c.Rules, id):
			return nil, inputErrf("RULE_NOT_IN_COMPARISON: %s is not a rule of %q (its rules: %s)", id, c.Name, strings.Join(c.Rules, ", "))
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, inputErrf("NO_RULES_SELECTED: choose at least one rule of %q", c.Name)
	}
	return ids, nil
}

// scopeFromClient gives a GST document that does not say whose GSTIN it is (a taught layout,
// or an export without the client's GSTIN) the client's GSTIN, when the client has exactly
// one: its rows are then matched with the GST portal's, which are kept per GSTIN (SPEC-008
// part 2). A client with several GSTINs is not guessed for; its rows stay without one and
// the checks say "cannot check". A GST document is one whose rows have a GSTIN column; a
// ledger's rows have none and are never given one. The report says the GSTIN was taken from
// the client's details, naming its last four characters only.
func scopeFromClient(tx *store.Tx, c store.Client, v *vocab.Vocab, read []readFile) error {
	d, found, err := tx.ClientDetails(c.ID)
	if err != nil || !found || len(d.GSTINs) != 1 {
		return err
	}
	gstin := d.GSTINs[0].GSTIN
	for _, rf := range read {
		res := rf.result
		rt, ok := profile.RecordTypeFor(v, rf.profile.DocumentType)
		if rf.profile.ReadsScope() || res.Context.Scope != "" || !ok ||
			!slices.ContainsFunc(rt.Fields, func(f string) bool { return strings.HasSuffix(f, "_gstin") }) {
			continue
		}
		res.Context.Scope = gstin
		for i := range res.Facts {
			res.Facts[i].Scope = gstin
		}
		for i := range res.Records {
			res.Records[i].Scope = gstin
		}
		res.Report.Warnings = append(res.Report.Warnings, ingest.Issue{Code: ingest.GSTINFromClient,
			Locator: domain.Locator{FileName: rf.name},
			Msg:     fmt.Sprintf("the file does not say whose GSTIN it is; the client's only GSTIN (ending %s) is used", gstin[max(0, len(gstin)-4):])})
		res.Report.Sort()
	}
	return nil
}

// checkClientPAN refuses a file whose PAN is not the client's (AC-23): the whole upload is
// an input error naming the file, so another client's document can never become a draft
// of this client's. A file without a PAN is not checked.
func checkClientPAN(tx *store.Tx, c store.Client, res *profile.Result) error {
	if res.Context.PAN == "" {
		return nil
	}
	h, err := tx.PANHash(res.Context.PAN)
	if err != nil {
		return err
	}
	if h != c.PANHash {
		return inputErrf("%s: %s: the PAN in the file (ending %s) is not the PAN of %s (ending %s)",
			ingest.ConflictingValues, res.Report.File, res.Report.PANLast4, c.Name, c.PANLast4)
	}
	res.Report.ClientMatch = ingest.ClientMatched
	return nil
}

// inputFiles returns the file, or every regular file directly in the folder, sorted by name.
func inputFiles(input string) ([]string, error) {
	st, err := os.Stat(input)
	if err != nil {
		return nil, inputErrf("--input: %v", err)
	}
	if !st.IsDir() {
		return []string{input}, nil
	}
	entries, err := os.ReadDir(input)
	if err != nil {
		return nil, inputErrf("--input: %v", err)
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			out = append(out, filepath.Join(input, e.Name()))
		}
	}
	slices.Sort(out)
	if len(out) == 0 {
		return nil, inputErrf("--input: the folder %s has no files", input)
	}
	return out, nil
}

// readOne detects and extracts one file: one document, or one per profile of a bundle (a
// file that holds several documents). An unknown file type, a damaged file and a file that
// matches no profile, or more than one outside a single bundle, are input errors carrying
// the stable code and the file name. A file read with validation errors is kept: its report says why, and its
// figures are not used.
func readOne(p string, reg *ingest.Registry, profiles *profile.Set, ty period.TaxYear) ([]readFile, error) {
	name := filepath.Base(p)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, inputErrf("%s: %v", name, err)
	}
	sum := sha256.Sum256(data)
	ad, err := reg.ForFile(data[:min(len(data), sniffBytes)], name)
	if err != nil {
		return nil, inputErr(err)
	}
	doc, err := ad.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		var ie *ingest.Error
		if errors.As(err, &ie) {
			ie.File = name
			return nil, inputErr(ie)
		}
		return nil, err
	}
	// One profile, or every profile of a bundle: a file that holds several documents gives
	// one document per profile (SPEC-005).
	prs, err := profile.DetectAll(profiles, doc, name)
	if err != nil {
		var ie *ingest.Error
		if errors.As(err, &ie) {
			return nil, inputErr(ie)
		}
		return nil, err
	}
	results, err := profile.ExtractAll(prs, doc, profile.Source{FileName: name})
	if err != nil {
		return nil, err
	}
	out := make([]readFile, len(prs))
	for i, res := range results {
		got := res.Context.TaxYear
		prior := prs[i].DocumentType == "ledger" && got == ty.Prior() && !got.IsZero()
		if !got.IsZero() && got != ty && !prior {
			res.Report.Errors = append(res.Report.Errors, ingest.Issue{Code: ingest.ConflictingValues,
				Msg: fmt.Sprintf("the file is for tax year %s; this run is for %s", got, ty)})
			res.Facts, res.Records = nil, nil
			res.Report.FactCount, res.Report.RecordCount = 0, 0
			res.Report.Sort()
		}
		out[i] = readFile{name: name, sha256: hex.EncodeToString(sum[:]), profile: prs[i], result: res, priorYear: prior}
	}
	return out, nil
}

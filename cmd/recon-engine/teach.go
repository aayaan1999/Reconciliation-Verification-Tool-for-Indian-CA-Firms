package main

import (
	"bytes"
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

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	csvadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/csv"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Teaching the tool a new JSON layout (SPEC-004 part 8.4). inspect lists what a file holds;
// profile-create turns the user's choices into a format profile in the office profiles
// folder, after checking that it reads the file. The shipped profiles are never changed.

// loadProfiles loads the shipped profiles and, when office is given and exists, the
// profiles taught in the office, as one set: a file must match exactly one of them all.
func loadProfiles(shipped, office string, v *vocab.Vocab) (*profile.Set, error) {
	set, err := profile.LoadDir(shipped, v)
	if err != nil {
		return nil, inputErrf("profiles %s:\n%v", shipped, err)
	}
	if office == "" {
		return set, nil
	}
	if _, err := os.Stat(office); errors.Is(err, fs.ErrNotExist) {
		return set, nil // nothing taught yet
	}
	off, err := profile.LoadDir(office, v)
	if err != nil {
		return nil, inputErrf("office profiles %s:\n%v", office, err)
	}
	all, err := profile.NewSet(append(set.Profiles(), off.Profiles()...)...)
	if err != nil {
		return nil, inputErrf("profiles: %v", err)
	}
	return all, nil
}

// previewRows is how many rows the mapping screen shows as the engine reads them (SPEC-016).
const previewRows = 20

// openJSON reads one JSON file as a document.
func openJSON(path string) (ingest.Doc, []byte, error) {
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, inputErrf("%s: %v", name, err)
	}
	reg := ingest.NewRegistry()
	if err := reg.Register(jsonadapter.New(0)); err != nil {
		return nil, nil, err
	}
	if err := reg.Register(csvadapter.New(0)); err != nil {
		return nil, nil, err
	}
	if err := reg.Register(xlsxadapter.New(0)); err != nil {
		return nil, nil, err
	}
	ad, err := reg.ForFile(data[:min(len(data), sniffBytes)], name)
	if err != nil {
		return nil, nil, inputErr(err)
	}
	doc, err := ad.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		var ie *ingest.Error
		if errors.As(err, &ie) {
			ie.File = name
			return nil, nil, inputErr(ie)
		}
		return nil, nil, err
	}
	return doc, data, nil
}

// teachableType is a document type whose rows can be taught, with its columns.
type teachableType struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	RecordType  string         `json:"record_type"`
	Columns     []teachableCol `json:"columns"`
}

type teachableCol struct {
	Name   string `json:"name"`
	Amount bool   `json:"amount"`
	Key    bool   `json:"key"` // rows are matched on it: it must be chosen
}

func teachableTypes(v *vocab.Vocab) []teachableType {
	out := []teachableType{}
	for _, dt := range v.DocumentTypes() {
		rt, ok := profile.RecordTypeFor(v, dt.Name)
		if !ok {
			continue
		}
		t := teachableType{Name: dt.Name, Description: dt.Description, RecordType: rt.Name}
		for _, f := range rt.Fields {
			t.Columns = append(t.Columns, teachableCol{Name: f, Key: slices.Contains(rt.MatchKey, f)})
		}
		for _, a := range rt.Amounts {
			t.Columns = append(t.Columns, teachableCol{Name: a, Amount: true})
		}
		out = append(out, t)
	}
	return out
}

// inspect lists the fields and lists of a JSON file, says which known format it is (if
// any) and the tax year it is for, and lists the document types a layout can be taught
// for. It only reads.
//
//	recon-engine inspect --input <file> [--profiles <folder>] [--office-profiles <folder>] [--vocab <folder>] --json <file>
func inspect(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	input := fs.String("input", "", "the JSON file")
	profilesDir := fs.String("profiles", "content/profiles", "format profiles folder")
	officeDir := fs.String("office-profiles", "", "profiles taught in the office")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the inspection as JSON to this file")
	if err := parseFlags(fs, args, "input", "json"); err != nil {
		return err
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	set, err := loadProfiles(*profilesDir, *officeDir, v)
	if err != nil {
		return err
	}
	doc, _, err := openJSON(*input)
	if err != nil {
		return err
	}
	in, err := profile.Inspect(doc)
	if err != nil {
		return inputErrf("%v", err)
	}
	recognised, taxYear := "", ""
	if ps, err := profile.DetectAll(set, doc, filepath.Base(*input)); err == nil {
		// A file of a bundle is recognised as each of its documents' profiles.
		keys := make([]string, len(ps))
		for i, p := range ps {
			keys[i] = p.Key()
		}
		recognised = strings.Join(keys, ", ")
		// The tax year the file is for, so the screen can suggest it before a run.
		if res, err := profile.Extract(ps[0], doc, profile.Source{FileName: filepath.Base(*input)}); err == nil && !res.Context.TaxYear.IsZero() {
			taxYear = res.Context.TaxYear.String()
		}
	}
	out := map[string]any{
		"file": filepath.Base(*input), "recognised_as": recognised, "tax_year": taxYear,
		"fields": in.Fields, "lists": in.Lists, "truncated": in.Truncated,
		"document_types": teachableTypes(v),
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s: %d fields, %d lists\n", filepath.Base(*input), len(in.Fields), len(in.Lists))
	return err
}

// profileCreate teaches a layout: the mapping (profile.Mapping, as JSON on standard input)
// becomes a format profile, which must recognise the sample as this layout and nothing
// else and read it without errors. It is saved, with the sample beside it, under
// --office-profiles; with --check-only nothing is saved. Either way --json gets a preview
// of the rows read. A refused mapping is INVALID_MAPPING, with each problem in --json.
//
//	recon-engine profile-create --office-profiles <folder> --sample <file> [--check-only] --json <file> < mapping.json
func profileCreate(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("profile-create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	sample := fs.String("sample", "", "a file in the layout")
	profilesDir := fs.String("profiles", "content/profiles", "format profiles folder")
	officeDir := fs.String("office-profiles", "", "where taught profiles are kept")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	checkOnly := fs.Bool("check-only", false, "check the mapping and preview the rows; save nothing")
	dbFile := fs.String("db", "", "also store the taught layout as an office format in this database (SPEC-016)")
	by := fs.String("by", "", "who taught the layout, written to the audit log; needed with --db")
	formatID := fs.String("format", "", "edit this office format: the mapping is saved as a draft version in --db and published later with format-publish (SPEC-016)")
	keepIt := fs.Bool("keep-sample", false, "keep the sample file in --db, so publishing a change can read it again (SPEC-016); the file may hold client figures")
	rulesDir := fs.String("rules", "content/rules", "engine rules folder, for the dry run of which rules can run on the layout")
	jsonOut := fs.String("json", "", "write the outcome as JSON to this file")
	if err := parseFlags(fs, args, "sample", "office-profiles", "json"); err != nil {
		return err
	}
	var m profile.Mapping
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return inputErrf("BAD_REQUEST: the mapping is not valid JSON: %v", err)
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	refuse := func(problems []profile.MappingProblem) error {
		if err := writeJSONFile(*jsonOut, map[string]any{"problems": problems}); err != nil {
			return err
		}
		parts := make([]string, len(problems))
		for i, p := range problems {
			parts[i] = p.String()
		}
		return inputErrf("INVALID_MAPPING: %s", strings.Join(parts, "; "))
	}
	doc, raw, err := openJSON(*sample)
	if err != nil {
		return err
	}
	// The layout is of the sample's file type: a workbook is taught as an Excel profile.
	if m.FileType = doc.FileType(); m.FileType != xlsxadapter.FileType && m.FileType != jsonadapter.FileType && m.FileType != csvadapter.FileType {
		return refuse([]profile.MappingProblem{{Field: "file", Code: "UNSUPPORTED_FILE_TYPE", Message: "only JSON, Excel (.xlsx) and CSV layouts can be taught"}})
	}
	// An edit of an office format is the layout's next version. It has to name the same layout, and
	// it supersedes the one before it, so a file that fits both is read by the newer (SPEC-016).
	if *formatID != "" {
		if *dbFile == "" {
			return inputErrf("--format needs --db")
		}
		if m.ID() != *formatID {
			return refuse([]profile.MappingProblem{{Field: "name", Code: "FORMAT_MISMATCH",
				Message: fmt.Sprintf("the layout is named %q for %s, which is not the format being edited; keep its name and document", m.Name, m.DocumentType)}})
		}
		next := 0
		if err := withTx(*dbFile, func(tx *store.Tx) error {
			st, ok, err := tx.FormatStateOf(*formatID)
			if err != nil {
				return err
			}
			if !ok {
				return inputErrf("FORMAT_NOT_FOUND: there is no format %s", *formatID)
			}
			cur, _, err := tx.FormatVersionByID(st.ActiveVersionID)
			if err != nil {
				return err
			}
			if cur.Source != store.OriginOffice {
				return inputErrf("FORMAT_READ_ONLY: %s is a standard format; teach an office format instead", *formatID)
			}
			versions, err := tx.FormatVersions(*formatID)
			if err != nil {
				return err
			}
			for _, fv := range versions {
				n, _ := strconv.Atoi(fv.ProfileVersion)
				next = max(next, n)
			}
			next++
			return nil
		}); err != nil {
			return err
		}
		m.Version = next
	}
	data, problems := m.YAML(v)
	if len(problems) > 0 {
		return refuse(problems)
	}
	p, err := profile.Load(m.Dir()+"/"+profile.FileName, data, v)
	if err != nil {
		var probs []profile.MappingProblem
		for _, line := range strings.Split(err.Error(), "\n") {
			probs = append(probs, profile.MappingProblem{Field: "mapping", Code: "BAD_PATH", Message: line})
		}
		return refuse(probs)
	}
	set, err := loadProfiles(*profilesDir, *officeDir, v)
	if err != nil {
		return err
	}
	if _, found := set.Lookup(p.ID, p.Version); found {
		return refuse([]profile.MappingProblem{{Field: "name", Code: "PROFILE_EXISTS",
			Message: fmt.Sprintf("a layout named %q is already taught for %s; choose another name", m.Name, m.DocumentType)}})
	}
	all, err := profile.NewSet(append(set.Profiles(), p)...)
	if err != nil {
		return err
	}
	name := filepath.Base(*sample)
	if fits, err := p.Matches(doc); err != nil || !fits {
		return refuse([]profile.MappingProblem{{Field: "rows", Code: "MAPPING_DOES_NOT_FIT",
			Message: "the chosen list or fields are not all in the file, or an identifying value differs; choose them again from the list of fields"}})
	}
	got, err := profile.Detect(all, doc, name)
	switch {
	case err != nil:
		return refuse([]profile.MappingProblem{{Field: "identify", Code: "PROFILE_AMBIGUOUS",
			Message: strings.TrimPrefix(err.Error(), "PROFILE_AMBIGUOUS: "+name+": ") + "; add a field and value that only this layout has"}})
	case got != p:
		return refuse([]profile.MappingProblem{{Field: "identify", Code: "PROFILE_AMBIGUOUS",
			Message: "the file is already read as " + got.Key()}})
	}
	res, err := profile.Extract(p, doc, profile.Source{FileName: name})
	if err != nil {
		return err
	}
	if len(res.Report.Errors) > 0 {
		var probs []profile.MappingProblem
		for i, e := range res.Report.Errors {
			if i == 10 {
				probs = append(probs, profile.MappingProblem{Field: "mapping", Code: "MAPPING_DOES_NOT_READ",
					Message: fmt.Sprintf("and %d more", len(res.Report.Errors)-10)})
				break
			}
			probs = append(probs, profile.MappingProblem{Field: "mapping", Code: string(e.Code), Message: locText(e.Locator) + e.Msg})
		}
		return refuse(probs)
	}
	if len(res.Records) == 0 {
		return refuse([]profile.MappingProblem{{Field: "rows", Code: "MAPPING_READS_NOTHING",
			Message: "no row was read from the file; check the list of rows and the row filter"}})
	}

	preview := make([]map[string]any, 0, previewRows)
	for _, r := range res.Records[:min(previewRows, len(res.Records))] {
		amounts := map[string]string{}
		for k, a := range r.Amounts {
			amounts[k] = domain.FormatValue(domain.MoneyValue(a))
		}
		preview = append(preview, map[string]any{"match_key": r.MatchKey, "locator": r.Locator.Path, "fields": r.Fields, "amounts": amounts})
	}
	ty := ""
	if !res.Context.TaxYear.IsZero() {
		ty = res.Context.TaxYear.String()
	}
	out := map[string]any{
		"profile_id": p.ID, "profile_version": p.Version, "document_type": p.DocumentType, "saved": !*checkOnly,
		"rows": len(res.Records), "tax_year": ty, "period": res.Context.Period, "pan_last4": res.Report.PANLast4,
		"warnings": len(res.Report.Warnings), "preview": preview, "profile_yaml": string(data),
	}
	mj, err := json.Marshal(m)
	if err != nil {
		return err
	}
	mappingJSON := string(mj)
	if dr, err := dryRunOf(p, v, *rulesDir); err != nil {
		return err
	} else if dr != nil {
		out["dry_run"] = dr
	}
	var actor string
	if !*checkOnly && *dbFile != "" { // a refusal comes before any file is written
		var err error
		if actor, err = trimmedName(*by); err != nil {
			return err
		}
	}
	if !*checkOnly && *formatID != "" {
		// An edit is saved as a draft only: no file is written and no run uses it until it is
		// published with format-publish.
		var revision int
		if err := withTx(*dbFile, func(tx *store.Tx) error {
			if _, err := tx.AddFormatDraft(store.FormatVersion{
				FormatID: p.ID, Name: m.Name, Source: store.OriginOffice, FileType: p.FileType, Document: p.DocumentType,
				BodyYAML: string(data), SHA256: sumHex(data), Note: "Edited from " + name, CreatedBy: actor,
				MappingJSON: mappingJSON, ProfileVersion: p.Version,
			}); err != nil {
				return err
			}
			vs, err := tx.FormatVersions(p.ID)
			if err != nil {
				return err
			}
			revision = vs[len(vs)-1].Revision
			if err := keepSample(tx, *keepIt, p.ID, name, raw, actor); err != nil {
				return err
			}
			return tx.Audit(nowUTC(), actor, "format.draft", "format", p.ID+" v"+strconv.Itoa(revision), map[string]string{"file": name})
		}); err != nil {
			return err
		}
		out["draft"] = true
		out["format_revision"] = revision
	} else if !*checkOnly {
		if err := saveProfile(*officeDir, m.Dir(), name, data, raw, profile.ExpectedFile(p, res)); err != nil {
			return err
		}
		// The layout is also kept as an office format, with every version, so the Formats
		// screen lists it (SPEC-016). The files above stay what the engine reads.
		if *dbFile != "" {
			if err := withTx(*dbFile, func(tx *store.Tx) error {
				if _, err := tx.AddFormatVersion(store.FormatVersion{
					FormatID: p.ID, Name: m.Name, Source: store.OriginOffice, FileType: p.FileType, Document: p.DocumentType,
					BodyYAML: string(data), SHA256: sumHex(data), Note: "Taught from " + name, Status: store.FormatActive, CreatedBy: actor,
					MappingJSON: mappingJSON, ProfileVersion: p.Version, PublishedAt: nowUTC(), PublishedBy: actor,
				}); err != nil {
					return err
				}
				if err := keepSample(tx, *keepIt, p.ID, name, raw, actor); err != nil {
					return err
				}
				return tx.Audit(nowUTC(), actor, "format.create", "format", p.ID, map[string]string{"name": m.Name, "file": name})
			}); err != nil {
				return err
			}
		}
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	verb := "checked"
	if !*checkOnly {
		verb = "saved"
	}
	_, err = fmt.Fprintf(stdout, "profile %s %s: %d rows read from %s\n", p.Key(), verb, len(res.Records), name)
	return err
}

func locText(l domain.Locator) string {
	if l.Path == "" {
		return ""
	}
	return l.Path + ": "
}

// saveProfile writes the profile, its sample and its expected result into a new folder,
// built beside its final place and renamed into it, so a half-written profile is never
// loaded. The sample is a client's file and the expected result holds its figures, so the
// folders and files are readable by the server's own account only (until the vault, §B12.2).
func saveProfile(office, rel, sampleName string, profileYAML, sample, expected []byte) error {
	final := filepath.Join(office, filepath.FromSlash(rel))
	if _, err := os.Stat(final); err == nil {
		return inputErrf("PROFILE_EXISTS: %s already exists", filepath.ToSlash(final))
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(final), ".new-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	base := strings.TrimSuffix(sampleName, filepath.Ext(sampleName))
	for _, f := range []struct {
		path string
		data []byte
	}{
		{profile.FileName, profileYAML},
		{filepath.Join("samples", sampleName), sample},
		{filepath.Join("expected", base+".facts.yaml"), expected},
	} {
		p := filepath.Join(tmp, f.path)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.data, 0o600); err != nil {
			return err
		}
	}
	return os.Rename(tmp, final)
}

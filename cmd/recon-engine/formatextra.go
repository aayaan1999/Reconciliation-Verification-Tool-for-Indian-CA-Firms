package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// Unrecognised files, kept samples and the export and import of formats (SPEC-016).

// ---------- unrecognised files ----------

func fileKindOf(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		return "json"
	case ".xlsx":
		return "xlsx"
	case ".csv":
		return "csv"
	}
	return ""
}

// noteUnrecognised records a file no profile matched, so the Partner can teach a format for it. It
// keeps the name, the hash and the size only. It is best effort: the run's own refusal is what the
// user sees, so a failure here is not reported.
func noteUnrecognised(dbFile, path string, clientID int64, taxYear string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = withTx(dbFile, func(tx *store.Tx) error {
		return tx.NoteUnrecognised(store.UnrecognisedFile{
			FileName: filepath.Base(path), FileSHA256: sumHex(data), SizeBytes: int64(len(data)), FileType: fileKindOf(path),
			ClientID: clientID, TaxYear: taxYear, NotedBy: "recon-engine", NotedAt: nowUTC(),
		})
	})
}

type unrecognisedOut struct {
	ID       int64  `json:"id"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size_bytes"`
	TaxYear  string `json:"tax_year"`
	NotedAt  string `json:"noted_at"`
}

type resolveChange struct {
	By   string `json:"by"`
	Note string `json:"note"`
}

// unrecognisedResolve closes a note: a format was taught for the file, or the Partner dismisses it.
//
//	recon-engine unrecognised-resolve --db <file> --id <n> --status taught|dismissed --json <file>   (by and note on standard input)
func unrecognisedResolve(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("unrecognised-resolve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	id := fs.Int64("id", 0, "the note's ID")
	status := fs.String("status", "", "taught or dismissed")
	jsonOut := fs.String("json", "", "write the outcome as JSON to this file")
	if err := parseFlags(fs, args, "db", "status", "json"); err != nil {
		return err
	}
	if *status != "taught" && *status != "dismissed" {
		return inputErrf("BAD_STATUS: close a file as taught or dismissed")
	}
	var in resolveChange
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	note := strings.TrimSpace(in.Note)
	if *status == "dismissed" && note == "" {
		return inputErrf("REASON_REQUIRED: say why the file is dismissed")
	}
	err = withTx(*dbFile, func(tx *store.Tx) error {
		at := nowUTC()
		ok, err := tx.ResolveUnrecognised(*id, *status, by, at, note)
		if err != nil {
			return err
		}
		if !ok {
			return inputErrf("NOT_FOUND: there is no open file %d", *id)
		}
		return tx.Audit(at, by, "unrecognised."+*status, "unrecognised_file", strconv.FormatInt(*id, 10), map[string]string{"note": note})
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, map[string]any{"id": *id, "status": *status}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "unrecognised-resolve: file %d is %s\n", *id, *status)
	return nil
}

// ---------- kept samples ----------

type sampleOut struct {
	ID      int64  `json:"id"`
	Name    string `json:"file_name"`
	Size    int64  `json:"size_bytes"`
	KeptBy  string `json:"kept_by"`
	KeptAt  string `json:"kept_at"`
	HasFile bool   `json:"has_content"`
}

// keepSample stores a sample for a format when the admin chose to keep it. The bytes may hold
// client figures, so this is only done on request, and the screen warns that a sample must be
// invented or masked.
func keepSample(tx *store.Tx, keep bool, formatID, name string, data []byte, by string) error {
	if !keep {
		return nil
	}
	_, err := tx.KeepSample(store.FormatSample{FormatID: formatID, FileSHA256: sumHex(data), FileName: filepath.Base(name), SizeBytes: int64(len(data)), Content: data, KeptBy: by})
	return err
}

// formatSampleDiscard deletes a kept sample. The audit entry stays.
//
//	recon-engine format-sample-discard --db <file> --id <n> --json <file>   (by on standard input)
func formatSampleDiscard(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("format-sample-discard", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	id := fs.Int64("id", 0, "the sample's ID")
	jsonOut := fs.String("json", "", "write the outcome as JSON to this file")
	if err := parseFlags(fs, args, "db", "id", "json"); err != nil {
		return err
	}
	var in formatChange
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	err = withTx(*dbFile, func(tx *store.Tx) error {
		s, ok, err := tx.FormatSampleByID(*id)
		if err != nil {
			return err
		}
		if !ok {
			return inputErrf("NOT_FOUND: there is no kept sample %d", *id)
		}
		if err := tx.RemoveSample(*id); err != nil {
			return err
		}
		return tx.Audit(nowUTC(), by, "format.sample_discard", "format", s.FormatID, map[string]string{"file": s.FileName})
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, map[string]any{"id": *id}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "format-sample-discard: sample %d discarded\n", *id)
	return nil
}

type sampleCheck struct {
	FileName string `json:"file_name"`
	Changed  bool   `json:"changed"`
	Detail   string `json:"detail"`
}

// readSampleWith reads a kept sample with a profile and returns what it gave, as text that is the
// same whenever the reading is the same.
func readSampleWith(p *profile.Profile, name string, data []byte) string {
	dir, err := os.MkdirTemp("", "recon-sample-*")
	if err != nil {
		return "error: " + err.Error()
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, filepath.Base(name))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "error: " + err.Error()
	}
	doc, _, err := openJSON(path)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	res, err := profile.Extract(p, doc, profile.Source{FileName: filepath.Base(name)})
	if err != nil {
		return "unreadable: " + err.Error()
	}
	b, err := json.Marshal(map[string]any{"records": res.Records, "facts": res.Facts, "errors": len(res.Report.Errors)})
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

// compareSamples reads every kept sample of a format with the version in use and with the draft,
// and says which results differ. It lets a publish show what it would change (SPEC-016).
func compareSamples(tx *store.Tx, formatID string, inUse, draft *profile.Profile) ([]sampleCheck, error) {
	samples, err := tx.FormatSamples(formatID)
	if err != nil {
		return nil, err
	}
	out := []sampleCheck{}
	for _, s := range samples {
		if s.Content == nil {
			continue
		}
		before, after := readSampleWith(inUse, s.FileName, s.Content), readSampleWith(draft, s.FileName, s.Content)
		c := sampleCheck{FileName: s.FileName, Changed: before != after}
		if c.Changed {
			c.Detail = fmt.Sprintf("the result differs (%d bytes before, %d after)", len(before), len(after))
		}
		out = append(out, c)
	}
	return out, nil
}

// ---------- export and import ----------

const bundleSchema = "recon.format-bundle/1"

type bundleFormat struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	FileType       string          `json:"file_type"`
	Document       string          `json:"document"`
	ProfileVersion string          `json:"profile_version"`
	ProfileYAML    string          `json:"profile_yaml"`
	Mapping        json.RawMessage `json:"mapping,omitempty"`
}

type bundleParameter struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Document    string   `json:"document"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
}

// A bundle is one format with the office parameters of its document, as one file for another
// office machine. It holds no client data.
type bundle struct {
	Schema       string            `json:"schema"`
	VocabVersion int               `json:"vocab_version"`
	Format       bundleFormat      `json:"format"`
	Parameters   []bundleParameter `json:"parameters"`
}

// formatExport writes a format's version in use, and the office parameters of its document, as a
// bundle. A standard format is not exported: every machine already has it.
//
//	recon-engine format-export --db <file> --format <ID> --json <file>
func formatExport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("format-export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	formatID := fs.String("format", "", "the format's ID")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the bundle to this file")
	if err := parseFlags(fs, args, "db", "format", "json"); err != nil {
		return err
	}
	v, err := loadVocab(*dbFile, *vocabDir)
	if err != nil {
		return vocabularyErr(err)
	}
	var out bundle
	err = withTx(*dbFile, func(tx *store.Tx) error {
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
			return inputErrf("FORMAT_READ_ONLY: %s is a standard format; every machine has it", *formatID)
		}
		out = bundle{Schema: bundleSchema, VocabVersion: v.Version(), Parameters: []bundleParameter{}, Format: bundleFormat{
			ID: cur.FormatID, Name: cur.Name, FileType: cur.FileType, Document: cur.Document, ProfileVersion: cur.ProfileVersion, ProfileYAML: cur.BodyYAML,
		}}
		if cur.MappingJSON != "" {
			out.Format.Mapping = json.RawMessage(cur.MappingJSON)
		}
		params, err := tx.Parameters()
		if err != nil {
			return err
		}
		for _, p := range params {
			if p.Version.Source != store.OriginOffice || p.Version.Document != cur.Document {
				continue
			}
			var d paramDoc
			_ = json.Unmarshal([]byte(p.Version.BodyYAML), &d)
			out.Parameters = append(out.Parameters, bundleParameter{Key: p.State.ParameterID, Name: p.Version.Name, Kind: p.Version.ParamType,
				Document: p.Version.Document, Description: d.Description, Aliases: append([]string{}, d.Aliases...)})
		}
		return tx.Audit(nowUTC(), "recon-engine", "format.export", "format", *formatID, map[string]string{"version": cur.Version})
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "format-export: %s exported with %d parameters\n", *formatID, len(out.Parameters))
	return nil
}

type importPlan struct {
	Format             string   `json:"format"`
	FormatName         string   `json:"format_name"`
	ParametersAdded    []string `json:"parameters_added"`
	ParametersSame     []string `json:"parameters_unchanged"`
	Applied            bool     `json:"applied"`
	ProfileDestination string   `json:"profile_folder"`
}

// formatImport reads a bundle from standard input, checks it with the same strict loaders, says
// what it would add, and applies it unless --check-only. An imported format is stored as testing
// and published only on its own machine's Partner's say. Nothing already stored is changed: a
// bundle that clashes with a different format or parameter is refused whole.
//
//	recon-engine format-import --db <file> --office-profiles <folder> [--check-only] --json <file>   (the bundle on standard input; by in --by)
func formatImport(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("format-import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	officeDir := fs.String("office-profiles", "", "where taught profiles are kept")
	profilesDir := fs.String("profiles", "content/profiles", "format profiles folder")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	by := fs.String("by", "", "who imports, written to the audit log")
	checkOnly := fs.Bool("check-only", false, "say what the bundle would add; change nothing")
	jsonOut := fs.String("json", "", "write the plan as JSON to this file")
	if err := parseFlags(fs, args, "db", "office-profiles", "json"); err != nil {
		return err
	}
	actor, err := trimmedName(*by)
	if err != nil {
		return err
	}
	var b bundle
	dec := json.NewDecoder(io.LimitReader(stdin, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return inputErrf("BAD_BUNDLE: the bundle is not a valid format bundle: %v", err)
	}
	v, err := loadVocab(*dbFile, *vocabDir)
	if err != nil {
		return vocabularyErr(err)
	}
	f := b.Format
	switch {
	case b.Schema != bundleSchema:
		return inputErrf("BAD_BUNDLE: the schema is %q, not %q", b.Schema, bundleSchema)
	case b.VocabVersion != v.Version():
		return inputErrf("BAD_BUNDLE: it was made for vocabulary version %d; this machine has %d", b.VocabVersion, v.Version())
	case !strings.HasPrefix(f.ID, profile.OfficePrefix) || f.ProfileYAML == "":
		return inputErrf("BAD_BUNDLE: the format must be an office format with its profile")
	}
	p, err := profile.Load(f.ID+"/"+profile.FileName, []byte(f.ProfileYAML), v)
	if err != nil {
		return inputErrf("BAD_BUNDLE: the profile does not load: %v", err)
	}
	if p.ID != f.ID || p.Version != f.ProfileVersion || p.DocumentType != f.Document {
		return inputErrf("BAD_BUNDLE: the profile does not match the format it is bundled as")
	}
	parts := strings.Split(strings.TrimPrefix(f.ID, profile.OfficePrefix), ".")
	if len(parts) != 2 || parts[1] != p.DocumentType {
		return inputErrf("BAD_BUNDLE: the format ID %q is not an office layout ID", f.ID)
	}
	folder := filepath.Join(*officeDir, parts[0], p.DocumentType, "v"+p.Version)
	for _, bp := range b.Parameters {
		if !paramKeyPattern.MatchString(bp.Key) || strings.TrimSpace(bp.Name) == "" || !slices.Contains(paramKinds, bp.Kind) {
			return inputErrf("BAD_BUNDLE: parameter %q is not valid", bp.Key)
		}
		if _, ok := v.DocumentType(bp.Document); !ok || v.IsFact(bp.Key) || v.IsRecordType(bp.Key) {
			return inputErrf("BAD_BUNDLE: parameter %q clashes with the vocabulary or names an unknown document", bp.Key)
		}
	}

	plan := importPlan{Format: f.ID, FormatName: f.Name, ParametersAdded: []string{}, ParametersSame: []string{}, ProfileDestination: filepath.ToSlash(folder)}
	err = withTx(*dbFile, func(tx *store.Tx) error {
		if _, exists, err := tx.FormatStateOf(f.ID); err != nil {
			return err
		} else if exists {
			return inputErrf("FORMAT_EXISTS: %s is already stored here; the bundle adds a new format only", f.ID)
		}
		if _, err := os.Stat(folder); err == nil {
			return inputErrf("PROFILE_EXISTS: %s already exists", filepath.ToSlash(folder))
		}
		existing, err := tx.Parameters()
		if err != nil {
			return err
		}
		taken, err := takenNames(tx, "")
		if err != nil {
			return err
		}
		type add struct {
			bp   bundleParameter
			body string
		}
		var adds []add
		for _, bp := range b.Parameters {
			aliases, err := cleanAliases(bp.Aliases)
			if err != nil {
				return err
			}
			body := paramBody(paramDoc{Name: strings.TrimSpace(bp.Name), Kind: bp.Kind, DocumentType: bp.Document, Description: strings.TrimSpace(bp.Description), Aliases: aliases, SinceVocab: v.Version()})
			if i := slices.IndexFunc(existing, func(e store.StoredParameter) bool { return e.State.ParameterID == bp.Key }); i >= 0 {
				if existing[i].Version.SHA256 != sumHex([]byte(body)) {
					return inputErrf("PARAMETER_EXISTS: %s is already stored here, and differently", bp.Key)
				}
				plan.ParametersSame = append(plan.ParametersSame, bp.Key)
				continue
			}
			for _, a := range append([]string{bp.Name}, aliases...) {
				if other, ok := taken[normAlias(a)]; ok {
					return inputErrf("ALIAS_TAKEN: %q is already a name or alias of %s", a, other)
				}
			}
			adds = append(adds, add{bp, body})
			plan.ParametersAdded = append(plan.ParametersAdded, bp.Key)
		}
		if *checkOnly {
			return nil
		}
		// The profile files first, checked by loading them; removed again if anything fails.
		if err := os.MkdirAll(folder, 0o700); err != nil {
			return err
		}
		fail := func(err error) error {
			_ = os.RemoveAll(folder)
			return err
		}
		if err := os.WriteFile(filepath.Join(folder, profile.FileName), []byte(f.ProfileYAML), 0o600); err != nil {
			return fail(err)
		}
		set, err := loadProfiles(*profilesDir, *officeDir, v)
		if err != nil {
			return fail(err)
		}
		if _, ok := set.Lookup(f.ID, p.Version); !ok {
			return fail(inputErrf("PROFILE_NOT_LOADED: the imported profile is not read by the engine"))
		}
		at := nowUTC()
		mapping := string(f.Mapping)
		if _, err := tx.AddFormatVersion(store.FormatVersion{
			FormatID: f.ID, Name: f.Name, Source: store.OriginOffice, FileType: f.FileType, Document: f.Document, BodyYAML: f.ProfileYAML,
			SHA256: sumHex([]byte(f.ProfileYAML)), Note: "Imported from a bundle", Status: store.FormatTesting, CreatedBy: actor,
			MappingJSON: mapping, ProfileVersion: p.Version, PublishedAt: at, PublishedBy: actor,
		}); err != nil {
			return fail(err)
		}
		for _, a := range adds {
			if _, err := tx.AddParameterVersion(store.ParameterVersion{ParameterID: a.bp.Key, Name: strings.TrimSpace(a.bp.Name), Source: store.OriginOffice,
				ParamType: a.bp.Kind, Document: a.bp.Document, BodyYAML: a.body, SHA256: sumHex([]byte(a.body)), CreatedBy: actor}); err != nil {
				return fail(err)
			}
		}
		plan.Applied = true
		return tx.Audit(at, actor, "format.import", "format", f.ID, map[string]string{"name": f.Name, "parameters": strconv.Itoa(len(adds))})
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, plan); err != nil {
		return err
	}
	verb := "would add"
	if plan.Applied {
		verb = "added"
	}
	fmt.Fprintf(stdout, "format-import: %s %s with %d new parameters\n", verb, f.ID, len(plan.ParametersAdded))
	return nil
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The commands of the Formats screen (SPEC-016): list the formats and parameters, switch a
// format off or on, switch back to an earlier version, add a parameter, switch one off or on and
// change its aliases. A standard format or parameter (loaded from content/) is read only; an
// office one is changed by adding a version. Each change writes one audit entry in the same
// transaction (RU-42), and nothing is deleted (RU-41, RU-44).

// formatChange is the standard input of every change: who, and why where a reason is needed.
type formatChange struct {
	By      string `json:"by"`
	Reason  string `json:"reason"`
	Confirm bool   `json:"confirm"` // format-publish: publish although a kept sample reads differently
}

func readChange(stdin io.Reader, into any) error {
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return inputErrf("BAD_REQUEST: the change is not valid JSON: %v", err)
	}
	return nil
}

// withTx opens the database, runs fn in one transaction and commits it only when fn succeeds.
func withTx(dbFile string, fn func(tx *store.Tx) error) error {
	if _, err := os.Stat(dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	db, err := store.Open(dbFile)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ---------- the list ----------

type formatVersionOut struct {
	Revision  int    `json:"revision"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	By        string `json:"by"`
	At        string `json:"at"`
	Note      string `json:"note"`
	Published bool   `json:"published"` // false for a draft: it is not in use and no run reads it
}

type formatOut struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	FileType       string             `json:"file_type"`
	Document       string             `json:"document"`
	Source         string             `json:"source"`
	Version        string             `json:"version"`
	Revision       int                `json:"revision"`
	Status         string             `json:"status"`
	DisabledReason string             `json:"disabled_reason"`
	PublishedBy    string             `json:"published_by"`
	ReadsFacts     []string           `json:"reads_facts"`
	ReadsRecords   []string           `json:"reads_records"`
	Versions       []formatVersionOut `json:"versions"`
	LastUsed       string             `json:"last_used"`   // when a file was last read with it, "" if never
	Needed         int                `json:"needed"`      // figures of its document that the rules read
	NeededRead     int                `json:"needed_read"` // of those, the ones this format reads
	Samples        []sampleOut        `json:"samples"`     // kept sample files, to re-read when a change is published
}

type ruleUse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type parameterOut struct {
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Document       string    `json:"document"`
	Description    string    `json:"description"`
	Aliases        []string  `json:"aliases"`
	Source         string    `json:"source"`
	Status         string    `json:"status"`
	DisabledReason string    `json:"disabled_reason"`
	Deprecated     bool      `json:"deprecated"`
	Version        string    `json:"version"`
	UsedByFormats  []string  `json:"used_by_formats"`
	UsedByRules    []ruleUse `json:"used_by_rules"`
}

type uncoveredRule struct {
	ruleUse
	Missing []string `json:"missing"`
}

type formatList struct {
	Formats      []formatOut       `json:"formats"`
	Parameters   []parameterOut    `json:"parameters"`
	Unsupplied   []string          `json:"unsupplied"`   // parameters no active format reads
	Uncovered    []uncoveredRule   `json:"uncovered"`    // rules with an input no active format reads
	Unrecognised []unrecognisedOut `json:"unrecognised"` // files no format recognised, waiting for one
}

// storedRuleInputs reads each runnable rule's inputs, with its status in the database.
func storedRuleInputs(tx *store.Tx, rulesDir string, v *vocab.Vocab) (map[string][]string, map[string]ruleUse, error) {
	inputs := map[string][]string{}
	uses := map[string]ruleUse{}
	if rulesDir == "" {
		return inputs, uses, nil
	}
	if _, err := os.Stat(rulesDir); err != nil {
		return inputs, uses, nil
	}
	set, err := rules.LoadDir(rulesDir, v, checks.All())
	if err != nil {
		return nil, nil, inputErrf("engine rules %s:\n%v", rulesDir, err)
	}
	stored, err := tx.Rules()
	if err != nil {
		return nil, nil, err
	}
	status := map[string]string{}
	for _, r := range stored {
		status[r.RuleID] = r.Status
	}
	for _, r := range set.Rules() {
		var names []string
		for _, in := range r.Inputs {
			if in.Fact != "" {
				names = append(names, in.Fact)
			}
			if in.RecordType != "" {
				names = append(names, in.RecordType)
			}
		}
		slices.Sort(names)
		inputs[r.ID] = slices.Compact(names)
		uses[r.ID] = ruleUse{ID: r.ID, Title: r.Title, Status: status[r.ID]}
	}
	return inputs, uses, nil
}

// formatsList lists every format and parameter with where each is used, and the gaps.
//
//	recon-engine formats --db <file> [--rules content/rules] [--vocab content/vocabulary] --json <file>
func formatsList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("formats", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	rulesDir := fs.String("rules", "content/rules", "engine rules folder, for where a parameter is used")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the list as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var out formatList
	err = withTx(*dbFile, func(tx *store.Tx) error {
		ruleInputs, ruleUses, err := storedRuleInputs(tx, *rulesDir, v)
		if err != nil {
			return err
		}
		stored, err := tx.Formats()
		if err != nil {
			return err
		}
		supplied := map[string]bool{}
		usedBy := map[string][]string{}
		out.Formats = []formatOut{}
		for _, f := range stored {
			versions, err := tx.FormatVersions(f.State.FormatID)
			if err != nil {
				return err
			}
			o := formatOut{
				ID: f.State.FormatID, Name: f.Version.Name, FileType: f.Version.FileType, Document: f.Version.Document,
				Source: f.Version.Source, Version: f.Version.Version, Revision: f.Version.Revision, Status: f.State.Status,
				DisabledReason: f.State.DisabledReason, PublishedBy: f.State.PublishedBy,
				ReadsFacts: []string{}, ReadsRecords: []string{}, Versions: []formatVersionOut{},
			}
			if p, err := profile.Load(f.State.FormatID+"/"+profile.FileName, []byte(f.Version.BodyYAML), v); err == nil {
				facts, records := p.Reads()
				o.ReadsFacts, o.ReadsRecords = append(o.ReadsFacts, facts...), append(o.ReadsRecords, records...)
			}
			for _, name := range append(slices.Clone(o.ReadsFacts), o.ReadsRecords...) {
				usedBy[name] = append(usedBy[name], o.ID)
				if o.Status == store.FormatActive {
					supplied[name] = true
				}
			}
			for _, ver := range versions {
				o.Versions = append(o.Versions, formatVersionOut{Revision: ver.Revision, Version: ver.Version, Status: ver.Status, By: ver.CreatedBy, At: ver.CreatedAt, Note: ver.Note, Published: ver.PublishedAt != ""})
			}
			if o.LastUsed, err = tx.LastUsedByProfile(o.ID); err != nil {
				return err
			}
			// How much of what the rules need from this document the format reads (the coverage).
			needed := map[string]bool{}
			for _, names := range ruleInputs {
				for _, name := range names {
					if nameDocument(v, name) == o.Document {
						needed[name] = true
					}
				}
			}
			o.Needed = len(needed)
			for _, name := range append(slices.Clone(o.ReadsFacts), o.ReadsRecords...) {
				if needed[name] {
					o.NeededRead++
				}
			}
			o.Samples = []sampleOut{}
			kept, err := tx.FormatSamples(o.ID)
			if err != nil {
				return err
			}
			for _, s := range kept {
				o.Samples = append(o.Samples, sampleOut{ID: s.ID, Name: s.FileName, Size: s.SizeBytes, KeptBy: s.KeptBy, KeptAt: s.KeptAt, HasFile: s.Content != nil})
			}
			out.Formats = append(out.Formats, o)
		}

		params, err := tx.Parameters()
		if err != nil {
			return err
		}
		out.Parameters = []parameterOut{}
		out.Unsupplied = []string{}
		for _, p := range params {
			var d paramDoc
			_ = json.Unmarshal([]byte(p.Version.BodyYAML), &d)
			po := parameterOut{
				Key: p.State.ParameterID, Name: p.Version.Name, Kind: p.Version.ParamType, Document: p.Version.Document,
				Description: d.Description, Aliases: append([]string{}, d.Aliases...), Source: p.Version.Source, Status: p.State.Status,
				DisabledReason: p.State.DisabledReason, Deprecated: d.DeprecatedIn > 0, Version: p.Version.Version,
				UsedByFormats: append([]string{}, usedBy[p.State.ParameterID]...), UsedByRules: []ruleUse{},
			}
			ids := make([]string, 0, len(ruleInputs))
			for id := range ruleInputs {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			for _, id := range ids {
				if slices.Contains(ruleInputs[id], p.State.ParameterID) {
					po.UsedByRules = append(po.UsedByRules, ruleUses[id])
				}
			}
			if po.Status == "active" && !po.Deprecated && !supplied[po.Key] {
				out.Unsupplied = append(out.Unsupplied, po.Key)
			}
			out.Parameters = append(out.Parameters, po)
		}

		out.Unrecognised = []unrecognisedOut{}
		open, err := tx.OpenUnrecognised()
		if err != nil {
			return err
		}
		for _, u := range open {
			out.Unrecognised = append(out.Unrecognised, unrecognisedOut{ID: u.ID, FileName: u.FileName, FileType: u.FileType, SHA256: u.FileSHA256, Size: u.SizeBytes, TaxYear: u.TaxYear, NotedAt: u.NotedAt})
		}
		out.Uncovered = []uncoveredRule{}
		ids := make([]string, 0, len(ruleInputs))
		for id := range ruleInputs {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			if ruleUses[id].Status == store.StatusDisabled {
				continue
			}
			var missing []string
			for _, name := range ruleInputs[id] {
				if !supplied[name] {
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				out.Uncovered = append(out.Uncovered, uncoveredRule{ruleUse: ruleUses[id], Missing: missing})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d formats, %d parameters, %d rules with an input no active format reads\n", len(out.Formats), len(out.Parameters), len(out.Uncovered))
	return nil
}

// ---------- changes to a format ----------

// formatSwitch is format-disable, format-enable and format-switch.
func formatSwitch(name string, args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	formatID := fs.String("format", "", "the format's ID")
	revision := fs.Int("version", 0, "format-switch: the revision to switch back to")
	jsonOut := fs.String("json", "", "write the new state as JSON to this file")
	if err := parseFlags(fs, args, "db", "format"); err != nil {
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
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return inputErrf("REASON_REQUIRED: say why")
	}
	var result map[string]string
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
			return inputErrf("FORMAT_READ_ONLY: %s is a standard format; make an office copy instead", *formatID)
		}
		at := nowUTC()
		var action string
		switch name {
		case "format-disable":
			if st.Status == store.FormatDisabled {
				return inputErrf("ALREADY_DISABLED: %s is already switched off", *formatID)
			}
			err, action = tx.SetFormatStatus(*formatID, store.FormatDisabled, reason, by, at), "format.disable"
		case "format-enable":
			if st.Status != store.FormatDisabled {
				return inputErrf("NOT_DISABLED: %s is not switched off", *formatID)
			}
			// As a rule does, a format switched on again returns to testing and must be published again.
			err, action = tx.SetFormatStatus(*formatID, store.FormatTesting, "", by, at), "format.enable"
		default:
			versions, verr := tx.FormatVersions(*formatID)
			if verr != nil {
				return verr
			}
			i := slices.IndexFunc(versions, func(v store.FormatVersion) bool { return v.Revision == *revision })
			if i < 0 {
				return inputErrf("VERSION_NOT_FOUND: format %s has no version %d", *formatID, *revision)
			}
			if versions[i].Revision == cur.Revision {
				return inputErrf("ALREADY_IN_USE: version %d is the one in use", *revision)
			}
			if versions[i].Status == store.FormatDisabled {
				return inputErrf("VERSION_NOT_USABLE: version %d was saved as disabled", *revision)
			}
			err, action = tx.UseFormatVersion(*formatID, versions[i].ID, store.FormatTesting), "format.switch_version"
		}
		if err != nil {
			return err
		}
		now, _, err := tx.FormatStateOf(*formatID)
		if err != nil {
			return err
		}
		cur, _, err = tx.FormatVersionByID(now.ActiveVersionID)
		if err != nil {
			return err
		}
		if err := tx.Audit(at, by, action, "format", *formatID+" "+cur.Version, map[string]string{"reason": reason}); err != nil {
			return err
		}
		result = map[string]string{"id": *formatID, "version": cur.Version, "status": now.Status}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, result); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s: format %s is now %s (%s)\n", name, result["id"], result["status"], result["version"])
	return nil
}

// ---------- changes to a parameter ----------

var (
	paramKeyPattern    = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)
	paramKinds         = []string{"money", "int", "text", "date", "bool", "rate"}
	maxAliasLength     = 80
	maxDescriptionSize = 300
)

type parameterAdd struct {
	By          string   `json:"by"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Document    string   `json:"document"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
}

func normAlias(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// takenNames lists every parameter name and alias, lower-cased, so an alias is never shared.
func takenNames(tx *store.Tx, except string) (map[string]string, error) {
	params, err := tx.Parameters()
	if err != nil {
		return nil, err
	}
	taken := map[string]string{}
	for _, p := range params {
		if p.State.ParameterID == except {
			continue
		}
		var d paramDoc
		_ = json.Unmarshal([]byte(p.Version.BodyYAML), &d)
		taken[normAlias(p.Version.Name)] = p.State.ParameterID
		for _, a := range d.Aliases {
			taken[normAlias(a)] = p.State.ParameterID
		}
	}
	return taken, nil
}

func cleanAliases(in []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, a := range in {
		a = strings.Join(strings.Fields(a), " ")
		if a == "" {
			continue
		}
		if len([]rune(a)) > maxAliasLength {
			return nil, inputErrf("BAD_ALIAS: an alias is at most %d characters", maxAliasLength)
		}
		if !seen[normAlias(a)] {
			seen[normAlias(a)] = true
			out = append(out, a)
		}
	}
	return out, nil
}

// parameterAddCmd adds an office parameter. Until the vocabulary loader reads office parameters,
// a profile cannot yet write the name, so the parameter is listed and described but not mapped.
//
//	recon-engine parameter-add --db <file> --vocab <folder> --json <file>   (the parameter on standard input)
func parameterAddCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("parameter-add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the new parameter as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	var in parameterAdd
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	key, name, desc := strings.TrimSpace(in.Key), strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	switch {
	case !paramKeyPattern.MatchString(key):
		return inputErrf(`BAD_KEY: the key is written like "gst.taxable_value": lower-case words with underscores, and a dot between the group and the name`)
	case name == "" || len([]rune(name)) > 80:
		return inputErrf("BAD_NAME: give the parameter a name of up to 80 characters")
	case !slices.Contains(paramKinds, in.Kind):
		return inputErrf("BAD_KIND: the type is one of %s", strings.Join(paramKinds, ", "))
	case len([]rune(desc)) > maxDescriptionSize:
		return inputErrf("BAD_DESCRIPTION: at most %d characters", maxDescriptionSize)
	case v.IsFact(key) || v.IsRecordType(key):
		return inputErrf("PARAMETER_EXISTS: %s is already in the vocabulary", key)
	}
	if _, ok := v.DocumentType(in.Document); !ok {
		return inputErrf("BAD_DOCUMENT: %q is not a document type of the vocabulary", in.Document)
	}
	aliases, err := cleanAliases(in.Aliases)
	if err != nil {
		return err
	}
	var result map[string]string
	err = withTx(*dbFile, func(tx *store.Tx) error {
		existing, err := tx.Parameters()
		if err != nil {
			return err
		}
		if slices.ContainsFunc(existing, func(p store.StoredParameter) bool { return p.State.ParameterID == key }) {
			return inputErrf("PARAMETER_EXISTS: a parameter with the key %s already exists", key)
		}
		taken, err := takenNames(tx, "")
		if err != nil {
			return err
		}
		for _, a := range append([]string{name}, aliases...) {
			if other, ok := taken[normAlias(a)]; ok {
				return inputErrf("ALIAS_TAKEN: %q is already a name or alias of %s", a, other)
			}
		}
		body := paramBody(paramDoc{Name: name, Kind: in.Kind, DocumentType: in.Document, Description: desc, Aliases: aliases, SinceVocab: v.Version()})
		if _, err := tx.AddParameterVersion(store.ParameterVersion{
			ParameterID: key, Name: name, Source: store.OriginOffice, ParamType: in.Kind, Document: in.Document,
			BodyYAML: body, SHA256: sumHex([]byte(body)), CreatedBy: by,
		}); err != nil {
			return err
		}
		if err := tx.Audit(nowUTC(), by, "parameter.add", "parameter", key, map[string]string{"name": name, "type": in.Kind, "document": in.Document}); err != nil {
			return err
		}
		result = map[string]string{"key": key, "status": "active"}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, result); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "parameter-add: %s added\n", key)
	return nil
}

// parameterSwitch is parameter-disable and parameter-enable. A parameter that a rule not switched
// off reads cannot be disabled.
func parameterSwitch(name string, args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	key := fs.String("parameter", "", "the parameter's key")
	rulesDir := fs.String("rules", "content/rules", "engine rules folder")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the new state as JSON to this file")
	if err := parseFlags(fs, args, "db", "parameter", "json"); err != nil {
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
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return inputErrf("REASON_REQUIRED: say why")
	}
	disable := name == "parameter-disable"
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var result map[string]string
	err = withTx(*dbFile, func(tx *store.Tx) error {
		params, err := tx.Parameters()
		if err != nil {
			return err
		}
		i := slices.IndexFunc(params, func(p store.StoredParameter) bool { return p.State.ParameterID == *key })
		if i < 0 {
			return inputErrf("PARAMETER_NOT_FOUND: there is no parameter %s", *key)
		}
		p := params[i]
		switch {
		case p.Version.Source != store.OriginOffice:
			return inputErrf("PARAMETER_READ_ONLY: %s is a standard parameter", *key)
		case disable && p.State.Status == "disabled":
			return inputErrf("ALREADY_DISABLED: %s is already switched off", *key)
		case !disable && p.State.Status != "disabled":
			return inputErrf("NOT_DISABLED: %s is not switched off", *key)
		}
		if disable {
			inputs, uses, err := storedRuleInputs(tx, *rulesDir, v)
			if err != nil {
				return err
			}
			var blocking []string
			for id, names := range inputs {
				if uses[id].Status == store.StatusActive && slices.Contains(names, *key) {
					blocking = append(blocking, id)
				}
			}
			slices.Sort(blocking)
			if len(blocking) > 0 {
				return inputErrf("PARAMETER_IN_USE: an active rule reads it: %s; switch the rule off first", strings.Join(blocking, ", "))
			}
		}
		status := "active"
		if disable {
			status = "disabled"
		}
		if err := tx.SetParameterStatus(*key, status, reason); err != nil {
			return err
		}
		action := "parameter.enable"
		if disable {
			action = "parameter.disable"
		}
		if err := tx.Audit(nowUTC(), by, action, "parameter", *key, map[string]string{"reason": reason}); err != nil {
			return err
		}
		result = map[string]string{"key": *key, "status": status}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, result); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s: %s is now %s\n", name, result["key"], result["status"])
	return nil
}

type aliasChange struct {
	By     string `json:"by"`
	Alias  string `json:"alias"`
	Remove bool   `json:"remove"`
}

// parameterAlias adds or removes an alias of an office parameter, as a new version.
func parameterAlias(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("parameter-alias", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	key := fs.String("parameter", "", "the parameter's key")
	jsonOut := fs.String("json", "", "write the aliases as JSON to this file")
	if err := parseFlags(fs, args, "db", "parameter", "json"); err != nil {
		return err
	}
	var in aliasChange
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	alias := strings.Join(strings.Fields(in.Alias), " ")
	if alias == "" || len([]rune(alias)) > maxAliasLength {
		return inputErrf("BAD_ALIAS: type an alias of up to %d characters", maxAliasLength)
	}
	var result map[string]any
	err = withTx(*dbFile, func(tx *store.Tx) error {
		params, err := tx.Parameters()
		if err != nil {
			return err
		}
		i := slices.IndexFunc(params, func(p store.StoredParameter) bool { return p.State.ParameterID == *key })
		if i < 0 {
			return inputErrf("PARAMETER_NOT_FOUND: there is no parameter %s", *key)
		}
		p := params[i]
		if p.Version.Source != store.OriginOffice {
			return inputErrf("PARAMETER_READ_ONLY: the aliases of a standard parameter cannot be changed")
		}
		var d paramDoc
		if err := json.Unmarshal([]byte(p.Version.BodyYAML), &d); err != nil {
			return err
		}
		if in.Remove {
			j := slices.IndexFunc(d.Aliases, func(a string) bool { return normAlias(a) == normAlias(alias) })
			if j < 0 {
				return inputErrf("ALIAS_NOT_FOUND: %q is not an alias of %s", alias, *key)
			}
			d.Aliases = slices.Delete(d.Aliases, j, j+1)
		} else {
			taken, err := takenNames(tx, "")
			if err != nil {
				return err
			}
			if other, ok := taken[normAlias(alias)]; ok {
				return inputErrf("ALIAS_TAKEN: %q is already a name or alias of %s", alias, other)
			}
			d.Aliases = append(d.Aliases, alias)
		}
		body := paramBody(d)
		if _, err := tx.AddParameterVersion(store.ParameterVersion{
			ParameterID: *key, Name: p.Version.Name, Source: store.OriginOffice, ParamType: p.Version.ParamType, Document: p.Version.Document,
			BodyYAML: body, SHA256: sumHex([]byte(body)), CreatedBy: by,
		}); err != nil {
			return err
		}
		action := "parameter.alias_add"
		if in.Remove {
			action = "parameter.alias_remove"
		}
		if err := tx.Audit(nowUTC(), by, action, "parameter", *key, map[string]string{"alias": alias}); err != nil {
			return err
		}
		result = map[string]any{"key": *key, "aliases": append([]string{}, d.Aliases...)}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, result); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "parameter-alias: %s changed\n", *key)
	return nil
}

// nameDocument is the document type a vocabulary fact or record type belongs to, or "".
func nameDocument(v *vocab.Vocab, name string) string {
	if f, ok := v.Lookup(name); ok {
		return f.DocumentType
	}
	if rt, ok := v.RecordType(name); ok {
		return rt.DocumentType
	}
	return ""
}

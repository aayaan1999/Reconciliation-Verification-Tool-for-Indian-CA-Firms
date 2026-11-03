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

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Editing a format (SPEC-016): the dry run of a layout, reading a format's mapping to edit it, and
// publishing a draft. An edit is saved by profile-create --format as a draft version in the
// database; publishing writes its profile files, so the engine reads them, and makes it the
// version in use.

type dryRule struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Missing []string `json:"missing,omitempty"` // the figures of this document the layout does not read
}

// dryRun says which rules that read this layout's document can run on what the layout reads, and
// which cannot check because one of its figures of the document is not read.
type dryRun struct {
	CanRun      []dryRule `json:"can_run"`
	CannotCheck []dryRule `json:"cannot_check"`
}

// dryRunOf works it out from the profile and the engine rules. A rule is about the layout when it
// reads at least one figure or record type of the layout's document type. It returns nil when
// there is no rules folder, so a run without rules shows no dry run.
func dryRunOf(p *profile.Profile, v *vocab.Vocab, rulesDir string) (*dryRun, error) {
	if rulesDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(rulesDir); err != nil {
		return nil, nil
	}
	set, err := rules.LoadDir(rulesDir, v, checks.All())
	if err != nil {
		return nil, inputErrf("engine rules %s:\n%v", rulesDir, err)
	}
	facts, records := p.Reads()
	reads := append(slices.Clone(facts), records...)
	out := &dryRun{CanRun: []dryRule{}, CannotCheck: []dryRule{}}
	list := set.Rules()
	slices.SortFunc(list, func(a, b *rules.Rule) int { return strings.Compare(a.ID, b.ID) })
	for _, r := range list {
		var mine, missing []string
		for _, in := range r.Inputs {
			name, doc := in.Fact, ""
			if name != "" {
				if f, ok := v.Lookup(name); ok {
					doc = f.DocumentType
				}
			} else if name = in.RecordType; name != "" {
				if rt, ok := v.RecordType(name); ok {
					doc = rt.DocumentType
				}
			}
			if name == "" || doc != p.DocumentType {
				continue
			}
			mine = append(mine, name)
			if !slices.Contains(reads, name) && !in.Optional {
				missing = append(missing, name)
			}
		}
		if len(mine) == 0 {
			continue
		}
		slices.Sort(missing)
		if len(missing) == 0 {
			out.CanRun = append(out.CanRun, dryRule{ID: r.ID, Title: r.Title})
		} else {
			out.CannotCheck = append(out.CannotCheck, dryRule{ID: r.ID, Title: r.Title, Missing: slices.Compact(missing)})
		}
	}
	return out, nil
}

// ---------- format-get ----------

type mappingView struct {
	Revision int             `json:"revision"`
	Version  string          `json:"version"`
	Mapping  json.RawMessage `json:"mapping"`
}

type formatGet struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Editable bool         `json:"editable"`
	Reason   string       `json:"reason"` // why it cannot be edited, when Editable is false
	InUse    *mappingView `json:"in_use"` // the version runs use
	Draft    *mappingView `json:"draft"`  // the latest version not yet published, if any
}

// formatGetCmd reads the mapping of an office format for the mapping screen to edit.
//
//	recon-engine format-get --db <file> --format <ID> --json <file>
func formatGetCmd(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("format-get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	formatID := fs.String("format", "", "the format's ID")
	jsonOut := fs.String("json", "", "write the mapping as JSON to this file")
	if err := parseFlags(fs, args, "db", "format", "json"); err != nil {
		return err
	}
	var out formatGet
	err := withTx(*dbFile, func(tx *store.Tx) error {
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
		out = formatGet{ID: *formatID, Name: cur.Name}
		switch {
		case cur.Source != store.OriginOffice:
			out.Reason = "A standard format cannot be changed."
		case cur.MappingJSON == "":
			out.Reason = "This format was not taught on the mapping screen, so its mapping cannot be edited."
		default:
			out.Editable = true
		}
		view := func(v store.FormatVersion) *mappingView {
			if v.MappingJSON == "" {
				return nil
			}
			return &mappingView{Revision: v.Revision, Version: v.Version, Mapping: json.RawMessage(v.MappingJSON)}
		}
		out.InUse = view(cur)
		versions, err := tx.FormatVersions(*formatID)
		if err != nil {
			return err
		}
		for i := len(versions) - 1; i >= 0; i-- {
			if versions[i].PublishedAt == "" && versions[i].Status == store.FormatDraft {
				out.Draft = view(versions[i])
				break
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
	fmt.Fprintf(stdout, "format-get: %s read\n", *formatID)
	return nil
}

// ---------- format-publish ----------

// formatPublish publishes a draft: its profile is written under the office profiles folder, the
// profiles are loaded again to check it, and it becomes the format's version in use, as testing or
// active. A refusal removes what was written.
//
//	recon-engine format-publish --db <file> --office-profiles <folder> --format <ID> --version <n> --status testing|active --json <file>
func formatPublish(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("format-publish", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	officeDir := fs.String("office-profiles", "", "where taught profiles are kept")
	profilesDir := fs.String("profiles", "content/profiles", "format profiles folder")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	rulesDir := fs.String("rules", "content/rules", "engine rules folder, for the dry run")
	formatID := fs.String("format", "", "the format's ID")
	revision := fs.Int("version", 0, "the draft's revision")
	status := fs.String("status", store.FormatTesting, "testing or active")
	jsonOut := fs.String("json", "", "write the outcome as JSON to this file")
	if err := parseFlags(fs, args, "db", "office-profiles", "format", "json"); err != nil {
		return err
	}
	if *status != store.FormatTesting && *status != store.FormatActive {
		return inputErrf("BAD_STATUS: publish as testing or active")
	}
	var in formatChange
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
	var written string // the folder this call created, removed again if anything after it fails
	var result map[string]any
	err = withTx(*dbFile, func(tx *store.Tx) error {
		versions, err := tx.FormatVersions(*formatID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(versions, func(x store.FormatVersion) bool { return x.Revision == *revision })
		if len(versions) == 0 {
			return inputErrf("FORMAT_NOT_FOUND: there is no format %s", *formatID)
		}
		if i < 0 {
			return inputErrf("VERSION_NOT_FOUND: format %s has no version %d", *formatID, *revision)
		}
		ver := versions[i]
		switch {
		case ver.Source != store.OriginOffice:
			return inputErrf("FORMAT_READ_ONLY: %s is a standard format", *formatID)
		case ver.PublishedAt != "":
			return inputErrf("ALREADY_PUBLISHED: version %d was published already; switch back to it instead", *revision)
		case ver.MappingJSON == "":
			return inputErrf("NOT_A_DRAFT: version %d has no mapping to publish", *revision)
		}
		var m profile.Mapping
		if err := json.Unmarshal([]byte(ver.MappingJSON), &m); err != nil {
			return fmt.Errorf("format %s version %d: the stored mapping is unreadable: %w", *formatID, *revision, err)
		}
		pv, err := strconv.Atoi(ver.ProfileVersion)
		if err != nil || pv < 1 {
			return fmt.Errorf("format %s version %d: the profile version %q is not a number", *formatID, *revision, ver.ProfileVersion)
		}
		m.Version = pv
		dir := filepath.Join(*officeDir, filepath.FromSlash(m.Dir()))
		if _, err := os.Stat(dir); err == nil {
			return inputErrf("PROFILE_EXISTS: %s already exists", filepath.ToSlash(dir))
		}
		// Kept samples are read with the version in use and with this draft, before anything is
		// written. A different result needs the admin's confirmation and a reason (SPEC-016).
		samples := []sampleCheck{}
		newP, err := profile.Load(m.Dir()+"/"+profile.FileName, []byte(ver.BodyYAML), v)
		if err != nil {
			return inputErrf("PROFILE_INVALID: the draft does not load: %v", err)
		}
		if st, found, err := tx.FormatStateOf(*formatID); err != nil {
			return err
		} else if found {
			inUse, _, err := tx.FormatVersionByID(st.ActiveVersionID)
			if err != nil {
				return err
			}
			if set0, err := loadProfiles(*profilesDir, *officeDir, v); err == nil {
				if old, ok := set0.Lookup(m.ID(), inUse.ProfileVersion); ok {
					if samples, err = compareSamples(tx, *formatID, old, newP); err != nil {
						return err
					}
				}
			}
		}
		var changedNames []string
		for _, c := range samples {
			if c.Changed {
				changedNames = append(changedNames, c.FileName)
			}
		}
		if len(changedNames) > 0 && (!in.Confirm || strings.TrimSpace(in.Reason) == "") {
			return inputErrf("SAMPLES_CHANGED: %d kept sample(s) read differently with this version: %s; confirm with a reason to publish it", len(changedNames), strings.Join(changedNames, ", "))
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		written = dir
		if err := os.WriteFile(filepath.Join(dir, profile.FileName), []byte(ver.BodyYAML), 0o600); err != nil {
			return err
		}
		set, err := loadProfiles(*profilesDir, *officeDir, v)
		if err != nil {
			return err
		}
		p, ok := set.Lookup(m.ID(), ver.ProfileVersion)
		if !ok {
			return inputErrf("PROFILE_NOT_LOADED: the published profile %s v%s is not read by the engine", m.ID(), ver.ProfileVersion)
		}
		at := nowUTC()
		if err := tx.PublishFormatVersion(*formatID, ver.ID, *status, by, at); err != nil {
			return err
		}
		if err := tx.Audit(at, by, "format.publish", "format", *formatID+" "+ver.Version, map[string]string{"status": *status, "samples_changed": strconv.Itoa(len(changedNames)), "reason": strings.TrimSpace(in.Reason)}); err != nil {
			return err
		}
		result = map[string]any{"id": *formatID, "version": ver.Version, "status": *status, "samples": samples}
		if dr, err := dryRunOf(p, v, *rulesDir); err != nil {
			return err
		} else if dr != nil {
			result["dry_run"] = dr
		}
		return nil
	})
	if err != nil {
		if written != "" {
			_ = os.RemoveAll(written) // nothing was published, so nothing is left on disk
		}
		return err
	}
	if err := writeJSONFile(*jsonOut, result); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "format-publish: %s %s published as %s\n", *formatID, result["version"], *status)
	return nil
}

// usableProfiles drops the office profiles a run must not read (SPEC-016): those of a format that
// is switched off, those not yet approved (testing) unless the run includes testing, and every
// version of a format but the one in use. A profile with no stored format (taught before formats
// were kept in the database) is kept, and so is every standard one.
func usableProfiles(dbFile string, set *profile.Set, includeTesting bool) (*profile.Set, error) {
	byID := map[string]store.StoredFormat{}
	err := withTx(dbFile, func(tx *store.Tx) error {
		list, err := tx.Formats()
		for _, f := range list {
			byID[f.State.FormatID] = f
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	var kept []*profile.Profile
	for _, p := range set.Profiles() {
		f, ok := byID[p.ID]
		if !ok || f.Version.Source != store.OriginOffice {
			kept = append(kept, p)
			continue
		}
		body, err := os.ReadFile(p.File)
		if err != nil {
			return nil, inputErr(err)
		}
		live := f.State.Status == store.FormatActive || (f.State.Status == store.FormatTesting && includeTesting)
		if live && sumHex(body) == f.Version.SHA256 {
			kept = append(kept, p)
		}
	}
	// A kept version whose predecessor was dropped no longer supersedes anything.
	have := map[string]bool{}
	for _, p := range kept {
		have[p.ID+" v"+p.Version] = true
	}
	for i, p := range kept {
		if p.Supersedes != "" && !have[p.ID+" v"+p.Supersedes] {
			c := *p
			c.Supersedes = ""
			kept[i] = &c
		}
	}
	return profile.NewSet(kept...)
}

// storedFormatVersion is the ID of the stored format version whose profile is this one, or 0 when
// none is stored (a profile outside the database). A run records it, so an earlier comparison
// names the exact version it was read with even after the format is edited (SPEC-016, SPEC-009).
func storedFormatVersion(tx *store.Tx, p *profile.Profile) int64 {
	body, err := os.ReadFile(p.File)
	if err != nil {
		return 0
	}
	v, ok, err := tx.FormatVersionBySHA(p.ID, sumHex(body))
	if err != nil || !ok {
		return 0
	}
	return v.ID
}

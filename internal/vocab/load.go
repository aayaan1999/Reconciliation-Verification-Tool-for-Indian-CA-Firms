package vocab

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// File names inside the vocabulary folder (content/vocabulary).
const (
	FactsFile   = "facts.yaml"
	RecordsFile = "records.yaml"
)

// Schema identifiers the files must declare.
const (
	FactsSchema   = "recon.vocab.facts/1"
	RecordsSchema = "recon.vocab.records/1"
)

var (
	factName   = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)
	simpleName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// The file shapes, exactly as written in YAML.

type factsFile struct {
	Schema        string            `yaml:"schema"`
	VocabVersion  int               `yaml:"vocab_version"`
	DocumentTypes []documentTypeDoc `yaml:"document_types"`
	Facts         []factDoc         `yaml:"facts"`
}

type documentTypeDoc struct {
	Name        string `yaml:"name"`
	Prefix      string `yaml:"prefix"`
	Description string `yaml:"description"`
	SinceVocab  int    `yaml:"since_vocab"`
}

type factDoc struct {
	Name         string `yaml:"name"`
	Kind         string `yaml:"kind"`
	Periodicity  string `yaml:"periodicity"`
	DocumentType string `yaml:"document_type"`
	Description  string `yaml:"description"`
	SinceVocab   int    `yaml:"since_vocab"`
	DeprecatedIn int    `yaml:"deprecated_in"`
}

type recordsFile struct {
	Schema       string          `yaml:"schema"`
	VocabVersion int             `yaml:"vocab_version"`
	RecordTypes  []recordTypeDoc `yaml:"record_types"`
}

type recordTypeDoc struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	MatchKey     []string `yaml:"match_key"`
	Amounts      []string `yaml:"amounts"`
	Fields       []string `yaml:"fields"`
	SinceVocab   int      `yaml:"since_vocab"`
	DeprecatedIn int      `yaml:"deprecated_in"`
}

// LoadDir loads facts.yaml and records.yaml from dir.
func LoadDir(dir string) (*Vocab, error) {
	facts, err := os.ReadFile(filepath.Join(dir, FactsFile))
	if err != nil {
		return nil, err
	}
	records, err := os.ReadFile(filepath.Join(dir, RecordsFile))
	if err != nil {
		return nil, err
	}
	return Load(filepath.Join(dir, FactsFile), facts, filepath.Join(dir, RecordsFile), records)
}

// Load parses and validates the two vocabulary files. The names are used in messages.
// Every problem in both files is returned together as strictyaml.Errors, each with its
// file and line.
func Load(factsName string, factsData []byte, recordsName string, recordsData []byte) (*Vocab, error) {
	var ff factsFile
	var rf recordsFile
	var errs strictyaml.Errors
	fpos, err := strictyaml.Decode(factsName, factsData, &ff)
	if err != nil {
		errs = append(errs, err.(strictyaml.Errors)...)
	}
	rpos, err := strictyaml.Decode(recordsName, recordsData, &rf)
	if err != nil {
		errs = append(errs, err.(strictyaml.Errors)...)
	}
	if len(errs) > 0 {
		return nil, errs
	}

	c := checker{}
	docTypes, facts := c.checkFacts(factsName, fpos, ff)
	records := c.checkRecords(recordsName, rpos, rf, ff.VocabVersion)
	if len(c.errs) > 0 {
		c.errs.Sort()
		return nil, c.errs
	}
	return newVocab(ff.VocabVersion, docTypes, facts, records), nil
}

type checker struct{ errs strictyaml.Errors }

func (c *checker) add(file string, line int, format string, args ...any) {
	c.errs = append(c.errs, &strictyaml.Error{File: file, Line: line, Msg: fmt.Sprintf(format, args...)})
}

// checkText requires non-empty, single-line text.
func (c *checker) checkText(file string, line int, what, s string) {
	switch {
	case strings.TrimSpace(s) == "":
		c.add(file, line, "%s is missing", what)
	case strings.ContainsAny(s, "\r\n"):
		c.add(file, line, "%s must be on one line", what)
	}
}

// checkVersions checks since_vocab and deprecated_in against the file's version.
func (c *checker) checkVersions(file string, pos strictyaml.Positions, path string, since, deprecated, version int) {
	if since < 1 || since > version {
		c.add(file, lineOf(pos, path+".since_vocab", path), "since_vocab must be between 1 and vocab_version (%d)", version)
	}
	if deprecated != 0 && (deprecated <= since || deprecated > version) {
		c.add(file, pos.Line(path+".deprecated_in"), "deprecated_in must be after since_vocab and not after vocab_version (%d)", version)
	}
}

func (c *checker) checkHeader(file string, pos strictyaml.Positions, schema, wantSchema string, version int) {
	if schema != wantSchema {
		c.add(file, pos.Line("schema"), "schema must be %q", wantSchema)
	}
	if version < 1 {
		c.add(file, pos.Line("vocab_version"), "vocab_version must be 1 or more")
	}
}

func (c *checker) checkFacts(file string, pos strictyaml.Positions, ff factsFile) ([]DocumentType, []Fact) {
	c.checkHeader(file, pos, ff.Schema, FactsSchema, ff.VocabVersion)
	version := ff.VocabVersion

	prefixes := map[string]string{} // document type name -> prefix
	usedPrefix := map[string]bool{}
	var docTypes []DocumentType
	for i, d := range ff.DocumentTypes {
		path := fmt.Sprintf("document_types[%d]", i)
		line := pos.Line(path)
		switch _, dup := prefixes[d.Name]; {
		case !simpleName.MatchString(d.Name):
			c.add(file, line, "document type name %q must be lower case letters, digits and _ (starting with a letter)", d.Name)
		case dup:
			c.add(file, line, "document type %q is listed twice", d.Name)
		}
		switch {
		case !simpleName.MatchString(d.Prefix):
			c.add(file, lineOf(pos, path+".prefix", path), "prefix %q of document type %q must be lower case letters, digits and _", d.Prefix, d.Name)
		case usedPrefix[d.Prefix]:
			c.add(file, pos.Line(path+".prefix"), "prefix %q is used by two document types", d.Prefix)
		}
		c.checkText(file, lineOf(pos, path+".description", path), fmt.Sprintf("description of document type %q", d.Name), d.Description)
		c.checkVersions(file, pos, path, d.SinceVocab, 0, version)
		prefixes[d.Name] = d.Prefix
		usedPrefix[d.Prefix] = true
		docTypes = append(docTypes, DocumentType{Name: d.Name, Prefix: d.Prefix, Description: d.Description, SinceVocab: d.SinceVocab})
	}

	seen := map[string]bool{}
	var facts []Fact
	for i, f := range ff.Facts {
		path := fmt.Sprintf("facts[%d]", i)
		line := pos.Line(path)
		switch {
		case !factName.MatchString(f.Name):
			c.add(file, line, "fact name %q must be lower case ASCII parts joined by dots, such as \"tar.depreciation_schedule.total\"", f.Name)
		case seen[f.Name]:
			c.add(file, line, "fact %q is listed twice; a name may appear only once", f.Name)
		}
		seen[f.Name] = true
		kind, ok := domain.ParseValueKind(f.Kind)
		if !ok {
			c.add(file, lineOf(pos, path+".kind", path), "kind %q of fact %q is unknown (allowed: money, int, text, date, bool, rate)", f.Kind, f.Name)
		}
		switch Periodicity(f.Periodicity) {
		case PeriodicityFY, PeriodicityQuarter, PeriodicityMonth:
		default:
			c.add(file, lineOf(pos, path+".periodicity", path), "periodicity %q of fact %q is unknown (allowed: FY, quarter, month)", f.Periodicity, f.Name)
		}
		if prefix, ok := prefixes[f.DocumentType]; !ok {
			c.add(file, lineOf(pos, path+".document_type", path), "document type %q of fact %q is not in document_types", f.DocumentType, f.Name)
		} else if first, _, _ := strings.Cut(f.Name, "."); first != prefix && factName.MatchString(f.Name) {
			c.add(file, line, "fact %q belongs to document type %q, so its name must start with %q", f.Name, f.DocumentType, prefix+".")
		}
		c.checkText(file, lineOf(pos, path+".description", path), fmt.Sprintf("description of fact %q", f.Name), f.Description)
		c.checkVersions(file, pos, path, f.SinceVocab, f.DeprecatedIn, version)
		facts = append(facts, Fact{Name: f.Name, Kind: kind, Periodicity: Periodicity(f.Periodicity), DocumentType: f.DocumentType,
			Description: f.Description, SinceVocab: f.SinceVocab, DeprecatedIn: f.DeprecatedIn})
	}
	return docTypes, facts
}

func (c *checker) checkRecords(file string, pos strictyaml.Positions, rf recordsFile, factsVersion int) []RecordType {
	c.checkHeader(file, pos, rf.Schema, RecordsSchema, rf.VocabVersion)
	if rf.VocabVersion != factsVersion {
		c.add(file, pos.Line("vocab_version"), "vocab_version is %d here but %d in %s; both files carry the same version", rf.VocabVersion, factsVersion, FactsFile)
	}

	seen := map[string]bool{}
	var records []RecordType
	for i, r := range rf.RecordTypes {
		path := fmt.Sprintf("record_types[%d]", i)
		line := pos.Line(path)
		switch {
		case !simpleName.MatchString(r.Name):
			c.add(file, line, "record type name %q must be lower case letters, digits and _ (starting with a letter)", r.Name)
		case seen[r.Name]:
			c.add(file, line, "record type %q is listed twice", r.Name)
		}
		seen[r.Name] = true
		c.checkText(file, lineOf(pos, path+".description", path), fmt.Sprintf("description of record type %q", r.Name), r.Description)

		columns := map[string]string{} // column name -> "amounts" or "fields"
		for _, list := range []struct {
			key   string
			names []string
		}{{"amounts", r.Amounts}, {"fields", r.Fields}} {
			for j, n := range list.names {
				l := lineOf(pos, fmt.Sprintf("%s.%s[%d]", path, list.key, j), path)
				switch prev, dup := columns[n]; {
				case !simpleName.MatchString(n):
					c.add(file, l, "column name %q in record type %q must be lower case letters, digits and _", n, r.Name)
				case dup && prev == list.key:
					c.add(file, l, "column %q is listed twice in %s of record type %q", n, list.key, r.Name)
				case dup:
					c.add(file, l, "column %q of record type %q is in both amounts and fields; it belongs in one", n, r.Name)
				}
				columns[n] = list.key
			}
		}
		if len(r.MatchKey) == 0 {
			c.add(file, line, "record type %q needs a match_key", r.Name)
		}
		keySeen := map[string]bool{}
		for j, k := range r.MatchKey {
			l := lineOf(pos, fmt.Sprintf("%s.match_key[%d]", path, j), path)
			switch {
			case keySeen[k]:
				c.add(file, l, "match_key of record type %q lists %q twice", r.Name, k)
			case columns[k] != "fields":
				c.add(file, l, "match_key of record type %q names %q, which is not one of its fields", r.Name, k)
			}
			keySeen[k] = true
		}
		c.checkVersions(file, pos, path, r.SinceVocab, r.DeprecatedIn, rf.VocabVersion)
		records = append(records, RecordType{Name: r.Name, Description: r.Description, MatchKey: r.MatchKey,
			Amounts: r.Amounts, Fields: r.Fields, SinceVocab: r.SinceVocab, DeprecatedIn: r.DeprecatedIn})
	}
	return records
}

// lineOf returns the line of path, or of fallback when path was not written in the file.
func lineOf(pos strictyaml.Positions, path, fallback string) int {
	if l := pos.Line(path); l > 0 {
		return l
	}
	return pos.Line(fallback)
}

// contentHash hashes the meaning of the vocabulary: a fixed JSON form of every entry in
// name order, with kinds written as names. encoding/json writes struct fields in
// declaration order, so the form is stable.
func contentHash(v *Vocab) string {
	type fact struct {
		Fact
		Kind string
	}
	facts := make([]fact, len(v.facts))
	for i, f := range v.facts {
		facts[i] = fact{f, f.Kind.String()}
	}
	b, err := json.Marshal(struct {
		Version       int
		DocumentTypes []DocumentType
		Facts         []fact
		RecordTypes   []RecordType
	}{v.version, v.docTypes, facts, v.recordTypes})
	if err != nil {
		panic(err) // plain structs of strings and ints always marshal
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

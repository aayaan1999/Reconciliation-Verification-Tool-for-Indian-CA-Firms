package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The Document Master commands (SPEC-022 part A): the Partner adds, edits, switches and deletes
// the kinds of document the tool can read. Each command does its work and writes its audit entry
// in one transaction; the server forwards and checks the permission.
//
//	recon-engine document-list     --db <file> [--vocab <folder>] [--active] [--file-type json|xlsx|csv] --json <file>
//	recon-engine document-get      --db <file> [--vocab <folder>] --code <code> --json <file>
//	recon-engine document-save     --db <file> [--vocab <folder>] [--edit] --json <file>   (the document on standard input)
//	recon-engine document-validate --db <file> [--vocab <folder>] [--edit] --json <file>   (the document on standard input; nothing is saved)
//	recon-engine document-status   --db <file> [--vocab <folder>] --json <file>            ({"by","code","active"} on standard input)
//	recon-engine document-delete   --db <file> [--vocab <folder>] --json <file>            ({"by","code"} on standard input)

// The audit actions of the Master.
const (
	auditDocAdd        = "document.add"
	auditDocEdit       = "document.edit"
	auditDocActivate   = "document.activate"
	auditDocDeactivate = "document.deactivate"
	auditDocDelete     = "document.delete"
	auditTypeAdd       = "record_type.add"
	auditTypeEdit      = "record_type.edit"
	auditTypeDelete    = "record_type.delete"
)

// builtinCategories gives each built-in document its default category. It is only a grouping
// for the Master's list; a document not named here is "Other".
var builtinCategories = map[string]string{
	"sales_register": "Books", "purchase_register": "Books", "ledger": "Books", "financial_statements": "Books",
	"gstr1": "GST", "gstr2a": "GST", "gstr2b": "GST", "gstr3b": "GST", "einvoice": "GST", "ewaybill": "GST", "ims": "GST",
	"computation": "Income tax", "tax_audit_report": "Income tax", "mat_report": "Income tax", "form_67": "Income tax",
	"tds_ledger": "TDS", "form140": "TDS", "tds_challan": "TDS", "oltas_status": "TDS", "tds_receivable": "TDS", "form168": "TDS", "form_26as": "TDS",
	"bank_book": "Bank", "bank_statement": "Bank", "bank_balances": "Bank",
}

// ---------- reading the Master into the vocabulary ----------

// A master is the Document Master as stored: every document and every office record type.
type master struct {
	docs  []store.DocType
	types []store.OfficeRecordType
}

func readMasterTx(tx *store.Tx) (master, error) {
	var m master
	var err error
	if m.docs, err = tx.DocTypes(); err != nil {
		return m, err
	}
	m.types, err = tx.OfficeRecordTypes()
	return m, err
}

// readMaster reads the Master from the database; with no database (or none yet) it is empty.
func readMaster(dbFile string) (master, error) {
	if dbFile == "" {
		return master{}, nil
	}
	if _, err := os.Stat(dbFile); errors.Is(err, fs.ErrNotExist) {
		return master{}, nil
	}
	var m master
	err := withTx(dbFile, func(tx *store.Tx) error {
		var err error
		m, err = readMasterTx(tx)
		return err
	})
	return m, err
}

// office is the Master's office documents and record types in the vocabulary's terms.
func (m master) office() vocab.Office {
	var o vocab.Office
	for _, d := range m.docs {
		if d.Origin != store.OriginOffice {
			continue
		}
		o.Documents = append(o.Documents, vocab.OfficeDocument{Code: d.Code, Name: d.Name, Category: d.Category,
			Description: d.Description, Formats: d.ApplicableFor, RecordType: d.RecordType, Active: d.Active})
	}
	for _, rt := range m.types {
		o.RecordTypes = append(o.RecordTypes, vocabRecordType(rt))
	}
	return o
}

func vocabRecordType(rt store.OfficeRecordType) vocab.OfficeRecordType {
	out := vocab.OfficeRecordType{Name: rt.Name, Description: rt.Description}
	for _, c := range rt.Columns {
		out.Columns = append(out.Columns, vocab.OfficeColumn{Name: c.Name, Label: c.Label, Type: c.Type, Key: c.Key, Required: c.Required})
	}
	return out
}

// loadVocab is the one way the commands read the vocabulary: the base files, plus the ready
// office documents in the database when there is one. With an empty Master the result equals
// the base. The error is not wrapped; callers say "vocabulary:" as they did for vocab.LoadDir.
func loadVocab(dbFile, dir string) (*vocab.Vocab, error) {
	v, _, err := loadVocabMaster(dbFile, dir)
	return v, err
}

// loadVocabMaster is loadVocab that also returns the Master it read, for the commands that
// need the active flags and formats too.
func loadVocabMaster(dbFile, dir string) (*vocab.Vocab, master, error) {
	base, err := vocab.LoadDir(dir)
	if err != nil {
		return nil, master{}, err
	}
	m, err := readMaster(dbFile)
	if err != nil {
		return nil, master{}, &masterError{err}
	}
	merged, err := base.WithOffice(m.office())
	if err != nil {
		return nil, master{}, fmt.Errorf("office documents: %w", err)
	}
	return merged, m, nil
}

// ---------- the built-in documents ----------

func prettify(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// seedBuiltins makes sure every built-in document type of the base vocabulary has its row. The
// YAML stays the definition (name, description, record type); the Partner's active switch and
// the category are kept. It writes nothing when nothing changed.
func seedBuiltins(tx *store.Tx, base *vocab.Vocab, at string) error {
	for _, dt := range base.DocumentTypes() {
		if dt.Office {
			continue
		}
		category := builtinCategories[dt.Name]
		if category == "" {
			category = "Other"
		}
		rt, _ := profile.RecordTypeFor(base, dt.Name)
		if _, err := tx.SeedBuiltinDocument(store.DocType{Code: dt.Name, Name: prettify(dt.Name), Category: category, Description: dt.Description,
			ApplicableFor: slices.Clone(vocab.OfficeFormats), RecordType: rt.Name,
			CreatedBy: "system", CreatedAt: at, UpdatedBy: "system", UpdatedAt: at}); err != nil {
			return err
		}
	}
	return nil
}

// ---------- the shapes the commands read and write ----------

type docColumnJSON struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Key      bool   `json:"key"`
	Required bool   `json:"required"`
}

type docRecordTypeIn struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CopyFrom    string          `json:"copy_from"` // a record type whose columns are copied when columns is empty
	Columns     []docColumnJSON `json:"columns"`
}

type docSaveIn struct {
	By            string           `json:"by"`
	Code          string           `json:"code"`
	Name          string           `json:"name"`
	Category      string           `json:"category"`
	Description   string           `json:"description"`
	ApplicableFor []string         `json:"applicable_for"`
	Status        string           `json:"status"`      // "active" or "inactive"; empty keeps (edit) or is active (add)
	RecordType    *docRecordTypeIn `json:"record_type"` // absent: a Draft on add, the structure unchanged on edit
}

type docProblemJSON struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type docRecordTypeOut struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CopiedFrom  string          `json:"copied_from"`
	Locked      bool            `json:"locked"` // something reads it: only labels, the description and new optional columns may change
	Columns     []docColumnJSON `json:"columns"`
}

type docUsageJSON struct {
	Formats     []string `json:"formats"`
	Rules       []string `json:"rules"`
	ActiveRules []string `json:"active_rules"`
	Records     int      `json:"records"`
}

type docRefs struct {
	Formats int `json:"formats"`
	Rules   int `json:"rules"`
}

type docOut struct {
	Code          string   `json:"code"`
	Origin        string   `json:"origin"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Description   string   `json:"description"`
	ApplicableFor []string `json:"applicable_for"`
	Status        string   `json:"status"` // draft, active or inactive
	RecordType    string   `json:"record_type"`
	UpdatedBy     string   `json:"updated_by"`
	UpdatedAt     string   `json:"updated_at"`
	ReferencedBy  docRefs  `json:"referenced_by"`
}

type docHistoryJSON struct {
	At      string `json:"at"`
	By      string `json:"by"`
	Action  string `json:"action"`
	Details string `json:"details"`
}

type docDetailOut struct {
	docOut
	Structure *docRecordTypeOut `json:"structure"`
	Usage     docUsageJSON      `json:"usage"`
	History   []docHistoryJSON  `json:"history"`
}

func docStatus(d store.DocType) string {
	switch {
	case d.Origin == store.OriginOffice && d.RecordType == "":
		return "draft"
	case !d.Active:
		return "inactive"
	}
	return "active"
}

func docProblems(ps []vocab.Problem) []docProblemJSON {
	out := make([]docProblemJSON, len(ps))
	for i, p := range ps {
		out[i] = docProblemJSON{Field: p.Field, Code: p.Code, Message: p.Msg}
	}
	return out
}

func columnsJSON(cols []vocab.OfficeColumn) []docColumnJSON {
	out := make([]docColumnJSON, len(cols))
	for i, c := range cols {
		out[i] = docColumnJSON{Name: c.Name, Label: c.Label, Type: c.Type, Key: c.Key, Required: c.Required}
	}
	return out
}

func vocabColumns(in []docColumnJSON) []vocab.OfficeColumn {
	out := make([]vocab.OfficeColumn, len(in))
	for i, c := range in {
		out[i] = vocab.OfficeColumn{Name: strings.TrimSpace(c.Name), Label: strings.TrimSpace(c.Label), Type: strings.TrimSpace(c.Type), Key: c.Key, Required: c.Required}
	}
	return out
}

// builtinColumns writes a base record type as columns: amounts are amounts, a field named
// *_date is a date, the rest are text, the match key columns are keys. A built-in record type has
// no labels or required flags, so the label is the name in words.
func builtinColumns(rt vocab.RecordType) []vocab.OfficeColumn {
	var out []vocab.OfficeColumn
	for _, f := range rt.Fields {
		typ := vocab.ColumnText
		if strings.HasSuffix(f, "_date") {
			typ = vocab.ColumnDate
		}
		out = append(out, vocab.OfficeColumn{Name: f, Label: prettify(f), Type: typ, Key: slices.Contains(rt.MatchKey, f)})
	}
	for _, a := range rt.Amounts {
		out = append(out, vocab.OfficeColumn{Name: a, Label: prettify(a), Type: vocab.ColumnAmount})
	}
	return out
}

// ---------- the details of one document ----------

func usageJSON(u store.Usage) docUsageJSON {
	return docUsageJSON{Formats: nonNil(u.Formats), Rules: nonNil(u.Rules), ActiveRules: nonNil(u.ActiveRules), Records: u.Records}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func docRow(d store.DocType, u store.Usage) docOut {
	return docOut{Code: d.Code, Origin: d.Origin, Name: d.Name, Category: d.Category, Description: d.Description,
		ApplicableFor: nonNil(d.ApplicableFor), Status: docStatus(d), RecordType: d.RecordType, UpdatedBy: d.UpdatedBy, UpdatedAt: d.UpdatedAt,
		ReferencedBy: docRefs{Formats: len(u.Formats), Rules: len(u.Rules)}}
}

// docDetail reads one document with its structure, what uses it and its audit history.
func docDetail(tx *store.Tx, base *vocab.Vocab, code string) (docDetailOut, error) {
	d, ok, err := tx.DocTypeByCode(code)
	if err != nil {
		return docDetailOut{}, err
	}
	if !ok {
		return docDetailOut{}, inputErrf("NOT_FOUND: there is no document with the code %q", code)
	}
	u, err := tx.DocUsage(code)
	if err != nil {
		return docDetailOut{}, err
	}
	out := docDetailOut{docOut: docRow(d, u), Usage: usageJSON(u), History: []docHistoryJSON{}}
	switch {
	case d.RecordType == "":
	case d.Origin == store.OriginBuiltin:
		if rt, ok := base.RecordType(d.RecordType); ok {
			out.Structure = &docRecordTypeOut{Name: rt.Name, Description: rt.Description, Locked: true, Columns: columnsJSON(builtinColumns(rt))}
		}
	default:
		rt, ok, err := tx.OfficeRecordTypeByName(d.RecordType)
		if err != nil {
			return docDetailOut{}, err
		}
		if ok {
			out.Structure = &docRecordTypeOut{Name: rt.Name, Description: rt.Description, CopiedFrom: rt.CopiedFrom, Locked: u.InUse(),
				Columns: columnsJSON(vocabRecordType(rt).Columns)}
		}
	}
	entries, err := tx.AuditEntries()
	if err != nil {
		return docDetailOut{}, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if e := entries[i]; e.ObjectType == "document" && e.ObjectID == code {
			out.History = append(out.History, docHistoryJSON{At: e.At, By: e.Actor, Action: e.Action, Details: e.Details})
		}
	}
	return out, nil
}

// ---------- document-list and document-get ----------

func documentList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("document-list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	onlyActive := fs.Bool("active", false, "only the documents a new layout or rule may use (ready and active)")
	fileType := fs.String("file-type", "", "only documents that accept this file type (json, xlsx or csv)")
	jsonOut := fs.String("json", "", "write the list as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	if *fileType != "" && !slices.Contains(vocab.OfficeFormats, *fileType) {
		return inputErrf("BAD_FORMATS: --file-type must be json, xlsx or csv")
	}
	base, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	out := struct {
		Documents []docOut `json:"documents"`
	}{Documents: []docOut{}}
	err = withTx(*dbFile, func(tx *store.Tx) error {
		if err := seedBuiltins(tx, base, nowUTC()); err != nil {
			return err
		}
		docs, err := tx.DocTypes()
		if err != nil {
			return err
		}
		for _, d := range docs {
			if *onlyActive && docStatus(d) != "active" {
				continue
			}
			if *fileType != "" && !slices.Contains(d.ApplicableFor, *fileType) {
				continue
			}
			u, err := tx.DocUsage(d.Code)
			if err != nil {
				return err
			}
			out.Documents = append(out.Documents, docRow(d, u))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "document-list: %d documents\n", len(out.Documents))
	return err
}

func documentGet(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("document-get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	code := fs.String("code", "", "the document's code")
	jsonOut := fs.String("json", "", "write the document as JSON to this file")
	if err := parseFlags(fs, args, "db", "code", "json"); err != nil {
		return err
	}
	base, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var out docDetailOut
	err = withTx(*dbFile, func(tx *store.Tx) error {
		if err := seedBuiltins(tx, base, nowUTC()); err != nil {
			return err
		}
		var err error
		out, err = docDetail(tx, base, *code)
		return err
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "document-get: %s (%s)\n", out.Code, out.Status)
	return err
}

// ---------- document-validate and document-save ----------

// docFieldProblems are the problems of the document's own fields; the others are about its
// record structure.
var docFieldNames = []string{"code", "name", "description", "category", "applicable_for"}

func isDocFieldProblem(p vocab.Problem) bool { return slices.Contains(docFieldNames, p.Field) }

// A plan is a document definition checked against the Master as it stands.
type docPlan struct {
	doc        vocab.OfficeDocument
	rt         vocab.OfficeRecordType // the structure as given (or the stored one when none was given)
	copiedFrom string
	given      bool // a structure was given
	existing   *store.DocType
	problems   []vocab.Problem // every problem, document and structure
}

func (p docPlan) docProblems() []vocab.Problem {
	var out []vocab.Problem
	for _, pr := range p.problems {
		if isDocFieldProblem(pr) {
			out = append(out, pr)
		}
	}
	return out
}

func (p docPlan) structureProblems() []vocab.Problem {
	var out []vocab.Problem
	for _, pr := range p.problems {
		if !isDocFieldProblem(pr) {
			out = append(out, pr)
		}
	}
	return out
}

// planDocument resolves and checks a definition: it copies columns when asked, compares names
// with the rest of the Master, and applies the locking rules when it edits a structure in use.
// It reads the store and writes nothing.
func planDocument(tx *store.Tx, base *vocab.Vocab, in docSaveIn, edit bool) (docPlan, error) {
	var p docPlan
	code := strings.TrimSpace(in.Code)
	m, err := readMasterTx(tx)
	if err != nil {
		return p, err
	}
	if i := slices.IndexFunc(m.docs, func(d store.DocType) bool { return d.Code == code }); i >= 0 {
		p.existing = &m.docs[i]
	}
	switch {
	case edit && p.existing == nil:
		return p, inputErrf("NOT_FOUND: there is no document with the code %q", code)
	case edit && p.existing.Origin != store.OriginOffice:
		return p, inputErrf("BUILTIN: %q is a built-in document; it can only be switched on or off", code)
	case !edit && p.existing != nil:
		return p, inputErrf("DUPLICATE_CODE: a document with the code %q already exists", code)
	}
	p.doc = vocab.OfficeDocument{Code: code, Name: strings.TrimSpace(in.Name), Category: strings.TrimSpace(in.Category),
		Description: strings.TrimSpace(in.Description), Formats: slices.Clone(in.ApplicableFor)}

	// the rest of the Master: every office document and record type but this one's own
	others := m.office()
	others.Documents = slices.DeleteFunc(others.Documents, func(d vocab.OfficeDocument) bool { return d.Code == code })
	stored := ""
	if p.existing != nil {
		stored = p.existing.RecordType
	}
	others.RecordTypes = slices.DeleteFunc(others.RecordTypes, func(r vocab.OfficeRecordType) bool { return stored != "" && r.Name == stored })

	var cur *store.OfficeRecordType
	if stored != "" {
		for i := range m.types {
			if m.types[i].Name == stored {
				cur = &m.types[i]
			}
		}
	}
	switch {
	case in.RecordType != nil:
		p.given = true
		rin := *in.RecordType
		cols := rin.Columns
		if len(cols) == 0 && rin.CopyFrom != "" {
			if cols, err = copySource(base, m, rin.CopyFrom); err != nil {
				return p, err
			}
		}
		p.copiedFrom = strings.TrimSpace(rin.CopyFrom)
		p.rt = vocab.OfficeRecordType{Name: strings.TrimSpace(rin.Name), Description: strings.TrimSpace(rin.Description), Columns: vocabColumns(cols)}
		p.doc.RecordType = p.rt.Name
	case cur != nil: // an edit that leaves the structure as it is
		p.rt = vocabRecordType(*cur)
		p.doc.RecordType = cur.Name
	}
	rtForCheck := p.rt
	if !p.given && cur == nil {
		// No structure yet: only the document's own fields are checked.
		rtForCheck = vocab.OfficeRecordType{}
	}
	p.problems = base.ValidateOfficeDocument(others, p.doc, rtForCheck)
	if !p.given && cur == nil {
		p.problems = slices.DeleteFunc(p.problems, func(pr vocab.Problem) bool { return !isDocFieldProblem(pr) })
	}

	if cur != nil && p.given && p.rt.Name != cur.Name {
		p.problems = append(p.problems, vocab.Problem{Field: "record_type.name", Code: vocab.BadRecordType,
			Msg: fmt.Sprintf("the record type %q cannot be renamed", cur.Name)})
	}
	if edit && p.existing != nil {
		// A file type a format still reads cannot be taken off the document.
		used, err := tx.DocFormatFileTypes(code)
		if err != nil {
			return p, err
		}
		for _, f := range used {
			if !slices.Contains(p.doc.Formats, f) {
				p.problems = append(p.problems, vocab.Problem{Field: "applicable_for", Code: vocab.BadFormats,
					Msg: fmt.Sprintf("a layout reads this document from %s files, so %s cannot be taken off", f, f)})
			}
		}
	}
	if cur != nil && p.given {
		u, err := tx.DocUsage(code)
		if err != nil {
			return p, err
		}
		if u.InUse() {
			p.problems = append(p.problems, lockProblems(vocabRecordType(*cur).Columns, p.rt.Columns)...)
		}
	}
	return p, nil
}

// lockProblems says what an edit of a locked structure changes that it may not: a column that is
// removed, renamed, retyped or whose key or required flag changes, and a new column that is a key
// or required. Labels, the description and new optional columns are allowed.
func lockProblems(old, now []vocab.OfficeColumn) []vocab.Problem {
	var ps []vocab.Problem
	bad := func(field, format string, args ...any) {
		ps = append(ps, vocab.Problem{Field: field, Code: "LOCKED", Msg: fmt.Sprintf(format, args...)})
	}
	for _, o := range old {
		i := slices.IndexFunc(now, func(c vocab.OfficeColumn) bool { return c.Name == o.Name })
		switch {
		case i < 0:
			bad("columns", "column %q cannot be removed or renamed: a layout, stored rows or a rule use this structure", o.Name)
		case now[i].Type != o.Type:
			bad(fmt.Sprintf("columns[%d].type", i), "column %q cannot change type: a layout, stored rows or a rule use this structure", o.Name)
		case now[i].Key != o.Key:
			bad(fmt.Sprintf("columns[%d].key", i), "column %q cannot change its key flag: a layout, stored rows or a rule use this structure", o.Name)
		case now[i].Required != o.Required:
			bad(fmt.Sprintf("columns[%d].required", i), "column %q cannot change its required flag: a layout, stored rows or a rule use this structure", o.Name)
		}
	}
	for i, c := range now {
		if slices.ContainsFunc(old, func(o vocab.OfficeColumn) bool { return o.Name == c.Name }) {
			continue
		}
		if c.Key || c.Required {
			bad(fmt.Sprintf("columns[%d]", i), "the new column %q must be neither a key nor required while the structure is in use", c.Name)
		}
	}
	return ps
}

// copySource reads the columns of a record type to copy: an office one first, then a built-in.
func copySource(base *vocab.Vocab, m master, from string) ([]docColumnJSON, error) {
	from = strings.TrimSpace(from)
	for _, rt := range m.types {
		if rt.Name == from {
			return columnsJSON(vocabRecordType(rt).Columns), nil
		}
	}
	if rt, ok := base.RecordType(from); ok {
		return columnsJSON(builtinColumns(rt)), nil
	}
	return nil, inputErrf("BAD_RECORD_TYPE: %q is not a record type that can be copied", from)
}

type docValidateOut struct {
	Valid      bool              `json:"valid"`
	Problems   []docProblemJSON  `json:"problems"`
	RecordType *docRecordTypeOut `json:"record_type"` // the structure as it would be saved, with copied columns filled in
}

func recordTypeOut(p docPlan) *docRecordTypeOut {
	if !p.given && p.rt.Name == "" {
		return nil
	}
	return &docRecordTypeOut{Name: p.rt.Name, Description: p.rt.Description, CopiedFrom: p.copiedFrom, Columns: columnsJSON(p.rt.Columns)}
}

func documentValidate(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("document-validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	edit := fs.Bool("edit", false, "check an edit of an existing document")
	jsonOut := fs.String("json", "", "write the result as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	var in docSaveIn
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	base, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var out docValidateOut
	err = withTx(*dbFile, func(tx *store.Tx) error {
		p, err := planDocument(tx, base, in, *edit)
		if err != nil {
			return err
		}
		out = docValidateOut{Valid: len(p.problems) == 0, Problems: docProblems(p.problems), RecordType: recordTypeOut(p)}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "document-validate: %d problems\n", len(out.Problems))
	return err
}

type docSaveOut struct {
	Saved    bool             `json:"saved"`
	Problems []docProblemJSON `json:"problems"` // for a Draft: what is missing
	Document *docDetailOut    `json:"document"`
}

func documentSave(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("document-save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	edit := fs.Bool("edit", false, "edit an existing office document (without it a new one is added)")
	jsonOut := fs.String("json", "", "write the result as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	var in docSaveIn
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	status := strings.TrimSpace(in.Status)
	if status != "" && status != "active" && status != "inactive" {
		return inputErrf("BAD_STATUS: status is active or inactive")
	}
	base, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var out docSaveOut
	var refused *docSaveOut
	err = withTx(*dbFile, func(tx *store.Tx) error {
		at := nowUTC()
		if err := seedBuiltins(tx, base, at); err != nil {
			return err
		}
		p, err := planDocument(tx, base, in, *edit)
		if err != nil {
			return err
		}
		// A problem in the document's own fields, or in the structure of a document that is ready
		// already, refuses the save. A structure that is not valid yet leaves a Draft.
		hard := p.docProblems()
		if p.existing != nil && p.existing.RecordType != "" {
			hard = p.problems
		}
		if len(hard) > 0 {
			refused = &docSaveOut{Saved: false, Problems: docProblems(hard)}
			return inputErrf("INVALID_DOCUMENT: %s (%d problem(s))", hard[0].Msg, len(hard))
		}
		structureOK := p.given && len(p.structureProblems()) == 0
		d := store.DocType{Code: p.doc.Code, Name: p.doc.Name, Category: p.doc.Category, Description: p.doc.Description,
			ApplicableFor: vocabFormats(p.doc.Formats), Active: true, CreatedBy: by, CreatedAt: at, UpdatedBy: by, UpdatedAt: at}
		if p.existing != nil {
			d.Active, d.RecordType, d.CreatedBy, d.CreatedAt = p.existing.Active, p.existing.RecordType, p.existing.CreatedBy, p.existing.CreatedAt
		}
		switch status {
		case "active":
			d.Active = true
		case "inactive":
			d.Active = false
		}
		structureWritten := ""
		if structureOK {
			cols := make([]store.OfficeColumn, len(p.rt.Columns))
			for i, c := range p.rt.Columns {
				cols[i] = store.OfficeColumn{Name: c.Name, Label: c.Label, Type: c.Type, Key: c.Key, Required: c.Required}
			}
			if d.RecordType == "" {
				if err := tx.AddOfficeRecordType(store.OfficeRecordType{Name: p.rt.Name, Description: p.rt.Description, CopiedFrom: p.copiedFrom,
					CreatedBy: by, CreatedAt: at, Columns: cols}); err != nil {
					return err
				}
				structureWritten = auditTypeAdd
			} else if cur, _, err := tx.OfficeRecordTypeByName(d.RecordType); err != nil {
				return err
			} else if !sameStructure(cur, p.rt) {
				if err := tx.ReplaceOfficeColumns(d.RecordType, p.rt.Description, cols); err != nil {
					return err
				}
				structureWritten = auditTypeEdit
			}
			d.RecordType = p.rt.Name
		}
		if *edit {
			err = tx.UpdateDocType(d)
		} else {
			err = tx.AddDocType(d)
		}
		if err != nil {
			return err
		}
		action := auditDocAdd
		if *edit {
			action = auditDocEdit
		}
		if err := tx.Audit(at, by, action, "document", d.Code, map[string]string{"name": d.Name, "category": d.Category,
			"formats": strings.Join(d.ApplicableFor, ","), "record_type": d.RecordType, "status": docStatus(d)}); err != nil {
			return err
		}
		if structureWritten != "" {
			if err := tx.Audit(at, by, structureWritten, "record_type", d.RecordType, map[string]string{"document": d.Code,
				"columns": fmt.Sprint(len(p.rt.Columns)), "copied_from": p.copiedFrom}); err != nil {
				return err
			}
		}
		detail, err := docDetail(tx, base, d.Code)
		if err != nil {
			return err
		}
		out = docSaveOut{Saved: true, Problems: []docProblemJSON{}, Document: &detail}
		if p.given && !structureOK {
			out.Problems = docProblems(p.structureProblems())
		}
		return nil
	})
	if refused != nil {
		if werr := writeJSONFile(*jsonOut, refused); werr != nil {
			return werr
		}
	}
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "document-save: %s (%s)\n", out.Document.Code, out.Document.Status)
	return err
}

// vocabFormats keeps the known formats once each, in the fixed order.
func vocabFormats(in []string) []string {
	out := []string{}
	for _, f := range vocab.OfficeFormats {
		if slices.Contains(in, f) {
			out = append(out, f)
		}
	}
	return out
}

// sameStructure says whether a stored record type already has this description and these columns.
func sameStructure(cur store.OfficeRecordType, rt vocab.OfficeRecordType) bool {
	if cur.Description != rt.Description || len(cur.Columns) != len(rt.Columns) {
		return false
	}
	for i, c := range cur.Columns {
		n := rt.Columns[i]
		if c.Name != n.Name || c.Label != n.Label || c.Type != n.Type || c.Key != n.Key || c.Required != n.Required {
			return false
		}
	}
	return true
}

// ---------- document-status ----------

type docStatusIn struct {
	By     string `json:"by"`
	Code   string `json:"code"`
	Active *bool  `json:"active"`
}

func documentStatus(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("document-status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the document as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	var in docStatusIn
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	if in.Active == nil || strings.TrimSpace(in.Code) == "" {
		return inputErrf("BAD_REQUEST: say which document (code) and whether it is active")
	}
	base, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var out docDetailOut
	err = withTx(*dbFile, func(tx *store.Tx) error {
		at := nowUTC()
		if err := seedBuiltins(tx, base, at); err != nil {
			return err
		}
		d, ok, err := tx.DocTypeByCode(in.Code)
		if err != nil {
			return err
		}
		if !ok {
			return inputErrf("NOT_FOUND: there is no document with the code %q", in.Code)
		}
		if *in.Active && d.Origin == store.OriginOffice && d.RecordType == "" {
			return inputErrf("NOT_READY: %q is a Draft; complete its record structure before activating it", in.Code)
		}
		if d.Active != *in.Active {
			// Deactivating is always allowed. The counts of what reads the document go into the
			// audit entry, so the warning the screen showed is on record.
			u, err := tx.DocUsage(in.Code)
			if err != nil {
				return err
			}
			if err := tx.SetDocActive(in.Code, *in.Active, by, at); err != nil {
				return err
			}
			action := auditDocDeactivate
			if *in.Active {
				action = auditDocActivate
			}
			if err := tx.Audit(at, by, action, "document", in.Code, map[string]string{"formats": fmt.Sprint(len(u.Formats)),
				"rules": fmt.Sprint(len(u.Rules)), "active_rules": fmt.Sprint(len(u.ActiveRules)), "records": fmt.Sprint(u.Records)}); err != nil {
				return err
			}
		}
		out, err = docDetail(tx, base, in.Code)
		return err
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "document-status: %s is %s\n", out.Code, out.Status)
	return err
}

// ---------- document-delete ----------

type docDeleteIn struct {
	By   string `json:"by"`
	Code string `json:"code"`
}

type docDeleteOut struct {
	Deleted bool         `json:"deleted"`
	Code    string       `json:"code"` // "IN_USE" or "BUILTIN" when refused
	Usage   docUsageJSON `json:"usage"`
}

func documentDelete(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("document-delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "write the result as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	var in docDeleteIn
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	by, err := trimmedName(in.By)
	if err != nil {
		return err
	}
	base, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	var out docDeleteOut
	var refusal error
	err = withTx(*dbFile, func(tx *store.Tx) error {
		at := nowUTC()
		if err := seedBuiltins(tx, base, at); err != nil {
			return err
		}
		d, ok, err := tx.DocTypeByCode(in.Code)
		if err != nil {
			return err
		}
		if !ok {
			return inputErrf("NOT_FOUND: there is no document with the code %q", in.Code)
		}
		u, err := tx.DocUsage(in.Code)
		if err != nil {
			return err
		}
		out = docDeleteOut{Usage: usageJSON(u)}
		switch {
		case d.Origin != store.OriginOffice:
			out.Code = "BUILTIN"
			refusal = inputErrf("BUILTIN: %q is a built-in document and cannot be deleted; deactivate it instead", in.Code)
			return refusal
		case u.InUse():
			out.Code = "IN_USE"
			refusal = inputErrf("IN_USE: %q is read by formats %v, %d stored rows and rules %v; deactivate it instead", in.Code, nonNil(u.Formats), u.Records, nonNil(u.Rules))
			return refusal
		}
		if err := tx.DeleteDocType(in.Code); err != nil {
			return err
		}
		if err := tx.Audit(at, by, auditDocDelete, "document", in.Code, map[string]string{"name": d.Name, "record_type": d.RecordType}); err != nil {
			return err
		}
		if d.RecordType != "" {
			if _, still, err := tx.OfficeRecordTypeByName(d.RecordType); err != nil {
				return err
			} else if !still {
				if err := tx.Audit(at, by, auditTypeDelete, "record_type", d.RecordType, map[string]string{"document": in.Code}); err != nil {
					return err
				}
			}
		}
		out.Deleted = true
		return nil
	})
	if refusal != nil {
		if werr := writeJSONFile(*jsonOut, out); werr != nil {
			return werr
		}
	}
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "document-delete: %s deleted\n", in.Code)
	return err
}

// A masterError is a failure to read the Document Master from the database. It is not a problem in
// the vocabulary files, so it is not reported as one: a database that cannot be opened is an
// internal error (exit 3), as it was before the Master was read here.
type masterError struct{ err error }

func (e *masterError) Error() string { return e.err.Error() }
func (e *masterError) Unwrap() error { return e.err }

// vocabularyErr turns what loadVocab returned into the command's error: a problem in the
// vocabulary (or in the office documents' names) is an input error; a database failure is passed
// on as it is.
func vocabularyErr(err error) error {
	var me *masterError
	if errors.As(err, &me) {
		return err
	}
	return inputErrf("vocabulary: %v", err)
}

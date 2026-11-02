package export

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// The Excel export (SPEC-013 §4, AC-7a, AC-7b). It holds the figures of one client and one tax
// year as the engine reported them: one sheet per review document with its findings, a
// cannot-check sheet and a Worklist sheet. Every sheet starts with a watermark line (firm, user,
// time and export ID). A finding marked deleted is left out.

// Watermark is written at the top of every sheet.
type Watermark struct {
	Firm     string
	User     string
	At       time.Time
	ExportID string
}

// NewID returns a new export ID: 16 random hexadecimal characters. The server makes it, so it
// can name the export in the watermark and in the audit entry.
func NewID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("export: id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Source is a client's figures for one tax year, read from the engine's result and Worklist.
type Source struct {
	Client      string
	TaxYear     string
	Groups      []Group           // the findings, by the review document that needs fixing
	CannotCheck []CannotCheck     // the rules that could not be checked, and what is missing
	Worklist    []WorklistItem    // the Worklist items of the client
	States      map[string]string // finding ID -> the decision's state in words; absent means Open
}

// Group is the findings of one review document.
type Group struct {
	ReviewDocument string    `json:"review_document"`
	Heading        string    `json:"heading"`
	Findings       []Finding `json:"findings"`
}

// Finding is one finding as the engine reports it.
type Finding struct {
	FindingID       string    `json:"finding_id"`
	RuleID          string    `json:"rule_id"`
	Title           string    `json:"title"`
	CodeText        string    `json:"code_text"`
	Subject         Subject   `json:"subject"`
	Values          []Labeled `json:"values"`
	DifferencePaise int64     `json:"difference_paise"`
	ShowDifference  bool      `json:"show_difference"`
	LawRefs         []LawRef  `json:"law_refs"`
}

// Subject is the period, scope and match key a finding is about.
type Subject struct {
	Period   string `json:"period"`
	Scope    string `json:"scope"`
	MatchKey string `json:"match_key"`
}

// Labeled is one figure of a finding, with the place it was read from.
type Labeled struct {
	Label   string  `json:"label"`
	Value   Value   `json:"value"`
	Locator Locator `json:"locator"`
}

// Value is one figure: money in paise, or an integer, text, date, flag or rate.
type Value struct {
	Kind   string  `json:"kind"`
	Paise  *int64  `json:"paise"`
	Int    *int64  `json:"int"`
	Text   *string `json:"text"`
	Date   *string `json:"date"`
	Bool   *bool   `json:"bool"`
	RateBP *int64  `json:"rate_bp"`
}

// Locator is the file and the place in it a figure was read from.
type Locator struct {
	FileName string `json:"file_name"`
	Path     string `json:"path"`
}

// LawRef is a law reference of a finding.
type LawRef struct {
	Act       string `json:"act"`
	Section   string `json:"section"`
	Reference string `json:"reference"`
}

// CannotCheck is a rule that could not be checked, with the figures it lacks.
type CannotCheck struct {
	RuleID   string    `json:"rule_id"`
	Title    string    `json:"title"`
	Category string    `json:"category"`
	Subject  Subject   `json:"subject"`
	Missing  []Missing `json:"missing"`
}

// Missing is one figure a rule lacks, and the document it should come from.
type Missing struct {
	Fact         string `json:"fact"`
	Label        string `json:"label"`
	DocumentType string `json:"document_type"`
	Document     string `json:"document"`
	Rows         bool   `json:"rows"`
}

// WorklistItem is a ledger head that is new, vanished or changed (SPEC-006 part 3).
type WorklistItem struct {
	ClientID       int64                   `json:"client_id"`
	Key            string                  `json:"key"`
	KindText       string                  `json:"kind_text"`
	Ledger         string                  `json:"ledger"`
	Group          string                  `json:"group"`
	Text           string                  `json:"text"`
	PriorTotal     int64                   `json:"prior_total"`
	CurrentTotal   int64                   `json:"current_total"`
	MissingMonths  []string                `json:"missing_months"`
	Classification *WorklistClassification `json:"classification"`
}

// WorklistClassification is what staff said an item is.
type WorklistClassification struct {
	Text string `json:"text"`
	By   string `json:"by"`
	At   string `json:"at"`
}

// Counts are the rows an export holds, for the audit entry.
type Counts struct {
	Sheets      int
	Findings    int
	CannotCheck int
	Worklist    int
}

// stateWords names a decision's state for the State column.
var stateWords = map[string]string{
	"fix":          "Fix",
	"not_an_issue": "Not an issue",
	"ask_client":   "Ask client",
	"open":         "Open again (figures changed)",
}

// report is the part of a run's report the export reads.
type report struct {
	Findings    []Group       `json:"findings"`
	CannotCheck []CannotCheck `json:"cannot_check"`
}

// ReadSource reads the engine's result of a client (recon-engine result --json) and its
// Worklist (recon-engine worklist --json), and keeps the Worklist items of clientID only. A
// finding or a cannot-check item that several runs of the year report is kept once.
func ReadSource(clientID int64, result, worklist []byte) (Source, error) {
	var res struct {
		Client struct {
			Name string `json:"name"`
		} `json:"client"`
		TaxYear string  `json:"tax_year"`
		Report  *report `json:"report"`
		Results []struct {
			Report *report `json:"report"`
		} `json:"results"`
		Decisions map[string]struct {
			State string `json:"state"`
		} `json:"decisions"`
	}
	if err := json.Unmarshal(result, &res); err != nil {
		return Source{}, fmt.Errorf("export: the client's result: %w", err)
	}
	reports := []*report{res.Report}
	for _, r := range res.Results {
		reports = append(reports, r.Report)
	}
	src := Source{Client: res.Client.Name, TaxYear: res.TaxYear, States: map[string]string{}}
	groupAt := map[string]int{}
	seenFinding := map[string]bool{}
	seenCannot := map[string]bool{}
	for _, r := range reports {
		if r == nil {
			continue
		}
		for _, g := range r.Findings {
			i, ok := groupAt[g.ReviewDocument]
			if !ok {
				i = len(src.Groups)
				groupAt[g.ReviewDocument] = i
				src.Groups = append(src.Groups, Group{ReviewDocument: g.ReviewDocument, Heading: g.Heading})
			}
			for _, f := range g.Findings {
				if seenFinding[f.FindingID] {
					continue
				}
				seenFinding[f.FindingID] = true
				src.Groups[i].Findings = append(src.Groups[i].Findings, f)
			}
		}
		for _, c := range r.CannotCheck {
			key := c.RuleID + "|" + c.Subject.Period + "|" + c.Subject.Scope + "|" + c.Subject.MatchKey
			if seenCannot[key] {
				continue
			}
			seenCannot[key] = true
			src.CannotCheck = append(src.CannotCheck, c)
		}
	}
	for id, d := range res.Decisions {
		if w, ok := stateWords[d.State]; ok {
			src.States[id] = w
		}
	}
	var list struct {
		Items []WorklistItem `json:"items"`
	}
	if err := json.Unmarshal(worklist, &list); err != nil {
		return Source{}, fmt.Errorf("export: the Worklist: %w", err)
	}
	for _, it := range list.Items {
		if it.ClientID == clientID {
			src.Worklist = append(src.Worklist, it)
		}
	}
	return src, nil
}

// sheet is one sheet of the workbook before it is written.
type sheet struct {
	name   string
	header []string
	rows   [][]any
}

var (
	findingHeader = []string{"Finding ID", "Rule", "Title", "State", "Period", "Scope", "Match key", "Result",
		"Figure", "Amount (paise)", "Figure as text", "File", "Place in file", "Difference (paise)", "Law reference"}
	cannotHeader   = []string{"Rule", "Title", "Category", "Period", "Scope", "Match key", "Missing figure", "Document", "Document type", "Rows"}
	worklistHeader = []string{"Client", "Kind", "Ledger", "Group", "What was counted", "Prior total (paise)",
		"Current total (paise)", "Missing months", "Classification", "Classified by", "Classified at"}
)

// Workbook builds the Excel file of a client's figures. deleted holds the IDs of findings
// marked deleted, which are left out. It returns the file and the rows it holds.
func Workbook(w Watermark, src Source, deleted map[string]bool) ([]byte, Counts, error) {
	var c Counts
	var sheets []sheet
	used := map[string]bool{}
	for _, g := range src.Groups {
		sh := sheet{name: sheetName(g.Heading, used), header: findingHeader}
		for _, f := range g.Findings {
			if deleted[f.FindingID] {
				continue
			}
			c.Findings++
			rows, err := findingRows(f, src.States[f.FindingID])
			if err != nil {
				return nil, Counts{}, err
			}
			sh.rows = append(sh.rows, rows...)
		}
		sheets = append(sheets, sh)
	}
	cannot := sheet{name: sheetName("Cannot check", used), header: cannotHeader}
	for _, cc := range src.CannotCheck {
		c.CannotCheck++
		if len(cc.Missing) == 0 {
			cannot.rows = append(cannot.rows, []any{cc.RuleID, cc.Title, cc.Category, cc.Subject.Period, cc.Subject.Scope, cc.Subject.MatchKey, "", "", "", ""})
			continue
		}
		for _, m := range cc.Missing {
			rows := ""
			if m.Rows {
				rows = "rows"
			}
			cannot.rows = append(cannot.rows, []any{cc.RuleID, cc.Title, cc.Category, cc.Subject.Period, cc.Subject.Scope, cc.Subject.MatchKey,
				m.Label, m.Document, m.DocumentType, rows})
		}
	}
	work := sheet{name: sheetName("Worklist", used), header: worklistHeader}
	for _, it := range src.Worklist {
		c.Worklist++
		cls, at, by := "", "", ""
		if it.Classification != nil {
			cls, at, by = it.Classification.Text, it.Classification.At, it.Classification.By
		}
		work.rows = append(work.rows, []any{src.Client, it.KindText, it.Ledger, it.Group, it.Text,
			it.PriorTotal, it.CurrentTotal, strings.Join(it.MissingMonths, ", "), cls, by, at})
	}
	sheets = append(sheets, cannot, work)
	c.Sheets = len(sheets)
	file, err := write(w, src, sheets)
	if err != nil {
		return nil, Counts{}, err
	}
	return file, c, nil
}

// findingRows gives one row per figure of a finding; a finding with no figure gives one row.
func findingRows(f Finding, state string) ([][]any, error) {
	if state == "" {
		state = "Open"
	}
	var laws []string
	for _, l := range f.LawRefs {
		laws = append(laws, strings.TrimSpace(l.Act+" "+l.Section+" "+l.Reference))
	}
	law := strings.Join(laws, "; ")
	var diff any = ""
	if f.ShowDifference {
		diff = f.DifferencePaise
	}
	base := func() []any {
		return []any{f.FindingID, f.RuleID, f.Title, state, f.Subject.Period, f.Subject.Scope, f.Subject.MatchKey, f.CodeText}
	}
	if len(f.Values) == 0 {
		return [][]any{append(base(), "", "", "", "", "", diff, law)}, nil
	}
	var rows [][]any
	for _, v := range f.Values {
		text, err := v.Value.text()
		if err != nil {
			return nil, fmt.Errorf("export: finding %s: %w", f.FindingID, err)
		}
		var paise any = ""
		if v.Value.Kind == "money" && v.Value.Paise != nil {
			paise = *v.Value.Paise
		}
		rows = append(rows, append(base(), v.Label, paise, text, v.Locator.FileName, v.Locator.Path, diff, law))
	}
	return rows, nil
}

// text shows a figure as the sheet writes it: money with Indian digit grouping, and a rate
// in per cent to two places (basis points, so no floating point).
func (v Value) text() (string, error) {
	switch v.Kind {
	case "money":
		if v.Paise == nil {
			return "", fmt.Errorf("a money figure has no paise")
		}
		return money.FormatIndian(money.Paise(*v.Paise), true)
	case "int":
		if v.Int != nil {
			return strconv.FormatInt(*v.Int, 10), nil
		}
	case "text":
		if v.Text != nil {
			return *v.Text, nil
		}
	case "date":
		if v.Date != nil {
			return *v.Date, nil
		}
	case "bool":
		if v.Bool != nil {
			return strconv.FormatBool(*v.Bool), nil
		}
	case "rate":
		if v.RateBP != nil {
			return fmt.Sprintf("%d.%02d%%", *v.RateBP/100, *v.RateBP%100), nil
		}
	}
	return "", nil
}

// sheetName turns a heading into a sheet name: at most 31 characters, none of the characters
// Excel forbids, and not one already used (case ignored).
func sheetName(heading string, used map[string]bool) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return -1
		}
		return r
	}, heading)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		clean = "Review document"
	}
	name := truncate(clean, 31)
	for i := 2; used[strings.ToLower(name)]; i++ {
		suffix := fmt.Sprintf(" %d", i)
		name = truncate(clean, 31-len(suffix)) + suffix
	}
	used[strings.ToLower(name)] = true
	return name
}

// truncate shortens s to at most n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// watermarkText is the line every sheet starts with.
func watermarkText(w Watermark) string {
	firm := w.Firm
	if firm == "" {
		firm = "(firm name not set)"
	}
	return fmt.Sprintf("Watermark: firm %s | exported by %s | %s | export ID %s",
		firm, w.User, w.At.UTC().Format(time.RFC3339), w.ExportID)
}

// write lays the sheets out and returns the file. Each sheet has the watermark on its first
// line, the client and tax year on its second, and its header on its fourth.
func write(w Watermark, src Source, sheets []sheet) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	for i, sh := range sheets {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", sh.name); err != nil {
				return nil, fmt.Errorf("export: sheet: %w", err)
			}
		} else if _, err := f.NewSheet(sh.name); err != nil {
			return nil, fmt.Errorf("export: sheet: %w", err)
		}
		put := func(col, row int, v any) error {
			cell, err := excelize.CoordinatesToCellName(col, row)
			if err != nil {
				return err
			}
			return f.SetCellValue(sh.name, cell, v)
		}
		if err := put(1, 1, watermarkText(w)); err != nil {
			return nil, err
		}
		if err := put(1, 2, fmt.Sprintf("Client: %s | Tax year: %s", src.Client, src.TaxYear)); err != nil {
			return nil, err
		}
		for j, h := range sh.header {
			if err := put(j+1, 4, h); err != nil {
				return nil, err
			}
		}
		for i, row := range sh.rows {
			for j, v := range row {
				if err := put(j+1, 5+i, v); err != nil {
					return nil, err
				}
			}
		}
		if err := f.SetColWidth(sh.name, "A", "O", 18); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("export: writing the file: %w", err)
	}
	return buf.Bytes(), nil
}

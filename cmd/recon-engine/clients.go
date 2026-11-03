package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/client"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// The client commands (SPEC-004 part 7). The engine is the database's single writer, so
// recon-server adds clients and reads their results through these commands.

// clientJSON is a client as the commands list it. The full PAN is never in it.
type clientJSON struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	PANLast4  string          `json:"pan_last4"`
	LastRunOn string          `json:"last_run_on"` // "" when never run
	Documents int             `json:"documents"`   // documents in the current view
	Counts    json.RawMessage `json:"counts"`      // the latest run's counts, or null
	// "active" or "disabled" (SPEC-004 part 10); a disabled client also says when, by whom and why.
	Status         string `json:"status"`
	DisabledOn     string `json:"disabled_on,omitempty"` // YYYY-MM-DD, UTC
	DisabledBy     string `json:"disabled_by,omitempty"`
	DisabledReason string `json:"disabled_reason,omitempty"`
}

// clientAdd adds a client. Its details come on standard input, never on the command line,
// where other programs on the computer could see the PAN: either a JSON object with every
// detail (client.Details), or a single line with the PAN alone, the name then coming from
// --name.
//
//	recon-engine client-add --db <file> --tax-year 2025-26 [--json <file>] < details.json
//	echo AAACI1234Q | recon-engine client-add --db <file> --name "..." --tax-year 2025-26
func clientAdd(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("client-add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	name := fs.String("name", "", "the client's name, when standard input holds the PAN alone")
	taxYear := fs.String("tax-year", "", "the tax year to open for the client, such as 2025-26")
	jsonOut := fs.String("json", "", "also write the new client as JSON to this file")
	if err := parseFlags(fs, args, "db", "tax-year"); err != nil {
		return err
	}
	ty, err := period.ParseTaxYear(*taxYear)
	if err != nil {
		return inputErrf("--tax-year: %v", err)
	}
	d, err := readDetails(stdin, *name)
	if err != nil {
		return err
	}
	if problems := d.Validate(d.PAN, true); len(problems) > 0 {
		return invalidClient(problems)
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
	c, err := tx.AddClientDetails(d)
	var ce *store.ConflictError
	if errors.As(err, &ce) {
		return inputErrf("CLIENT_EXISTS: %v", err)
	}
	if err != nil {
		return err
	}
	if _, err := tx.ClientEngagement(c.ID, ty.String()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"client": clientJSON{ID: c.ID, Name: c.Name, PANLast4: c.PANLast4, Counts: json.RawMessage("null"), Status: "active"}}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "client %d added: %s (PAN ending %s), tax year %s\n", c.ID, c.Name, c.PANLast4, ty)
	return err
}

// readDetails reads a client's details: a JSON object, or one line with the PAN alone.
func readDetails(r io.Reader, name string) (client.Details, error) {
	data, err := io.ReadAll(io.LimitReader(r, 64<<10))
	if err != nil {
		return client.Details{}, err
	}
	var d client.Details
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '{' {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d); err != nil {
			return d, inputErrf("BAD_REQUEST: the client details are not valid JSON: %v", err)
		}
	} else {
		line, _, _ := strings.Cut(string(trimmed), "\n")
		d = client.Details{LegalName: name, PAN: line}
	}
	d.Normalise()
	d.PANLast4 = ""
	return d, nil
}

func invalidClient(problems []client.Problem) error {
	parts := make([]string, len(problems))
	for i, p := range problems {
		parts[i] = p.String()
	}
	return inputErrf("INVALID_CLIENT: %s", strings.Join(parts, "; "))
}

// clientUpdate replaces a client's details, read from standard input as JSON. An empty
// "pan" keeps the PAN; every GSTIN must contain the client's PAN (checked against its hash).
//
//	recon-engine client-update --db <file> --client <id> [--json <file>] < details.json
func clientUpdate(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("client-update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	jsonOut := fs.String("json", "", "also write the client's details as JSON to this file")
	if err := parseFlags(fs, args, "db"); err != nil {
		return err
	}
	d, err := readDetails(stdin, "")
	if err != nil {
		return err
	}
	if problems := d.Validate(d.PAN, false); len(problems) > 0 {
		return invalidClient(problems)
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
	existing, found, err := tx.ClientByID(*clientID)
	if err != nil {
		return err
	}
	if !found {
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
	}
	if existing.Disabled {
		return clientDisabled(existing)
	}
	if d.PAN == "" {
		var problems []client.Problem
		for i, g := range d.GSTINs {
			ok, err := tx.GSTINPANMatches(g.GSTIN, existing.PANHash)
			if err != nil {
				return err
			}
			if !ok {
				problems = append(problems, client.Problem{Field: fmt.Sprintf("gstins[%d]", i), Code: "GSTIN_PAN_MISMATCH",
					Message: fmt.Sprintf("%s does not contain the client's PAN (ending %s)", g.GSTIN, existing.PANLast4)})
			}
		}
		if len(problems) > 0 {
			return invalidClient(problems)
		}
	}
	c, err := tx.UpdateClient(*clientID, d)
	var ce *store.ConflictError
	if errors.As(err, &ce) {
		return inputErrf("CLIENT_EXISTS: %v", err)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if *jsonOut != "" {
		if err := writeDetailsJSON(db, c.ID, *jsonOut); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "client %d updated: %s (PAN ending %s)\n", c.ID, c.Name, c.PANLast4)
	return err
}

// clientDisabled is the refusal of a change to a disabled client (SPEC-004 part 10).
func clientDisabled(c store.Client) error {
	return inputErrf("CLIENT_DISABLED: client %d is disabled; a Partner can enable it again", c.ID)
}

// clientDisable takes a client out of use (SPEC-004 part 10). Nothing is deleted: the client
// keeps its details, documents, runs and decisions, and leaves the Season board and the
// Worklist. The name and the reason come on standard input; the reason is required.
//
//	echo '{"by":"A. Partner","reason":"..."}' | recon-engine client-disable --db <file> --client <id> [--json <file>]
func clientDisable(args []string, stdin io.Reader, stdout io.Writer) error {
	return clientSetDisabled("client-disable", args, stdin, stdout, true)
}

// clientEnable takes a disabled client back into use, exactly as it was. The reason is required.
func clientEnable(args []string, stdin io.Reader, stdout io.Writer) error {
	return clientSetDisabled("client-enable", args, stdin, stdout, false)
}

func clientSetDisabled(name string, args []string, stdin io.Reader, stdout io.Writer, disable bool) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	jsonOut := fs.String("json", "", "write the client's new status as JSON to this file")
	if err := parseFlags(fs, args, "db", "client"); err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	var in struct {
		By     string `json:"by"`
		Reason string `json:"reason"`
	}
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
	switch {
	case reason == "" && disable:
		return inputErrf("REASON_REQUIRED: say why the client is disabled")
	case reason == "":
		return inputErrf("REASON_REQUIRED: say why the client is enabled again")
	case len([]rune(reason)) > 300:
		return inputErrf("BAD_REASON: the reason has at most 300 characters")
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
	c, found, err := tx.ClientByID(*clientID)
	if err != nil {
		return err
	}
	switch {
	case !found:
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
	case disable && c.Disabled:
		return inputErrf("ALREADY_DISABLED: client %d is already disabled", c.ID)
	case !disable && !c.Disabled:
		return inputErrf("NOT_DISABLED: client %d is not disabled", c.ID)
	}
	at := nowUTC()
	if err := tx.SetClientDisabled(c.ID, disable, by, at, reason); err != nil {
		return err
	}
	action, status := "client.enable", "active"
	if disable {
		action, status = "client.disable", "disabled"
	}
	if err := tx.Audit(at, by, action, "client", strconv.FormatInt(c.ID, 10), map[string]string{"reason": reason}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"client_id": c.ID, "status": status, "reason": reason}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "client %d is now %s\n", c.ID, status)
	return err
}

// clientGet writes all of a client's details (never the full PAN). It only reads.
//
//	recon-engine client-get --db <file> --client <id> --json <file>
func clientGet(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("client-get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	jsonOut := fs.String("json", "", "write the client's details as JSON to this file")
	if err := parseFlags(fs, args, "db", "json"); err != nil {
		return err
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	db, err := store.Open(*dbFile)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := writeDetailsJSON(db, *clientID, *jsonOut); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "client %d details written to %s\n", *clientID, *jsonOut)
	return err
}

func writeDetailsJSON(db *store.DB, id int64, file string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // read only
	d, found, err := tx.ClientDetails(id)
	if err != nil {
		return err
	}
	if !found {
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", id)
	}
	d.PAN = ""
	return writeJSONFile(file, map[string]any{"id": id, "details": d})
}

// clientsList lists the clients with, for the tax year, the date and counts of each one's
// latest run. It only reads.
//
//	recon-engine clients --db <file> --tax-year 2025-26 [--json <file>]
func clientsList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("clients", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	jsonOut := fs.String("json", "", "also write the list as JSON to this file")
	all := fs.Bool("all", false, "also list disabled clients")
	if err := parseFlags(fs, args, "db", "tax-year"); err != nil {
		return err
	}
	ty, err := period.ParseTaxYear(*taxYear)
	if err != nil {
		return inputErrf("--tax-year: %v", err)
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
	defer tx.Rollback() // read only
	listClients := tx.Clients
	if *all {
		listClients = tx.AllClients
	}
	cs, err := listClients()
	if err != nil {
		return err
	}
	list := make([]clientJSON, 0, len(cs))
	var b strings.Builder
	for _, c := range cs {
		cj, err := clientSummary(tx, c, ty.String())
		if err != nil {
			return err
		}
		list = append(list, cj)
		mark := ""
		if c.Disabled {
			mark = "  (disabled)"
		}
		fmt.Fprintf(&b, "%4d  %-40s  PAN ending %s  last run %s  %d documents%s\n", c.ID, c.Name, c.PANLast4, orDash(cj.LastRunOn), cj.Documents, mark)
	}
	fmt.Fprintf(&b, "%d clients\n", len(list))
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"tax_year": ty.String(), "clients": list}); err != nil {
			return err
		}
	}
	_, err = io.WriteString(stdout, b.String())
	return err
}

func clientSummary(tx *store.Tx, c store.Client, taxYear string) (clientJSON, error) {
	cj := clientJSON{ID: c.ID, Name: c.Name, PANLast4: c.PANLast4, Counts: json.RawMessage("null"), Status: "active"}
	if c.Disabled {
		cj.Status, cj.DisabledBy, cj.DisabledReason = "disabled", c.StatusBy, c.StatusReason
		cj.DisabledOn, _, _ = strings.Cut(c.StatusAt, "T")
	}
	eng, found, err := tx.FindClientEngagement(c.ID, taxYear)
	if err != nil || !found {
		return cj, err
	}
	docs, _, err := tx.CurrentView(eng)
	if err != nil {
		return cj, err
	}
	cj.Documents = len(docs)
	runs, err := tx.LatestRuns(eng)
	if err != nil {
		return cj, err
	}
	// The board shows the sum over the latest result of each type of check (SPEC-004 part 9).
	sum := map[string]int{}
	for _, run := range runs {
		if len(run.Report) == 0 {
			continue
		}
		var rep struct {
			Counts map[string]int `json:"counts"`
		}
		if err := json.Unmarshal(run.Report, &rep); err != nil {
			return cj, fmt.Errorf("client %d: stored report: %w", c.ID, err)
		}
		for k, n := range rep.Counts {
			sum[k] += n
		}
		cj.LastRunOn = max(cj.LastRunOn, run.FinishedOn)
	}
	if len(sum) > 0 {
		if cj.Counts, err = json.Marshal(sum); err != nil {
			return cj, err
		}
	}
	return cj, nil
}

// typeResult is the latest result of one type of check for a client.
type typeResult struct {
	ComparisonID string          `json:"comparison_id"` // "" for a run of every rule
	LastRunOn    string          `json:"last_run_on"`
	Report       json.RawMessage `json:"report"`
}

// clientResult writes a client's latest report for the tax year and the figures of its
// current view. It only reads.
//
//	recon-engine result --db <file> --client <id> --tax-year 2025-26 --json <file>
func clientResult(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("result", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	jsonOut := fs.String("json", "", "write the result as JSON to this file")
	if err := parseFlags(fs, args, "db", "tax-year", "json"); err != nil {
		return err
	}
	ty, err := period.ParseTaxYear(*taxYear)
	if err != nil {
		return inputErrf("--tax-year: %v", err)
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
	defer tx.Rollback() // read only
	c, found, err := tx.ClientByID(*clientID)
	if err != nil {
		return err
	}
	if !found {
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
	}
	cj, err := clientSummary(tx, c, ty.String())
	if err != nil {
		return err
	}
	report := json.RawMessage("null")
	results := []typeResult{}
	facts := []byte(`{"engagements":[]}`)
	if eng, found, err := tx.FindClientEngagement(c.ID, ty.String()); err != nil {
		return err
	} else if found {
		if run, ok, err := tx.LatestRun(eng); err != nil {
			return err
		} else if ok && len(run.Report) > 0 {
			report = json.RawMessage(bytes.TrimSpace(run.Report))
		}
		runs, err := tx.LatestRuns(eng)
		if err != nil {
			return err
		}
		for _, run := range runs {
			if len(run.Report) > 0 {
				results = append(results, typeResult{ComparisonID: run.Comparison, LastRunOn: run.FinishedOn, Report: json.RawMessage(bytes.TrimSpace(run.Report))})
			}
		}
		list, err := factList(tx, []store.EngagementRef{{ID: eng, TaxYear: ty.String()}})
		if err != nil {
			return err
		}
		if facts, err = list.JSON(); err != nil {
			return err
		}
	}
	// The tax years with a run, so the screen can offer them and open on the latest.
	years, err := tx.ClientRunTaxYears(c.ID)
	if err != nil {
		return err
	}
	decisions := map[string]any{}
	if eng, found, err := tx.FindClientEngagement(c.ID, ty.String()); err != nil {
		return err
	} else if found {
		var reports [][]byte
		for _, r := range results {
			reports = append(reports, r.Report)
		}
		if decisions, err = decisionsOf(tx, eng, reports); err != nil {
			return err
		}
	}
	if err := writeJSONFile(*jsonOut, map[string]any{"client": cj, "tax_year": ty.String(), "run_tax_years": years, "report": report,
		"results": results, "decisions": decisions, "facts": json.RawMessage(bytes.TrimSpace(facts))}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "client %d: %s, last run %s\n", c.ID, c.Name, orDash(cj.LastRunOn))
	return err
}

func writeJSONFile(path string, v any) error {
	if st, err := os.Stat(filepath.Dir(path)); err != nil || !st.IsDir() {
		return inputErrf("--json: the folder of %s does not exist", path)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

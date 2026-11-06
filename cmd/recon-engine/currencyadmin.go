package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/currency"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// The commands of the Admin tab's Currency page (SPEC-021): read the dollar rate with its history,
// and set a new one. Setting writes usd-rate.yaml in the currency folder (the file the run reads,
// SPEC-019) and one audit entry, `currency.rate_set`, in one transaction. The history is read from
// the audit log, so it cannot differ from what was done.

const (
	currencyAction = "currency.rate_set"
	maxNoteRunes   = 200
	historyLimit   = 50
)

// The rate must be a plausible number of rupees per dollar: this stops a slip such as 8325 or 0.83.
// It is a guard on typing, not a statement about the market.
var (
	minUSDRate = money.USDRate(1 * 10000)
	maxUSDRate = money.USDRate(1000 * 10000)
)

type currencyChange struct {
	By            string `json:"by"`
	USDRate       string `json:"usd_rate"`
	EffectiveFrom string `json:"effective_from"`
	Note          string `json:"note"`
}

type currencyHistoryOut struct {
	At            string `json:"at"`
	By            string `json:"by"`
	USDRate       string `json:"usd_rate"`
	Previous      string `json:"previous"` // "" when there was no rate before
	EffectiveFrom string `json:"effective_from"`
	Note          string `json:"note"`
}

type currencyOut struct {
	Set           bool                 `json:"set"`
	USDRate       string               `json:"usd_rate"` // four decimal places, "" when not set
	EffectiveFrom string               `json:"effective_from"`
	Note          string               `json:"note"`
	Problem       string               `json:"problem"` // "" when the rate is usable, else why it is not
	Detail        string               `json:"detail"`
	History       []currencyHistoryOut `json:"history"`
}

// currencyView reads the rate in force and the history.
func currencyView(dir string, entries func() (rows []currencyHistoryOut, err error)) (currencyOut, error) {
	r := currency.Load(dir)
	out := currencyOut{History: []currencyHistoryOut{}}
	if r.OK() {
		out.Set, out.USDRate = true, r.Value.String()
	} else {
		out.Problem, out.Detail = r.Problem, r.Detail
	}
	out.EffectiveFrom = r.Effective
	if r.OK() || r.Problem == currency.ReasonNotSet {
		out.Note = r.Note // the note as the loader decoded it, whatever way the file quotes it
	}
	rows, err := entries()
	if err != nil {
		return out, err
	}
	out.History = rows
	return out, nil
}

// currencyHistory lists the rate changes in the audit log, newest first.
func currencyHistory(tx *store.Tx) ([]currencyHistoryOut, error) {
	entries, err := tx.AuditEntries()
	if err != nil {
		return nil, err
	}
	rows := []currencyHistoryOut{}
	for i := len(entries) - 1; i >= 0 && len(rows) < historyLimit; i-- {
		e := entries[i]
		if e.Action != currencyAction {
			continue
		}
		var d map[string]string
		if err := json.Unmarshal([]byte(e.Details), &d); err != nil {
			continue
		}
		rows = append(rows, currencyHistoryOut{At: e.At, By: e.Actor, USDRate: d["usd_rate"], Previous: d["previous"], EffectiveFrom: d["effective_from"], Note: d["note"]})
	}
	return rows, nil
}

func currencyGet(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("currency-get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	dir := fs.String("currency", "", "folder holding usd-rate.yaml")
	jsonOut := fs.String("json", "", "write the rate and its history as JSON to this file")
	if err := parseFlags(fs, args, "db", "currency"); err != nil {
		return err
	}
	var view currencyOut
	err := withTx(*dbFile, func(tx *store.Tx) error {
		var err error
		view, err = currencyView(*dir, func() ([]currencyHistoryOut, error) { return currencyHistory(tx) })
		return err
	})
	if err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, view); err != nil {
		return err
	}
	if view.Set {
		fmt.Fprintf(stdout, "currency-get: %s rupees per dollar, effective %s\n", view.USDRate, view.EffectiveFrom)
	} else {
		fmt.Fprintln(stdout, "currency-get: no rate set")
	}
	return nil
}

// checkedCurrencyChange validates a change and returns the rate and the cleaned fields.
func checkedCurrencyChange(in currencyChange) (money.USDRate, string, string, string, error) {
	by, err := trimmedName(in.By)
	if err != nil {
		return 0, "", "", "", err
	}
	rate, err := money.ParseUSDRate(in.USDRate)
	if err != nil {
		return 0, "", "", "", inputErrf("BAD_RATE: usd_rate must be rupees per dollar written with digits and up to four decimal places, such as 83.2500")
	}
	if rate < minUSDRate || rate >= maxUSDRate {
		return 0, "", "", "", inputErrf("BAD_RATE: usd_rate must be at least 1 and below 1,000 rupees per dollar; check the figure")
	}
	date := strings.TrimSpace(in.EffectiveFrom)
	if _, err := period.ParseDate(date); err != nil {
		return 0, "", "", "", inputErrf("BAD_DATE: effective_from must be a real date written yyyy-mm-dd")
	}
	note := strings.TrimSpace(in.Note)
	if len([]rune(note)) > maxNoteRunes {
		return 0, "", "", "", inputErrf("BAD_NOTE: the note is at most %d characters", maxNoteRunes)
	}
	for _, r := range note {
		if !unicode.IsPrint(r) {
			return 0, "", "", "", inputErrf("BAD_NOTE: the note is one line of printable characters")
		}
	}
	return rate, by, date, note, nil
}

func currencySet(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("currency-set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	dir := fs.String("currency", "", "folder holding usd-rate.yaml")
	jsonOut := fs.String("json", "", "write the new state as JSON to this file")
	if err := parseFlags(fs, args, "db", "currency"); err != nil {
		return err
	}
	var in currencyChange
	if err := readChange(stdin, &in); err != nil {
		return err
	}
	rate, by, date, note, err := checkedCurrencyChange(in)
	if err != nil {
		return err
	}
	// The file is written before the change is committed, and a file cannot be rolled back with the database,
	// so what it held is kept to put back if the change does not complete: a rate in the file with no audit
	// entry for it would be a rate nobody can account for.
	ratePath := filepath.Join(*dir, currency.FileName)
	previous, readErr := os.ReadFile(ratePath)
	had := readErr == nil
	if readErr != nil && !errors.Is(readErr, iofs.ErrNotExist) {
		return fmt.Errorf("currency file %s cannot be read, so it was not changed: %w", ratePath, readErr)
	}
	written := false
	var view currencyOut
	err = withTx(*dbFile, func(tx *store.Tx) error {
		prev := ""
		if old := currency.Load(*dir); old.OK() {
			prev = old.Value.String()
		}
		if err := tx.Audit(nowUTC(), by, currencyAction, "currency", "usd", map[string]string{
			"usd_rate": rate.String(), "previous": prev, "effective_from": date, "note": note,
		}); err != nil {
			return err
		}
		if err := writeRateFile(*dir, rate, date, note); err != nil {
			return err
		}
		written = true
		if err := afterRateWrite(); err != nil {
			return err
		}
		var verr error
		view, verr = currencyView(*dir, func() ([]currencyHistoryOut, error) { return currencyHistory(tx) })
		return verr
	})
	if err != nil {
		if written {
			if rerr := restoreRateFile(*dir, previous, had); rerr != nil {
				return fmt.Errorf("%w; and the earlier rate file could not be put back (%v): check %s", err, rerr, ratePath)
			}
		}
		return err
	}
	if err := writeJSONFile(*jsonOut, view); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "currency-set: %s rupees per dollar, effective %s\n", rate, date)
	return nil
}

// writeRateFile writes usd-rate.yaml whole: to a new file beside it, then renamed over it, so a
// run never reads half a file. Every text value is in double quotes, as the file's rules say.
func writeRateFile(dir string, rate money.USDRate, date, note string) error {
	body := "# The fixed rate for amounts written in US dollars (SPEC-019). Written by the Admin tab's\n" +
		"# Currency page (SPEC-021); the history of changes is in the audit log.\n" +
		"schema: " + strconv.Quote(currency.Schema) + "\n" +
		"usd_rate: " + strconv.Quote(rate.String()) + "\n" +
		"effective_from: " + strconv.Quote(date) + "\n" +
		"note: " + strconv.Quote(note) + "\n"
	return replaceRateFile(dir, []byte(body))
}

// replaceRateFile puts body in place of usd-rate.yaml in one step: it is written to a new file beside
// it and renamed over it.
func replaceRateFile(dir string, body []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("currency folder: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".usd-rate-*.tmp")
	if err != nil {
		return fmt.Errorf("currency file: %w", err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(name)
		return fmt.Errorf("currency file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("currency file: %w", err)
	}
	if err := os.Rename(name, filepath.Join(dir, currency.FileName)); err != nil {
		os.Remove(name)
		return fmt.Errorf("currency file: %w", err)
	}
	return nil
}

// restoreRateFile puts usd-rate.yaml back as it was before a change that did not complete: the earlier
// text when there was a file, no file when there was none.
func restoreRateFile(dir string, previous []byte, had bool) error {
	if had {
		return replaceRateFile(dir, previous)
	}
	if err := os.Remove(filepath.Join(dir, currency.FileName)); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return fmt.Errorf("currency file: %w", err)
	}
	return nil
}

// afterRateWrite is called once the new rate file is in place and before the change is committed. It
// does nothing; a test sets it to fail, to prove that a change that does not complete leaves the old file.
var afterRateWrite = func() error { return nil }

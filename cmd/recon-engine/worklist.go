package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/modules/newheads"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// The Worklist (SPEC-006 part 3): the new, vanished and changed ledger heads of each
// client's latest run with both years' ledgers, and the classification staff gave each.
// The tool never classifies (CLAUDE.md invariant 2).

// worklistItems returns the items of a client's latest Worklist, each with its current
// classification ("classification": null when there is none), in the run's order.
func worklistItems(tx *store.Tx, eng int64) ([]map[string]any, error) {
	data, _, found, err := tx.LatestWorklist(eng)
	if err != nil || !found {
		return nil, err
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("stored worklist: %w", err)
	}
	cls, err := tx.Classifications(eng)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		key, _ := it["key"].(string)
		if c, ok := cls[key]; ok {
			it["classification"] = map[string]any{"text": c.Classification, "by": c.ClassifiedBy, "at": c.ClassifiedAt}
		} else {
			it["classification"] = nil
		}
	}
	return items, nil
}

// worklist lists the Worklist of every client for a tax year:
//
//	recon-engine worklist --db <file> --tax-year 2025-26 [--json <file>]
func worklist(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("worklist", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	jsonOut := fs.String("json", "", "also write the Worklist as JSON to this file")
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
	cs, err := tx.Clients()
	if err != nil {
		return err
	}
	list := make([]any, 0)
	var b strings.Builder
	open := 0
	for _, c := range cs {
		eng, found, err := tx.FindClientEngagement(c.ID, ty.String())
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		items, err := worklistItems(tx, eng)
		if err != nil {
			return err
		}
		for _, it := range items {
			it["client_id"], it["client_name"] = c.ID, c.Name
			list = append(list, it)
			state := "not classified"
			if cl, ok := it["classification"].(map[string]any); ok {
				state = fmt.Sprintf("classified by %s: %s", cl["by"], cl["text"])
			} else {
				open++
			}
			fmt.Fprintf(&b, "%4d  %s  %s\n      %s\n", c.ID, c.Name, it["text"], state)
		}
	}
	fmt.Fprintf(&b, "%d items, %d not classified\n", len(list), open)
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"tax_year": ty.String(), "items": list}); err != nil {
			return err
		}
	}
	_, err = io.WriteString(stdout, b.String())
	return err
}

// classify records the classification staff gave one Worklist item of a client. It comes
// on standard input as JSON:
//
//	recon-engine classify --db <file> --client <id> --tax-year 2025-26 --json <file> < classification.json
//	{"kind": "new_head", "ledger": "...", "classification": "...", "by": "..."}
func classify(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("classify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	jsonOut := fs.String("json", "", "write the client's Worklist items as JSON to this file")
	if err := parseFlags(fs, args, "db", "client", "tax-year", "json"); err != nil {
		return err
	}
	ty, err := period.ParseTaxYear(*taxYear)
	if err != nil {
		return inputErrf("--tax-year: %v", err)
	}
	var in struct {
		Kind           string `json:"kind"`
		Ledger         string `json:"ledger"`
		Classification string `json:"classification"`
		By             string `json:"by"`
	}
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the classification is not valid JSON: %v", err)
	}
	in.Classification, in.By = strings.TrimSpace(in.Classification), strings.TrimSpace(in.By)
	switch {
	case !slices.Contains(newheads.Kinds, in.Kind):
		return inputErrf("BAD_KIND: kind %q must be one of %s", in.Kind, strings.Join(newheads.Kinds, ", "))
	case in.By == "" || utf8.RuneCountInString(in.By) > 100:
		return inputErrf("BAD_NAME: say who classifies (a name of up to 100 characters)")
	case in.Classification == "" || utf8.RuneCountInString(in.Classification) > 500:
		return inputErrf("BAD_CLASSIFICATION: write the classification (up to 500 characters)")
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
	defer tx.Rollback()
	c, ok, err := tx.ClientByID(*clientID)
	if err != nil {
		return err
	}
	if !ok {
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
	}
	if c.Disabled {
		return clientDisabled(c)
	}
	eng, ok, err := tx.FindClientEngagement(c.ID, ty.String())
	if err != nil {
		return err
	}
	var items []map[string]any
	if ok {
		if items, err = worklistItems(tx, eng); err != nil {
			return err
		}
	}
	key := in.Kind + "|" + in.Ledger
	if !slices.ContainsFunc(items, func(it map[string]any) bool { return it["key"] == key }) {
		return inputErrf("WORKLIST_ITEM_NOT_FOUND: %q (%s) is not an item of the client's latest Worklist for %s", in.Ledger, in.Kind, ty)
	}
	if err := tx.AddClassification(eng, store.Classification{Kind: in.Kind, Ledger: in.Ledger, Classification: in.Classification,
		ClassifiedBy: in.By, ClassifiedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		return err
	}
	if items, err = worklistItems(tx, eng); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, it := range items {
		it["client_id"], it["client_name"] = c.ID, c.Name
	}
	if err := writeJSONFile(*jsonOut, map[string]any{"tax_year": ty.String(), "items": items}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s (%s): classified by %s\n", in.Ledger, in.Kind, in.By)
	return err
}

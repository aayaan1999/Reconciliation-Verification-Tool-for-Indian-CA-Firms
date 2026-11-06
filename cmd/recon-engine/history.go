package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/findings"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// comparisonJSON is one run of a type of check as the history lists it (SPEC-009).
type comparisonJSON struct {
	No              int             `json:"no"`
	FinishedAt      string          `json:"finished_at"`
	IncludesTesting bool            `json:"includes_testing"`
	Kept            bool            `json:"kept"`   // false for a run made before reports were kept
	Counts          json.RawMessage `json:"counts"` // null when not kept
	Files           []comparisonDoc `json:"files"`
}

type comparisonDoc struct {
	DocumentType string `json:"document_type"`
	FileName     string `json:"file_name"`
	DraftNo      int64  `json:"draft_no"`
}

// history lists every run of one type of check for a client and tax year as Comparison 1,
// 2, 3…, oldest first, with the drafts each read. With --no it gives that comparison's
// report exactly as stored and the figures of the drafts it read, so it can be explored as
// it was. It only reads, and never runs a rule again.
//
//	recon-engine history --db <file> --client <id> --tax-year 2025-26 --comparison <id> [--no <n>] --json <file>
func history(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	comparison := fs.String("comparison", "", `type of check; "" for runs of every rule`)
	no := fs.Int("no", 0, "the comparison to open")
	jsonOut := fs.String("json", "", "write the history as JSON to this file")
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
	if _, found, err := tx.ClientByID(*clientID); err != nil {
		return err
	} else if !found {
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
	}
	eng, found, err := tx.FindClientEngagement(*clientID, ty.String())
	if err != nil {
		return err
	}
	var runs []store.Comparison
	if found {
		if runs, err = tx.Comparisons(eng, *comparison); err != nil {
			return err
		}
	}
	list := make([]comparisonJSON, 0, len(runs))
	for _, r := range runs {
		docs, _, err := tx.RunView(eng, r.ID)
		if err != nil {
			return err
		}
		c := comparisonJSON{No: r.No, FinishedAt: r.FinishedAt, IncludesTesting: r.IncludeTesting, Kept: len(r.Report) > 0,
			Counts: json.RawMessage("null"), Files: []comparisonDoc{}}
		if c.Kept {
			var rep struct {
				Counts json.RawMessage `json:"counts"`
			}
			if err := json.Unmarshal(r.Report, &rep); err != nil {
				return fmt.Errorf("comparison %d: stored report: %w", r.No, err)
			}
			c.Counts = rep.Counts
		}
		for _, d := range docs {
			c.Files = append(c.Files, comparisonDoc{DocumentType: d.Slot.DocumentType, FileName: d.FileName, DraftNo: d.DraftNo})
		}
		list = append(list, c)
	}
	out := map[string]any{"comparison_id": *comparison, "tax_year": ty.String(), "comparisons": list}
	if *no != 0 {
		if *no < 0 || *no > len(runs) || !list[*no-1].Kept {
			return inputErrf("COMPARISON_NOT_FOUND: there is no kept comparison %d of this type of check", *no)
		}
		r := runs[*no-1]
		docs, facts, err := tx.RunView(eng, r.ID)
		if err != nil {
			return err
		}
		records, err := tx.RecordsOf(eng, docs)
		if err != nil {
			return err
		}
		fl := findings.FactList{Engagements: []findings.EngagementFacts{engagementFacts(ty.String(), docs, facts, records)}}
		js, err := fl.JSON()
		if err != nil {
			return err
		}
		out["comparison"] = list[*no-1]
		out["report"] = json.RawMessage(bytes.TrimSpace(r.Report))
		out["facts"] = json.RawMessage(bytes.TrimSpace(js))
		// The documents the Partner had added when the run was made, with their columns and labels, as kept
		// with the run: editing or switching off a document later does not change this comparison (SPEC-022).
		hash, body, err := tx.RunOfficeVocab(r.ID)
		if err != nil {
			return err
		}
		out["office_vocab_hash"] = hash
		if body != "" {
			out["office_vocab"] = json.RawMessage(body)
		} else {
			out["office_vocab"] = nil
		}
	}
	if err := writeJSONFile(*jsonOut, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%d comparisons\n", len(list))
	return err
}

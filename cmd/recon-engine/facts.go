package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/findings"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// facts lists the stored figures of the engagement named by --input (the same key run
// uses): for every tax year, each document's latest draft and every figure read from it,
// with its file and place. It only reads; nothing is written.
func facts(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("facts", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	input := fs.String("input", "", "the file or folder a run read")
	jsonOut := fs.String("json", "", "also write the list as canonical JSON to this file")
	if err := parseFlags(fs, args, "db", "input"); err != nil {
		return err
	}
	if *jsonOut != "" {
		if st, err := os.Stat(filepath.Dir(*jsonOut)); err != nil || !st.IsDir() {
			return inputErrf("--json: the folder of %s does not exist", *jsonOut)
		}
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	key, err := filepath.Abs(*input)
	if err != nil {
		return err
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
	defer tx.Rollback() // read only: nothing is committed
	engs, err := tx.EngagementsByKey(filepath.ToSlash(key))
	if err != nil {
		return err
	}
	if len(engs) == 0 {
		return inputErrf("no run has read %s into this database", *input)
	}
	list, err := factList(tx, engs)
	if err != nil {
		return err
	}
	text, err := list.Text()
	if err != nil {
		return err
	}
	if *jsonOut != "" {
		js, err := list.JSON()
		if err != nil {
			return err
		}
		if err := os.WriteFile(*jsonOut, js, 0o644); err != nil {
			return err
		}
	}
	_, err = io.WriteString(stdout, text)
	return err
}

// factList builds the current view of each engagement: every document's latest draft and
// the figures read from it.
func factList(tx *store.Tx, engs []store.EngagementRef) (findings.FactList, error) {
	var list findings.FactList
	for _, e := range engs {
		docs, all, err := tx.CurrentView(e.ID)
		if err != nil {
			return list, err
		}
		records, err := tx.CurrentRecords(e.ID)
		if err != nil {
			return list, err
		}
		list.Engagements = append(list.Engagements, engagementFacts(e.TaxYear, docs, all, records))
	}
	return list, nil
}

// engagementFacts lists the figures and rows of the document versions docs.
func engagementFacts(taxYear string, docs []store.DocVersion, all []domain.Fact, records []domain.Record) findings.EngagementFacts {
	ef := findings.EngagementFacts{TaxYear: taxYear}
	for _, d := range docs {
		df := findings.DocumentFacts{DocumentType: d.Slot.DocumentType, Period: d.Slot.Period, Scope: d.Slot.Scope, Part: d.Slot.Part,
			FileName: d.FileName, SHA256: d.SHA256, DraftNo: d.DraftNo, ProfileID: d.ProfileID, ProfileVersion: d.ProfileVersion}
		for _, f := range all {
			if f.Locator.DocumentVersionID == d.ID {
				df.Facts = append(df.Facts, f)
			}
		}
		for _, r := range records {
			if r.Locator.DocumentVersionID == d.ID {
				df.Records = append(df.Records, r)
			}
		}
		ef.Documents = append(ef.Documents, df)
	}
	return ef
}

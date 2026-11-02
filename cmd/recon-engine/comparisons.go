package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/comparison"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// comparisonsList lists the comparison types, each with the documents it needs (with their
// names) and its rules (SPEC-004 part 8). It reads no database.
//
//	recon-engine comparisons [--comparisons <file>] [--vocab <folder>] [--json <file>]
func comparisonsList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("comparisons", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("comparisons", "content/comparisons/comparisons.yaml", "comparison types file")
	vocabDir := fs.String("vocab", "content/vocabulary", "fact vocabulary folder")
	jsonOut := fs.String("json", "", "also write the list as JSON to this file")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return inputErrf("vocabulary: %v", err)
	}
	list, err := comparison.Load(*file, v)
	if err != nil {
		return inputErrf("comparisons %s:\n%v", *file, err)
	}
	type doc struct {
		DocumentType string `json:"document_type"`
		Name         string `json:"name"`
	}
	type item struct {
		comparison.Comparison
		DocumentNames []doc `json:"document_names"`
	}
	out := make([]item, 0, len(list))
	var b strings.Builder
	for _, c := range list {
		it := item{Comparison: c}
		for _, d := range c.Documents {
			dt, _ := v.DocumentType(d)
			it.DocumentNames = append(it.DocumentNames, doc{d, dt.Description})
		}
		out = append(out, it)
		fmt.Fprintf(&b, "%-26s %s (%s)\n", c.ID, c.Name, strings.Join(c.Documents, " + "))
	}
	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, map[string]any{"comparisons": out}); err != nil {
			return err
		}
	}
	_, err = io.WriteString(stdout, b.String())
	return err
}

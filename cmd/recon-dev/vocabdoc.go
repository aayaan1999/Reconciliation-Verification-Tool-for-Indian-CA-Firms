package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// vocabDoc loads and validates the vocabulary and writes the generated document. It
// refuses when the vocabulary changed without a version increase.
func vocabDoc(args []string) error {
	flags := flag.NewFlagSet("vocab-doc", flag.ContinueOnError)
	dir := flags.String("dir", "content/vocabulary", "folder holding facts.yaml and records.yaml")
	out := flags.String("out", vocab.DocPath, "document to write")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	v, err := vocab.LoadDir(*dir)
	if err != nil {
		return err
	}
	old, err := os.ReadFile(*out)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := vocab.CheckVersionBumped(old, v); err != nil {
			return err
		}
	}
	if err := os.WriteFile(*out, vocab.Markdown(v), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (vocabulary version %d, %d facts, %d record types)\n", *out, v.Version(), len(v.Facts()), len(v.RecordTypes()))
	return nil
}

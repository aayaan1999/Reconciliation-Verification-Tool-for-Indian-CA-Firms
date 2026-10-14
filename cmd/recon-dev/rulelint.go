package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// ruleLint loads every rule under a folder and checks it and its fixtures; each rule needs
// at least one pass and one fail fixture (RU-43). Problems are printed with file and line.
//
//	go run ./cmd/recon-dev rule-lint [-vocab dir] [folder]
//
// The folder defaults to content/rules; a trailing "/..." is allowed.
func ruleLint(args []string) error {
	fs := flag.NewFlagSet("rule-lint", flag.ContinueOnError)
	vocabDir := fs.String("vocab", "content/vocabulary", "vocabulary folder")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "content/rules"
	switch fs.NArg() {
	case 0:
	case 1:
		root = strings.TrimSuffix(strings.TrimSuffix(filepath.ToSlash(fs.Arg(0)), "..."), "/")
	default:
		return errors.New("give at most one folder")
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return err
	}
	set, err := rules.LoadDir(root, v, checks.All())
	if err != nil {
		return err
	}
	if len(set.Rules()) == 0 {
		return fmt.Errorf("no rules found under %s", root)
	}
	bad := 0
	for _, r := range set.Rules() {
		fx, err := rules.LoadFixtures(r, v)
		var problems []string
		switch {
		case err != nil:
			problems = strings.Split(err.Error(), "\n")
		default:
			if len(fx.Pass) == 0 {
				problems = append(problems, filepath.ToSlash(filepath.Join(filepath.Dir(r.File), "fixtures", "pass"))+": no pass fixture; every rule needs at least one (RU-43)")
			}
			if len(fx.Fail) == 0 {
				problems = append(problems, filepath.ToSlash(filepath.Join(filepath.Dir(r.File), "fixtures", "fail"))+": no fail fixture; every rule needs at least one (RU-43)")
			}
		}
		if len(problems) == 0 {
			fmt.Printf("OK    %s  (%d pass, %d fail fixtures)\n", r.Key(), len(fx.Pass), len(fx.Fail))
			continue
		}
		bad++
		fmt.Printf("FAIL  %s\n", r.Key())
		for _, p := range problems {
			fmt.Printf("        %s\n", p)
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d of %d rules have problems", bad, len(set.Rules()))
	}
	fmt.Printf("all %d rules passed\n", len(set.Rules()))
	return nil
}

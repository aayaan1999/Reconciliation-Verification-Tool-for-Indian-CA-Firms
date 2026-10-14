package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// profileTest runs every profile's samples against its expected facts (IN-22). Detection
// always uses every installed profile, so a sample another profile would also claim fails.
//
//	go run ./cmd/recon-dev profile-test [-vocab dir] [-profiles dir] [folder ...]
//
// A folder ending in "/..." (as in "./content/profiles/...") means everything under it.
// With no folder, every profile is tested.
func profileTest(args []string) error {
	fs := flag.NewFlagSet("profile-test", flag.ContinueOnError)
	vocabDir := fs.String("vocab", "content/vocabulary", "vocabulary folder")
	profilesDir := fs.String("profiles", "content/profiles", "folder holding every installed profile")
	if err := fs.Parse(args); err != nil {
		return err
	}
	v, err := vocab.LoadDir(*vocabDir)
	if err != nil {
		return err
	}
	set, err := profile.LoadDir(*profilesDir, v)
	if err != nil {
		return err
	}
	reg := ingest.NewRegistry()
	if err := reg.Register(jsonadapter.New(0)); err != nil {
		return err
	}

	var only []string
	for _, a := range fs.Args() {
		a = strings.TrimSuffix(filepath.ToSlash(a), "...")
		abs, err := filepath.Abs(a)
		if err != nil {
			return err
		}
		only = append(only, abs)
	}
	inScope := func(p *profile.Profile) bool {
		if len(only) == 0 {
			return true
		}
		abs, _ := filepath.Abs(p.File)
		for _, o := range only {
			if rel, err := filepath.Rel(o, abs); err == nil && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
		return false
	}

	tested, failed := 0, 0
	for _, p := range set.Profiles() {
		if !inScope(p) {
			continue
		}
		for _, r := range profile.TestSamples(p, set, reg) {
			tested++
			if len(r.Problems) == 0 {
				fmt.Printf("PASS  %s  %s\n", r.Profile, r.Sample)
				continue
			}
			failed++
			fmt.Printf("FAIL  %s  %s\n", r.Profile, r.Sample)
			for _, pr := range r.Problems {
				fmt.Printf("        %s\n", pr)
			}
		}
	}
	switch {
	case tested == 0:
		return errors.New("no profiles found to test")
	case failed > 0:
		return fmt.Errorf("%d of %d samples failed", failed, tested)
	}
	fmt.Fprintf(os.Stdout, "all %d samples passed\n", tested)
	return nil
}

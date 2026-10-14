// Command recon-dev holds developer tools: profile tests, rule lint, vocabulary
// docs, synthetic data, load tests and content-pack building. Never shipped to firms.
//
// Run it from the repository root: go run ./cmd/recon-dev <tool> [flags]
package main

import (
	"fmt"
	"os"
	"slices"
)

// tools maps each sub-command to its function, which receives the remaining arguments.
var tools = map[string]struct {
	run  func(args []string) error
	help string
}{
	"profile-test":     {profileTest, "run every format profile against its samples and expected facts"},
	"rule-lint":        {ruleLint, "check every rule and its fixtures (each rule needs a pass and a fail fixture)"},
	"rulestore-check":  {rulestoreCheck, "check every rule and document file of the YAML rule store"},
	"rulestore-import": {rulestoreImport, "write the YAML rule store from the rule-book workbook"},
	"vocab-doc":        {vocabDoc, "regenerate docs/fact-vocabulary.md from content/vocabulary"},
}

func main() {
	if len(os.Args) < 2 || tools[os.Args[1]].run == nil {
		usage()
		os.Exit(2)
	}
	if err := tools[os.Args[1]].run(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "recon-dev %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./cmd/recon-dev <tool> [flags]\n\ntools:")
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "  %-12s %s\n", name, tools[name].help)
	}
}

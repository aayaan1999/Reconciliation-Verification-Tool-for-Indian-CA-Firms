// Command recon-engine reads uploaded files, runs the reconciliation rules and
// produces findings. It must never import a network package (see internal/archtest).
//
//	recon-engine load-rules --db <file> --store <rule store folder> --rules <engine rules folder>
//	recon-engine run --db <file> --input <file or folder> --tax-year 2025-26 [--include-testing] [--json <file>]
//	recon-engine facts --db <file> --input <file or folder> [--json <file>]
//	recon-engine rules --db <file> [--json <file>]
//	recon-engine client-add --db <file> --name <name> --tax-year 2025-26 [--json <file>]   (PAN on standard input)
//	recon-engine client-update --db <file> --client <id> [--json <file>]   (details as JSON on standard input)
//	recon-engine client-get --db <file> --client <id> --json <file>
//	recon-engine clients --db <file> --tax-year 2025-26 [--json <file>]
//	recon-engine result --db <file> --client <id> --tax-year 2025-26 --json <file>
//	recon-engine run ... --client <id>   (a client's run: its documents are kept across runs)
//	recon-engine run ... --comparison <id>   (only the rules of that type of check)
//	recon-engine comparisons [--json <file>]
//	recon-engine decide --db <file> --client <id> --tax-year 2025-26 --json <file>   (the decision on standard input)
//	recon-engine worklist --db <file> --tax-year 2025-26 [--json <file>]
//	recon-engine classify --db <file> --client <id> --tax-year 2025-26 --json <file>   (the classification on standard input)
//	recon-engine user-add --db <file> --json <file>   (the user on standard input: {"as", ...}, with the password)
//	recon-engine user-set-role --db <file> --json <file>   (the change on standard input: {"as", "login_name", "role", "reason"})
//	recon-engine user-reset-password --db <file> --json <file>   (standard input: {"as", "login_name", "password"})
//
// Exit codes: 0 no findings, 1 findings present, 2 input or rule error, 3 internal error
// (SPEC-003). The daemon mode for recon-server comes later (technical spec §A3.1).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// stdin is where client-add reads the PAN; tests replace it.
var stdin io.Reader = os.Stdin

// EngineVersion is recorded in every run's snapshot (RU-53).
const EngineVersion = "0.1.0"

// Exit codes.
const (
	exitOK       = 0
	exitFindings = 1
	exitInput    = 2
	exitInternal = 3
)

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr))
}

// realMain runs one command and returns its exit code.
func realMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitInput
	}
	var (
		code int
		err  error
	)
	switch args[0] {
	case "load-rules":
		err = loadRules(args[1:], stdout)
	case "run":
		code, err = run(args[1:], stdout)
	case "facts":
		err = facts(args[1:], stdout)
	case "rules":
		err = rulesList(args[1:], stdout)
	case "client-add":
		err = clientAdd(args[1:], stdin, stdout)
	case "client-update":
		err = clientUpdate(args[1:], stdin, stdout)
	case "client-get":
		err = clientGet(args[1:], stdout)
	case "clients":
		err = clientsList(args[1:], stdout)
	case "result":
		err = clientResult(args[1:], stdout)
	case "history":
		err = history(args[1:], stdout)
	case "comparisons":
		err = comparisonsList(args[1:], stdout)
	case "inspect":
		err = inspect(args[1:], stdout)
	case "profile-create":
		err = profileCreate(args[1:], stdin, stdout)
	case "decide":
		err = decide(args[1:], stdin, stdout)
	case "worklist":
		err = worklist(args[1:], stdout)
	case "classify":
		err = classify(args[1:], stdin, stdout)
	case "rule-save":
		err = ruleSave(args[1:], stdin, stdout)
	case "rule-get":
		err = ruleGet(args[1:], stdout)
	case "rule-disable":
		err = ruleDisable(args[1:], stdin, stdout)
	case "rule-enable":
		err = ruleEnable(args[1:], stdin, stdout)
	case "rule-approve":
		err = ruleApprove(args[1:], stdin, stdout)
	case "rulebook-save":
		err = rulebookSave(args[1:], stdin, stdout)
	case "rule-options":
		err = ruleOptions(args[1:], stdout)
	case "rule-versions":
		err = ruleVersions(args[1:], stdin, stdout)
	case "user-add":
		err = userAdd(args[1:], stdin, stdout)
	case "user-set-role":
		err = userSetRole(args[1:], stdin, stdout)
	case "user-reset-password":
		err = userResetPassword(args[1:], stdin, stdout)
	default:
		usage(stderr)
		return exitInput
	}
	if err == nil {
		return code
	}
	var ie *inputError
	if errors.As(err, &ie) {
		fmt.Fprintf(stderr, "recon-engine %s: %v\n", args[0], err)
		return exitInput
	}
	fmt.Fprintf(stderr, "recon-engine %s: internal error: %v\n", args[0], err)
	return exitInternal
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `usage:
  recon-engine load-rules --db <file> --store <rule store folder> --rules <engine rules folder> [--office-rules <folder>] [--vocab <folder>]
  recon-engine run --db <file> --input <file or folder> --tax-year <YYYY-YY> [--profiles <folder>] [--office-profiles <folder>] [--office-rules <folder>] [--vocab <folder>] [--include-testing] [--comparison <id>] [--client <id>] [--worklist <file>] [--json <file>]
  recon-engine facts --db <file> --input <file or folder> [--json <file>]
  recon-engine rules --db <file> [--vocab <folder>] [--json <file>]
  recon-engine client-add --db <file> --name <name> --tax-year <YYYY-YY> [--json <file>]   (the PAN is read from standard input)
  recon-engine client-update --db <file> --client <id> [--json <file>]   (the details are read from standard input as JSON)
  recon-engine client-get --db <file> --client <id> --json <file>
  recon-engine clients --db <file> --tax-year <YYYY-YY> [--json <file>]
  recon-engine result --db <file> --client <id> --tax-year <YYYY-YY> --json <file>
  recon-engine history --db <file> --client <id> --tax-year <YYYY-YY> --comparison <id> [--no <n>] --json <file>
  recon-engine comparisons [--comparisons <file>] [--vocab <folder>] [--json <file>]
  recon-engine inspect --input <file> [--profiles <folder>] [--office-profiles <folder>] [--vocab <folder>] --json <file>
  recon-engine profile-create --office-profiles <folder> --sample <file> [--check-only] --json <file>   (the mapping is read from standard input as JSON)
  recon-engine decide --db <file> --client <id> --tax-year <year> --json <file>   (the decision is read from standard input as JSON)
  recon-engine worklist --db <file> --tax-year <YYYY-YY> [--json <file>]
  recon-engine classify --db <file> --client <id> --tax-year <YYYY-YY> --json <file>   (the classification is read from standard input as JSON)
  recon-engine rule-save --db <file> --rules <content/rules> --office-rules <folder> --json <file>   (the form is read from standard input as JSON)
  recon-engine rule-get --db <file> --rules <content/rules> --office-rules <folder> --rule <ID> --json <file>
  recon-engine rule-disable --db <file> --rule <ID> --json <file>   (the reason is read from standard input as JSON)
  recon-engine rule-enable --db <file> --rule <ID> --json <file>   (the reason is read from standard input as JSON)
  recon-engine rule-approve --db <file> --rules <content/rules> --office-rules <folder> --rule <ID> --json <file>   (the Partner's name and reason are read from standard input as JSON)
  recon-engine rule-options --rules <content/rules> [--vocab <folder>] --json <file>
  recon-engine rule-versions --db <file> --rule <ID> [--switch <version ID> | --reset] --json <file>
  recon-engine rulebook-save --db <file> --rule <ID> --json <file>   (a Partner's edit of a rule-book rule, as JSON on standard input)
  recon-engine user-add --db <file> --json <file>   (standard input: {"as", "login_name", "display_name", "role", "password"})
  recon-engine user-set-role --db <file> --json <file>   (standard input: {"as", "login_name", "role", "reason"})
  recon-engine user-reset-password --db <file> --json <file>   (standard input: {"as", "login_name", "password"})`)
}

// An inputError is a problem with the files, rules or flags given (exit code 2). Every
// other error is internal (exit code 3).
type inputError struct{ err error }

func (e *inputError) Error() string { return e.err.Error() }
func (e *inputError) Unwrap() error { return e.err }

func inputErr(err error) error { return &inputError{err: err} }

func inputErrf(format string, args ...any) error { return inputErr(fmt.Errorf(format, args...)) }

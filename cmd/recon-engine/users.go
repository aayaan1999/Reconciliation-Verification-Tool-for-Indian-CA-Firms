package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// The user commands (SPEC-012). The Admin tab does the same through recon-server; these let
// the office computer add a user, change a role or reset a password without a browser. The
// password is read from standard input, never from the command line, so it stays out of
// shell history. Each command names the Partner who acts (as), and that Partner must be an
// active Partner in the database. No command deletes a user (users are disabled, never
// deleted; migration 0008).
//
//	recon-engine user-add --db <file> --json <file>            (standard input: {"as", "login_name", "display_name", "role", "password"})
//	recon-engine user-set-role --db <file> --json <file>       (standard input: {"as", "login_name", "role", "reason"})
//	recon-engine user-reset-password --db <file> --json <file> (standard input: {"as", "login_name", "password"})

// userCommandBody is the largest standard input a user command reads.
const userCommandBody = 1 << 14

// readUserCommand decodes a command's JSON strictly: unknown keys are refused.
func readUserCommand(stdin io.Reader, v any) error {
	dec := json.NewDecoder(io.LimitReader(stdin, userCommandBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return inputErrf("BAD_REQUEST: the standard input is not valid JSON for this command: %v", err)
	}
	return nil
}

// openUserService opens the database and returns the service for a user command, with the
// Partner who acts. A refusal by the rules is an input error (exit code 2), not an internal one.
func openUserService(dbFile, as string) (*store.DB, *auth.Service, auth.Account, error) {
	if dbFile == "" {
		return nil, nil, auth.Account{}, inputErrf("the flag --db is required")
	}
	if _, err := os.Stat(dbFile); err != nil {
		return nil, nil, auth.Account{}, inputErrf("--db: %v", err)
	}
	db, err := store.Open(dbFile)
	if err != nil {
		return nil, nil, auth.Account{}, err
	}
	svc, err := auth.NewService(db, time.Now)
	if err != nil {
		db.Close()
		return nil, nil, auth.Account{}, err
	}
	actor, ok, err := svc.AccountByLogin(as)
	if err != nil {
		db.Close()
		return nil, nil, auth.Account{}, err
	}
	if !ok || !actor.Active {
		db.Close()
		return nil, nil, auth.Account{}, inputErrf("NOT_SIGNED_IN: the acting user %q is not an active user", as)
	}
	return db, svc, actor, nil
}

// userRefusal turns a refusal of the rules into an input error and passes other errors on.
func userRefusal(err error) error {
	var r *auth.Refusal
	if errors.As(err, &r) {
		return inputErrf("%s: %s", r.Code, r.Message)
	}
	return err
}

// writeUserResult writes an account as JSON when --json is given. It never holds a password.
func writeUserResult(path string, stdout io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if path == "" {
		_, err = stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// userAccountJSON is the account of a user as the commands report it.
type userAccountJSON struct {
	LoginName          string `json:"login_name"`
	DisplayName        string `json:"display_name"`
	Role               string `json:"role"`
	Active             bool   `json:"active"`
	MustChangePassword bool   `json:"must_change_password"`
}

func accountJSON(a auth.Account) userAccountJSON {
	return userAccountJSON{LoginName: a.LoginName, DisplayName: a.DisplayName, Role: string(a.Role),
		Active: a.Active, MustChangePassword: a.MustChangePassword}
}

// userAdd adds a user with a starting password, who must change it at first sign-in.
func userAdd(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("user-add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	jsonOut := fs.String("json", "", "also write the new user as JSON to this file")
	if err := parseFlags(fs, args, "db"); err != nil {
		return err
	}
	var in struct {
		As          string `json:"as"`
		LoginName   string `json:"login_name"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if err := readUserCommand(stdin, &in); err != nil {
		return err
	}
	db, svc, actor, err := openUserService(*dbFile, in.As)
	if err != nil {
		return err
	}
	defer db.Close()
	acc, err := svc.AddUser(actor, auth.NewUser{LoginName: in.LoginName, DisplayName: in.DisplayName, Role: in.Role, Password: in.Password})
	if err != nil {
		return userRefusal(err)
	}
	return writeUserResult(*jsonOut, stdout, accountJSON(acc))
}

// userSetRole changes a user's role, with a reason. The change applies at the user's next request.
func userSetRole(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("user-set-role", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	jsonOut := fs.String("json", "", "also write the user as JSON to this file")
	if err := parseFlags(fs, args, "db"); err != nil {
		return err
	}
	var in struct {
		As        string `json:"as"`
		LoginName string `json:"login_name"`
		Role      string `json:"role"`
		Reason    string `json:"reason"`
	}
	if err := readUserCommand(stdin, &in); err != nil {
		return err
	}
	role, err := auth.ParseRole(in.Role)
	if err != nil {
		return inputErrf("BAD_INPUT: %v", err)
	}
	db, svc, actor, err := openUserService(*dbFile, in.As)
	if err != nil {
		return err
	}
	defer db.Close()
	target, ok, err := svc.AccountByLogin(in.LoginName)
	if err != nil {
		return err
	}
	if !ok {
		return inputErrf("USER_NOT_FOUND: there is no user %q", in.LoginName)
	}
	if err := svc.SetRole(actor, target.ID, role, in.Reason); err != nil {
		return userRefusal(err)
	}
	after, _, err := svc.AccountByLogin(in.LoginName)
	if err != nil {
		return err
	}
	return writeUserResult(*jsonOut, stdout, accountJSON(after))
}

// userResetPassword sets a new starting password; the user must change it at next sign-in.
func userResetPassword(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("user-reset-password", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	jsonOut := fs.String("json", "", "also write the user as JSON to this file")
	if err := parseFlags(fs, args, "db"); err != nil {
		return err
	}
	var in struct {
		As        string `json:"as"`
		LoginName string `json:"login_name"`
		Password  string `json:"password"`
	}
	if err := readUserCommand(stdin, &in); err != nil {
		return err
	}
	db, svc, actor, err := openUserService(*dbFile, in.As)
	if err != nil {
		return err
	}
	defer db.Close()
	target, ok, err := svc.AccountByLogin(in.LoginName)
	if err != nil {
		return err
	}
	if !ok {
		return inputErrf("USER_NOT_FOUND: there is no user %q", in.LoginName)
	}
	if err := svc.ResetPassword(actor, target.ID, in.Password); err != nil {
		return userRefusal(err)
	}
	after, _, err := svc.AccountByLogin(in.LoginName)
	if err != nil {
		return err
	}
	return writeUserResult(*jsonOut, stdout, accountJSON(after))
}

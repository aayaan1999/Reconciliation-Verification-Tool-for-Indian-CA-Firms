package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// The user commands (SPEC-012): a Partner acts through them; the password comes on standard
// input and never appears in the output.

const (
	cliPartnerPassword = "partner-cli-pass-2026"
	cliStartPassword   = "cli-starting-pass-2026"
	cliResetPassword   = "cli-reset-pass-2026"
)

// userDB makes a database with one Partner, "priya", and returns its file.
func userDB(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "users.db")
	db, err := store.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.NewService(db, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Setup(auth.Setup{LoginName: "priya", DisplayName: "A. Partner", Password: cliPartnerPassword}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	return file
}

// runUser runs one user command and returns what it wrote and its error.
func runUser(t *testing.T, cmd func([]string, *strings.Reader, *bytes.Buffer) error, args []string, body string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmd(args, strings.NewReader(body), &out)
	return out.String(), err
}

func userAddCmd(args []string, in *strings.Reader, out *bytes.Buffer) error {
	return userAdd(args, in, out)
}

func userRoleCmd(args []string, in *strings.Reader, out *bytes.Buffer) error {
	return userSetRole(args, in, out)
}

func userResetCmd(args []string, in *strings.Reader, out *bytes.Buffer) error {
	return userResetPassword(args, in, out)
}

// AC-2 and AC-5 through the commands: add a user, change the role, reset the password.
func TestUserCommandsAddChangeAndReset(t *testing.T) {
	file := userDB(t)
	out, err := runUser(t, userAddCmd, []string{"--db", file}, `{"as":"priya","login_name":"asha","display_name":"Asha","role":"staff","password":"`+cliStartPassword+`"}`)
	if err != nil {
		t.Fatalf("user-add: %v", err)
	}
	if !strings.Contains(out, `"must_change_password": true`) || strings.Contains(out, cliStartPassword) {
		t.Errorf("user-add output %q: want the user with a forced change and no password", out)
	}
	out, err = runUser(t, userRoleCmd, []string{"--db", file}, `{"as":"priya","login_name":"asha","role":"manager","reason":"takes on exports"}`)
	if err != nil || !strings.Contains(out, `"role": "manager"`) {
		t.Fatalf("user-set-role: %v, %q", err, out)
	}
	out, err = runUser(t, userResetCmd, []string{"--db", file}, `{"as":"priya","login_name":"asha","password":"`+cliResetPassword+`"}`)
	if err != nil || strings.Contains(out, cliResetPassword) {
		t.Fatalf("user-reset-password: %v, %q", err, out)
	}
}

// The commands refuse a Partner who is not active, a staff member acting, and a missing user.
func TestUserCommandsCheckTheActingPartner(t *testing.T) {
	file := userDB(t)
	if _, err := runUser(t, userAddCmd, []string{"--db", file}, `{"as":"priya","login_name":"asha","display_name":"Asha","role":"staff","password":"`+cliStartPassword+`"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := runUser(t, userAddCmd, []string{"--db", file}, `{"as":"nobody","login_name":"x1","display_name":"X","role":"staff","password":"`+cliStartPassword+`"}`); exitCode(err) != exitInput {
		t.Errorf("an unknown acting user: %v, want an input error", err)
	}
	if _, err := runUser(t, userAddCmd, []string{"--db", file}, `{"as":"asha","login_name":"x2","display_name":"X","role":"staff","password":"`+cliStartPassword+`"}`); exitCode(err) != exitInput || !strings.Contains(err.Error(), "FORBIDDEN") {
		t.Errorf("a staff user acting: %v, want FORBIDDEN", err)
	}
	if _, err := runUser(t, userRoleCmd, []string{"--db", file}, `{"as":"priya","login_name":"ghost","role":"staff","reason":"x"}`); exitCode(err) != exitInput || !strings.Contains(err.Error(), "USER_NOT_FOUND") {
		t.Errorf("an unknown target: %v", err)
	}
	if _, err := runUser(t, userRoleCmd, []string{"--db", file}, `{"as":"priya","login_name":"asha","role":"staff","reason":""}`); exitCode(err) != exitInput || !strings.Contains(err.Error(), "REASON_REQUIRED") {
		t.Errorf("a role change without a reason: %v", err)
	}
}

// Standard input is read strictly: an unknown key is refused, so a misspelt field cannot be
// silently ignored.
func TestUserCommandsRefuseUnknownKeys(t *testing.T) {
	file := userDB(t)
	if _, err := runUser(t, userAddCmd, []string{"--db", file}, `{"as":"priya","login_name":"asha","role":"staff","password":"x","extra":1}`); exitCode(err) != exitInput {
		t.Errorf("an unknown key: %v, want an input error", err)
	}
}

// AC-15: no command removes a user. The command names are read from the program's dispatch
// and none may say delete or remove, except document-delete (SPEC-022), which removes an unused
// office document definition and cannot reach a user.
func TestNoCommandDeletesAUser(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	cases := regexp.MustCompile(`case "([a-z-]+)":`).FindAllStringSubmatch(string(src), -1)
	if len(cases) < 10 {
		t.Fatalf("found only %d commands in main.go", len(cases))
	}
	for _, c := range cases {
		if c[1] == "document-delete" {
			continue // SPEC-022: it removes an office document definition that nothing uses, never a user
		}
		if strings.Contains(c[1], "delete") || strings.Contains(c[1], "remove") {
			t.Errorf("command %s would remove something", c[1])
		}
	}
	for _, name := range []string{"user-add", "user-set-role", "user-reset-password"} {
		found := false
		for _, c := range cases {
			found = found || c[1] == name
		}
		if !found {
			t.Errorf("command %s is not dispatched", name)
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return exitOK
	}
	var ie *inputError
	if errors.As(err, &ie) {
		return exitInput
	}
	return exitInternal
}

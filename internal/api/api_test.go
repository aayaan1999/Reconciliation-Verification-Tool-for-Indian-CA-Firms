package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
)

// The API tests of SPEC-004 part 6 (AC-15, AC-16, AC-18). They build the real recon-engine
// once and run every request against it, as recon-server does.

var (
	repo       = filepath.Join("..", "..")
	enginePath string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "recon-api-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enginePath = filepath.Join(dir, "recon-engine")
	if runtime.GOOS == "windows" {
		enginePath += ".exe"
	}
	build := exec.Command("go", "build", "-o", enginePath, "./cmd/recon-engine")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building recon-engine: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// The Partner every test signs in as (SPEC-012). Their password is a test value.
const (
	testPartner         = "priya"
	testPartnerPassword = "partner-test-2026"
)

// env is a running server and a client that holds the session cookie of the test Partner. The
// server is served over TLS so the browser-style Secure cookie is sent back, as it would be to
// a browser.
type env struct {
	srv    *httptest.Server
	client *http.Client
	temp   string
	db     string
	server *Server
}

func newEnv(t *testing.T) env {
	t.Helper()
	return newEnvWithDB(t, filepath.Join(t.TempDir(), "ui.db"))
}

// newEnvWithDB starts a server on an existing (or new) database, as a restart would. On a new
// database it sets up the test Partner; then it signs that Partner in.
func newEnvWithDB(t *testing.T, db string) env {
	t.Helper()
	e := startEnv(t, db)
	if needed, err := e.server.auth.SetupNeeded(); err != nil {
		t.Fatal(err)
	} else if needed {
		if _, err := e.server.auth.Setup(auth.Setup{LoginName: testPartner, DisplayName: "A. Partner", Password: testPartnerPassword}); err != nil {
			t.Fatal(err)
		}
	}
	e.signInAs(t, testPartner, testPartnerPassword)
	return e
}

// startEnv starts a server on a database with no sign-in and no setup: the first start.
func startEnv(t *testing.T, db string) env {
	t.Helper()
	temp := t.TempDir()
	s, err := New(Config{
		Engine: enginePath, DB: db,
		Store:    filepath.Join(repo, "docs", "Recon_Tool_YAML_Rule_Store", "recon_yaml"),
		Rules:    filepath.Join(repo, "content", "rules"),
		Vocab:    filepath.Join(repo, "content", "vocabulary"),
		Profiles: filepath.Join(repo, "content", "profiles"),
		TempDir:  temp,
	}, fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>t</title>")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	srv := httptest.NewTLSServer(s.Handler())
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := clientOf(srv, jar)
	return env{srv: srv, client: client, temp: temp, db: db, server: s}
}

// newClient returns a second browser-like client on the same server, with its own cookies.
func (e env) newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return clientOf(e.srv, jar)
}

// clientOf copies the TLS client of a test server (which trusts its certificate) and gives the
// copy its own cookie jar. The server's own client is shared, so it is never changed.
func clientOf(srv *httptest.Server, jar http.CookieJar) *http.Client {
	c := *srv.Client()
	c.Jar = jar
	return &c
}

// signInAs signs the client in; the session cookie is kept in its jar.
func (e env) signInAs(t *testing.T, login, password string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"login_name": login, "password": password})
	resp, err := e.client.Post(e.srv.URL+"/api/v1/login", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sign in as %s: status %d", login, resp.StatusCode)
	}
}

type upload struct {
	name string
	data []byte
}

func sample(t *testing.T, parts ...string) upload {
	t.Helper()
	p := filepath.Join(append([]string{repo, "testdata", "run"}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return upload{filepath.Base(p), b}
}

func (e env) post(t *testing.T, taxYear string, includeTesting bool, files ...upload) (int, map[string]json.RawMessage) {
	t.Helper()
	return e.postFor(t, "", taxYear, includeTesting, files...)
}

// postFor uploads files for a client ("" for none).
func (e env) postFor(t *testing.T, clientID, taxYear string, includeTesting bool, files ...upload) (int, map[string]json.RawMessage) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if clientID != "" {
		mw.WriteField("client_id", clientID)
	}
	mw.WriteField("tax_year", taxYear)
	mw.WriteField("include_testing", fmt.Sprint(includeTesting))
	for _, f := range files {
		w, _ := mw.CreateFormFile("files", f.name)
		w.Write(f.data)
	}
	mw.Close()
	resp, err := e.client.Post(e.srv.URL+"/api/v1/runs", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("status %d: response is not JSON: %v", resp.StatusCode, err)
	}
	return resp.StatusCode, out
}

func (e env) assertNoTempFiles(t *testing.T) {
	t.Helper()
	left, _ := os.ReadDir(e.temp)
	if len(left) > 0 {
		t.Errorf("%d temporary entries left after the request", len(left))
	}
}

// AC-15: uploading the tax-suite sample shows the Rs 90 finding with both sources.
func TestRunFindsTheDifference(t *testing.T) {
	e := newEnv(t)
	status, out := e.post(t, "2025-26", true, sample(t, "taxsuite", "comp_v3_2025-26_differs.json"))
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, out["message"])
	}
	report := string(out["report"])
	for _, s := range []string{`"rule_id":"RECON-DEP-001"`, `"difference_paise":9000`, `"paise":9876000`, `"paise":9867000`,
		`"path":"$.income.business_profession.allowed_depreciation"`, `"path":"$.fixed_assets.schedule_total"`, `"category":"R1"`} {
		if !strings.Contains(report, s) {
			t.Errorf("report lacks %s:\n%s", s, report)
		}
	}
	if string(out["exit_code"]) != "1" || !strings.Contains(string(out["facts"]), `"name":"computation.gross_total_income"`) {
		t.Errorf("exit_code %s, facts %s", out["exit_code"], out["facts"])
	}
	e.assertNoTempFiles(t)
}

// AC-16: unknown and damaged files come back with their stable code and file name, and
// nothing is stored.
func TestInputErrors(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct {
		file upload
		code string
	}{
		{sample(t, "unknown", "balance_sheet.json"), "PROFILE_NO_MATCH"},
		{sample(t, "damaged", "comp_damaged.json"), "PARSE_ERROR"},
	} {
		status, out := e.post(t, "2025-26", true, c.file)
		var code, msg string
		json.Unmarshal(out["error_code"], &code)
		json.Unmarshal(out["message"], &msg)
		if status != http.StatusUnprocessableEntity || code != c.code || !strings.Contains(msg, c.file.name) {
			t.Errorf("%s: status %d, code %q, message %q", c.file.name, status, code, msg)
		}
	}
	// The engine wrote nothing: the database holds rules only, no document.
	cmd := exec.Command(enginePath, "facts", "--db", e.db, "--input", e.temp)
	if err := cmd.Run(); err == nil {
		t.Error("an input was stored after a refused run")
	}
	e.assertNoTempFiles(t)
}

// AC-18: the request checks and limits.
func TestUploadChecks(t *testing.T) {
	e := newEnv(t)
	good := sample(t, "clean", "comp_2025-26_agrees.json")
	many := make([]upload, MaxFiles+1)
	for i := range many {
		many[i] = upload{fmt.Sprintf("f%d.json", i), good.data}
	}
	for _, c := range []struct {
		name    string
		taxYear string
		files   []upload
		status  int
		code    string
	}{
		{"not a type read", "2025-26", []upload{{"ledger.pdf", []byte("%PDF")}}, http.StatusUnsupportedMediaType, "UNSUPPORTED_FILE_TYPE"},
		{"old Excel", "2025-26", []upload{{"ledger.xls", []byte{0xD0, 0xCF}}}, http.StatusUnsupportedMediaType, "UNSUPPORTED_FILE_TYPE"},
		{"no files", "2025-26", nil, http.StatusBadRequest, "NO_FILES"},
		{"bad tax year", "2025", []upload{good}, http.StatusBadRequest, "BAD_TAX_YEAR"},
		{"same name twice", "2025-26", []upload{good, good}, http.StatusBadRequest, "DUPLICATE_FILE_NAME"},
		{"too many files", "2025-26", many, http.StatusBadRequest, "TOO_MANY_FILES"},
	} {
		t.Run(c.name, func(t *testing.T) {
			status, out := e.post(t, c.taxYear, false, c.files...)
			var code string
			json.Unmarshal(out["error_code"], &code)
			if status != c.status || code != c.code {
				t.Errorf("status %d, code %q; want %d, %q", status, code, c.status, c.code)
			}
		})
	}
	t.Run("file over the limit", func(t *testing.T) {
		big := upload{"big.json", bytes.Repeat([]byte(" "), MaxFileBytes+1)}
		status, out := e.post(t, "2025-26", false, big)
		var code string
		json.Unmarshal(out["error_code"], &code)
		if status != http.StatusRequestEntityTooLarge || code != "FILE_TOO_LARGE" {
			t.Errorf("status %d, code %q", status, code)
		}
	})
	t.Run("a path in the file name is dropped", func(t *testing.T) {
		status, out := e.post(t, "2025-26", false, upload{`..\..\evil.json`, good.data})
		if status != http.StatusOK || !strings.Contains(string(out["report"]), `"file":"evil.json"`) {
			t.Errorf("status %d, report %s", status, out["report"])
		}
	})
	e.assertNoTempFiles(t)
}

func TestRulesAndTaxYears(t *testing.T) {
	e := newEnv(t)
	resp, err := e.client.Get(e.srv.URL + "/api/v1/rules")
	if err != nil {
		t.Fatal(err)
	}
	var rules struct {
		Rules []struct {
			ID, Source, Status, Category string
			Runnable                     bool
		} `json:"rules"`
	}
	json.NewDecoder(resp.Body).Decode(&rules)
	resp.Body.Close()
	runnable, disabled := 0, 0
	for _, r := range rules.Rules {
		if r.Runnable {
			runnable++
		}
		if r.Status == "disabled" {
			disabled++
		}
	}
	engine, _ := filepath.Glob(filepath.Join(repo, "content", "rules", "*", "*", "*", "rule.yaml"))
	if len(rules.Rules) != len(engine)+304 || runnable != len(engine) || disabled != 304 {
		t.Errorf("%d rules, %d runnable, %d disabled; want %d, %d, 304", len(rules.Rules), runnable, disabled, len(engine)+304, len(engine))
	}
	resp, err = e.client.Get(e.srv.URL + "/api/v1/tax-years")
	if err != nil {
		t.Fatal(err)
	}
	var ty struct {
		TaxYears []string `json:"tax_years"`
	}
	json.NewDecoder(resp.Body).Decode(&ty)
	resp.Body.Close()
	if fmt.Sprint(ty.TaxYears) != "[2025-26 2026-27]" {
		t.Errorf("tax years %v", ty.TaxYears)
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Error("no Content-Security-Policy header")
	}
	e.assertNoTempFiles(t)
}

func TestUIIsServedReadOnly(t *testing.T) {
	e := newEnv(t)
	for path, want := range map[string]int{"/": 200, "/rules": 200, "/api/v1/nothing": 404} {
		resp, err := e.client.Get(e.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("GET %s: %d, want %d", path, resp.StatusCode, want)
		}
	}
	resp, err := e.client.Post(e.srv.URL+"/", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /: %d, want 405", resp.StatusCode)
	}
}

// SPEC-008 AC-7: an Excel workbook is uploaded like a JSON file; an old .xls file is refused
// with what to do instead.
func TestUploadExcel(t *testing.T) {
	e := newEnv(t)
	b, err := os.ReadFile(filepath.Join(repo, "content", "profiles", "generic_xlsx", "sales_register", "v1", "samples", "SALES_REG_Aug2026.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	status, out := e.post(t, "2026-27", true, upload{"SALES_REG_Aug2026.xlsx", b}, sample(t, "gst-sales", "GSTR1_Aug2026.json"))
	if status != http.StatusOK || string(out["exit_code"]) != "1" {
		t.Fatalf("status %d, exit_code %s: %s", status, out["exit_code"], out["message"])
	}
	if rep := string(out["report"]); !strings.Contains(rep, `"file_type":"xlsx"`) || !strings.Contains(rep, `"path":"Sales Register!6:6"`) {
		t.Errorf("the report does not show the workbook and its rows: %.500s", rep)
	}
	status, out = e.post(t, "2026-27", true, upload{"SALES_REG_Aug2026.xls", []byte{0xD0, 0xCF, 0x11, 0xE0}})
	var msg string
	json.Unmarshal(out["message"], &msg)
	if status != http.StatusUnsupportedMediaType || !strings.Contains(msg, "save it as .xlsx or .csv") {
		t.Errorf("an .xls file: status %d, %q", status, msg)
	}
}

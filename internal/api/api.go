package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/client"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// Limits on one upload (SPEC-004 part 6).
const (
	MaxFiles     = 20
	MaxFileBytes = 64 << 20
	MaxTotal     = 256 << 20
)

// Config says where the engine and its content are. Paths may be relative to the server's
// working folder; New makes them absolute.
type Config struct {
	Engine      string // the recon-engine program
	DB          string // the database file the engine writes
	Store       string // the YAML rule store
	Rules       string // the engine rules (recon.rule/1)
	Vocab       string // the fact vocabulary
	Profiles    string // the format profiles
	Office      string // the profiles taught in the office (mapping screen); "" means <db folder>/profiles
	OfficeRules string // the rules written in the office by the Rule Builder (SPEC-010); "" means <db folder>/office-rules
	Comparisons string // the comparison types file
	Worklist    string // the thresholds of the year-on-year ledger comparison; "" means <vocab>/../worklist/newheads.yaml
	TempDir     string // where uploads are kept during a run; "" means the system temp folder
	// Hosts are the computer names, besides localhost and IP addresses, that browsers may
	// use to reach the server (such as "office-pc"). A request naming any other host is
	// refused, so a web page cannot reach the server through a name it controls.
	Hosts []string
	// Firm is the firm's name, written on every sheet of an Excel export (SPEC-013 §4). "" shows
	// that the name is not set.
	Firm string
}

// A Server answers the API and serves the embedded UI. It never evaluates a rule itself:
// every check runs in a separate recon-engine process (technical spec §A3.1), and runs are
// taken one at a time, because the engine is the database's single writer.
type Server struct {
	cfg  Config
	ui   fs.FS
	mu   sync.Mutex
	db   *store.DB     // the users, sessions and their audit entries (SPEC-012); rules and findings are still written by the engine
	auth *auth.Service // sign-in, sessions, roles and user administration

	viewMu sync.Mutex        // guards viewed
	viewed map[string]string // user|client|tax year -> the minute of its last client.view (SPEC-013 §2)
}

// New checks the configuration and loads the rules into the database once, so the first
// request finds them.
func New(cfg Config, ui fs.FS) (*Server, error) {
	if cfg.Comparisons == "" {
		cfg.Comparisons = filepath.Join(filepath.Dir(cfg.Vocab), "comparisons", "comparisons.yaml")
	}
	if cfg.Office == "" {
		cfg.Office = filepath.Join(filepath.Dir(cfg.DB), "profiles")
	}
	if cfg.Worklist == "" {
		cfg.Worklist = filepath.Join(filepath.Dir(cfg.Vocab), "worklist", "newheads.yaml")
	}
	if cfg.OfficeRules == "" {
		cfg.OfficeRules = filepath.Join(filepath.Dir(cfg.DB), "office-rules")
	}
	for _, p := range []*string{&cfg.Engine, &cfg.DB, &cfg.Store, &cfg.Rules, &cfg.Vocab, &cfg.Profiles, &cfg.Office, &cfg.OfficeRules, &cfg.Comparisons, &cfg.Worklist} {
		abs, err := filepath.Abs(*p)
		if err != nil {
			return nil, err
		}
		*p = abs
	}
	if _, err := os.Stat(cfg.Engine); err != nil {
		return nil, fmt.Errorf("the engine program %s is missing; build it with: go build -o build/ ./cmd/recon-engine", cfg.Engine)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DB), 0o755); err != nil {
		return nil, err
	}
	db, err := store.Open(cfg.DB)
	if err != nil {
		return nil, err
	}
	svc, err := auth.NewService(db, time.Now)
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &Server{cfg: cfg, ui: ui, db: db, auth: svc}
	if res := s.engine("load-rules", "--db", cfg.DB, "--store", cfg.Store, "--rules", cfg.Rules, "--office-rules", cfg.OfficeRules, "--vocab", cfg.Vocab, "--profiles", cfg.Profiles); res.code != 0 {
		db.Close()
		return nil, fmt.Errorf("loading the rules failed: %s", res.stderr)
	}
	return s, nil
}

// Close releases the database. The server keeps it open for its whole run.
func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// An endpoint is one API route with the permission it needs (technical spec §B11). Every
// route is declared here and registered through authorize, so none can exist without a
// permission. TestEveryEndpointDeclaresAPermission and the AST check in auth_test.go hold this.
type endpoint struct {
	pattern string
	perm    auth.Permission
	handler http.HandlerFunc
}

// endpoints lists every API route. The permission is the one the matrix in internal/auth
// grants, or PermOpen (no sign-in) or PermSignedIn (any signed-in user, also while a password
// change is pending).
func (s *Server) endpoints() []endpoint {
	return []endpoint{
		// Sign-in and first start: no sign-in yet.
		{"POST /api/v1/login", auth.PermOpen, s.login},
		{"GET /api/v1/setup", auth.PermOpen, s.setupStatus},
		{"POST /api/v1/setup", auth.PermOpen, s.setup},
		{"/api/", auth.PermOpen, func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "no such API endpoint")
		}},

		// Any signed-in user, also while a password change is pending.
		{"POST /api/v1/logout", auth.PermSignedIn, s.logout},
		{"GET /api/v1/me", auth.PermSignedIn, s.me},
		{"POST /api/v1/me/password", auth.PermSignedIn, s.changePassword},

		{"GET /api/v1/rules", auth.PermView, s.rules},
		{"GET /api/v1/tax-years", auth.PermView, s.taxYears},
		{"POST /api/v1/runs", auth.PermRun, s.runs},
		{"GET /api/v1/clients", auth.PermView, s.clients},
		{"POST /api/v1/clients", auth.PermRun, s.addClient},
		{"GET /api/v1/clients/{id}/result", auth.PermView, s.clientResult},
		{"GET /api/v1/clients/{id}/comparisons", auth.PermView, s.clientHistory},
		{"POST /api/v1/clients/{id}/exports", auth.PermExport, s.exportClient},
		{"POST /api/v1/clients/{id}/query-letter", auth.PermExport, s.queryLetter},
		{"POST /api/v1/clients/{id}/findings/{finding}/delete", auth.PermDeleteFindings, s.deleteFinding},
		{"GET /api/v1/clients/{id}", auth.PermView, s.clientDetails},
		{"PUT /api/v1/clients/{id}", auth.PermRun, s.updateClient},
		{"POST /api/v1/clients/{id}/disable", auth.PermDisableClients, s.disableClient}, // SPEC-004 part 10: Partner only
		{"POST /api/v1/clients/{id}/enable", auth.PermDisableClients, s.enableClient},
		{"POST /api/v1/clients/{id}/decisions", auth.PermDecide, s.decide},
		{"GET /api/v1/worklist", auth.PermView, s.worklist},
		{"POST /api/v1/clients/{id}/worklist", auth.PermDecide, s.classify},
		{"GET /api/v1/comparisons", auth.PermView, s.comparisons},
		{"POST /api/v1/inspect", auth.PermRun, s.inspect},
		{"POST /api/v1/profiles", auth.PermRun, s.createProfile},
		{"GET /api/v1/rule-options", auth.PermView, s.ruleOptions},
		{"GET /api/v1/rules/{id}", auth.PermView, s.ruleCurrent},
		{"POST /api/v1/rules", auth.PermDraftRules, s.createRule},
		{"PUT /api/v1/rules/{id}", auth.PermDraftRules, s.updateRule},
		{"POST /api/v1/rules/{id}/disable", auth.PermDisableRules, s.disableRule},
		{"POST /api/v1/rules/bulk/disable", auth.PermDisableRules, s.bulkDisableRules}, // SPEC-010 scope 9: several rules, all or none
		{"POST /api/v1/rules/bulk/enable", auth.PermActivateRules, s.bulkEnableRules},
		{"POST /api/v1/rules/{id}/enable", auth.PermActivateRules, s.enableRule},
		{"POST /api/v1/rules/{id}/approve", auth.PermActivateRules, s.approveRule},
		{"GET /api/v1/rules/{id}/versions", auth.PermView, s.ruleVersions},
		{"POST /api/v1/rules/{id}/versions/{version}/switch", auth.PermActivateRules, s.switchRuleVersion},
		{"POST /api/v1/rules/{id}/reset", auth.PermActivateRules, s.resetRule},
		{"PUT /api/v1/rules/{id}/rulebook", auth.PermActivateRules, s.updateRulebookRule}, // SPEC-010 scope 8: Partner only

		// The Formats screen (Partner only; SPEC-016).
		{"GET /api/v1/formats", auth.PermManageFormats, s.formats},
		{"POST /api/v1/formats/{id}/disable", auth.PermManageFormats, s.disableFormat},
		{"POST /api/v1/formats/{id}/enable", auth.PermManageFormats, s.enableFormat},
		{"GET /api/v1/formats/{id}/mapping", auth.PermManageFormats, s.formatMapping},
		{"POST /api/v1/formats/{id}/publish", auth.PermManageFormats, s.publishFormat},
		{"GET /api/v1/formats/{id}/export", auth.PermManageFormats, s.exportFormat},
		{"POST /api/v1/formats/import", auth.PermManageFormats, s.importFormat},
		{"POST /api/v1/format-samples/{id}/remove", auth.PermManageFormats, s.removeSample},
		{"POST /api/v1/unrecognised/{id}/resolve", auth.PermManageFormats, s.resolveUnrecognised},
		{"POST /api/v1/formats/{id}/versions/{version}/switch", auth.PermManageFormats, s.switchFormatVersion},
		{"POST /api/v1/parameters", auth.PermManageFormats, s.addParameter},
		{"POST /api/v1/parameters/{key}/disable", auth.PermManageFormats, s.disableParameter},
		{"POST /api/v1/parameters/{key}/enable", auth.PermManageFormats, s.enableParameter},
		{"POST /api/v1/parameters/{key}/aliases", auth.PermManageFormats, s.changeAlias},

		// The Admin tab (Partner only).
		{"GET /api/v1/admin/users", auth.PermManageUsers, s.adminUsers},
		{"POST /api/v1/admin/users", auth.PermManageUsers, s.adminAddUser},
		{"POST /api/v1/admin/users/{id}/role", auth.PermManageUsers, s.adminSetRole},
		{"POST /api/v1/admin/users/{id}/password", auth.PermManageUsers, s.adminResetPassword},
		{"POST /api/v1/admin/users/{id}/active", auth.PermManageUsers, s.adminSetActive},
		{"POST /api/v1/admin/users/{id}/unlock", auth.PermManageUsers, s.adminUnlock},
		{"POST /api/v1/admin/lockout", auth.PermManageUsers, s.adminLockout},
		{"GET /api/v1/admin/audit", auth.PermManageUsers, s.adminAudit},
	}
}

// Handler returns the routes: the API under /api/v1 and the UI everywhere else. The API routes
// come from endpoints(), each behind authorize.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, e := range s.endpoints() {
		mux.HandleFunc(e.pattern, s.authorize(e.pattern, e.perm, e.handler))
	}
	if s.ui != nil {
		mux.Handle("/", uiHandler(s.ui))
	}
	return securityHeaders(s.guard(mux))
}

// guard refuses requests that a web page on another site could make through the staff's
// browser, until logins with a CSRF token arrive (technical spec §B11):
//   - a Host that is not localhost, an IP address or one of Config.Hosts (DNS rebinding: a
//     page whose own name is pointed at this server could otherwise read its answers);
//   - a POST or PUT from another site (its Origin is not this server, or Sec-Fetch-Site
//     says cross-site), since a form on any page can post to the server;
//   - a POST or PUT to the API whose body is neither JSON nor a file upload.
//
// Programs that send no Origin (the tests, curl) pass the Origin check; browsers always
// send one with a POST.
func (s *Server) guard(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.knownHost(r.Host) {
			writeError(w, http.StatusMisdirectedRequest, "UNKNOWN_HOST", "the server is not reached by that name; use its IP address or a name given with --host")
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			origin := r.Header.Get("Origin")
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" ||
				(origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host) {
				writeError(w, http.StatusForbidden, "CROSS_SITE", "requests from another web site are refused")
				return
			}
			mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if strings.HasPrefix(r.URL.Path, "/api/") && mt != "application/json" && mt != "multipart/form-data" {
				writeError(w, http.StatusUnsupportedMediaType, "BAD_CONTENT_TYPE", "send JSON (application/json) or files (multipart/form-data)")
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// knownHost reports whether a request's Host names this server: localhost, an IP address
// (which no web page can re-point) or a name in Config.Hosts.
func (s *Server) knownHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return true
	}
	return slices.ContainsFunc(s.cfg.Hosts, func(n string) bool { return strings.EqualFold(n, host) })
}

// result is one engine invocation.
type result struct {
	code   int
	stdout string
	stderr string
}

func (s *Server) engine(args ...string) result { return s.engineIn("", args...) }

// engineIn runs the engine with input on its standard input (the PAN for client-add, so it
// never appears on a command line).
func (s *Server) engineIn(input string, args ...string) result {
	cmd := exec.Command(s.cfg.Engine, args...)
	cmd.Stdin = strings.NewReader(input)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		code = ee.ExitCode()
	case err != nil:
		return result{code: 3, stderr: err.Error()}
	}
	return result{code: code, stdout: out.String(), stderr: strings.TrimSpace(errOut.String())}
}

// rulesJSON asks the engine for the stored rules.
func (s *Server) rulesJSON() ([]byte, error) {
	dir, err := os.MkdirTemp(s.cfg.TempDir, "recon-rules-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "rules.json")
	s.mu.Lock()
	res := s.engine("rules", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab, "--rules", s.cfg.Rules, "--office-rules", s.cfg.OfficeRules, "--json", out)
	s.mu.Unlock()
	if res.code != 0 {
		return nil, fmt.Errorf("%s", res.stderr)
	}
	return os.ReadFile(out)
}

func (s *Server) rules(w http.ResponseWriter, _ *http.Request) {
	b, err := s.rulesJSON()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(b))
}

func (s *Server) taxYears(w http.ResponseWriter, _ *http.Request) {
	b, err := s.rulesJSON()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	var list struct {
		TaxYears []string `json:"tax_years"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tax_years": list.TaxYears})
}

// uploadTypes are the file extensions a run accepts (requirement IN-02, SPEC-008). The
// engine still identifies each file by its contents.
var uploadTypes = []string{".json", ".xlsx", ".csv"}

var (
	taxYearPattern    = regexp.MustCompile(`^\d{4}-\d{2}$`)
	unsafeName        = regexp.MustCompile(`[^A-Za-z0-9._ ()-]`)
	errorCode         = regexp.MustCompile(`\b([A-Z][A-Z_]{3,}):`)
	idPattern         = regexp.MustCompile(`^[1-9][0-9]{0,17}$`)
	comparisonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	ruleListPattern   = regexp.MustCompile(`^[A-Za-z0-9_., -]*$`)
)

// runs saves the uploaded files in a fresh temporary folder, runs the engine on them and
// returns its report and the figures it stored. The folder is removed before the response
// is written, so nothing of an upload outlives its request.
func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	status, body := s.run(w, r)
	writeJSON(w, status, body)
}

func errBody(code, msg string) map[string]string {
	return map[string]string{"error_code": code, "message": msg}
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) (int, any) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxTotal+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return http.StatusBadRequest, errBody("BAD_REQUEST", "send the files as multipart/form-data")
	}
	dir, err := os.MkdirTemp(s.cfg.TempDir, "recon-upload-*")
	if err != nil {
		return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input")
	if err := os.Mkdir(input, 0o700); err != nil {
		return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
	}

	taxYear, includeTesting, clientID := "", false, ""
	var comparisonIDs []string        // the types of check, in the order sent
	ruleChoice := map[string]string{} // comparison ID → the chosen rule IDs, separated by commas
	names := map[string]bool{}
	var total int64
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return http.StatusRequestEntityTooLarge, errBody("UPLOAD_TOO_LARGE", fmt.Sprintf("the upload could not be read; at most %d MB in all", MaxTotal>>20))
		}
		switch part.FormName() {
		case "tax_year":
			b, _ := io.ReadAll(io.LimitReader(part, 64))
			taxYear = strings.TrimSpace(string(b))
		case "include_testing":
			b, _ := io.ReadAll(io.LimitReader(part, 16))
			includeTesting = strings.TrimSpace(string(b)) == "true"
		case "client_id":
			b, _ := io.ReadAll(io.LimitReader(part, 24))
			clientID = strings.TrimSpace(string(b))
		case "comparison":
			b, _ := io.ReadAll(io.LimitReader(part, 64))
			id := strings.TrimSpace(string(b))
			if !comparisonPattern.MatchString(id) {
				return http.StatusBadRequest, errBody("BAD_COMPARISON", "comparison is the id of a type of check")
			}
			if !slices.Contains(comparisonIDs, id) {
				comparisonIDs = append(comparisonIDs, id)
			}
		case "files":
			name := unsafeName.ReplaceAllString(filepath.Base(part.FileName()), "_")
			switch {
			case name == "" || name == "." || name == "..":
				return http.StatusBadRequest, errBody("BAD_FILE_NAME", "a file has no name")
			case strings.EqualFold(filepath.Ext(name), ".xls"):
				return http.StatusUnsupportedMediaType, errBody("UNSUPPORTED_FILE_TYPE", name+": old Excel files (.xls) are not read; open it in Excel and save it as .xlsx or .csv")
			case !slices.ContainsFunc(uploadTypes, func(ext string) bool { return strings.EqualFold(filepath.Ext(name), ext) }):
				return http.StatusUnsupportedMediaType, errBody("UNSUPPORTED_FILE_TYPE", name+": only JSON, Excel (.xlsx) and CSV files can be uploaded")
			case names[strings.ToLower(name)]:
				return http.StatusBadRequest, errBody("DUPLICATE_FILE_NAME", name+" is uploaded twice")
			case len(names) == MaxFiles:
				return http.StatusBadRequest, errBody("TOO_MANY_FILES", fmt.Sprintf("at most %d files can be uploaded at once", MaxFiles))
			}
			names[strings.ToLower(name)] = true
			n, err := saveFile(filepath.Join(input, name), part)
			total += n
			switch {
			case errors.Is(err, errFileTooLarge):
				return http.StatusRequestEntityTooLarge, errBody("FILE_TOO_LARGE", fmt.Sprintf("%s is over %d MB", name, MaxFileBytes>>20))
			case err != nil || total > MaxTotal:
				return http.StatusRequestEntityTooLarge, errBody("UPLOAD_TOO_LARGE", fmt.Sprintf("the upload is over %d MB in all", MaxTotal>>20))
			}
		default:
			// rules:<comparison id> holds the rules chosen for that type (SPEC-004 part 9).
			if id, ok := strings.CutPrefix(part.FormName(), "rules:"); ok {
				b, _ := io.ReadAll(io.LimitReader(part, 4096))
				list := strings.TrimSpace(string(b))
				if !comparisonPattern.MatchString(id) || !ruleListPattern.MatchString(list) {
					return http.StatusBadRequest, errBody("BAD_RULES", "rules:<type of check> is a list of rule IDs separated by commas")
				}
				ruleChoice[id] = list
			}
		}
	}
	switch {
	case len(names) == 0:
		return http.StatusBadRequest, errBody("NO_FILES", "choose at least one JSON file")
	case !taxYearPattern.MatchString(taxYear):
		return http.StatusBadRequest, errBody("BAD_TAX_YEAR", "choose a tax year such as 2025-26")
	case clientID != "" && !idPattern.MatchString(clientID):
		return http.StatusBadRequest, errBody("BAD_CLIENT_ID", "client_id is a whole number")
	}
	for id := range ruleChoice {
		if !slices.Contains(comparisonIDs, id) {
			return http.StatusBadRequest, errBody("BAD_RULES", fmt.Sprintf("rules are chosen for %q, which is not a type of check of this run", id))
		}
	}
	// Every type and every chosen rule is checked before anything runs, so a refused type
	// never leaves the types before it stored (SPEC-004 part 9).
	if status, body, ok := s.checkRuleChoice(comparisonIDs, ruleChoice); !ok {
		return status, body
	}

	// One engine run per type of check, one after the other; with no type, one run of
	// every rule. Each run's type is stored with it, so a client keeps one result per type.
	runs := comparisonIDs
	if len(runs) == 0 {
		runs = []string{""}
	}
	runArgs := func(comparison, report string) []string {
		args := []string{"run", "--db", s.cfg.DB, "--input", input, "--tax-year", taxYear,
			"--profiles", s.cfg.Profiles, "--office-profiles", s.cfg.Office, "--office-rules", s.cfg.OfficeRules, "--vocab", s.cfg.Vocab, "--worklist", s.cfg.Worklist, "--comparisons", s.cfg.Comparisons, "--json", report}
		if includeTesting {
			args = append(args, "--include-testing")
		}
		if comparison != "" {
			args = append(args, "--comparison", comparison)
			if list, ok := ruleChoice[comparison]; ok {
				args = append(args, "--rules", list)
			}
		}
		if clientID != "" {
			// A client's run: its earlier documents count too, so the figures come from the
			// client's whole current view.
			args = append(args, "--client", clientID)
		}
		return args
	}
	reports := make([]string, len(runs))
	exitCode := 0
	s.mu.Lock()
	var res, next result
	for i, c := range runs {
		reports[i] = filepath.Join(dir, fmt.Sprintf("report-%d.json", i))
		if res = s.engine(runArgs(c, reports[i])...); res.code > 1 {
			break
		}
		exitCode = max(exitCode, res.code)
	}
	resultFile, facts := filepath.Join(dir, "result.json"), filepath.Join(dir, "facts.json")
	if res.code <= 1 {
		if clientID != "" {
			next = s.engine("result", "--db", s.cfg.DB, "--client", clientID, "--tax-year", taxYear, "--json", resultFile)
		} else {
			next = s.engine("facts", "--db", s.cfg.DB, "--input", input, "--json", facts)
		}
	}
	s.mu.Unlock()
	if status, body, failed := engineFailure(res, next); failed {
		return status, body
	}

	if clientID != "" {
		// The client's latest result of every type, including types run in earlier uploads.
		var out struct {
			Report    json.RawMessage `json:"report"`
			Results   json.RawMessage `json:"results"`
			Decisions json.RawMessage `json:"decisions"`
			Facts     json.RawMessage `json:"facts"`
		}
		b, err := os.ReadFile(resultFile)
		if err == nil {
			err = json.Unmarshal(b, &out)
		}
		if err != nil {
			return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
		}
		return http.StatusOK, map[string]any{"exit_code": exitCode, "report": out.Report, "results": out.Results, "decisions": out.Decisions, "facts": out.Facts}
	}
	results := make([]map[string]any, len(runs))
	for i, c := range runs {
		rb, err := os.ReadFile(reports[i])
		if err != nil {
			return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
		}
		results[i] = map[string]any{"comparison_id": c, "last_run_on": "", "report": json.RawMessage(bytes.TrimSpace(rb))}
	}
	fb, err := os.ReadFile(facts)
	if err != nil {
		return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
	}
	return http.StatusOK, map[string]any{"exit_code": exitCode, "report": results[0]["report"], "results": results, "facts": json.RawMessage(fb)}
}

// comparisonRules is the part of a type of check that a rule choice is checked against.
type comparisonRules struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Rules []string `json:"rules"`
}

// checkRuleChoice refuses, before any run, a type of check that does not exist, a rule
// chosen that is not one of its type's rules, and a type with rules but none chosen. The
// engine checks the same again on each run.
func (s *Server) checkRuleChoice(comparisonIDs []string, choice map[string]string) (int, any, bool) {
	if len(comparisonIDs) == 0 {
		return 0, nil, true
	}
	status, body := s.engineJSON("", "comparisons", "--comparisons", s.cfg.Comparisons, "--vocab", s.cfg.Vocab)
	if status != http.StatusOK {
		return status, body, false
	}
	var list struct {
		Comparisons []comparisonRules `json:"comparisons"`
	}
	if err := json.Unmarshal(body.(json.RawMessage), &list); err != nil {
		return http.StatusInternalServerError, errBody("INTERNAL", err.Error()), false
	}
	for _, id := range comparisonIDs {
		i := slices.IndexFunc(list.Comparisons, func(c comparisonRules) bool { return c.ID == id })
		if i < 0 {
			return http.StatusUnprocessableEntity, errBody("COMPARISON_NOT_FOUND", fmt.Sprintf("there is no type of check %q", id)), false
		}
		c := list.Comparisons[i]
		raw, chosen := choice[id]
		if !chosen {
			continue // every rule of the type runs
		}
		n := 0
		for _, r := range strings.Split(raw, ",") {
			if r = strings.TrimSpace(r); r == "" {
				continue
			}
			if !slices.Contains(c.Rules, r) {
				return http.StatusUnprocessableEntity, errBody("RULE_NOT_IN_COMPARISON", fmt.Sprintf("%s is not a rule of %q", r, c.Name)), false
			}
			n++
		}
		if n == 0 {
			return http.StatusUnprocessableEntity, errBody("NO_RULES_SELECTED", fmt.Sprintf("choose at least one rule of %q", c.Name)), false
		}
	}
	return 0, nil, true
}

// engineFailure turns a failed run (exit 2: the input or a rule; above: internal) or a
// failed follow-up command into an error response.
func engineFailure(run, next result) (int, any, bool) {
	switch {
	case run.code == 2:
		code := "INPUT_ERROR"
		if m := errorCode.FindStringSubmatch(run.stderr); m != nil {
			code = m[1]
		}
		status := http.StatusUnprocessableEntity
		switch code {
		case "CLIENT_NOT_FOUND":
			status = http.StatusNotFound
		case "CLIENT_DISABLED":
			status = http.StatusConflict
		}
		return status, errBody(code, strings.TrimPrefix(run.stderr, "recon-engine run: ")), true
	case run.code > 2 || next.code != 0:
		return http.StatusInternalServerError, errBody("INTERNAL", run.stderr+next.stderr), true
	}
	return 0, nil, false
}

// clients lists the clients with their latest counts for a tax year.
func (s *Server) clients(w http.ResponseWriter, r *http.Request) {
	ty := r.URL.Query().Get("tax_year")
	if !taxYearPattern.MatchString(ty) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	args := []string{"clients", "--db", s.cfg.DB, "--tax-year", ty}
	if r.URL.Query().Get("all") == "1" {
		args = append(args, "--all") // disabled clients too (SPEC-004 part 10)
	}
	status, body := s.engineJSON("", args...)
	writeJSON(w, status, body)
}

// disableClient takes a client out of use (SPEC-004 part 10). The reason is required; the name
// is the signed-in user's. Nothing is deleted.
func (s *Server) disableClient(w http.ResponseWriter, r *http.Request) {
	s.setClientDisabled(w, r, "client-disable")
}

// enableClient takes a disabled client back into use. The reason is required.
func (s *Server) enableClient(w http.ResponseWriter, r *http.Request) {
	s.setClientDisabled(w, r, "client-enable")
}

func (s *Server) setClientDisabled(w http.ResponseWriter, r *http.Request, command string) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decodeAuthBody(w, r, &in, `send {"reason": "..."} as JSON`) {
		return
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r), "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), command, "--db", s.cfg.DB, "--client", id)
	writeJSON(w, status, out)
}

// clientRequest is the body of POST and PUT /api/v1/clients: the client's details, and for
// a new client the tax year to open. The older {"name", "pan"} form is still accepted.
type clientRequest struct {
	TaxYear string         `json:"tax_year"`
	Details client.Details `json:"details"`
	Name    string         `json:"name"`
	PAN     string         `json:"pan"`
}

// readClient decodes and checks a client request. It writes the error response itself and
// reports false when the request is refused.
func readClient(w http.ResponseWriter, r *http.Request, panRequired bool) (clientRequest, bool) {
	var in clientRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"tax_year", "details": {...}} as JSON`)
		return in, false
	}
	if in.Name != "" || in.PAN != "" {
		in.Details.LegalName, in.Details.PAN = in.Name, in.PAN
	}
	in.Details.Normalise()
	in.Details.PANLast4 = ""
	if problems := in.Details.Validate(in.Details.PAN, panRequired); len(problems) > 0 {
		code, msg := "INVALID_CLIENT", problems[0].Message
		if len(problems) == 1 {
			code = problems[0].Code
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error_code": code, "message": msg, "problems": problems})
		return in, false
	}
	return in, true
}

// addClient adds a client with all its details. The details, PAN included, go to the
// engine on its standard input; the PAN is never logged, kept by the server or returned.
func (s *Server) addClient(w http.ResponseWriter, r *http.Request) {
	in, ok := readClient(w, r, true)
	if !ok {
		return
	}
	if !taxYearPattern.MatchString(in.TaxYear) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	details, err := json.Marshal(in.Details)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, body := s.engineJSON(string(details), "client-add", "--db", s.cfg.DB, "--tax-year", in.TaxYear)
	if status == http.StatusOK {
		status = http.StatusCreated
	}
	writeJSON(w, status, body)
}

// updateClient replaces a client's details; an empty PAN keeps the PAN.
func (s *Server) updateClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	in, ok := readClient(w, r, false)
	if !ok {
		return
	}
	details, err := json.Marshal(in.Details)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, body := s.engineJSON(string(details), "client-update", "--db", s.cfg.DB, "--client", id)
	writeJSON(w, status, body)
}

// clientDetails returns all of a client's details (never the full PAN).
func (s *Server) clientDetails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	ty := r.URL.Query().Get("tax_year")
	if ty != "" && !taxYearPattern.MatchString(ty) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	status, body := s.engineJSON("", "client-get", "--db", s.cfg.DB, "--client", id)
	if raw, ok := body.(json.RawMessage); ok && status == http.StatusOK {
		acc := accountFrom(r)
		if err := s.recordView(acc, id, ty); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		if !seesFullIdentity(acc.Role) {
			masked, err := maskDetails(raw)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			body = json.RawMessage(masked)
		}
	}
	writeJSON(w, status, body)
}

// decide records a decision on one of a client's findings (SPEC-006 part 3): Fix, Not an
// issue (with a note) or Ask client, with the name of who decides. The engine checks it and
// keeps it; the answer is the finding's decisions, newest last.
func (s *Server) decide(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	var in struct {
		TaxYear   string `json:"tax_year"`
		FindingID string `json:"finding_id"`
		Decision  string `json:"decision"`
		Note      string `json:"note"`
		By        string `json:"by"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send the decision as JSON: tax_year, finding_id, decision, note, by")
		return
	}
	if !taxYearPattern.MatchString(in.TaxYear) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	body, err := json.Marshal(map[string]string{"finding_id": in.FindingID, "decision": in.Decision, "note": in.Note, "by": actorOf(r)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "decide", "--db", s.cfg.DB, "--client", id, "--tax-year", in.TaxYear)
	writeJSON(w, status, out)
}

// worklist lists the Worklist items of every client for a tax year, with their
// classifications (SPEC-006 part 3).
func (s *Server) worklist(w http.ResponseWriter, r *http.Request) {
	ty := r.URL.Query().Get("tax_year")
	if !taxYearPattern.MatchString(ty) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	status, body := s.engineJSON("", "worklist", "--db", s.cfg.DB, "--tax-year", ty)
	if raw, ok := body.(json.RawMessage); ok && status == http.StatusOK {
		// Each client whose items are listed has its Worklist viewed (SPEC-013 §2).
		ids, err := workItemClients(raw)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		acc := accountFrom(r)
		for _, id := range ids {
			if err := s.recordView(acc, id, ty); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
		}
	}
	writeJSON(w, status, body)
}

// classify records the classification staff gave one of a client's Worklist items. The
// engine checks that the item is in the client's latest Worklist and keeps it; the answer
// is the client's items. The tool never classifies (CLAUDE.md invariant 2).
func (s *Server) classify(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	var in struct {
		TaxYear        string `json:"tax_year"`
		Kind           string `json:"kind"`
		Ledger         string `json:"ledger"`
		Classification string `json:"classification"`
		By             string `json:"by"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send the classification as JSON: tax_year, kind, ledger, classification, by")
		return
	}
	if !taxYearPattern.MatchString(in.TaxYear) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	body, err := json.Marshal(map[string]string{"kind": in.Kind, "ledger": in.Ledger, "classification": in.Classification, "by": actorOf(r)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "classify", "--db", s.cfg.DB, "--client", id, "--tax-year", in.TaxYear)
	writeJSON(w, status, out)
}

// comparisons lists the types of check the CA can pick.
func (s *Server) comparisons(w http.ResponseWriter, _ *http.Request) {
	status, body := s.engineJSON("", "comparisons", "--comparisons", s.cfg.Comparisons, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, body)
}

// clientResult returns a client's latest report and current figures for a tax year.
func (s *Server) clientResult(w http.ResponseWriter, r *http.Request) {
	id, ty := r.PathValue("id"), r.URL.Query().Get("tax_year")
	switch {
	case !idPattern.MatchString(id):
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	case !taxYearPattern.MatchString(ty):
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	status, body := s.engineJSON("", "result", "--db", s.cfg.DB, "--client", id, "--tax-year", ty)
	if status == http.StatusOK {
		shown, err := s.shownResult(r, id, ty, body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		body = shown
	}
	writeJSON(w, status, body)
}

// shownResult applies the client file's rules to a result or a history the engine answered: the
// view is recorded (one entry per user, client and tax year a minute, SPEC-013 §2), and the
// findings deleted by a Partner are hidden from the other roles.
func (s *Server) shownResult(r *http.Request, clientID, taxYear string, body any) (any, error) {
	acc := accountFrom(r)
	if err := s.recordView(acc, clientID, taxYear); err != nil {
		return nil, err
	}
	raw, ok := body.(json.RawMessage)
	if !ok {
		return body, nil
	}
	deleted, err := s.deletedFindings(clientID, taxYear)
	if err != nil {
		return nil, err
	}
	out, err := filterDeleted(raw, acc.Role == auth.RolePartner, deleted)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

// clientHistory lists a client's runs of one type of check as Comparison 1, 2, 3… for a tax
// year, or with no= gives one of them as it was reported, to explore read only (SPEC-009).
func (s *Server) clientHistory(w http.ResponseWriter, r *http.Request) {
	id, q := r.PathValue("id"), r.URL.Query()
	ty, comparison, no := q.Get("tax_year"), q.Get("comparison"), q.Get("no")
	switch {
	case !idPattern.MatchString(id):
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	case !taxYearPattern.MatchString(ty):
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	case comparison != "" && !comparisonPattern.MatchString(comparison):
		writeError(w, http.StatusBadRequest, "BAD_COMPARISON", "choose a type of check")
		return
	case no != "" && !idPattern.MatchString(no):
		writeError(w, http.StatusBadRequest, "BAD_COMPARISON_NO", "the comparison number is a whole number")
		return
	}
	args := []string{"history", "--db", s.cfg.DB, "--client", id, "--tax-year", ty, "--comparison", comparison}
	if no != "" {
		args = append(args, "--no", no)
	}
	status, body := s.engineJSON("", args...)
	if status == http.StatusOK {
		shown, err := s.shownResult(r, id, ty, body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		body = shown
	}
	writeJSON(w, status, body)
}

// engineJSON runs an engine command that writes JSON with --json and returns that JSON, or
// the engine's refusal (exit 2) with its stable code: 404 for an unknown client, 409 for a
// client that already exists, 422 otherwise.
func (s *Server) engineJSON(input string, args ...string) (int, any) {
	dir, err := os.MkdirTemp(s.cfg.TempDir, "recon-json-*")
	if err != nil {
		return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.json")
	s.mu.Lock()
	res := s.engineIn(input, append(args, "--json", out)...)
	s.mu.Unlock()
	switch {
	case res.code == 2:
		code := "INPUT_ERROR"
		if m := errorCode.FindStringSubmatch(res.stderr); m != nil {
			code = m[1]
		}
		status := http.StatusUnprocessableEntity
		switch code {
		case "CLIENT_NOT_FOUND", "FINDING_NOT_FOUND", "WORKLIST_ITEM_NOT_FOUND", "COMPARISON_NOT_FOUND", "RULE_NOT_FOUND", "VERSION_NOT_FOUND", "FORMAT_NOT_FOUND", "PARAMETER_NOT_FOUND", "NOT_FOUND":
			status = http.StatusNotFound
		case "CLIENT_EXISTS", "CLIENT_DISABLED", "ALREADY_DISABLED", "NOT_DISABLED", "ALREADY_IN_USE", "FORMAT_READ_ONLY", "PARAMETER_READ_ONLY", "PARAMETER_EXISTS", "PARAMETER_IN_USE", "ALIAS_TAKEN", "SAMPLES_CHANGED", "FORMAT_EXISTS", "ALREADY_PUBLISHED":
			status = http.StatusConflict
		}
		msg := res.stderr
		if i := strings.Index(msg, code+": "); i >= 0 {
			msg = msg[i+len(code)+2:]
		}
		body := map[string]any{"error_code": code, "message": msg}
		// A rule the engine refuses lists each problem with its field (SPEC-010 AC-3).
		if b, err := os.ReadFile(out); err == nil {
			var refused struct {
				Problems json.RawMessage `json:"problems"`
			}
			if json.Unmarshal(b, &refused) == nil && len(refused.Problems) > 0 {
				body["problems"] = refused.Problems
			}
		}
		return status, body
	case res.code != 0:
		return http.StatusInternalServerError, errBody("INTERNAL", res.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return http.StatusInternalServerError, errBody("INTERNAL", err.Error())
	}
	return http.StatusOK, json.RawMessage(bytes.TrimSpace(b))
}

var errFileTooLarge = errors.New("file too large")

// saveFile copies one uploaded file, refusing one over MaxFileBytes.
func saveFile(path string, r io.Reader) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, io.LimitReader(r, MaxFileBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > MaxFileBytes {
		err = errFileTooLarge
	}
	return n, err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error_code": code, "message": msg})
}

// uiHandler serves the built UI read-only; an unknown path gets index.html, so a reload on
// any screen still opens the app.
func uiHandler(files fs.FS) http.Handler {
	static := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "read only", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimLeft(r.URL.Path, "/")
		if p == "" {
			p = "."
		}
		if _, err := fs.Stat(files, p); err != nil {
			http.ServeFileFS(w, r, files, "index.html")
			return
		}
		static.ServeHTTP(w, r)
	})
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		h.ServeHTTP(w, r)
	})
}

// oneFile reads a multipart upload of one JSON or Excel file ("file"; with csv, a CSV file too) and short text fields. The
// file is saved in dir under its own (cleaned) name.
func oneFile(w http.ResponseWriter, r *http.Request, dir string, fields map[string]*string, csv bool) (string, int, any) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxFileBytes+2<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return "", http.StatusBadRequest, errBody("BAD_REQUEST", "send the file as multipart/form-data")
	}
	path := ""
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", http.StatusRequestEntityTooLarge, errBody("FILE_TOO_LARGE", fmt.Sprintf("the upload could not be read; at most %d MB", MaxFileBytes>>20))
		}
		if dst, ok := fields[part.FormName()]; ok {
			b, _ := io.ReadAll(io.LimitReader(part, 1<<20))
			*dst = string(b)
			continue
		}
		if part.FormName() != "file" || path != "" {
			continue
		}
		name := unsafeName.ReplaceAllString(filepath.Base(part.FileName()), "_")
		switch {
		case name == "" || name == "." || name == "..":
			return "", http.StatusBadRequest, errBody("BAD_FILE_NAME", "the file has no name")
		case strings.EqualFold(filepath.Ext(name), ".csv") && csv:
		case !strings.EqualFold(filepath.Ext(name), ".json") && !strings.EqualFold(filepath.Ext(name), ".xlsx"):
			return "", http.StatusUnsupportedMediaType, errBody("UNSUPPORTED_FILE_TYPE", name+": only JSON, Excel (.xlsx) and CSV layouts can be taught")
		}
		path = filepath.Join(dir, name)
		if _, err := saveFile(path, part); errors.Is(err, errFileTooLarge) {
			return "", http.StatusRequestEntityTooLarge, errBody("FILE_TOO_LARGE", fmt.Sprintf("%s is over %d MB", name, MaxFileBytes>>20))
		} else if err != nil {
			return "", http.StatusInternalServerError, errBody("INTERNAL", err.Error())
		}
	}
	if path == "" {
		return "", http.StatusBadRequest, errBody("NO_FILES", "choose a JSON or Excel (.xlsx) file")
	}
	return path, 0, nil
}

// inspect lists the fields of an uploaded file for the mapping screen (SPEC-004 part 8.4).
func (s *Server) inspect(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp(s.cfg.TempDir, "recon-inspect-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	defer os.RemoveAll(dir)
	path, status, body := oneFile(w, r, dir, nil, true) // a CSV is inspected for its tax year
	if path == "" {
		writeJSON(w, status, body)
		return
	}
	status, body = s.engineJSON("", "inspect", "--input", path, "--profiles", s.cfg.Profiles,
		"--office-profiles", s.cfg.Office, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, body)
}

// createProfile teaches a layout: the mapping (JSON) and a file in the layout. With
// check_only=true it only previews the rows. A refused mapping is 422 INVALID_MAPPING with
// each problem; a name already taught is 409 PROFILE_EXISTS.
func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp(s.cfg.TempDir, "recon-teach-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	defer os.RemoveAll(dir)
	var mapping, checkOnly, format, keep string
	path, status, body := oneFile(w, r, dir, map[string]*string{"mapping": &mapping, "check_only": &checkOnly, "format": &format, "keep_sample": &keep}, true)
	if path == "" {
		writeJSON(w, status, body)
		return
	}
	if strings.TrimSpace(mapping) == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "the mapping is missing")
		return
	}
	out := filepath.Join(dir, "out.json")
	args := []string{"profile-create", "--sample", path, "--profiles", s.cfg.Profiles, "--office-profiles", s.cfg.Office,
		"--vocab", s.cfg.Vocab, "--db", s.cfg.DB, "--by", actorOf(r), "--rules", s.cfg.Rules, "--json", out}
	if format = strings.TrimSpace(format); format != "" {
		// Teaching a layout needs the run permission; editing a format is the Partner's (SPEC-016).
		if !auth.Allowed(accountFrom(r).Role, auth.PermManageFormats) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error_code": "FORBIDDEN", "message": "your role does not allow this: it needs the manage_formats permission", "permission": string(auth.PermManageFormats)})
			return
		}
		if !formatIDPattern.MatchString(format) {
			writeError(w, http.StatusBadRequest, "BAD_FORMAT_ID", "that is not a format ID")
			return
		}
		args = append(args, "--format", format)
	}
	// Keeping the sample file stores its bytes, which may hold client figures: only the Partner may (SPEC-016).
	if strings.TrimSpace(keep) == "true" {
		if !auth.Allowed(accountFrom(r).Role, auth.PermManageFormats) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error_code": "FORBIDDEN", "message": "your role does not allow this: it needs the manage_formats permission", "permission": string(auth.PermManageFormats)})
			return
		}
		args = append(args, "--keep-sample")
	}
	if strings.TrimSpace(checkOnly) == "true" {
		args = append(args, "--check-only")
	}
	s.mu.Lock()
	res := s.engineIn(mapping, args...)
	s.mu.Unlock()
	switch {
	case res.code == 2:
		code := "INPUT_ERROR"
		if m := errorCode.FindStringSubmatch(res.stderr); m != nil {
			code = m[1]
		}
		var refused struct {
			Problems []map[string]string `json:"problems"`
		}
		if b, err := os.ReadFile(out); err == nil {
			_ = json.Unmarshal(b, &refused)
		}
		status := http.StatusUnprocessableEntity
		msg := res.stderr
		if i := strings.Index(msg, code+": "); i >= 0 {
			msg = msg[i+len(code)+2:]
		}
		for _, p := range refused.Problems {
			if p["code"] == "PROFILE_EXISTS" {
				status = http.StatusConflict
			}
		}
		if len(refused.Problems) > 0 {
			msg = refused.Problems[0]["message"]
		}
		writeJSON(w, status, map[string]any{"error_code": code, "message": msg, "problems": refused.Problems})
		return
	case res.code != 0:
		writeError(w, http.StatusInternalServerError, "INTERNAL", res.stderr)
		return
	}
	b, err := os.ReadFile(out)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(bytes.TrimSpace(b)))
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

var (
	teachData    = filepath.Join(repo, "testdata", "teach")
	teachSample  = filepath.Join(teachData, "office_sales_Aug2026.json")
	teachMapping = filepath.Join(teachData, "office_sales.mapping.json")
)

func readJSON(t *testing.T, file string, v any) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
}

func mappingText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(teachMapping)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func createProfile(t *testing.T, office, mapping, sample string, extra ...string) (result, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.json")
	args := append([]string{"profile-create", "--office-profiles", office, "--sample", sample,
		"--profiles", profDir, "--vocab", vocabDir, "--json", out}, extra...)
	return withStdin(t, mapping, args...), out
}

// AC-29: a file in an unknown layout is refused, inspected, taught once on the mapping
// screen, and from then on read like any other.
func TestTeachLayout(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := newDB(t)
	run := func() result {
		return runCmd(t, db, teachSample, "--tax-year", "2026-27", "--office-profiles", office)
	}
	if r := run(); r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_NO_MATCH: office_sales_Aug2026.json") {
		t.Fatalf("before teaching: exit %d\n%s", r.code, r.stderr)
	}

	// inspect lists every field once, with list positions as [*], and the teachable types.
	ins := filepath.Join(t.TempDir(), "inspect.json")
	if r := cmd(t, "inspect", "--input", teachSample, "--profiles", profDir, "--vocab", vocabDir, "--json", ins); r.code != exitOK {
		t.Fatalf("inspect: exit %d\n%s", r.code, r.stderr)
	}
	var in struct {
		RecognisedAs  string                   `json:"recognised_as"`
		Fields        []profile.InspectedField `json:"fields"`
		Lists         []profile.InspectedList  `json:"lists"`
		DocumentTypes []teachableType          `json:"document_types"`
	}
	readJSON(t, ins, &in)
	if in.RecognisedAs != "" || len(in.Lists) != 1 || in.Lists[0] != (profile.InspectedList{Path: "$.Vouchers[*]", Count: 5}) {
		t.Errorf("inspect: recognised %q, lists %+v", in.RecognisedAs, in.Lists)
	}
	want := profile.InspectedField{Path: "$.Vouchers[*]['Bill No']", Kind: "text", Example: "DBS/26-27/0451", Count: 5}
	found := false
	for _, f := range in.Fields {
		f.Values = nil // the different values are checked in the profile package
		found = found || reflect.DeepEqual(f, want)
	}
	if !found {
		t.Errorf("inspect fields lack %+v: %+v", want, in.Fields)
	}
	var names []string
	for _, d := range in.DocumentTypes {
		names = append(names, d.Name)
	}
	if strings.Join(names, ",") != "bank_book,bank_statement,einvoice,ewaybill,form168,gstr1,gstr2a,gstr2b,gstr3b,ims,ledger,oltas_status,purchase_register,sales_register,tds_ledger,tds_receivable" {
		t.Errorf("teachable document types = %v", names)
	}

	// --check-only previews the rows and saves nothing.
	r, out := createProfile(t, office, mappingText(t), teachSample, "--check-only")
	if r.code != exitOK {
		t.Fatalf("check-only: exit %d\n%s", r.code, r.stderr)
	}
	var created struct {
		ProfileID string           `json:"profile_id"`
		Saved     bool             `json:"saved"`
		Rows      int              `json:"rows"`
		Period    string           `json:"period"`
		PANLast4  string           `json:"pan_last4"`
		Preview   []map[string]any `json:"preview"`
	}
	readJSON(t, out, &created)
	if created.Saved || created.Rows != 4 || created.Period != "2026-08" || created.PANLast4 != "565Z" || len(created.Preview) != 4 {
		t.Errorf("check-only outcome = %+v", created)
	}
	if _, err := os.Stat(office); err == nil {
		t.Error("check-only saved something")
	}

	// Saving writes the profile, its sample and its expected result; the run then reads
	// the file as the taught layout, and the saved profile passes its own sample test.
	if r, out = createProfile(t, office, mappingText(t), teachSample); r.code != exitOK {
		t.Fatalf("profile-create: exit %d\n%s", r.code, r.stderr)
	}
	readJSON(t, out, &created)
	if !created.Saved || created.ProfileID != "office.office_sales_export.sales_register" {
		t.Errorf("profile-create outcome = %+v", created)
	}
	if r := run(); r.code > exitFindings || !strings.Contains(r.stdout, "Taught in this office: Office sales export") ||
		!strings.Contains(r.stdout, "Rows read     : 4") {
		t.Errorf("after teaching: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := loadProfiles(profDir, office, v)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := set.Lookup(created.ProfileID, "1")
	if !ok {
		t.Fatal("the taught profile is not loaded")
	}
	reg := ingest.NewRegistry()
	if err := reg.Register(jsonadapter.New(0)); err != nil {
		t.Fatal(err)
	}
	for _, s := range profile.TestSamples(p, set, reg) {
		if len(s.Problems) > 0 {
			t.Errorf("sample test of the taught profile: %v", s.Problems)
		}
	}

	// The same name again is refused, and so is a mapping for a file already known.
	if r, _ := createProfile(t, office, mappingText(t), teachSample); r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_EXISTS") {
		t.Errorf("taught twice: exit %d\n%s", r.code, r.stderr)
	}
	renamed := strings.Replace(mappingText(t), "Office sales export", "Second name", 1)
	if r, _ := createProfile(t, office, renamed, teachSample); r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_AMBIGUOUS") {
		t.Errorf("a layout already taught: exit %d\n%s", r.code, r.stderr)
	}
}

// A mapping that cannot work is refused with each problem, in the user's terms.
func TestTeachLayoutRefused(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	for name, c := range map[string]struct{ from, to, code string }{
		"no key column":   {`"name": "invoice_no"`, `"name": "place_of_supply"`, "MISSING_KEY_COLUMN"},
		"unknown column":  {`"name": "cess"`, `"name": "discount"`, "UNKNOWN_COLUMN"},
		"rows not a list": {`"rows": "$.Vouchers[*]"`, `"rows": "$.Vouchers"`, "BAD_ROWS"},
		"field not there": {`"path": ".Amounts.Taxable"`, `"path": ".Amounts.Net"`, "MAPPING_DOES_NOT_FIT"},
		"bad period":      {`"path": "$.Header.Month"`, `"path": "$.Header['Company GSTIN']"`, "MAPPING_DOES_NOT_READ"},
	} {
		t.Run(name, func(t *testing.T) {
			m := mappingText(t)
			if !strings.Contains(m, c.from) {
				t.Fatalf("mapping lacks %s", c.from)
			}
			r, out := createProfile(t, office, strings.Replace(m, c.from, c.to, 1), teachSample)
			if r.code != exitInput || !strings.Contains(r.stderr, "INVALID_MAPPING") {
				t.Fatalf("exit %d\n%s", r.code, r.stderr)
			}
			var got struct {
				Problems []profile.MappingProblem `json:"problems"`
			}
			readJSON(t, out, &got)
			codes := []string{}
			for _, p := range got.Problems {
				codes = append(codes, p.Code)
			}
			if !strings.Contains(strings.Join(codes, ","), c.code) && !(c.code == "MAPPING_DOES_NOT_READ" && len(codes) > 0) {
				t.Errorf("problems %v, want %s", got.Problems, c.code)
			}
		})
	}
	if _, err := os.Stat(office); err == nil {
		t.Error("a refused mapping saved something")
	}
}

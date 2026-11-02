package profile

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// SPEC-005: a file that holds several documents (a bundle) is read once per profile of the
// bundle; any other double match is still refused, and a value not read is listed once.

const packJSON = `{
  "meta": { "kind": "PACK", "ty": "2025-26", "pan": "AAACZ1234Q", "note": "free text" },
  "audit": { "dep_total": 135862, "remark": "x" },
  "comp": { "allowed": 138310, "schedule": 135862 },
  "extra": { "form_67": "not read by any profile" }
}`

// bundleProfile is one document of the test pack: its document type, id, bundle, section
// and facts ("<fact> <path>" lines).
func bundleProfile(docType, id, bundle, section string, facts ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `schema: "recon.profile/1"
profile_id: "%s"
profile_version: "1"
source_system: "Test pack"
document_type: "%s"
file_type: "json"
regime: "ITA1961"
`, id, docType)
	if bundle != "" {
		fmt.Fprintf(&b, "bundle: %q\nsection: %q\n", bundle, section)
	}
	b.WriteString(`detect:
  - json_equals: { path: "$.meta.kind", value: "PACK" }
context:
  tax_year: { json_path: "$.meta.ty" }
  pan: { json_path: "$.meta.pan" }
  period: { const: "FY" }
facts:
`)
	for _, f := range facts {
		name, path, _ := strings.Cut(f, " ")
		fmt.Fprintf(&b, "  - { fact: %q, json_path: %q, unit: \"rupees\", required: true }\n", name, path)
	}
	return b.String()
}

func audit(bundle string) string {
	return bundleProfile("tax_audit_report", "test.pack.audit", bundle, "$.audit", "tar.depreciation_schedule.total $.audit.dep_total")
}

func comp(bundle string) string {
	return bundleProfile("computation", "test.pack.computation", bundle, "$.comp",
		"computation.allowed_depreciation $.comp.allowed", "computation.schedule_total $.comp.schedule")
}

func set(t *testing.T, texts ...string) *Set {
	t.Helper()
	var ps []*Profile
	for _, text := range texts {
		ps = append(ps, load(t, text))
	}
	s, err := NewSet(ps...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBundleReadsEachDocumentOnce(t *testing.T) {
	doc := openJSON(t, packJSON)
	parts, err := DetectAll(set(t, comp("test.pack"), audit("test.pack")), doc, "pack.json")
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, p := range parts {
		types = append(types, p.DocumentType)
	}
	if !slices.Equal(types, []string{"computation", "tax_audit_report"}) {
		t.Fatalf("documents %v", types)
	}
	results, err := ExtractAll(parts, doc, Source{FileName: "pack.json"})
	if err != nil {
		t.Fatal(err)
	}
	unread := func(r *Result) []string {
		var out []string
		for _, u := range r.Report.Unread {
			out = append(out, u.Locator.Path+" | "+string(u.Kind)+" | "+u.Reason)
		}
		return out
	}
	// The computation (first by document type) carries what lies outside every section.
	if got, want := unread(results[0]), []string{
		"$.extra.form_67 | not_read | text; not part of any document this file holds",
		"$.meta.note | not_read | text; not part of any document this file holds",
	}; !slices.Equal(got, want) {
		t.Errorf("computation unread %q, want %q", got, want)
	}
	if got, want := unread(results[1]), []string{"$.audit.remark | not_read | text; this format does not read this part of the file"}; !slices.Equal(got, want) {
		t.Errorf("audit unread %q, want %q", got, want)
	}
	if len(results[0].Facts) != 2 || len(results[1].Facts) != 1 || results[1].Report.DocumentType != "tax_audit_report" {
		t.Errorf("facts %v / %v", results[0].Facts, results[1].Facts)
	}
}

func TestBundleOtherDoubleMatchesRefused(t *testing.T) {
	doc := openJSON(t, packJSON)
	for name, s := range map[string]*Set{
		"two bundles":            set(t, comp("test.pack"), audit("other.pack")),
		"a bundle and a profile": set(t, comp("test.pack"), audit("")),
	} {
		_, err := DetectAll(s, doc, "pack.json")
		var ie *ingest.Error
		if !errors.As(err, &ie) || ie.Code != ingest.ProfileAmbiguous {
			t.Errorf("%s: err %v, want PROFILE_AMBIGUOUS", name, err)
		}
	}
	// Detect, which wants one profile, refuses a bundle file too.
	if _, err := Detect(set(t, comp("test.pack"), audit("test.pack")), doc, "pack.json"); err == nil {
		t.Error("Detect accepted a file that holds two documents")
	}
}

// A figure may come from outside the profile's section; it is then not listed as unread by
// the document whose section holds it.
func TestBundleFigureOutsideSection(t *testing.T) {
	doc := openJSON(t, packJSON)
	audit := bundleProfile("tax_audit_report", "test.pack.audit", "test.pack", "$.audit",
		"tar.depreciation_schedule.total $.audit.dep_total", "tar.other_total $.comp.schedule")
	comp := bundleProfile("computation", "test.pack.computation", "test.pack", "$.comp", "computation.allowed_depreciation $.comp.allowed")
	parts, err := DetectAll(set(t, comp, audit), doc, "pack.json")
	if err != nil {
		t.Fatal(err)
	}
	results, err := ExtractAll(parts, doc, Source{FileName: "pack.json"})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range results[0].Report.Unread {
		if u.Locator.Path == "$.comp.schedule" {
			t.Errorf("$.comp.schedule is read by the audit report's profile but listed as unread in the computation")
		}
	}
}

func TestBundleProfileRefusals(t *testing.T) {
	v := testVocab(t)
	for name, text := range map[string]string{
		"bundle without section": strings.Replace(audit("test.pack"), "section: \"$.audit\"\n", "", 1),
		"badly named bundle":     audit("Test Pack"),
	} {
		if _, err := Load("profile.yaml", []byte(text), v); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

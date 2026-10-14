package vocab

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// factWith loads goodFacts with the name of the first fact (a tar.* fact) replaced.
func factWith(name string) string {
	return strings.Replace(goodFacts, `"tar.depreciation_schedule.total"`, name, 1)
}

func TestExtraFactNameValidation(t *testing.T) {
	valid := []string{"tar.a", "tar.a.b.c", "tar.a1", "tar.1", "tar._x", "tar.x_", "tar.0_9", "tar.a__b", "tar.depreciation_schedule.total"}
	invalid := []string{
		"tar", "tar.", ".tar.a", "tar..a", "tar.a.", "Tar.a", "tar.A", "tar.a-b", "tar.a b", " tar.a", "tar.a ", "tar.a\n", "tar.a\t",
		"1tar.a", "_tar.a", "tar.é", "tar.a\u200b", "tar/a", "tar:a", "tar.a,b", "TAR.A", "tar.a.\u00e9", "",
	}
	for _, n := range valid {
		t.Run("valid "+n, func(t *testing.T) {
			if _, err := load(t, factWith(`"`+n+`"`), goodRecords); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
	for _, n := range invalid {
		t.Run(fmt.Sprintf("invalid %q", n), func(t *testing.T) {
			_, err := load(t, factWith(`"`+n+`"`), goodRecords)
			var es strictyaml.Errors
			if !errors.As(err, &es) {
				t.Fatalf("accepted %q (err=%v)", n, err)
			}
			if !strings.Contains(err.Error(), "facts.yaml:13:") || !strings.Contains(err.Error(), "fact name") {
				t.Errorf("not reported at the fact name line: %v", err)
			}
		})
	}
}

func TestExtraSimpleNameValidation(t *testing.T) {
	valid := []string{"a", "a1", "a_b", "a__", "abc_def_9"}
	invalid := []string{"", "1a", "_a", "A", "aB", "a.b", "a-b", "a b", "a\n", "é", "a\u00e9"}
	for _, n := range valid {
		t.Run("record valid "+n, func(t *testing.T) {
			if _, err := load(t, goodFacts, strings.Replace(goodRecords, `"gst_invoice"`, `"`+n+`"`, 1)); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
		t.Run("column valid "+n, func(t *testing.T) {
			r := strings.Replace(goodRecords, `"place_of_supply"`, `"`+n+`"`, 1)
			if _, err := load(t, goodFacts, r); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
		t.Run("document type and prefix valid "+n, func(t *testing.T) {
			f := strings.Replace(goodFacts, `prefix: "tar"`, `prefix: "`+n+`"`, 1)
			f = strings.Replace(f, `"tar.depreciation_schedule.total"`, `"`+n+`.depreciation_schedule.total"`, 1)
			if _, err := load(t, f, goodRecords); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
	for _, n := range invalid {
		t.Run(fmt.Sprintf("record invalid %q", n), func(t *testing.T) {
			if _, err := load(t, goodFacts, strings.Replace(goodRecords, `"gst_invoice"`, `"`+n+`"`, 1)); err == nil {
				t.Errorf("record type %q accepted", n)
			}
		})
		t.Run(fmt.Sprintf("column invalid %q", n), func(t *testing.T) {
			if _, err := load(t, goodFacts, strings.Replace(goodRecords, `"place_of_supply"`, `"`+n+`"`, 1)); err == nil {
				t.Errorf("column %q accepted", n)
			}
		})
		t.Run(fmt.Sprintf("prefix invalid %q", n), func(t *testing.T) {
			if _, err := load(t, strings.Replace(goodFacts, `prefix: "tar"`, `prefix: "`+n+`"`, 1), goodRecords); err == nil {
				t.Errorf("prefix %q accepted", n)
			}
		})
		t.Run(fmt.Sprintf("document type invalid %q", n), func(t *testing.T) {
			if _, err := load(t, strings.Replace(goodFacts, `- name: "tax_audit_report"`, `- name: "`+n+`"`, 1), goodRecords); err == nil {
				t.Errorf("document type %q accepted", n)
			}
		})
	}
}

func TestExtraKindsAndPeriodicity(t *testing.T) {
	for _, k := range []string{"money", "int", "text", "date", "bool", "rate"} {
		t.Run("kind "+k, func(t *testing.T) {
			v := mustLoad(t, strings.Replace(goodFacts, `kind: "text"`, `kind: "`+k+`"`, 1), goodRecords)
			f, _ := v.Lookup("computation.tds.section")
			want, ok := domain.ParseValueKind(k)
			if !ok || f.Kind != want {
				t.Errorf("kind = %v, want %v", f.Kind, want)
			}
		})
	}
	for _, k := range []string{"", "Money", "MONEY", "float", "decimal", "number", "string", "paise", "money ", " money", "int64", "unit"} {
		t.Run(fmt.Sprintf("bad kind %q", k), func(t *testing.T) {
			_, err := load(t, strings.Replace(goodFacts, `kind: "text"`, `kind: "`+k+`"`, 1), goodRecords)
			if err == nil || !strings.Contains(err.Error(), "is unknown (allowed: money, int, text, date, bool, rate)") {
				t.Errorf("err = %v", err)
			}
		})
	}
	for _, p := range []string{"FY", "quarter", "month"} {
		t.Run("periodicity "+p, func(t *testing.T) {
			v := mustLoad(t, strings.Replace(goodFacts, `periodicity: "quarter"`, `periodicity: "`+p+`"`, 1), goodRecords)
			f, _ := v.Lookup("computation.tds.section")
			if string(f.Periodicity) != p {
				t.Errorf("periodicity = %q", f.Periodicity)
			}
		})
	}
	for _, p := range []string{"", "fy", "Fy", "Quarter", "MONTH", "year", "week", "day", "annual", "FY "} {
		t.Run(fmt.Sprintf("bad periodicity %q", p), func(t *testing.T) {
			if _, err := load(t, strings.Replace(goodFacts, `periodicity: "quarter"`, `periodicity: "`+p+`"`, 1), goodRecords); err == nil {
				t.Error("accepted")
			}
		})
	}
	t.Run("a unit field does not exist", func(t *testing.T) {
		_, err := load(t, strings.Replace(goodFacts, "    since_vocab: 2", "    since_vocab: 2\n    unit: \"paise\"", 1), goodRecords)
		if err == nil || !strings.Contains(err.Error(), `unknown key "unit"`) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("missing kind and periodicity", func(t *testing.T) {
		f := strings.Replace(goodFacts, "    kind: \"text\"\n    periodicity: \"quarter\"\n", "", 1)
		_, err := load(t, f, goodRecords)
		if err == nil || !strings.Contains(err.Error(), `kind ""`) || !strings.Contains(err.Error(), `periodicity ""`) {
			t.Errorf("err = %v", err)
		}
	})
}

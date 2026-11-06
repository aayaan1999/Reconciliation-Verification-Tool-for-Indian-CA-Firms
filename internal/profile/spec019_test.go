package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/currency"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// SPEC-019: an amount written in US dollars is converted at the run's fixed rate; with no usable
// rate it is left out with a warning, never zero. Illustrative values only.

var rate8325 = currency.Rate{Value: money.USDRate(832500), Effective: "2026-04-01"}

func factOf(r *Result, name string) (string, bool) {
	for _, f := range r.Facts {
		if f.Name == name {
			return domain.FormatValue(f.Value), true
		}
	}
	return "", false
}

func TestDollarFactConverted(t *testing.T) {
	p := load(t, goodProfile)
	doc := strings.Replace(goodDoc, `"total": "1,12,450.50"`, `"total": "$1,250.50"`, 1)
	r := extract(t, p, doc, Source{FileName: "comp.json", USD: rate8325})
	if r.Report.HasErrors() || len(r.Report.Warnings) != 0 {
		t.Fatalf("errors %v warnings %v", r.Report.Errors, r.Report.Warnings)
	}
	var got money.Paise
	for _, f := range r.Facts {
		if f.Name == "computation.schedule_total" {
			got = f.Value.Money
		}
	}
	if got != money.Paise(10410413) {
		t.Errorf("schedule total = %d paise, want 10,410,413 (at 83.2500)", got)
	}
	// The rupee figure of the same file is read exactly as before.
	for _, f := range r.Facts {
		if f.Name == "computation.allowed_depreciation" && f.Value.Money != money.Paise(11245050) {
			t.Errorf("rupee figure = %d", f.Value.Money)
		}
	}
	// A negative dollar amount in brackets.
	doc = strings.Replace(goodDoc, `"total": "1,12,450.50"`, `"total": "($1,250.50)"`, 1)
	r = extract(t, p, doc, Source{FileName: "comp.json", USD: rate8325})
	for _, f := range r.Facts {
		if f.Name == "computation.schedule_total" && f.Value.Money != money.Paise(-10410413) {
			t.Errorf("negative dollar figure = %d", f.Value.Money)
		}
	}
	// A new rate gives new paise with the same text and no other change (AC-7 at this level).
	r84 := currency.Rate{Value: money.USDRate(840000)}
	r = extract(t, p, strings.Replace(goodDoc, `"total": "1,12,450.50"`, `"total": "$1,250.50"`, 1), Source{FileName: "comp.json", USD: r84})
	for _, f := range r.Facts {
		if f.Name == "computation.schedule_total" && f.Value.Money != money.Paise(10504200) {
			t.Errorf("at 84 = %d, want 10,504,200", f.Value.Money)
		}
	}
}

func TestDollarFactWithoutRate(t *testing.T) {
	p := load(t, goodProfile)
	doc := strings.Replace(goodDoc, `"total": "1,12,450.50"`, `"total": "$1,250.50"`, 1)
	for name, usd := range map[string]currency.Rate{
		"zero value":  {},
		"not set":     {Problem: currency.ReasonNotSet},
		"not valid":   {Problem: currency.ReasonBad},
		"value only":  {Value: 0},
		"bad+value":   {Value: money.USDRate(832500), Problem: currency.ReasonBad},
		"none helper": currency.None(),
	} {
		r := extract(t, p, doc, Source{FileName: "comp.json", USD: usd})
		if r.Report.HasErrors() {
			t.Errorf("%s: a dollar amount with no rate made the file an error: %v", name, r.Report.Errors)
		}
		if _, ok := factOf(r, "computation.schedule_total"); ok {
			t.Errorf("%s: a figure was read with no rate (never zero, never a guess)", name)
		}
		if len(r.Report.Warnings) != 1 || r.Report.Warnings[0].Code != ingest.DollarRate ||
			!strings.Contains(r.Report.Warnings[0].Msg, "dollar rate not") || strings.Contains(r.Report.Warnings[0].Msg, "1,250") {
			t.Errorf("%s: warnings %+v", name, r.Report.Warnings)
		}
		if _, ok := factOf(r, "computation.allowed_depreciation"); !ok {
			t.Errorf("%s: the rupee figure of the same file was lost", name)
		}
	}
	// The reason is the one that applies.
	r := extract(t, p, doc, Source{FileName: "comp.json", USD: currency.Rate{Problem: currency.ReasonBad}})
	if !strings.Contains(r.Report.Warnings[0].Msg, currency.ReasonBad) {
		t.Errorf("warning %q lacks %q", r.Report.Warnings[0].Msg, currency.ReasonBad)
	}
	// A file with rupee amounts only needs no rate at all.
	r = extract(t, p, goodDoc, Source{FileName: "comp.json"})
	if r.Report.HasErrors() || len(r.Report.Warnings) != 0 || len(r.Facts) != 2 {
		t.Errorf("rupee-only file: %v %v %d facts", r.Report.Errors, r.Report.Warnings, len(r.Facts))
	}
	// A required figure in dollars with no rate is reported as missing, not taken as zero.
	req := strings.Replace(goodDoc, `112450.50`, `"$5.00"`, 1)
	r = extract(t, p, req, Source{FileName: "comp.json"})
	if len(r.Facts) != 0 || len(r.Report.Errors) != 1 || r.Report.Errors[0].Code != ingest.RequiredFieldMissing {
		t.Errorf("required dollar figure: facts %v errors %v", r.Facts, r.Report.Errors)
	}
}

func TestDollarFactBadText(t *testing.T) {
	p := load(t, goodProfile)
	for _, bad := range []string{"$1,250.505", "$1,25,000.00", "$-5", "USD $5"} {
		doc := strings.Replace(goodDoc, `"total": "1,12,450.50"`, `"total": "`+bad+`"`, 1)
		r := extract(t, p, doc, Source{FileName: "comp.json", USD: rate8325})
		if len(r.Report.Errors) != 1 || r.Report.Errors[0].Code != ingest.ParseError || strings.Contains(r.Report.Errors[0].Msg, "1,250") {
			t.Errorf("%q: errors %+v", bad, r.Report.Errors)
		}
	}
}

// A dollar amount in a row of a register: converted with a rate; without one the amount is left
// out of the row (not zero) and a warning says why, so the checks that need it cannot be made.
func TestDollarRecordAmount(t *testing.T) {
	ty := period.TaxYear{StartYear: 2026}
	p := load(t, fromRunProfile)
	doc := `{"rows": [{"no": "T1", "tx": "$1,250.50"}, {"no": "T2", "tx": "1,250.50"}, {"no": "T3", "tx": 1250.5}]}`
	r := extract(t, p, doc, Source{FileName: "reg.json", TaxYear: ty, USD: rate8325})
	if r.Report.HasErrors() || len(r.Records) != 3 {
		t.Fatalf("errors %v, %d records", r.Report.Errors, len(r.Records))
	}
	amt := map[string]money.Paise{}
	for _, rec := range r.Records {
		amt[rec.Fields["invoice_no"]] = rec.Amounts["taxable_value"]
	}
	if amt["T1"] != 10410413 || amt["T2"] != 125050 || amt["T3"] != 125050 {
		t.Errorf("amounts %v: T1 in dollars at 83.2500, T2 and T3 in rupees as before", amt)
	}
	r = extract(t, p, doc, Source{FileName: "reg.json", TaxYear: ty})
	if r.Report.HasErrors() || len(r.Records) != 3 || len(r.Report.Warnings) != 1 {
		t.Fatalf("no rate: errors %v, %d records, warnings %v", r.Report.Errors, len(r.Records), r.Report.Warnings)
	}
	for _, rec := range r.Records {
		v, has := rec.Amounts["taxable_value"]
		switch rec.Fields["invoice_no"] {
		case "T1":
			if has {
				t.Errorf("T1 has an amount (%d) with no rate", v)
			}
		default:
			if !has || v != 125050 {
				t.Errorf("%s: rupee amount %d, %v", rec.Fields["invoice_no"], v, has)
			}
		}
	}
	if w := r.Report.Warnings[0]; w.Code != ingest.DollarRate || w.Locator.Path == "" || !strings.Contains(w.Msg, currency.ReasonNotSet) {
		t.Errorf("warning %+v", w)
	}
	// The same text and rate always give the same result.
	again := extract(t, p, doc, Source{FileName: "reg.json", TaxYear: ty, USD: rate8325})
	first := extract(t, p, doc, Source{FileName: "reg.json", TaxYear: ty, USD: rate8325})
	if len(again.Records) != len(first.Records) {
		t.Error("not deterministic")
	}
}

package engine

import (
	"bytes"
	"cmp"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

var root = filepath.Join("..", "..")

func load(t *testing.T) (*vocab.Vocab, *rules.Set) {
	t.Helper()
	v, err := vocab.LoadDir(filepath.Join(root, "content", "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rules.LoadDir(filepath.Join(root, "content", "rules"), v, checks.All())
	if err != nil {
		t.Fatal(err)
	}
	return v, set
}

// TestRuleFixtures runs every rule on each of its fixtures (RU-43, CLAUDE.md invariant 6):
// every rule has at least one pass and one fail fixture, and each fixture gives exactly
// the evaluations it expects.
func TestRuleFixtures(t *testing.T) {
	v, set := load(t)
	for _, r := range set.Rules() {
		t.Run(r.ID, func(t *testing.T) {
			fx, err := rules.LoadFixtures(r, v)
			if err != nil {
				t.Fatal(err)
			}
			if len(fx.Pass) == 0 || len(fx.Fail) == 0 {
				t.Fatalf("%d pass and %d fail fixtures; every rule needs at least one of each", len(fx.Pass), len(fx.Fail))
			}
			for _, f := range append(slices.Clone(fx.Pass), fx.Fail...) {
				evs, err := Evaluate([]*rules.Rule{r}, f.Facts, f.Records, f.TaxYear, checks.All(), 2)
				if err != nil {
					t.Fatalf("%s: %v", f.File, err)
				}
				var got []rules.Expected
				for _, e := range evs {
					got = append(got, rules.Expected{Status: e.Status.String(), Period: e.Subject.Period, Scope: e.Subject.Scope, MatchKey: e.Subject.MatchKey, Code: e.Code, Missing: e.MissingFacts})
				}
				want := slices.Clone(f.Expected)
				order := func(a, b rules.Expected) int {
					return cmp.Or(cmp.Compare(a.Period, b.Period), cmp.Compare(a.Scope, b.Scope), cmp.Compare(a.MatchKey, b.MatchKey))
				}
				slices.SortFunc(want, order)
				slices.SortFunc(got, order)
				if !slices.EqualFunc(got, want, func(a, b rules.Expected) bool {
					return a.Status == b.Status && a.Period == b.Period && a.Scope == b.Scope && a.MatchKey == b.MatchKey && a.Code == b.Code && slices.Equal(a.Missing, b.Missing)
				}) {
					t.Errorf("%s: got %+v, want %+v", f.File, got, want)
				}
			}
		})
	}
}

func fact(name string, rupees int64, file string) domain.Fact {
	return domain.Fact{Name: name, Period: "FY2025-26", Value: domain.MoneyValue(money.Paise(rupees * 100)),
		Locator: domain.Locator{FileName: file, Kind: domain.LocatorJSONPath, Path: "$." + name}}
}

var ty2025 = period.TaxYear{StartYear: 2025}

// A missing input gives Cannot check naming the figure, never Fail (RU-52).
func TestMissingInputIsCannotCheck(t *testing.T) {
	_, set := load(t)
	facts := []domain.Fact{fact("computation.allowed_depreciation", 1000, "comp.json")}
	evs, err := Evaluate(set.ForTaxYear(ty2025), facts, nil, ty2025, checks.All(), 4)
	if err != nil {
		t.Fatal(err)
	}
	rs := set.ForTaxYear(ty2025)
	if len(evs) != len(rs) {
		t.Fatalf("got %d evaluations, want one per rule (%d)", len(evs), len(rs))
	}
	// Each rule names every figure or row type it reads except the one supplied and those
	// that are optional with a default (RU-52: their absence has a meaning, such as nil).
	want := map[string][]string{}
	for _, r := range rs {
		for _, in := range r.Inputs {
			n := in.Fact + in.RecordType
			if n != "computation.allowed_depreciation" && !(in.Optional && in.Default != nil) {
				want[r.ID] = append(want[r.ID], n)
			}
		}
		slices.Sort(want[r.ID])
		want[r.ID] = slices.Compact(want[r.ID])
	}
	if !slices.Equal(want["RECON-DEP-001"], []string{"computation.fixed_asset_schedule.total"}) {
		t.Errorf("RECON-DEP-001 reads %v", want["RECON-DEP-001"])
	}
	for _, e := range evs {
		if e.Status != domain.StatusCannotCheck || !slices.Equal(e.MissingFacts, want[e.RuleID]) || e.Subject.Period != "FY2025-26" {
			t.Errorf("%s: status %s, missing %q, period %s", e.RuleID, e.Status, e.MissingFacts, e.Subject.Period)
		}
	}
}

// The result does not depend on the number of workers or on the order of the facts.
func TestEvaluateIsDeterministic(t *testing.T) {
	_, set := load(t)
	facts := []domain.Fact{
		fact("computation.allowed_depreciation", 1000, "comp.json"),
		fact("computation.fixed_asset_schedule.total", 1200, "comp.json"),
		fact("tar.depreciation_schedule.total", 1000, "tar.json"),
	}
	var first []byte
	for workers := 1; workers <= 8; workers++ {
		slices.Reverse(facts)
		evs, err := Evaluate(set.ForTaxYear(ty2025), facts, nil, ty2025, checks.All(), workers)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		for _, e := range evs {
			j, err := e.CanonicalJSON()
			if err != nil {
				t.Fatal(err)
			}
			b.Write(j)
			b.WriteByte('\n')
		}
		if first == nil {
			first = b.Bytes()
			if !strings.Contains(b.String(), `"difference":20000`) {
				t.Errorf("RECON-DEP-001 difference not 20000 paise: %s", b.String())
			}
			continue
		}
		if !bytes.Equal(first, b.Bytes()) {
			t.Fatalf("workers=%d gave different output:\n%s\nvs\n%s", workers, b.String(), first)
		}
	}
}

// The same figure with two different values in the current view is refused, never guessed.
func TestConflictingFactsAreRefused(t *testing.T) {
	_, set := load(t)
	facts := []domain.Fact{
		fact("computation.allowed_depreciation", 1000, "a.json"),
		fact("computation.allowed_depreciation", 1001, "b.json"),
		fact("computation.fixed_asset_schedule.total", 1000, "a.json"),
	}
	_, err := Evaluate(set.ForTaxYear(ty2025), facts, nil, ty2025, checks.All(), 2)
	var ce *ConflictError
	if !errors.As(err, &ce) || ce.Fact != "computation.allowed_depreciation" {
		t.Fatalf("err = %v, want a ConflictError", err)
	}
}

func TestIndex(t *testing.T) {
	_, set := load(t)
	got := Index(set.ForTaxYear(ty2025))
	want := []Dependency{
		{"RECON-DEP-001", []string{"computation.allowed_depreciation", "computation.fixed_asset_schedule.total"}, nil},
		{"RECON-DEP-002", []string{"computation.allowed_depreciation", "tar.depreciation_schedule.total"}, nil},
		{"RECON-GST-001", nil, []string{"gstr1_b2b_invoice", "sales_b2b_invoice"}},
		{"RECON-GST-013", nil, []string{"gstr2b_b2b_invoice", "purchase_b2b_invoice"}},
		{"RECON-MAT-001", []string{"computation.mat.book_profit", "mat.book_profit"}, nil},
		{"RECON-MAT-002", []string{"computation.mat.tax_payable", "mat.tax_payable"}, nil},
		{"RECON-PL-001", []string{"computation.net_profit_as_per_pl", "fs.pl.profit_before_tax"}, nil},
		{"RECON-PL-002", []string{"computation.depreciation_as_per_books", "fs.pl.depreciation"}, nil},
	}
	// Every rule is in the index, in order; these keep the dependencies they always had.
	if len(got) != len(set.ForTaxYear(ty2025)) {
		t.Errorf("%d dependencies for %d rules", len(got), len(set.ForTaxYear(ty2025)))
	}
	for _, w := range want {
		i := slices.IndexFunc(got, func(d Dependency) bool { return d.RuleID == w.RuleID })
		if i < 0 || !slices.Equal(got[i].Facts, w.Facts) || !slices.Equal(got[i].Records, w.Records) {
			t.Errorf("%s: dependency %+v, want %+v", w.RuleID, got[max(i, 0)], w)
		}
	}
	if !slices.IsSortedFunc(got, func(a, b Dependency) int { return strings.Compare(a.RuleID, b.RuleID) }) {
		t.Error("the index is not in rule order")
	}
}

// Rows of a record type that share a match key (two asset classes in one depreciation
// block) each get their own subject, so their findings have different stable IDs and a
// decision on one never lands on the other. The first keeps the key; the later one, in file
// order, is marked "|#2" (code review finding 1).
func TestRowsSharingAKeyGetTheirOwnSubject(t *testing.T) {
	_, set := load(t)
	var rule *rules.Rule
	for _, r := range set.Rules() {
		if r.ID == "RECON-TAR-018" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("RECON-TAR-018 not loaded")
	}
	block := func(path string, closing int64) domain.Record {
		return domain.Record{RecordType: "tar_depreciation_block", Period: "FY2025-26", MatchKey: "15%",
			Amounts: map[string]money.Paise{"opening_wdv": 100000, "additions": 0, "deductions": 0, "depreciation": 15000, "closing_wdv": money.Paise(closing)},
			Locator: domain.Locator{FileName: "TAR.json", Kind: domain.LocatorJSONPath, Path: path}}
	}
	// The later row ($[10]) is listed first, and does not add up.
	records := []domain.Record{block("$.clause18[10]", 99999), block("$.clause18[2]", 85000)}
	evs, err := Evaluate([]*rules.Rule{rule}, nil, records, ty2025, checks.All(), 1)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	got := map[string]domain.EvalStatus{}
	for _, e := range evs {
		id := domain.StableID(e.RuleID, 1, e.Subject)
		if ids[id] {
			t.Errorf("stable ID %s given twice", id)
		}
		ids[id] = true
		got[e.Subject.MatchKey] = e.Status
	}
	want := map[string]domain.EvalStatus{"15%": domain.StatusPass, "15%" + RepeatedKeyMark + "2": domain.StatusFail}
	if len(got) != len(want) || got["15%"] != want["15%"] || got["15%|#2"] != want["15%|#2"] {
		t.Errorf("subjects %v, want %v", got, want)
	}
}

// A key with no repeat keeps its match key unchanged, so existing finding IDs stay.
func TestUniqueRowKeysLeavesSingleKeys(t *testing.T) {
	o := []rules.RecordOutcome{{MatchKey: "A"}, {MatchKey: "B"}, {MatchKey: "A"}, {MatchKey: "A"}}
	uniqueRowKeys(o)
	var keys []string
	for _, x := range o {
		keys = append(keys, x.MatchKey)
	}
	if want := []string{"A", "B", "A|#2", "A|#3"}; !slices.Equal(keys, want) {
		t.Errorf("keys %v, want %v", keys, want)
	}
}

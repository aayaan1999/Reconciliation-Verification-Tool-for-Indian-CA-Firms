package equal

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

func moneyRule(t rules.Tolerance) *rules.Rule {
	return &rules.Rule{ID: "X-001", Check: rules.Check{Type: Name, Tolerance: t}, Inputs: []rules.Input{
		{Name: "a", Fact: "f.a", Kind: domain.KindMoney, Label: "A"},
		{Name: "b", Fact: "f.b", Kind: domain.KindMoney, Label: "B"},
	}}
}

func found(a, b domain.Value) map[string]domain.Fact {
	return map[string]domain.Fact{"a": {Name: "f.a", Value: a}, "b": {Name: "f.b", Value: b}}
}

func TestEvaluateMoney(t *testing.T) {
	m := func(p int64) domain.Value { return domain.MoneyValue(money.Paise(p)) }
	for _, c := range []struct {
		name string
		tol  rules.Tolerance
		a, b int64
		pass bool
		diff int64
	}{
		{"equal", rules.Tolerance{}, 100, 100, true, 0},
		{"one paisa, exact", rules.Tolerance{}, 100, 101, false, 1},
		{"at paise tolerance", rules.Tolerance{Kind: rules.TolerancePaise, Value: 100}, 1000, 1100, true, 100},
		{"over paise tolerance", rules.Tolerance{Kind: rules.TolerancePaise, Value: 100}, 1100, 999, false, 101},
		{"at 1% of larger", rules.Tolerance{Kind: rules.ToleranceBPOfLarger, Value: 100}, 9900, 10000, true, 100},
		{"over 1% of larger", rules.Tolerance{Kind: rules.ToleranceBPOfLarger, Value: 100}, 9899, 10000, false, 101},
		{"negative amounts", rules.Tolerance{Kind: rules.ToleranceBPOfLarger, Value: 100}, -9900, -10000, true, 100},
		{"huge amounts do not overflow", rules.Tolerance{Kind: rules.ToleranceBPOfLarger, Value: 1}, 1 << 61, 1<<61 - 1, true, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			o, err := Check{}.Evaluate(moneyRule(c.tol), found(m(c.a), m(c.b)))
			if err != nil {
				t.Fatal(err)
			}
			if (o.Status == domain.StatusPass) != c.pass || int64(o.Difference) != c.diff {
				t.Errorf("status %s, difference %d; want pass=%v, difference %d", o.Status, o.Difference, c.pass, c.diff)
			}
			if len(o.Values) != 2 || o.Values[0].Label != "A" || o.Values[1].Input != "b" {
				t.Errorf("values = %+v", o.Values)
			}
		})
	}
}

func TestEvaluateText(t *testing.T) {
	r := moneyRule(rules.Tolerance{})
	for i := range r.Inputs {
		r.Inputs[i].Kind = domain.KindText
	}
	o, err := Check{}.Evaluate(r, found(domain.TextValue("27AAA"), domain.TextValue("27AAB")))
	if err != nil || o.Status != domain.StatusFail || o.Difference != 0 {
		t.Errorf("status %s, difference %d, err %v", o.Status, o.Difference, err)
	}
	if _, err := (Check{}).Evaluate(r, map[string]domain.Fact{"a": {}}); err == nil {
		t.Error("a missing input was evaluated; the engine gives Cannot check before calling Evaluate")
	}
}

// An unknown parameter is reported together with the other problems of the rule, in one
// load, as the other check types do (code review finding 10).
func TestValidateParamsReportsEveryProblem(t *testing.T) {
	r := moneyRule(rules.Tolerance{})
	r.Check.CompareAmounts = []string{"igst"} // a parameter equal does not use
	r.Inputs[1].Kind = domain.KindText        // and a and b of different kinds
	err := Check{}.ValidateParams(r, nil)
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"does not use", "same kind"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

package newheads

import (
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// RecordType is the vocabulary record type the comparison reads: one ledger head's amount
// for one month.
const RecordType = "ledger_month"

// The kinds of Worklist item, in the order they are listed.
const (
	KindNewHead          = "new_head"          // this year, not last year
	KindVanished         = "vanished"          // last year, not this year
	KindBrokenRecurrence = "broken_recurrence" // every month last year, some months missing this year
	KindStepChange       = "step_change"       // this year's total far above or below last year's
)

// Kinds lists the kinds in the order they are listed.
var Kinds = []string{KindNewHead, KindVanished, KindBrokenRecurrence, KindStepChange}

// ThresholdsSchema is the value of the "schema" key of content/worklist/newheads.yaml.
const ThresholdsSchema = "recon.newheads/1"

// Thresholds say when a year-on-year movement is a step change. Times are in hundredths
// (200 is "2.00 times"), so no floating point is used.
type Thresholds struct {
	UpHundredths   int64       // this year at least this many hundredths of last year
	DownHundredths int64       // or this year at most this many hundredths of last year
	Minimum        money.Paise // and the larger of the two years at least this much
}

type thresholdsYAML struct {
	Schema     string `yaml:"schema"`
	StepChange struct {
		UpTimes    string `yaml:"up_times"`
		DownTimes  string `yaml:"down_times"`
		MinimumINR string `yaml:"minimum_inr"`
	} `yaml:"step_change"`
}

// ParseThresholds reads the thresholds file (its name is used in messages).
func ParseThresholds(file string, data []byte) (Thresholds, error) {
	var f thresholdsYAML
	pos, err := strictyaml.Decode(file, data, &f)
	if err != nil {
		return Thresholds{}, err
	}
	var errs strictyaml.Errors
	fail := func(at, format string, a ...any) {
		errs = append(errs, &strictyaml.Error{File: file, Line: pos.Line(at), Msg: fmt.Sprintf(format, a...)})
	}
	if f.Schema != ThresholdsSchema {
		fail("schema", "schema must be %q", ThresholdsSchema)
	}
	// "2.00" read as rupees is 200 paise: hundredths of a time.
	read := func(at, s string) money.Paise {
		p, err := money.Parse(s, money.UnitRupees)
		if err != nil || p <= 0 {
			fail(at, "%q is not a positive number with at most two decimals", s)
		}
		return p
	}
	th := Thresholds{
		UpHundredths:   int64(read("step_change.up_times", f.StepChange.UpTimes)),
		DownHundredths: int64(read("step_change.down_times", f.StepChange.DownTimes)),
		Minimum:        read("step_change.minimum_inr", f.StepChange.MinimumINR),
	}
	if len(errs) == 0 && (th.UpHundredths <= 100 || th.DownHundredths >= 100) {
		fail("step_change", "up_times must be more than 1.00 and down_times less than 1.00")
	}
	if len(errs) > 0 {
		return Thresholds{}, errs
	}
	return th, nil
}

// An Item is one Worklist item: a classification task for staff, never a finding. It
// states only what was counted; the classification is staff's.
type Item struct {
	Kind          string      `json:"kind"`
	Ledger        string      `json:"ledger"`
	Group         string      `json:"group"`
	PriorTotal    money.Paise `json:"-"`
	CurrentTotal  money.Paise `json:"-"`
	PriorMonths   int         `json:"prior_months"`
	CurrentMonths int         `json:"current_months"`
	// MissingMonths are the months of this year's ledger with no entry for the head
	// (broken recurrence only), in order.
	MissingMonths []string `json:"missing_months,omitempty"`
	// Times is this year's total over last year's, to two decimals, rounded half up
	// (step change only).
	Times string `json:"times,omitempty"`
	// Locators are the rows read, both years, in order.
	PriorLocators   []string `json:"prior_rows,omitempty"`
	CurrentLocators []string `json:"current_rows,omitempty"`
}

// Key identifies the item for its classification: kind and ledger head.
func (it Item) Key() string { return it.Kind + "|" + it.Ledger }

type head struct {
	group    string
	total    money.Paise
	months   map[string]bool
	rows     []row
	locators []string
}

type row struct{ month, locator string }

// collect totals the ledger rows by head. The months set is every month that has a row
// for any head: the months the ledger covers.
func collect(records []domain.Record) (map[string]*head, map[string]bool, error) {
	heads := map[string]*head{}
	months := map[string]bool{}
	for _, r := range records {
		if r.RecordType != RecordType {
			continue
		}
		name, month := r.Fields["ledger_head"], r.Fields["month"]
		h := heads[name]
		if h == nil {
			h = &head{months: map[string]bool{}}
			heads[name] = h
		}
		if h.group == "" {
			h.group = r.Fields["group"]
		}
		t, err := money.Add(h.total, r.Amounts["amount"])
		if err != nil {
			return nil, nil, fmt.Errorf("newheads: %s: %w", name, err)
		}
		h.total = t
		h.months[month] = true
		months[month] = true
		h.rows = append(h.rows, row{month, r.Locator.Display()})
	}
	for _, h := range heads { // by month, then place: not in the order rows were stored
		slices.SortFunc(h.rows, func(a, b row) int {
			if d := strings.Compare(a.month, b.month); d != 0 {
				return d
			}
			return strings.Compare(a.locator, b.locator)
		})
		for _, r := range h.rows {
			h.locators = append(h.locators, r.locator)
		}
	}
	return heads, months, nil
}

// Compare compares last year's ledger rows with this year's and returns the Worklist
// items, sorted by kind (in the order of Kinds) and then by ledger head. Records of other
// types are ignored. The same rows always give the same items.
//
//   - new head: a head with rows this year and none last year;
//   - vanished: a head with rows last year and none this year;
//   - broken recurrence: a head with a row in every month last year's ledger covers, and
//     no row in some month this year's ledger covers (the months are listed);
//   - step change: a head in both years whose total this year is at least UpHundredths,
//     or at most DownHundredths, hundredths of last year's, the larger total being at
//     least Minimum.
func Compare(prior, current []domain.Record, th Thresholds) ([]Item, error) {
	ph, pm, err := collect(prior)
	if err != nil {
		return nil, err
	}
	ch, cm, err := collect(current)
	if err != nil {
		return nil, err
	}
	var items []Item
	item := func(kind, name string, p, c *head) Item {
		it := Item{Kind: kind, Ledger: name}
		if p != nil {
			it.Group, it.PriorTotal, it.PriorMonths, it.PriorLocators = p.group, p.total, len(p.months), p.locators
		}
		if c != nil {
			it.Group, it.CurrentTotal, it.CurrentMonths, it.CurrentLocators = c.group, c.total, len(c.months), c.locators
		}
		return it
	}
	for name, c := range ch {
		p := ph[name]
		if p == nil {
			if len(pm) > 0 { // with no ledger last year, nothing is new
				items = append(items, item(KindNewHead, name, nil, c))
			}
			continue
		}
		if len(p.months) == len(pm) && len(pm) > 0 {
			var missing []string
			for m := range cm {
				if !c.months[m] {
					missing = append(missing, m)
				}
			}
			if len(missing) > 0 {
				slices.Sort(missing)
				it := item(KindBrokenRecurrence, name, p, c)
				it.MissingMonths = missing
				items = append(items, it)
			}
		}
		if times, ok := stepChange(p.total, c.total, th); ok {
			it := item(KindStepChange, name, p, c)
			it.Times = times
			items = append(items, it)
		}
	}
	if len(cm) > 0 { // with no ledger this year, nothing has vanished
		for name, p := range ph {
			if ch[name] == nil {
				items = append(items, item(KindVanished, name, p, nil))
			}
		}
	}
	slices.SortFunc(items, func(a, b Item) int {
		if d := slices.Index(Kinds, a.Kind) - slices.Index(Kinds, b.Kind); d != 0 {
			return d
		}
		return strings.Compare(a.Ledger, b.Ledger)
	})
	return items, nil
}

// stepChange reports whether the movement from prior to current crosses a threshold, and
// current/prior to two decimals. Products are taken in big integers, so no total overflows.
func stepChange(prior, current money.Paise, th Thresholds) (string, bool) {
	if prior <= 0 || current < 0 {
		return "", false
	}
	if max(prior, current) < th.Minimum {
		return "", false
	}
	c100 := new(big.Int).Mul(big.NewInt(int64(current)), big.NewInt(100))
	up := new(big.Int).Mul(big.NewInt(int64(prior)), big.NewInt(th.UpHundredths))
	down := new(big.Int).Mul(big.NewInt(int64(prior)), big.NewInt(th.DownHundredths))
	if c100.Cmp(up) < 0 && c100.Cmp(down) > 0 {
		return "", false
	}
	// Hundredths of a time, half up: (200*current + prior) / (2*prior).
	num := new(big.Int).Add(new(big.Int).Mul(c100, big.NewInt(2)), big.NewInt(int64(prior)))
	h := new(big.Int).Quo(num, big.NewInt(2*int64(prior))).Int64()
	return fmt.Sprintf("%d.%02d", h/100, h%100), true
}

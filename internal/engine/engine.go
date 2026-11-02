package engine

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// A Dependency is one rule and the vocabulary facts and record types it reads (technical
// spec §B6 step 2).
type Dependency struct {
	RuleID  string
	Facts   []string // sorted, no repeats
	Records []string // record types, sorted, no repeats
}

// Index returns the dependency index of rs: for each rule, in the given order, the facts
// its inputs read. A later incremental run uses it to re-evaluate only the rules whose
// facts changed (SC-06).
func Index(rs []*rules.Rule) []Dependency {
	out := make([]Dependency, 0, len(rs))
	for _, r := range rs {
		var names, records []string
		for _, in := range r.Inputs {
			if in.Fact != "" {
				names = append(names, in.Fact)
			}
			if in.RecordType != "" {
				records = append(records, in.RecordType)
			}
		}
		slices.Sort(names)
		slices.Sort(records)
		out = append(out, Dependency{RuleID: r.ID, Facts: slices.Compact(names), Records: slices.Compact(records)})
	}
	return out
}

// A ConflictError is one figure found twice for the same period and scope with different
// values, in different documents of the current view. The engine never picks one.
type ConflictError struct {
	Fact, Period, Scope string
	Places              []string // where each value was found
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s for %s is found with different values in %s", e.Fact, e.Period, strings.Join(e.Places, " and "))
}

// Evaluate runs every rule of rs over facts and records for tax year ty on a pool of
// workers and returns every evaluation sorted by (rule ID, subject key), so the result never
// depends on the number of workers or on scheduling (NF-01). A rule with a missing input
// gives Cannot check for that subject, never Fail (RU-52). A rule on facts needs an
// rules.Evaluator check type; a rule on records an rules.RecordEvaluator.
func Evaluate(rs []*rules.Rule, facts []domain.Fact, records []domain.Record, ty period.TaxYear, cts *rules.CheckTypes, workers int) ([]domain.Evaluation, error) {
	byName := map[string][]domain.Fact{}
	for _, f := range facts {
		byName[f.Name] = append(byName[f.Name], f)
	}
	byType := map[string][]domain.Record{}
	for _, r := range records {
		byType[r.RecordType] = append(byType[r.RecordType], r)
	}
	deps := Index(rs)
	results := make([][]domain.Evaluation, len(rs))
	errs := make([]error, len(rs))

	workers = max(1, min(workers, len(rs)))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if len(deps[i].Records) > 0 && len(rs[i].Inputs) == 1 {
					results[i], errs[i] = evaluateRowRule(rs[i], byType, ty, cts)
					continue
				}
				if len(deps[i].Records) > 0 {
					results[i], errs[i] = evaluateRecordRule(rs[i], byType, ty, cts)
					continue
				}
				var mine []domain.Fact
				for _, name := range deps[i].Facts {
					mine = append(mine, byName[name]...)
				}
				results[i], errs[i] = evaluateRule(rs[i], mine, ty, cts)
			}
		}()
	}
	for i := range rs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	var out []domain.Evaluation
	for i := range rs {
		if errs[i] != nil {
			return nil, errs[i] // the first failing rule in the given order: the same every time
		}
		out = append(out, results[i]...)
	}
	slices.SortStableFunc(out, func(a, b domain.Evaluation) int {
		return cmp.Or(cmp.Compare(a.RuleID, b.RuleID), cmp.Compare(a.Subject.Key(), b.Subject.Key()))
	})
	return out, nil
}

// evaluateRule evaluates r for every subject: each period and scope in the tax year for
// which any input fact exists, or the whole year when none does.
func evaluateRule(r *rules.Rule, facts []domain.Fact, ty period.TaxYear, cts *rules.CheckTypes) ([]domain.Evaluation, error) {
	ct, ok := cts.Lookup(r.Check.Type)
	if !ok {
		return nil, fmt.Errorf("rule %s: check type %q is not known", r.ID, r.Check.Type)
	}
	ev, ok := ct.(rules.Evaluator)
	if !ok {
		return nil, fmt.Errorf("rule %s: check type %q cannot be evaluated yet", r.ID, r.Check.Type)
	}
	for _, in := range r.Inputs {
		if in.Fact == "" {
			return nil, fmt.Errorf("rule %s: input %s reads rows (%s), which the engine cannot evaluate yet", r.ID, in.Name, in.RecordType)
		}
	}

	year := period.Year(ty)
	var subjects []domain.Subject
	for _, f := range facts {
		p, err := period.Parse(f.Period)
		if err != nil || !period.Contains(year, p) {
			continue
		}
		subjects = append(subjects, domain.Subject{Period: f.Period, Scope: f.Scope})
	}
	if len(subjects) == 0 {
		subjects = []domain.Subject{{Period: year.String()}}
	}
	slices.SortFunc(subjects, func(a, b domain.Subject) int { return cmp.Compare(a.Key(), b.Key()) })
	subjects = slices.Compact(subjects)

	var out []domain.Evaluation
	for _, s := range subjects {
		e := domain.Evaluation{RuleID: r.ID, RuleVersion: r.Version, Subject: s, LawRefs: slices.Clone(r.Law)}
		found := map[string]domain.Fact{}
		for _, in := range r.Inputs {
			f, ok, err := lookup(facts, in.Fact, s)
			switch {
			case err != nil:
				return nil, err
			case ok:
				found[in.Name] = f
			case in.Optional && in.Default != nil:
				found[in.Name] = domain.Fact{Name: in.Fact, Period: s.Period, Scope: s.Scope, Value: *in.Default}
			default:
				e.MissingFacts = append(e.MissingFacts, in.Fact)
			}
		}
		if len(e.MissingFacts) > 0 {
			slices.Sort(e.MissingFacts)
			e.MissingFacts = slices.Compact(e.MissingFacts)
			e.Status = domain.StatusCannotCheck
			out = append(out, e)
			continue
		}
		o, err := ev.Evaluate(r, found)
		if err != nil {
			return nil, err
		}
		e.Status, e.Values, e.Difference = o.Status, o.Values, o.Difference
		if len(o.Missing) > 0 {
			e.Status, e.MissingFacts = domain.StatusCannotCheck, slices.Sorted(slices.Values(o.Missing))
		} else if o.Status == domain.StatusCannotCheck {
			return nil, fmt.Errorf("rule %s: check type %q gave Cannot check without saying what is missing", r.ID, r.Check.Type)
		}
		out = append(out, e)
	}
	return out, nil
}

// evaluateRowRule evaluates a rule on the rows of one record type (SPEC-006): for each
// period and scope in the tax year that has rows, the check type tests each row on its own.
// With no rows at all the result is Cannot check naming the record type.
func evaluateRowRule(r *rules.Rule, byType map[string][]domain.Record, ty period.TaxYear, cts *rules.CheckTypes) ([]domain.Evaluation, error) {
	ct, ok := cts.Lookup(r.Check.Type)
	if !ok {
		return nil, fmt.Errorf("rule %s: check type %q is not known", r.ID, r.Check.Type)
	}
	ev, ok := ct.(rules.RowEvaluator)
	if !ok {
		return nil, fmt.Errorf("rule %s: check type %q cannot test rows one by one", r.ID, r.Check.Type)
	}
	in := r.Inputs[0]
	year := period.Year(ty)
	type subj struct{ period, scope string }
	rows := map[subj][]domain.Record{}
	var subjects []subj
	for _, rec := range byType[in.RecordType] {
		p, err := period.Parse(rec.Period)
		if err != nil || !period.Contains(year, p) {
			continue
		}
		s := subj{rec.Period, rec.Scope}
		if rows[s] == nil {
			subjects = append(subjects, s)
		}
		rows[s] = append(rows[s], rec)
	}
	if len(subjects) == 0 {
		return []domain.Evaluation{{RuleID: r.ID, RuleVersion: r.Version, Subject: domain.Subject{Period: year.String()},
			Status: domain.StatusCannotCheck, MissingFacts: []string{in.RecordType}, LawRefs: slices.Clone(r.Law)}}, nil
	}
	slices.SortFunc(subjects, func(a, b subj) int {
		return cmp.Compare(domain.Subject{Period: a.period, Scope: a.scope}.Key(), domain.Subject{Period: b.period, Scope: b.scope}.Key())
	})
	var out []domain.Evaluation
	for _, s := range subjects {
		outcomes, err := ev.EvaluateRows(r, rows[s])
		if err != nil {
			return nil, err
		}
		uniqueRowKeys(outcomes)
		for _, o := range outcomes {
			out = append(out, domain.Evaluation{RuleID: r.ID, RuleVersion: r.Version,
				Subject: domain.Subject{Period: s.period, Scope: s.scope, MatchKey: o.MatchKey},
				Status:  o.Status, Code: o.Code, Values: o.Values, Difference: o.Difference, MissingFacts: o.Missing, LawRefs: slices.Clone(r.Law)})
		}
	}
	return out, nil
}

// RepeatedKeyMark joins a row's match key and its place among the rows sharing that key:
// the second row keyed "15%" becomes "15%|#2".
const RepeatedKeyMark = "|#"

// uniqueRowKeys gives every outcome of one row rule and subject its own match key, so each
// finding has its own stable ID and a decision on one row never lands on another. Rows of
// a record type may share a match key (two asset classes in one depreciation block); the
// first keeps the key and each later one, in the order the check type gave them (file
// order within a key), gets RepeatedKeyMark and its place. A key that has no repeat is
// left as it is, so existing finding IDs do not change.
func uniqueRowKeys(outcomes []rules.RecordOutcome) {
	seen := map[string]int{}
	for i := range outcomes {
		k := outcomes[i].MatchKey
		seen[k]++
		if n := seen[k]; n > 1 {
			outcomes[i].MatchKey = k + RepeatedKeyMark + strconv.Itoa(n)
		}
	}
}

// evaluateRecordRule evaluates a rule on rows: for each period and scope in the tax year
// for which either input has rows, the check type compares the two inputs' rows key by key.
// If one input has no rows for that period and scope, the result is Cannot check naming it.
func evaluateRecordRule(r *rules.Rule, byType map[string][]domain.Record, ty period.TaxYear, cts *rules.CheckTypes) ([]domain.Evaluation, error) {
	ct, ok := cts.Lookup(r.Check.Type)
	if !ok {
		return nil, fmt.Errorf("rule %s: check type %q is not known", r.ID, r.Check.Type)
	}
	ev, ok := ct.(rules.RecordEvaluator)
	if !ok {
		return nil, fmt.Errorf("rule %s: check type %q cannot compare rows", r.ID, r.Check.Type)
	}
	if len(r.Inputs) != 2 {
		return nil, fmt.Errorf("rule %s: rows are compared between exactly two inputs", r.ID)
	}
	inA, inB := r.Inputs[0], r.Inputs[1]
	year := period.Year(ty)
	type subj struct{ period, scope string }
	rows := map[subj]map[string][]domain.Record{}
	var subjects []subj
	for _, in := range []rules.Input{inA, inB} {
		for _, rec := range byType[in.RecordType] {
			p, err := period.Parse(rec.Period)
			if err != nil || !period.Contains(year, p) {
				continue
			}
			s := subj{rec.Period, rec.Scope}
			if rows[s] == nil {
				rows[s] = map[string][]domain.Record{}
				subjects = append(subjects, s)
			}
			rows[s][in.Name] = append(rows[s][in.Name], rec)
		}
	}
	if len(subjects) == 0 {
		subjects = []subj{{period: year.String()}}
	}
	slices.SortFunc(subjects, func(a, b subj) int {
		return cmp.Compare(domain.Subject{Period: a.period, Scope: a.scope}.Key(), domain.Subject{Period: b.period, Scope: b.scope}.Key())
	})
	var out []domain.Evaluation
	for _, s := range subjects {
		ra, rb := rows[s][inA.Name], rows[s][inB.Name]
		if len(ra) == 0 || len(rb) == 0 {
			e := domain.Evaluation{RuleID: r.ID, RuleVersion: r.Version, Subject: domain.Subject{Period: s.period, Scope: s.scope},
				Status: domain.StatusCannotCheck, LawRefs: slices.Clone(r.Law)}
			for _, in := range []rules.Input{inA, inB} {
				if len(rows[s][in.Name]) == 0 {
					e.MissingFacts = append(e.MissingFacts, in.RecordType)
				}
			}
			slices.Sort(e.MissingFacts)
			out = append(out, e)
			continue
		}
		outcomes, err := ev.EvaluateRecords(r, ra, rb)
		if err != nil {
			return nil, err
		}
		for _, o := range outcomes {
			out = append(out, domain.Evaluation{RuleID: r.ID, RuleVersion: r.Version,
				Subject: domain.Subject{Period: s.period, Scope: s.scope, MatchKey: o.MatchKey},
				Status:  o.Status, Code: o.Code, Values: o.Values, Difference: o.Difference, MissingFacts: o.Missing, LawRefs: slices.Clone(r.Law)})
		}
	}
	return out, nil
}

// lookup returns the fact called name for subject s. The same figure in two places with the
// same value is one figure (the first place, by file and path, is shown); with different
// values it is a ConflictError.
func lookup(facts []domain.Fact, name string, s domain.Subject) (domain.Fact, bool, error) {
	var hits []domain.Fact
	for _, f := range facts {
		if f.Name == name && f.Period == s.Period && f.Scope == s.Scope {
			hits = append(hits, f)
		}
	}
	if len(hits) == 0 {
		return domain.Fact{}, false, nil
	}
	slices.SortFunc(hits, func(a, b domain.Fact) int {
		return cmp.Or(cmp.Compare(a.Locator.FileName, b.Locator.FileName), cmp.Compare(a.Locator.Path, b.Locator.Path))
	})
	for _, h := range hits[1:] {
		if h.Value != hits[0].Value {
			places := make([]string, len(hits))
			for i, x := range hits {
				places[i] = x.Locator.Display()
			}
			return domain.Fact{}, false, &ConflictError{Fact: name, Period: s.Period, Scope: s.Scope, Places: places}
		}
	}
	return hits[0], true, nil
}

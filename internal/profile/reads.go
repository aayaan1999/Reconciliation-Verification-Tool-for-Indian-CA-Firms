package profile

import "slices"

// Reads lists the vocabulary names the profile reads: its facts and its record types, each
// sorted and without repeats. The Formats screen uses it to say which parameters a format
// supplies (SPEC-016).
func (p *Profile) Reads() (facts, records []string) {
	for _, f := range p.facts {
		facts = append(facts, f.fact.Name)
	}
	for _, r := range p.records {
		records = append(records, r.rt.Name)
	}
	slices.Sort(facts)
	slices.Sort(records)
	return slices.Compact(facts), slices.Compact(records)
}

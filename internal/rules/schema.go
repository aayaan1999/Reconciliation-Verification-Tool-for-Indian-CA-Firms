package rules

// These types mirror the rule file (schema recon.rule/1, technical spec §B4.2). They are
// decoded by internal/strictyaml, so every text value must be quoted, unknown keys are
// refused, and an absent text value is "".

// Schema is the value of the "schema" key of every rule file.
const Schema = "recon.rule/1"

type ruleYAML struct {
	Schema         string               `yaml:"schema"`
	ID             string               `yaml:"id"`
	Version        string               `yaml:"version"`
	Source         string               `yaml:"source"`
	Module         string               `yaml:"module"`
	Category       string               `yaml:"category"`
	Title          string               `yaml:"title"`
	RuleTextEn     string               `yaml:"rule_text_en"`
	Notes          string               `yaml:"notes"`
	Law            []lawYAML            `yaml:"law"`
	Applies        appliesYAML          `yaml:"applies"`
	Inputs         map[string]inputYAML `yaml:"inputs"`
	Check          checkYAML            `yaml:"check"`
	Severity       string               `yaml:"severity"`
	ReviewDocument string               `yaml:"review_document"`
}

type lawYAML struct {
	Act       string `yaml:"act"`
	Section   string `yaml:"section"`
	Reference string `yaml:"reference"`
}

type appliesYAML struct {
	TaxYears  []string `yaml:"tax_years"`
	ValidFrom string   `yaml:"valid_from"`
	ValidTo   string   `yaml:"valid_to"`
}

// inputYAML names one figure (fact) or one kind of row (record_type) the rule reads.
type inputYAML struct {
	Fact       string `yaml:"fact"`
	RecordType string `yaml:"record_type"`
	Label      string `yaml:"label"`    // shown in finding text next to the value
	Optional   bool   `yaml:"optional"` // only where the law makes absence meaningful (§B4.4)
	Default    string `yaml:"default"`  // the value used when an optional fact is absent
}

// checkYAML holds the parameters of every check type. A check type's ValidateParams
// refuses the parameters it does not use; later check types add their own fields here.
type checkYAML struct {
	Type      string         `yaml:"type"`
	Tolerance *toleranceYAML `yaml:"tolerance"`
	// match_records only (SPEC-004 part 8).
	CompareAmounts  []string `yaml:"compare_amounts"`   // amounts compared key by key, within the tolerance
	CompareFields   []string `yaml:"compare_fields"`    // other columns that must be identical
	DuplicatesIn    []string `yaml:"duplicates_in"`     // inputs whose rows are whole invoices: two rows with one key are a duplicate
	InvoiceAmounts  []string `yaml:"invoice_amounts"`   // amounts stated once per invoice and repeated on each of its lines (invoice value)
	PairUnmatchedOn []string `yaml:"pair_unmatched_on"` // fields on which a row only in a and a row only in b, with equal amounts, are one invoice numbered differently
	// SPEC-006.
	Parts      []partYAML      `yaml:"parts"`       // sum_equals: the figures added up
	Total      string          `yaml:"total"`       // sum_equals: the input the sum is compared with
	TotalField string          `yaml:"total_field"` // sum_equals on rows (SPEC-007): the amount of each row compared
	Base       string          `yaml:"base"`        // percent_of: the input the rate is applied to
	Rate       string          `yaml:"rate"`        // percent_of: a percentage, "15.00"
	Value      string          `yaml:"value"`       // percent_of: the input compared; table_lookup: the input looked up
	When       []conditionYAML `yaml:"when"`        // present_together: the conditions, all of which hold for a finding
	Keys       []string        `yaml:"keys"`        // table_lookup: the inputs whose values select a row
	Table      []tableRowYAML  `yaml:"table"`       // table_lookup: the rows, each with its source
}

type partYAML struct {
	Input string `yaml:"input"`
	Field string `yaml:"field"` // for a record-type input: the amount of each row (SPEC-007)
	Sign  string `yaml:"sign"`  // "+" or "-"
}

type conditionYAML struct {
	Input  string   `yaml:"input"`
	Field  string   `yaml:"field"` // for a record-type input: the column tested on each row
	Is     string   `yaml:"is"`
	Value  string   `yaml:"value"`
	Values []string `yaml:"values"`
}

type tableRowYAML struct {
	Keys   []string `yaml:"keys"`
	Value  string   `yaml:"value"`
	Source string   `yaml:"source"`
}

type toleranceYAML struct {
	Paise      *int64 `yaml:"paise"`
	BPOfLarger *int64 `yaml:"bp_of_larger"`
}

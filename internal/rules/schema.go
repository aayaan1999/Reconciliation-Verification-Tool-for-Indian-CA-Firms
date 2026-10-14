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
}

type toleranceYAML struct {
	Paise      *int64 `yaml:"paise"`
	BPOfLarger *int64 `yaml:"bp_of_larger"`
}

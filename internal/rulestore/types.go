// Package rulestore holds the file structures of the YAML rule store
// (docs/Recon_Tool_YAML_Rule_Store/recon_yaml, technical spec §B4.2) and reads it. Every
// text value is quoted (RU-02), so each file loads through internal/strictyaml; an empty
// text value is "" (never null). recon-dev writes and checks the store; recon-engine loads
// it into the database. The rules in it are descriptive and are never evaluated.
package rulestore

// SchemaVersion is the schema_version of every rule, document, index and category file.
const SchemaVersion = "1.0"

// The types below mirror the files. Field names follow RULE_TEMPLATE.yaml and
// DOCUMENT_TEMPLATE.yaml.

type Rule struct {
	SchemaVersion  string         `yaml:"schema_version" json:"schema_version"`
	Rule           RuleID         `yaml:"rule" json:"rule"`
	Governance     Governance     `yaml:"governance" json:"governance"`
	Validity       Validity       `yaml:"validity" json:"validity"`
	Legal          Legal          `yaml:"legal" json:"legal"`
	Inputs         Inputs         `yaml:"inputs" json:"inputs"`
	Matching       Matching       `yaml:"matching" json:"matching"`
	Calculation    Calculation    `yaml:"calculation" json:"calculation"`
	Parameters     []Param        `yaml:"parameters" json:"parameters"`
	Outcomes       Outcomes       `yaml:"outcomes" json:"outcomes"`
	Tests          Tests          `yaml:"tests" json:"tests"`
	Implementation Implementation `yaml:"implementation" json:"implementation"`
	Sources        []Source       `yaml:"sources" json:"sources"`
	Notes          string         `yaml:"notes" json:"notes"`
}

type RuleID struct {
	ID       string `yaml:"id" json:"id"`
	Version  string `yaml:"version" json:"version"`
	Type     string `yaml:"type" json:"type"`
	Domain   string `yaml:"domain" json:"domain"`
	Category string `yaml:"category" json:"category"`
	Name     string `yaml:"name" json:"name"`
	Purpose  string `yaml:"purpose" json:"purpose"`
}

type Governance struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	VerificationStatus string `yaml:"verification_status" json:"verification_status"`
	VerificationLabel  string `yaml:"verification_label" json:"verification_label"`
	ReviewedBy         string `yaml:"reviewed_by" json:"reviewed_by"`
	ReviewedOn         string `yaml:"reviewed_on" json:"reviewed_on"`
	ReviewDecision     string `yaml:"review_decision" json:"review_decision"`
	LastModifiedBy     string `yaml:"last_modified_by" json:"last_modified_by"`
	LastModifiedOn     string `yaml:"last_modified_on" json:"last_modified_on"`
	ChangeNote         string `yaml:"change_note" json:"change_note"`
}

type Validity struct {
	ValidFrom     string `yaml:"valid_from" json:"valid_from"`
	ValidTo       string `yaml:"valid_to" json:"valid_to"`
	EffectiveText string `yaml:"effective_text" json:"effective_text"`
	PeriodBasis   string `yaml:"period_basis" json:"period_basis"`
}

type Legal struct {
	Provisions           string `yaml:"provisions" json:"provisions"`
	Applicability        string `yaml:"applicability" json:"applicability"`
	ExemptionsExceptions string `yaml:"exemptions_exceptions" json:"exemptions_exceptions"`
	ThresholdsRates      string `yaml:"thresholds_rates" json:"thresholds_rates"`
	Rounding             string `yaml:"rounding" json:"rounding"`
	DueDates             string `yaml:"due_dates" json:"due_dates"`
}

type Inputs struct {
	Documents        []DocUse `yaml:"documents" json:"documents"`
	Fields           []string `yaml:"fields" json:"fields"`
	Prerequisites    string   `yaml:"prerequisites" json:"prerequisites"`
	InputValidations string   `yaml:"input_validations" json:"input_validations"`
}

type DocUse struct {
	Role     string `yaml:"role" json:"role"`
	DocCode  string `yaml:"doc_code" json:"doc_code"`
	Required bool   `yaml:"required" json:"required"`
}

type Matching struct {
	Keys             []Pair `yaml:"keys" json:"keys"`
	KeyNormalisation string `yaml:"key_normalisation" json:"key_normalisation"`
	Aggregation      string `yaml:"aggregation" json:"aggregation"`
	Comparisons      []Pair `yaml:"comparisons" json:"comparisons"`
	Tolerance        string `yaml:"tolerance" json:"tolerance"`
}

// Pair is left and right (DOC.field each), or an expression.
type Pair struct {
	Left       string `yaml:"left,omitempty" json:"left,omitempty"`
	Right      string `yaml:"right,omitempty" json:"right,omitempty"`
	Expression string `yaml:"expression,omitempty" json:"expression,omitempty"`
}

type Calculation struct {
	Formula string   `yaml:"formula" json:"formula"`
	Steps   []string `yaml:"steps" json:"steps"`
}

// Param is a number a CA may change. Value, Min and Max are text; Type says how Value
// is read (integer | number | boolean | date | string), so no value is ever a float.
type Param struct {
	Key           string   `yaml:"key" json:"key"`
	Value         string   `yaml:"value" json:"value"`
	Type          string   `yaml:"type" json:"type"`
	Unit          string   `yaml:"unit" json:"unit"`
	Label         string   `yaml:"label" json:"label"`
	Editable      bool     `yaml:"editable" json:"editable"`
	Min           string   `yaml:"min,omitempty" json:"min,omitempty"`
	Max           string   `yaml:"max,omitempty" json:"max,omitempty"`
	AllowedValues []string `yaml:"allowed_values,omitempty" json:"allowed_values,omitempty"`
	Note          string   `yaml:"note,omitempty" json:"note,omitempty"`
}

type Outcomes struct {
	Categories          []Category `yaml:"categories" json:"categories"`
	DefaultSeverity     string     `yaml:"default_severity" json:"default_severity"`
	ExpectedOutput      string     `yaml:"expected_output" json:"expected_output"`
	RecommendedAction   string     `yaml:"recommended_action" json:"recommended_action"`
	CommonDiscrepancies string     `yaml:"common_discrepancies" json:"common_discrepancies"`
}

type Category struct {
	Code        string `yaml:"code" json:"code"`
	Description string `yaml:"description" json:"description"`
	Severity    string `yaml:"severity" json:"severity"`
	Action      string `yaml:"action,omitempty" json:"action,omitempty"`
}

type Tests struct {
	WorkedExample string `yaml:"worked_example" json:"worked_example"`
	Positive      string `yaml:"positive" json:"positive"`
	Negative      string `yaml:"negative" json:"negative"`
}

type Implementation struct {
	Handler    string `yaml:"handler" json:"handler"`
	Pseudocode string `yaml:"pseudocode" json:"pseudocode"`
}

type Source struct {
	Title    string `yaml:"title" json:"title"`
	URL      string `yaml:"url" json:"url"`
	Ref      string `yaml:"ref" json:"ref"`
	Official bool   `yaml:"official" json:"official"`
}

type DocFile struct {
	SchemaVersion string   `yaml:"schema_version" json:"schema_version"`
	Document      Document `yaml:"document" json:"document"`
	Fields        []Field  `yaml:"fields" json:"fields"`
}

type Document struct {
	Code               string   `yaml:"code" json:"code"`
	Name               string   `yaml:"name" json:"name"`
	IssuerSystem       string   `yaml:"issuer_system" json:"issuer_system"`
	LegalBasis         string   `yaml:"legal_basis" json:"legal_basis"`
	NewFormNo          string   `yaml:"new_form_no" json:"new_form_no"`
	Frequency          string   `yaml:"frequency" json:"frequency"`
	FormatsAvailable   string   `yaml:"formats_available" json:"formats_available"`
	HowObtained        string   `yaml:"how_obtained" json:"how_obtained"`
	DueDate            string   `yaml:"due_date" json:"due_date"`
	TablesUsed         string   `yaml:"tables_used" json:"tables_used"`
	UsedByRules        []string `yaml:"used_by_rules" json:"used_by_rules"`
	VerificationStatus string   `yaml:"verification_status" json:"verification_status"`
	Notes              string   `yaml:"notes" json:"notes"`
	Sources            []Source `yaml:"sources" json:"sources"`
}

// Field is one field of a document. Required is "yes", "no" or "" (not known).
type Field struct {
	Name        string `yaml:"name" json:"name"`
	Label       string `yaml:"label" json:"label"`
	SourcePath  string `yaml:"source_path" json:"source_path"`
	DataType    string `yaml:"data_type" json:"data_type"`
	Required    string `yaml:"required" json:"required"`
	Validation  string `yaml:"validation" json:"validation"`
	Description string `yaml:"description" json:"description"`
	Origin      string `yaml:"origin" json:"origin"`
}

type RuleIndex struct {
	SchemaVersion string         `yaml:"schema_version" json:"schema_version"`
	GeneratedOn   string         `yaml:"generated_on" json:"generated_on"`
	Count         int            `yaml:"count" json:"count"`
	Rules         []RuleIndexRow `yaml:"rules" json:"rules"`
}

type RuleIndexRow struct {
	ID                 string   `yaml:"id" json:"id"`
	Type               string   `yaml:"type" json:"type"`
	Domain             string   `yaml:"domain" json:"domain"`
	Name               string   `yaml:"name" json:"name"`
	Documents          []string `yaml:"documents" json:"documents"`
	VerificationStatus string   `yaml:"verification_status" json:"verification_status"`
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	File               string   `yaml:"file" json:"file"`
}

type DocIndex struct {
	SchemaVersion string        `yaml:"schema_version" json:"schema_version"`
	Count         int           `yaml:"count" json:"count"`
	Documents     []DocIndexRow `yaml:"documents" json:"documents"`
}

type DocIndexRow struct {
	Code   string `yaml:"code" json:"code"`
	Name   string `yaml:"name" json:"name"`
	Fields int    `yaml:"fields" json:"fields"`
	File   string `yaml:"file" json:"file"`
}

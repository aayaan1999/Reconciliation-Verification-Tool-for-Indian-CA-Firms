package main

// The YAML rule store (docs/Recon_Tool_YAML_Rule_Store/recon_yaml, technical spec §B4.2):
// one file per rule and per document type, every text value quoted (RU-02), so each file
// loads through internal/strictyaml. rulestore-import writes it from the rule-book
// workbook; rulestore-check validates it. An empty text value is "" (never null).

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

const (
	rsSchemaVersion   = "1.0"
	rsDefaultStore    = "docs/Recon_Tool_YAML_Rule_Store/recon_yaml"
	rsDefaultWorkbook = "docs/Recon_Tool_Rulebook_v2.xlsx"
)

type rsRule struct {
	SchemaVersion  string           `yaml:"schema_version"`
	Rule           rsRuleID         `yaml:"rule"`
	Governance     rsGovernance     `yaml:"governance"`
	Validity       rsValidity       `yaml:"validity"`
	Legal          rsLegal          `yaml:"legal"`
	Inputs         rsInputs         `yaml:"inputs"`
	Matching       rsMatching       `yaml:"matching"`
	Calculation    rsCalculation    `yaml:"calculation"`
	Parameters     []rsParam        `yaml:"parameters"`
	Outcomes       rsOutcomes       `yaml:"outcomes"`
	Tests          rsTests          `yaml:"tests"`
	Implementation rsImplementation `yaml:"implementation"`
	Sources        []rsSource       `yaml:"sources"`
	Notes          string           `yaml:"notes"`
}

type rsRuleID struct {
	ID       string `yaml:"id"`
	Version  string `yaml:"version"`
	Type     string `yaml:"type"`
	Domain   string `yaml:"domain"`
	Category string `yaml:"category"`
	Name     string `yaml:"name"`
	Purpose  string `yaml:"purpose"`
}

type rsGovernance struct {
	Enabled            bool   `yaml:"enabled"`
	VerificationStatus string `yaml:"verification_status"`
	VerificationLabel  string `yaml:"verification_label"`
	ReviewedBy         string `yaml:"reviewed_by"`
	ReviewedOn         string `yaml:"reviewed_on"`
	ReviewDecision     string `yaml:"review_decision"`
	LastModifiedBy     string `yaml:"last_modified_by"`
	LastModifiedOn     string `yaml:"last_modified_on"`
	ChangeNote         string `yaml:"change_note"`
}

type rsValidity struct {
	ValidFrom     string `yaml:"valid_from"`
	ValidTo       string `yaml:"valid_to"`
	EffectiveText string `yaml:"effective_text"`
	PeriodBasis   string `yaml:"period_basis"`
}

type rsLegal struct {
	Provisions           string `yaml:"provisions"`
	Applicability        string `yaml:"applicability"`
	ExemptionsExceptions string `yaml:"exemptions_exceptions"`
	ThresholdsRates      string `yaml:"thresholds_rates"`
	Rounding             string `yaml:"rounding"`
	DueDates             string `yaml:"due_dates"`
}

type rsInputs struct {
	Documents        []rsDocUse `yaml:"documents"`
	Fields           []string   `yaml:"fields"`
	Prerequisites    string     `yaml:"prerequisites"`
	InputValidations string     `yaml:"input_validations"`
}

type rsDocUse struct {
	Role     string `yaml:"role"`
	DocCode  string `yaml:"doc_code"`
	Required bool   `yaml:"required"`
}

type rsMatching struct {
	Keys             []rsPair `yaml:"keys"`
	KeyNormalisation string   `yaml:"key_normalisation"`
	Aggregation      string   `yaml:"aggregation"`
	Comparisons      []rsPair `yaml:"comparisons"`
	Tolerance        string   `yaml:"tolerance"`
}

// rsPair is left and right (DOC.field each), or an expression.
type rsPair struct {
	Left       string `yaml:"left,omitempty"`
	Right      string `yaml:"right,omitempty"`
	Expression string `yaml:"expression,omitempty"`
}

type rsCalculation struct {
	Formula string   `yaml:"formula"`
	Steps   []string `yaml:"steps"`
}

// rsParam is a number a CA may change. Value, Min and Max are text; Type says how Value
// is read (integer | number | boolean | date | string), so no value is ever a float.
type rsParam struct {
	Key           string   `yaml:"key"`
	Value         string   `yaml:"value"`
	Type          string   `yaml:"type"`
	Unit          string   `yaml:"unit"`
	Label         string   `yaml:"label"`
	Editable      bool     `yaml:"editable"`
	Min           string   `yaml:"min,omitempty"`
	Max           string   `yaml:"max,omitempty"`
	AllowedValues []string `yaml:"allowed_values,omitempty"`
	Note          string   `yaml:"note,omitempty"`
}

type rsOutcomes struct {
	Categories          []rsCategory `yaml:"categories"`
	DefaultSeverity     string       `yaml:"default_severity"`
	ExpectedOutput      string       `yaml:"expected_output"`
	RecommendedAction   string       `yaml:"recommended_action"`
	CommonDiscrepancies string       `yaml:"common_discrepancies"`
}

type rsCategory struct {
	Code        string `yaml:"code"`
	Description string `yaml:"description"`
	Severity    string `yaml:"severity"`
	Action      string `yaml:"action,omitempty"`
}

type rsTests struct {
	WorkedExample string `yaml:"worked_example"`
	Positive      string `yaml:"positive"`
	Negative      string `yaml:"negative"`
}

type rsImplementation struct {
	Handler    string `yaml:"handler"`
	Pseudocode string `yaml:"pseudocode"`
}

type rsSource struct {
	Title    string `yaml:"title"`
	URL      string `yaml:"url"`
	Ref      string `yaml:"ref"`
	Official bool   `yaml:"official"`
}

type rsDocFile struct {
	SchemaVersion string     `yaml:"schema_version"`
	Document      rsDocument `yaml:"document"`
	Fields        []rsField  `yaml:"fields"`
}

type rsDocument struct {
	Code               string     `yaml:"code"`
	Name               string     `yaml:"name"`
	IssuerSystem       string     `yaml:"issuer_system"`
	LegalBasis         string     `yaml:"legal_basis"`
	NewFormNo          string     `yaml:"new_form_no"`
	Frequency          string     `yaml:"frequency"`
	FormatsAvailable   string     `yaml:"formats_available"`
	HowObtained        string     `yaml:"how_obtained"`
	DueDate            string     `yaml:"due_date"`
	TablesUsed         string     `yaml:"tables_used"`
	UsedByRules        []string   `yaml:"used_by_rules"`
	VerificationStatus string     `yaml:"verification_status"`
	Notes              string     `yaml:"notes"`
	Sources            []rsSource `yaml:"sources"`
}

// rsField is one field of a document. Required is "yes", "no" or "" (not known).
type rsField struct {
	Name        string `yaml:"name"`
	Label       string `yaml:"label"`
	SourcePath  string `yaml:"source_path"`
	DataType    string `yaml:"data_type"`
	Required    string `yaml:"required"`
	Validation  string `yaml:"validation"`
	Description string `yaml:"description"`
	Origin      string `yaml:"origin"`
}

type rsRuleIndex struct {
	SchemaVersion string           `yaml:"schema_version"`
	GeneratedOn   string           `yaml:"generated_on"`
	Count         int              `yaml:"count"`
	Rules         []rsRuleIndexRow `yaml:"rules"`
}

type rsRuleIndexRow struct {
	ID                 string   `yaml:"id"`
	Type               string   `yaml:"type"`
	Domain             string   `yaml:"domain"`
	Name               string   `yaml:"name"`
	Documents          []string `yaml:"documents"`
	VerificationStatus string   `yaml:"verification_status"`
	Enabled            bool     `yaml:"enabled"`
	File               string   `yaml:"file"`
}

type rsDocIndex struct {
	SchemaVersion string          `yaml:"schema_version"`
	Count         int             `yaml:"count"`
	Documents     []rsDocIndexRow `yaml:"documents"`
}

type rsDocIndexRow struct {
	Code   string `yaml:"code"`
	Name   string `yaml:"name"`
	Fields int    `yaml:"fields"`
	File   string `yaml:"file"`
}

// rsWrite writes v as YAML with every text value quoted (multi-line text as a | block),
// then reads it back through internal/strictyaml and checks nothing changed.
func rsWrite(path string, v any, back any) error {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return err
	}
	quoteStrings(&n)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&n); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if _, err := strictyaml.Decode(path, buf.Bytes(), back); err != nil {
		return fmt.Errorf("written YAML does not read back strictly: %w", err)
	}
	if err := sameYAML(v, back); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// quoteStrings gives every string scalar a quoted (or | block) style. Mapping keys stay plain.
func quoteStrings(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			quoteStrings(c)
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			quoteStrings(n.Content[i])
		}
	case yaml.ScalarNode:
		if n.Tag == "!!str" {
			n.Style = yaml.DoubleQuotedStyle
			if bytes.ContainsRune([]byte(n.Value), '\n') && blockSafe(n.Value) {
				n.Style = yaml.LiteralStyle
			}
		}
	}
}

// blockSafe reports whether text survives a | block unchanged: no carriage returns,
// tabs or trailing spaces, and no leading blank line.
func blockSafe(s string) bool {
	if s == "" || s[0] == '\n' || s[0] == ' ' {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r', '\t':
			return false
		case '\n':
			if i > 0 && s[i-1] == ' ' {
				return false
			}
		}
	}
	return s[len(s)-1] != ' ' && s[len(s)-1] != '\n'
}

// sameYAML compares what was written with what was read back.
func sameYAML(a, b any) error {
	ea, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	eb, err := yaml.Marshal(b)
	if err != nil {
		return err
	}
	if !bytes.Equal(ea, eb) {
		return fmt.Errorf("the file reads back differently from what was written")
	}
	return nil
}

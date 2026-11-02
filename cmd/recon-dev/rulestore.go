package main

// The YAML rule store (docs/Recon_Tool_YAML_Rule_Store/recon_yaml, technical spec §B4.2):
// one file per rule and per document type, every text value quoted (RU-02), so each file
// loads through internal/strictyaml. rulestore-import writes it from the rule-book
// workbook; rulestore-check validates it. An empty text value is "" (never null).

import (
	"os"
	"path/filepath"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
)

const (
	rsSchemaVersion   = rulestore.SchemaVersion
	rsDefaultStore    = "docs/Recon_Tool_YAML_Rule_Store/recon_yaml"
	rsDefaultWorkbook = "docs/Recon_Tool_Rulebook_v2.xlsx"
)

// The file structures live in internal/rulestore, shared with recon-engine.
type (
	rsRule           = rulestore.Rule
	rsRuleID         = rulestore.RuleID
	rsGovernance     = rulestore.Governance
	rsValidity       = rulestore.Validity
	rsLegal          = rulestore.Legal
	rsInputs         = rulestore.Inputs
	rsDocUse         = rulestore.DocUse
	rsMatching       = rulestore.Matching
	rsPair           = rulestore.Pair
	rsCalculation    = rulestore.Calculation
	rsParam          = rulestore.Param
	rsOutcomes       = rulestore.Outcomes
	rsCategory       = rulestore.Category
	rsTests          = rulestore.Tests
	rsImplementation = rulestore.Implementation
	rsSource         = rulestore.Source
	rsDocFile        = rulestore.DocFile
	rsDocument       = rulestore.Document
	rsField          = rulestore.Field
	rsRuleIndex      = rulestore.RuleIndex
	rsRuleIndexRow   = rulestore.RuleIndexRow
	rsDocIndex       = rulestore.DocIndex
	rsDocIndexRow    = rulestore.DocIndexRow
)

// rsWrite writes v as YAML with every text value quoted (multi-line text as a | block),
// after reading it back through internal/strictyaml and checking nothing changed.
func rsWrite(path string, v any, back any) error {
	data, err := rulestore.EncodeYAML(path, v, back)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

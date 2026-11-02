package findings

import (
	"encoding/json"
	"fmt"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// storedReport mirrors ValidationReportObject.
type storedReport struct {
	File           string `json:"file"`
	FileType       string `json:"file_type"`
	DocumentType   string `json:"document_type"` // missing from reports stored before SPEC-005
	ProfileID      string `json:"profile_id"`
	ProfileVersion string `json:"profile_version"`
	SourceSystem   string `json:"source_system"`
	TaxYear        string `json:"tax_year"`
	ClientMatch    string `json:"client_match"`
	PANLast4       string `json:"pan_last4"`
	FactCount      int    `json:"fact_count"`
	RecordCount    int    `json:"record_count"`
	Unread         []struct {
		Path   string `json:"path"`
		Kind   string `json:"kind"` // missing from reports stored before 15 October 2026
		Row    string `json:"row"`
		Reason string `json:"reason"`
	} `json:"unread"`
	Warnings []storedIssue `json:"warnings"`
	Errors   []storedIssue `json:"errors"`
}

type storedIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// ParseValidationReport reads back a validation report written by MarshalValidationReport,
// so a client's later run can show the reports of documents uploaded earlier.
func ParseValidationReport(b []byte) (ingest.ValidationReport, error) {
	var s storedReport
	if err := json.Unmarshal(b, &s); err != nil {
		return ingest.ValidationReport{}, fmt.Errorf("findings: stored validation report: %w", err)
	}
	r := ingest.ValidationReport{
		File: s.File, FileType: s.FileType, DocumentType: s.DocumentType, ProfileID: s.ProfileID, ProfileVersion: s.ProfileVersion,
		SourceSystem: s.SourceSystem, ClientMatch: ingest.ClientMatch(s.ClientMatch), PANLast4: s.PANLast4,
		FactCount: s.FactCount, RecordCount: s.RecordCount,
	}
	if s.TaxYear != "" {
		ty, err := period.ParseTaxYear(s.TaxYear)
		if err != nil {
			return ingest.ValidationReport{}, fmt.Errorf("findings: stored validation report: %w", err)
		}
		r.TaxYear = ty
	}
	for _, u := range s.Unread {
		kind := ingest.UnreadKind(u.Kind)
		if kind == "" {
			kind = ingest.UnreadOther
		}
		r.Unread = append(r.Unread, ingest.Unread{Locator: domain.Locator{FileName: s.File, Path: u.Path}, Kind: kind, Row: u.Row, Reason: u.Reason})
	}
	issue := func(i storedIssue) ingest.Issue {
		return ingest.Issue{Code: ingest.Code(i.Code), Msg: i.Message, Locator: domain.Locator{FileName: s.File, Path: i.Path}}
	}
	for _, w := range s.Warnings {
		r.Warnings = append(r.Warnings, issue(w))
	}
	for _, e := range s.Errors {
		r.Errors = append(r.Errors, issue(e))
	}
	return r, nil
}

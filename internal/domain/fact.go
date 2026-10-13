package domain

import "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"

// A Fact is one figure read from one document (FV-01; spec v0.2 M2 model): a standard
// name from the vocabulary, a value, the period and scope it belongs to, and where it
// was found. The embedded Value holds the figure, so f.Money, f.Text and so on read it.
type Fact struct {
	EngagementID int64
	Name         string // from the vocabulary, e.g. "computation.allowed_depreciation"
	Period       string // canonical period: "FY2025-26" | "FY2025-26:Q1" | "2025-04"
	Scope        string // "" | GSTIN | branch code | Tally company id (IN-11)
	Value
	Locator Locator
}

// A Record is one row of a tabular source: a voucher, invoice, challan or ECR line.
// Records exist because GST, TDS and payroll matching is row by row, not one figure
// against another (RC-03..05).
type Record struct {
	EngagementID int64
	RecordType   string // from the vocabulary: "voucher" | "gst_invoice" | "tds_challan" | "ecr_row" ...
	Period       string
	Scope        string
	MatchKey     string                 // canonical per record type, e.g. "27ABCDE1234F1Z5|INV-0042|2025-05-14"
	Amounts      map[string]money.Paise // named per record-type schema
	Fields       map[string]string      // named per record-type schema
	Locator      Locator
}

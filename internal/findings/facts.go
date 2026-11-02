package findings

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// A FactList is the current view of one or more engagements as stored: every document's
// latest draft and every figure and row read from it, with the file and place it came from
// (FV-01, FD-02). Like Report, it holds no database ID or time, so it is byte-stable.
type FactList struct {
	Engagements []EngagementFacts // sorted by tax year
}

// EngagementFacts is the current view of one engagement.
type EngagementFacts struct {
	TaxYear   string
	Documents []DocumentFacts // sorted by document type, period, scope and part
}

// DocumentFacts is one document's latest draft, its figures and its rows.
type DocumentFacts struct {
	DocumentType, Period, Scope, Part string
	FileName, SHA256                  string
	DraftNo                           int64
	ProfileID, ProfileVersion         string
	Facts                             []domain.Fact   // sorted by name, period, scope and place
	Records                           []domain.Record // rows such as invoices; sorted by type, key and place
}

// Text renders the list for the terminal: one line per figure, with its value and source.
// Money is always shown to the paisa, exactly as stored.
func (l *FactList) Text() (string, error) {
	var b strings.Builder
	for i, e := range l.Engagements {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Tax year %s: %s\n", e.TaxYear, plural(len(e.Documents), "document"))
		for _, d := range e.Documents {
			fmt.Fprintf(&b, "\n%s, %s, draft %d: %s (profile %s v%s)\n", d.DocumentType, orDash(d.Period), d.DraftNo, d.FileName, d.ProfileID, d.ProfileVersion)
			if d.Scope != "" || d.Part != "" {
				fmt.Fprintf(&b, "  Scope %s, part %s\n", orDash(d.Scope), orDash(d.Part))
			}
			if len(d.Facts) == 0 && len(d.Records) == 0 {
				b.WriteString("  No figures or rows (the file had errors, or the profile reads none)\n")
			}
			for _, f := range d.Facts {
				v, err := factValueText(f.Value)
				if err != nil {
					return "", fmt.Errorf("%s: %w", f.Name, err)
				}
				where := f.Period
				if f.Scope != "" {
					where += ", " + f.Scope
				}
				fmt.Fprintf(&b, "  %s (%s): %s  [%s]\n", f.Name, where, v, f.Locator.Display())
			}
			if n := len(d.Records); n > 0 {
				fmt.Fprintf(&b, "  %s:\n", plural(n, "row"))
			}
			for _, r := range d.Records {
				var parts []string
				for _, k := range slices.Sorted(maps.Keys(r.Amounts)) {
					v, err := money.FormatIndian(r.Amounts[k], true)
					if err != nil {
						return "", fmt.Errorf("row %s, %s: %w", r.MatchKey, k, err)
					}
					parts = append(parts, k+" "+v)
				}
				fmt.Fprintf(&b, "  %s %s: %s  [%s]\n", r.RecordType, r.MatchKey, strings.Join(parts, ", "), r.Locator.Display())
			}
		}
	}
	return b.String(), nil
}

func factValueText(v domain.Value) (string, error) {
	if v.Kind == domain.KindMoney {
		return money.FormatIndian(v.Money, true)
	}
	return domain.FormatValue(v), nil
}

// JSON renders the same content as canonical JSON, for the screens.
func (l *FactList) JSON() ([]byte, error) {
	engs := make([]any, 0, len(l.Engagements))
	for _, e := range l.Engagements {
		docs := make([]any, 0, len(e.Documents))
		for _, d := range e.Documents {
			facts := make([]any, 0, len(d.Facts))
			for _, f := range d.Facts {
				val, err := valueJSON(f.Value)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", f.Name, err)
				}
				facts = append(facts, map[string]any{
					"name": f.Name, "period": f.Period, "scope": f.Scope, "value": val,
					"locator": map[string]any{"file_name": f.Locator.FileName, "kind": f.Locator.Kind, "path": f.Locator.Path},
				})
			}
			records := make([]any, 0, len(d.Records))
			for _, r := range d.Records {
				amounts := map[string]any{}
				for k, v := range r.Amounts {
					amounts[k] = int64(v)
				}
				records = append(records, map[string]any{
					"record_type": r.RecordType, "match_key": r.MatchKey, "period": r.Period, "scope": r.Scope,
					"fields": r.Fields, "amounts": amounts,
					"locator": map[string]any{"file_name": r.Locator.FileName, "kind": r.Locator.Kind, "path": r.Locator.Path},
				})
			}
			docs = append(docs, map[string]any{
				"document_type": d.DocumentType, "period": d.Period, "scope": d.Scope, "part": d.Part,
				"file_name": d.FileName, "file_sha256": d.SHA256, "draft_no": d.DraftNo,
				"profile_id": d.ProfileID, "profile_version": d.ProfileVersion, "facts": facts, "records": records,
			})
		}
		engs = append(engs, map[string]any{"tax_year": e.TaxYear, "documents": docs})
	}
	return marshal(map[string]any{"engagements": engs})
}

// valueJSON writes a value as the canonical encoding does: only the field its kind names.
func valueJSON(v domain.Value) (map[string]any, error) {
	switch v.Kind {
	case domain.KindMoney:
		return map[string]any{"kind": "money", "paise": int64(v.Money)}, nil
	case domain.KindInt:
		return map[string]any{"kind": "int", "int": v.Int}, nil
	case domain.KindText:
		return map[string]any{"kind": "text", "text": v.Text}, nil
	case domain.KindDate:
		return map[string]any{"kind": "date", "date": v.Date}, nil
	case domain.KindBool:
		return map[string]any{"kind": "bool", "bool": v.Bool}, nil
	case domain.KindRate:
		return map[string]any{"kind": "rate", "rate_bp": v.RateBP}, nil
	}
	return nil, fmt.Errorf("invalid value kind %d", v.Kind)
}

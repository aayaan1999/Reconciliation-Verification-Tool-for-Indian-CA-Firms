package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
)

// Access to client data (SPEC-013). This file holds what the routes share: the audit entries
// the server writes (a refusal, a view of a client file, an export, a deleted finding), the
// masking of GSTINs for roles that only read, and the hiding of deleted findings from roles
// other than the Partner. The role checks themselves are in authorize (auth.go) and the matrix.

// viewWindow is how long one user's reloads of one client file's year count as one view.
const viewWindow = "2006-01-02T15:04" // one minute, UTC

// audit writes one audit entry in its own transaction. The actor is a login name the session
// gave, never a name the screen typed.
func (s *Server) audit(actor, action, objectType, objectID string, details map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.Audit(time.Now().UTC().Format(time.RFC3339), actor, action, objectType, objectID, details); err != nil {
		return err
	}
	return tx.Commit()
}

// recordDenied writes the auth.denied entry for a request the role check refused: who asked,
// the route and the permission. It holds no client figures.
func (s *Server) recordDenied(acc auth.Account, route string, perm auth.Permission) error {
	return s.audit(acc.LoginName, "auth.denied", "route", route, map[string]string{
		"permission": string(perm), "role": string(acc.Role)})
}

// recordView writes one client.view entry for a user, a client and a tax year, and not another
// for the same user, client and year within the same minute (SPEC-013 §2). The memory of the
// last view is kept by the server and reset on restart; the audit rows are not changed by it.
func (s *Server) recordView(acc auth.Account, clientID, taxYear string) error {
	window := time.Now().UTC().Format(viewWindow)
	key := acc.LoginName + "|" + clientID + "|" + taxYear
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.viewed == nil {
		s.viewed = map[string]string{}
	}
	if s.viewed[key] == window {
		return nil
	}
	if err := s.audit(acc.LoginName, "client.view", "client", clientID, map[string]string{"tax_year": taxYear}); err != nil {
		return err
	}
	s.viewed[key] = window
	return nil
}

// deletion is a finding's deletion as the audit log holds it.
type deletion struct {
	By     string
	At     string
	Reason string
}

// deletedFindings returns the findings of a client's tax year that were marked deleted, by
// finding ID. A deletion is the finding.deleted audit entry (SPEC-013 §4); the finding is kept
// and is never removed from the database.
func (s *Server) deletedFindings(clientID, taxYear string) (map[string]deletion, error) {
	entries, err := s.auth.AuditEntries()
	if err != nil {
		return nil, err
	}
	out := map[string]deletion{}
	for _, e := range entries {
		if e.Action != "finding.deleted" {
			continue
		}
		var d map[string]string
		if err := json.Unmarshal([]byte(e.Details), &d); err != nil {
			continue
		}
		if d["client_id"] != clientID || d["tax_year"] != taxYear {
			continue
		}
		out[e.ObjectID] = deletion{By: e.Actor, At: e.At, Reason: d["reason"]}
	}
	return out, nil
}

// numbers are read as json.Number, so every paise amount keeps its exact digits.
func decodeKeepingNumbers(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// reportsOf returns the reports inside a result or a history: the latest report of any type and
// the report of each type of check.
func reportsOf(m map[string]any) []map[string]any {
	var out []map[string]any
	if rep, ok := m["report"].(map[string]any); ok {
		out = append(out, rep)
	}
	list, _ := m["results"].([]any)
	for _, r := range list {
		if rm, ok := r.(map[string]any); ok {
			if rep, ok := rm["report"].(map[string]any); ok {
				out = append(out, rep)
			}
		}
	}
	return out
}

// filterDeleted hides the deleted findings of a result or history from roles other than the
// Partner. The Partner still sees them, marked deleted with who deleted them and why. The fail
// count of each report is brought into line with the findings it still lists.
func filterDeleted(body []byte, partner bool, deleted map[string]deletion) ([]byte, error) {
	if len(deleted) == 0 {
		return body, nil
	}
	m, err := decodeKeepingNumbers(body)
	if err != nil {
		return nil, fmt.Errorf("the result: %w", err)
	}
	for _, rep := range reportsOf(m) {
		removed := hideIn(rep, partner, deleted)
		if counts, ok := rep["counts"].(map[string]any); ok && removed > 0 {
			if fail, ok := counts["fail"].(json.Number); ok {
				n, err := fail.Int64()
				if err != nil {
					return nil, fmt.Errorf("the result: fail count: %w", err)
				}
				counts["fail"] = json.Number(strconv.FormatInt(n-int64(removed), 10))
			}
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// hideIn removes (or, for the Partner, marks) the deleted findings of one report and returns how
// many it removed.
func hideIn(rep map[string]any, partner bool, deleted map[string]deletion) int {
	removed := 0
	groups, _ := rep["findings"].([]any)
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		list, _ := gm["findings"].([]any)
		kept := []any{}
		for _, f := range list {
			fm, ok := f.(map[string]any)
			if !ok {
				kept = append(kept, f)
				continue
			}
			id, _ := fm["finding_id"].(string)
			d, isDeleted := deleted[id]
			switch {
			case !isDeleted:
				kept = append(kept, f)
			case partner:
				fm["deleted"] = map[string]any{"by": d.By, "at": d.At, "reason": d.Reason}
				kept = append(kept, f)
			default:
				removed++
			}
		}
		gm["findings"] = kept
	}
	return removed
}

// findingRule returns the rule of a finding in a result or history, and false when the result
// does not list it.
func findingRule(m map[string]any, findingID string) (string, bool) {
	for _, rep := range reportsOf(m) {
		groups, _ := rep["findings"].([]any)
		for _, g := range groups {
			gm, _ := g.(map[string]any)
			list, _ := gm["findings"].([]any)
			for _, f := range list {
				fm, _ := f.(map[string]any)
				if id, _ := fm["finding_id"].(string); id == findingID {
					rule, _ := fm["rule_id"].(string)
					return rule, true
				}
			}
		}
	}
	return "", false
}

// maskGSTIN shows only the last four characters of a GSTIN.
func maskGSTIN(g string) string {
	if len(g) <= 4 {
		return strings.Repeat("*", len(g))
	}
	return strings.Repeat("*", len(g)-4) + g[len(g)-4:]
}

// maskDetails masks the GSTINs of a client's details for a role that only reads (SPEC-013 §3).
// The PAN is already sent as its last four characters only. The body is {"id", "details"}.
func maskDetails(body []byte) ([]byte, error) {
	m, err := decodeKeepingNumbers(body)
	if err != nil {
		return nil, fmt.Errorf("the client's details: %w", err)
	}
	d, _ := m["details"].(map[string]any)
	gstins, _ := d["gstins"].([]any)
	for _, g := range gstins {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := gm["gstin"].(string); ok {
			gm["gstin"] = maskGSTIN(v)
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// seesFullIdentity reports whether a role sees a client's GSTINs in full: the roles that enter
// client details (the matrix's run permission). Read-only does not.
func seesFullIdentity(role auth.Role) bool {
	return auth.Allowed(role, auth.PermRun)
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/export"
)

// The Excel export, the two exports not built yet, and the deletion of a finding (SPEC-013 §4).
// Each route declares its permission in endpoints(): exports and the query letter need export,
// a deletion needs delete_findings. The server makes the export file itself and gives it a
// generated name; nothing is written under a name from the request.

// findingIDPattern is the form of a finding's stable ID: 16 characters of base32 (domain
// identity.go).
var findingIDPattern = regexp.MustCompile(`^[A-Z2-7]{16}$`)

// exportMIME is the type of an Excel workbook.
const exportMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// exportRequest is the body of POST /api/v1/clients/{id}/exports.
type exportRequest struct {
	TaxYear string `json:"tax_year"`
	Format  string `json:"format"`
}

// exportClient builds the Excel export of a client's findings, cannot-check items and Worklist
// for a tax year, writes its export.created entry and sends the file. A PDF is not available yet
// (AC-7d); it is refused with the same message as the query letter, and nothing is written.
func (s *Server) exportClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	var in exportRequest
	if !decodeAuthBody(w, r, &in, `send {"tax_year": "2025-26", "format": "xlsx"} as JSON`) {
		return
	}
	if !taxYearPattern.MatchString(in.TaxYear) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	switch in.Format {
	case "xlsx":
	case "pdf":
		writeNotAvailable(w, "The PDF export is not available yet. It comes after the PDF library licence review.")
		return
	default:
		writeError(w, http.StatusBadRequest, "BAD_FORMAT", "the format is xlsx")
		return
	}
	clientID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	acc := accountFrom(r)
	status, body := s.engineJSON("", "result", "--db", s.cfg.DB, "--client", id, "--tax-year", in.TaxYear)
	if status != http.StatusOK {
		writeJSON(w, status, body)
		return
	}
	status, wl := s.engineJSON("", "worklist", "--db", s.cfg.DB, "--tax-year", in.TaxYear)
	if status != http.StatusOK {
		writeJSON(w, status, wl)
		return
	}
	resultRaw, ok1 := body.(json.RawMessage)
	worklistRaw, ok2 := wl.(json.RawMessage)
	if !ok1 || !ok2 {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "the engine's answer could not be read")
		return
	}
	src, err := export.ReadSource(clientID, resultRaw, worklistRaw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	deleted, err := s.deletedFindings(id, in.TaxYear)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	hidden := map[string]bool{}
	for f := range deleted {
		hidden[f] = true
	}
	exportID, err := export.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	file, counts, err := export.Workbook(export.Watermark{Firm: s.cfg.Firm, User: acc.LoginName, At: time.Now(), ExportID: exportID}, src, hidden)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	// The entry is written before the file is sent: an export that cannot be recorded is not
	// given out. It holds the row counts, and no figure of the client.
	if err := s.audit(acc.LoginName, "export.created", "export", exportID, map[string]string{
		"client_id":    id,
		"tax_year":     in.TaxYear,
		"format":       "xlsx",
		"sheets":       strconv.Itoa(counts.Sheets),
		"findings":     strconv.Itoa(counts.Findings),
		"cannot_check": strconv.Itoa(counts.CannotCheck),
		"worklist":     strconv.Itoa(counts.Worklist),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	w.Header().Set("Content-Type", exportMIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="recon-export-%s.xlsx"`, exportID))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file)
}

// queryLetter is the client query letter, which is not available yet (SPEC-013 §4, AC-7d).
func (s *Server) queryLetter(w http.ResponseWriter, _ *http.Request) {
	writeNotAvailable(w, "The client query letter is not available yet. It comes after the PDF library licence review.")
}

// writeNotAvailable answers a request for an output that is not built yet: 501, with the
// stable code NOT_AVAILABLE_YET. It writes no export and no audit entry.
func writeNotAvailable(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE_YET", msg)
}

// deleteRequest is the body of POST /api/v1/clients/{id}/findings/{finding}/delete.
type deleteRequest struct {
	TaxYear string `json:"tax_year"`
	Reason  string `json:"reason"`
}

// deleteFinding marks a finding deleted, with a reason (SPEC-013 §4, AC-7c). The finding is not
// removed: the finding.deleted entry marks it, and a Partner still lists it. The finding must be
// in the client's result for the tax year, and not already deleted.
func (s *Server) deleteFinding(w http.ResponseWriter, r *http.Request) {
	id, finding := r.PathValue("id"), r.PathValue("finding")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_CLIENT_ID", "the client ID is a whole number")
		return
	}
	if !findingIDPattern.MatchString(finding) {
		writeError(w, http.StatusBadRequest, "BAD_FINDING_ID", "the finding ID is 16 capital letters and digits")
		return
	}
	var in deleteRequest
	if !decodeAuthBody(w, r, &in, `send {"tax_year": "2025-26", "reason": "..."} as JSON`) {
		return
	}
	if !taxYearPattern.MatchString(in.TaxYear) {
		writeError(w, http.StatusBadRequest, "BAD_TAX_YEAR", "choose a tax year such as 2025-26")
		return
	}
	reason := strings.TrimSpace(in.Reason)
	switch {
	case reason == "":
		writeError(w, http.StatusBadRequest, "REASON_REQUIRED", "say why the finding is deleted")
		return
	case len([]rune(reason)) > 300:
		writeError(w, http.StatusBadRequest, "BAD_REASON", "the reason has at most 300 characters")
		return
	}
	acc := accountFrom(r)
	status, body := s.engineJSON("", "result", "--db", s.cfg.DB, "--client", id, "--tax-year", in.TaxYear)
	if status != http.StatusOK {
		writeJSON(w, status, body)
		return
	}
	raw, ok := body.(json.RawMessage)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "the engine's answer could not be read")
		return
	}
	m, err := decodeKeepingNumbers(raw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	rule, found := findingRule(m, finding)
	if !found {
		writeError(w, http.StatusNotFound, "FINDING_NOT_FOUND", "this client's result for that tax year has no such finding")
		return
	}
	deleted, err := s.deletedFindings(id, in.TaxYear)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	if _, already := deleted[finding]; already {
		writeError(w, http.StatusConflict, "ALREADY_DELETED", "this finding is already deleted")
		return
	}
	if err := s.audit(acc.LoginName, "finding.deleted", "finding", finding, map[string]string{
		"client_id": id,
		"tax_year":  in.TaxYear,
		"rule_id":   rule,
		"reason":    reason,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"finding_id": finding, "deleted": true})
}

// workItemClients returns the client IDs in a Worklist answer, each once, in order.
func workItemClients(raw []byte) ([]string, error) {
	var list struct {
		Items []struct {
			ClientID int64 `json:"client_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	var ids []int64
	for _, it := range list.Items {
		if !seen[it.ClientID] {
			seen[it.ClientID] = true
			ids = append(ids, it.ClientID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out, nil
}

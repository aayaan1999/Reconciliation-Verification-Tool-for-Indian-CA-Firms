package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// The Rule Builder's endpoints (SPEC-010). The server checks the shape of what it is sent and
// passes the rest to recon-engine, which does the validation and the writes: the engine is the
// only code that loads rules, so a rule the server accepts is one the engine loads (§B9).

var ruleIDPattern = regexp.MustCompile(`^RECON-[A-Z][A-Z0-9]*-[0-9]{3}$`)

// rulebookIDPattern matches the rule-book IDs: RC-ACC-003, GST-001, TDS-049.
var rulebookIDPattern = regexp.MustCompile(`^[A-Z]{2,5}(-[A-Z]{2,5})?-[0-9]{3}$`)

// ruleBody is what the window sends with a save: who makes it and the form.
type ruleBody struct {
	By   string     `json:"by"`
	Rule rules.Form `json:"rule"`
}

// ruleOptions lists what the window offers in its lists.
func (s *Server) ruleOptions(w http.ResponseWriter, _ *http.Request) {
	status, body := s.engineJSON("", "rule-options", "--db", s.cfg.DB, "--rules", s.cfg.Rules, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, body)
}

// ruleCurrent returns a rule as the window edits it: its form, its fixtures and its status.
func (s *Server) ruleCurrent(w http.ResponseWriter, r *http.Request) {
	id, ok := ruleIDOf(w, r)
	if !ok {
		return
	}
	status, body := s.engineJSON("", "rule-get", "--db", s.cfg.DB, "--rules", s.cfg.Rules, "--office-rules", s.cfg.OfficeRules,
		"--vocab", s.cfg.Vocab, "--rule", id)
	writeJSON(w, status, body)
}

// createRule saves a new rule: the engine checks it, its fixtures and its ID, and writes it as
// a testing office rule. A refusal is 422 with each problem and its field.
func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	in, ok := readRuleBody(w, r)
	if !ok {
		return
	}
	in.By, in.Rule.Mode = actorOf(r), "create"
	s.saveRule(w, in, http.StatusCreated)
}

// updateRule saves a changed rule. A built-in rule becomes an office version; an office rule
// gets a new version. The rule's ID is the one in the path.
func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	id, ok := engineRuleIDOf(w, r)
	if !ok {
		return
	}
	in, ok := readRuleBody(w, r)
	if !ok {
		return
	}
	if in.Rule.ID != "" && in.Rule.ID != id {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "the rule's ID in the form does not match the rule being changed")
		return
	}
	in.By, in.Rule.ID, in.Rule.Mode = actorOf(r), id, "edit"
	s.saveRule(w, in, http.StatusOK)
}

// saveRule passes a checked-for-shape save to the engine.
func (s *Server) saveRule(w http.ResponseWriter, in ruleBody, okStatus int) {
	body, err := json.Marshal(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "rule-save", "--db", s.cfg.DB, "--rules", s.cfg.Rules,
		"--office-rules", s.cfg.OfficeRules, "--vocab", s.cfg.Vocab)
	if status == http.StatusOK {
		status = okStatus
	}
	writeJSON(w, status, out)
}

// disableRule switches a rule off. The reason is required and kept in the audit log; the files
// and versions stay.
func (s *Server) disableRule(w http.ResponseWriter, r *http.Request) {
	s.switchRule(w, r, "rule-disable")
}

// bulkDisableRules switches several rules off with one reason, all or none (SPEC-010 scope 9).
func (s *Server) bulkDisableRules(w http.ResponseWriter, r *http.Request) {
	s.switchRules(w, r, "rule-disable")
}

// bulkEnableRules switches several rules on again with one reason, all or none.
func (s *Server) bulkEnableRules(w http.ResponseWriter, r *http.Request) {
	s.switchRules(w, r, "rule-enable")
}

// switchRules sends the chosen rule IDs to the engine in one call, which changes them in one
// transaction with one audit entry each. The name is the signed-in user's.
func (s *Server) switchRules(w http.ResponseWriter, r *http.Request, command string) {
	var in struct {
		RuleIDs []string `json:"rule_ids"`
		Reason  string   `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send the change as JSON: rule_ids, reason")
		return
	}
	switch {
	case len(in.RuleIDs) == 0:
		writeError(w, http.StatusBadRequest, "NO_RULES_SELECTED", "choose at least one rule")
		return
	case len(in.RuleIDs) > 500:
		writeError(w, http.StatusBadRequest, "TOO_MANY_RULES", "at most 500 rules can be changed at once")
		return
	}
	for _, id := range in.RuleIDs {
		if !ruleIDPattern.MatchString(id) && !rulebookIDPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, "BAD_RULE_ID", fmt.Sprintf("%q is not a rule ID", id))
			return
		}
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r), "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), command, "--db", s.cfg.DB, "--rules", strings.Join(in.RuleIDs, ","))
	writeJSON(w, status, out)
}

// enableRule switches a rule back on, as testing. The reason is required.
func (s *Server) enableRule(w http.ResponseWriter, r *http.Request) {
	s.switchRule(w, r, "rule-enable")
}

func (s *Server) switchRule(w http.ResponseWriter, r *http.Request, command string) {
	id, ok := ruleIDOf(w, r)
	if !ok {
		return
	}
	var in struct {
		By     string `json:"by"`
		Reason string `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send the change as JSON: by, reason")
		return
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r), "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), command, "--db", s.cfg.DB, "--rule", id)
	writeJSON(w, status, out)
}

// approveRule makes a testing rule active (SPEC-010 §4). The approver's name and a reason are
// required; the engine checks the gate (the fixtures pass, and a client's run included the rule
// with testing rules on) and writes the audit entry. A refusal names the reason.
func (s *Server) approveRule(w http.ResponseWriter, r *http.Request) {
	id, ok := ruleIDOf(w, r)
	if !ok {
		return
	}
	var in struct {
		By     string `json:"by"`
		Reason string `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send the approval as JSON: by, reason")
		return
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r), "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "rule-approve", "--db", s.cfg.DB, "--rules", s.cfg.Rules,
		"--office-rules", s.cfg.OfficeRules, "--vocab", s.cfg.Vocab, "--rule", id)
	writeJSON(w, status, out)
}

// ruleVersions lists a rule's stored versions, oldest first, and which one is in use.
func (s *Server) ruleVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := ruleIDOf(w, r)
	if !ok {
		return
	}
	status, out := s.engineJSON("", "rule-versions", "--db", s.cfg.DB, "--rule", id)
	writeJSON(w, status, out)
}

// switchRuleVersion makes an earlier (or other) version of the rule the one in use. It is
// recorded in the audit log with who made the change.
func (s *Server) switchRuleVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := ruleIDOf(w, r)
	if !ok {
		return
	}
	vid := r.PathValue("version")
	if !idPattern.MatchString(vid) {
		writeError(w, http.StatusBadRequest, "BAD_VERSION", "the version is a number from the list")
		return
	}
	by, ok := actorJSON(w, r)
	if !ok {
		return
	}
	status, out := s.engineJSON(by, "rule-versions", "--db", s.cfg.DB, "--rule", id, "--switch", vid)
	writeJSON(w, status, out)
}

// resetRule makes the standard (built-in) version the one in use again. The office edits stay
// in the office folder and in the database.
func (s *Server) resetRule(w http.ResponseWriter, r *http.Request) {
	id, ok := ruleIDOf(w, r)
	if !ok {
		return
	}
	by, ok := actorJSON(w, r)
	if !ok {
		return
	}
	status, out := s.engineJSON(by, "rule-versions", "--db", s.cfg.DB, "--rule", id, "--reset")
	writeJSON(w, status, out)
}

// ruleIDOf reads and checks the rule ID in the path: an engine rule (RECON-DEP-002) or a
// rule-book rule (RC-ACC-003, GST-001), as the Rules tab lists them.
func ruleIDOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !ruleIDPattern.MatchString(id) && !rulebookIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_RULE_ID", "the rule ID looks like RECON-DEP-002 or RC-ACC-003")
		return "", false
	}
	return id, true
}

// engineRuleIDOf is ruleIDOf for the Rule Builder's form, which edits engine rules only.
func engineRuleIDOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !ruleIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_RULE_ID", "the rule ID looks like RECON-DEP-002")
		return "", false
	}
	return id, true
}

// updateRulebookRule saves a Partner's edit of a rule-book rule (SPEC-010 scope 8). The route
// needs activate_rules: the rule book is CA-approved content. The engine checks the rule, keeps
// it as a new office version and writes the audit entry; the rule-book file is never written.
func (s *Server) updateRulebookRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !rulebookIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_RULE_ID", "the rule-book rule ID looks like RC-ACC-003")
		return
	}
	var in struct {
		By     string          `json:"by"`
		Reason string          `json:"reason"`
		Rule   json.RawMessage `json:"rule"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || len(in.Rule) == 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"reason", "rule": {...}} as JSON`)
		return
	}
	body, err := json.Marshal(map[string]any{"by": actorOf(r), "reason": in.Reason, "rule": in.Rule})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "rulebook-save", "--db", s.cfg.DB, "--rule", id)
	writeJSON(w, status, out)
}

// readRuleBody decodes a save: who makes it and the form, with no unknown fields.
func readRuleBody(w http.ResponseWriter, r *http.Request) (ruleBody, bool) {
	var in ruleBody
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send {\"by\", \"rule\": {...}} as JSON")
		return in, false
	}
	return in, true
}

// actorJSON is the engine's standard input for a change the signed-in user makes. The request
// may send nothing, or a "by" the screen no longer needs: the actor is always the session's
// user (SPEC-012 §6), never a name typed on the screen.
func actorJSON(w http.ResponseWriter, r *http.Request) (string, bool) {
	var in struct {
		By string `json:"by"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {} as JSON`)
		return "", false
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return "", false
	}
	return string(body), true
}

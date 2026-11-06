package api

import (
	"encoding/json"
	"net/http"
)

// The Admin tab's Currency page (SPEC-021): the fixed rate, in rupees per US dollar, that a run
// uses for an amount written in dollars (SPEC-019). Reading it needs only sign-in, so any user can
// see which rate is in force; changing it needs the Admin permission (the Partner). A change goes
// to the engine, which validates it, writes the rate file whole and writes one audit entry, with
// the signed-in user as the actor (SPEC-012 §6). The server only forwards.

// currencyGet returns the rate in force and the history of changes.
func (s *Server) currencyGet(w http.ResponseWriter, _ *http.Request) {
	status, body := s.engineJSON("", "currency-get", "--db", s.cfg.DB, "--currency", s.cfg.Currency)
	writeJSON(w, status, body)
}

// currencySet sets a new rate. The body is {"usd_rate": "83.2500", "effective_from": "2026-11-04",
// "note": ""}; the actor is added here, never taken from the request.
func (s *Server) currencySet(w http.ResponseWriter, r *http.Request) {
	var in struct {
		USDRate       string `json:"usd_rate"`
		EffectiveFrom string `json:"effective_from"`
		Note          string `json:"note"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"usd_rate": "83.2500", "effective_from": "2026-11-04", "note": ""} as JSON`)
		return
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r), "usd_rate": in.USDRate, "effective_from": in.EffectiveFrom, "note": in.Note})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "currency-set", "--db", s.cfg.DB, "--currency", s.cfg.Currency)
	writeJSON(w, status, out)
}

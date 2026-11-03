package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
)

// The Formats screen's endpoints (SPEC-016). Reading and changing both need manage_formats
// (the Partner): formats and parameters decide how every file is read. Each change goes to the
// engine, which makes it in one transaction with one audit entry, and the actor is always the
// signed-in user (SPEC-012 §6).

var (
	formatIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
	paramKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)
)

// formats lists the formats and parameters with where each is used, and the gaps.
func (s *Server) formats(w http.ResponseWriter, _ *http.Request) {
	status, body := s.engineJSON("", "formats", "--db", s.cfg.DB, "--rules", s.cfg.Rules, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, body)
}

// reasonBody reads {"reason"} and returns the engine's standard input with the signed-in user.
func reasonBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	var in struct {
		Reason string `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"reason": "..."} as JSON`)
		return "", false
	}
	b, err := json.Marshal(map[string]string{"by": actorOf(r), "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return "", false
	}
	return string(b), true
}

func formatIDOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !formatIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_FORMAT_ID", "that is not a format ID")
		return "", false
	}
	return id, true
}

func (s *Server) switchFormat(w http.ResponseWriter, r *http.Request, command string, extra ...string) {
	id, ok := formatIDOf(w, r)
	if !ok {
		return
	}
	body, ok := reasonBody(w, r)
	if !ok {
		return
	}
	args := append([]string{command, "--db", s.cfg.DB, "--format", id}, extra...)
	status, out := s.engineJSON(body, args...)
	writeJSON(w, status, out)
}

func (s *Server) disableFormat(w http.ResponseWriter, r *http.Request) {
	s.switchFormat(w, r, "format-disable")
}
func (s *Server) enableFormat(w http.ResponseWriter, r *http.Request) {
	s.switchFormat(w, r, "format-enable")
}

// switchFormatVersion points a format at an earlier version; the version is the revision number.
func (s *Server) switchFormatVersion(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "BAD_VERSION", "the version is a number from 1")
		return
	}
	s.switchFormat(w, r, "format-switch", "--version", strconv.Itoa(n))
}

// addParameter adds an office parameter.
func (s *Server) addParameter(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key         string   `json:"key"`
		Name        string   `json:"name"`
		Kind        string   `json:"kind"`
		Document    string   `json:"document"`
		Description string   `json:"description"`
		Aliases     []string `json:"aliases"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send key, name, kind, document, description and aliases as JSON")
		return
	}
	body, err := json.Marshal(map[string]any{
		"by": actorOf(r), "key": in.Key, "name": in.Name, "kind": in.Kind, "document": in.Document,
		"description": in.Description, "aliases": in.Aliases,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "parameter-add", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab)
	if status == http.StatusOK {
		status = http.StatusCreated
	}
	writeJSON(w, status, out)
}

func (s *Server) switchParameter(w http.ResponseWriter, r *http.Request, command string) {
	key := r.PathValue("key")
	if !paramKeyPattern.MatchString(key) {
		writeError(w, http.StatusBadRequest, "BAD_KEY", "that is not a parameter key")
		return
	}
	body, ok := reasonBody(w, r)
	if !ok {
		return
	}
	status, out := s.engineJSON(body, command, "--db", s.cfg.DB, "--parameter", key, "--rules", s.cfg.Rules, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, out)
}

func (s *Server) disableParameter(w http.ResponseWriter, r *http.Request) {
	s.switchParameter(w, r, "parameter-disable")
}
func (s *Server) enableParameter(w http.ResponseWriter, r *http.Request) {
	s.switchParameter(w, r, "parameter-enable")
}

// changeAlias adds an alias to an office parameter, or removes one ({"alias", "remove"}).
func (s *Server) changeAlias(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !paramKeyPattern.MatchString(key) {
		writeError(w, http.StatusBadRequest, "BAD_KEY", "that is not a parameter key")
		return
	}
	var in struct {
		Alias  string `json:"alias"`
		Remove bool   `json:"remove"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"alias": "...", "remove": false} as JSON`)
		return
	}
	body, err := json.Marshal(map[string]any{"by": actorOf(r), "alias": in.Alias, "remove": in.Remove})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "parameter-alias", "--db", s.cfg.DB, "--parameter", key)
	writeJSON(w, status, out)
}

// formatMapping reads the mapping of an office format, for the mapping screen to edit.
func (s *Server) formatMapping(w http.ResponseWriter, r *http.Request) {
	id, ok := formatIDOf(w, r)
	if !ok {
		return
	}
	status, out := s.engineJSON("", "format-get", "--db", s.cfg.DB, "--format", id)
	writeJSON(w, status, out)
}

// publishFormat publishes a draft version as testing or active: the engine writes its profile
// files, checks that they load, and makes it the version in use.
func (s *Server) publishFormat(w http.ResponseWriter, r *http.Request) {
	id, ok := formatIDOf(w, r)
	if !ok {
		return
	}
	var in struct {
		Revision int    `json:"revision"`
		Status   string `json:"status"`
		Confirm  bool   `json:"confirm"` // publish although a kept sample reads differently
		Reason   string `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || in.Revision < 1 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"revision": 2, "status": "testing"} as JSON`)
		return
	}
	body, err := json.Marshal(map[string]any{"by": actorOf(r), "confirm": in.Confirm, "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "format-publish", "--db", s.cfg.DB, "--office-profiles", s.cfg.Office,
		"--profiles", s.cfg.Profiles, "--vocab", s.cfg.Vocab, "--rules", s.cfg.Rules, "--format", id,
		"--version", strconv.Itoa(in.Revision), "--status", in.Status)
	writeJSON(w, status, out)
}

// resolveUnrecognised closes a file no format recognised: a format was taught for it, or the
// Partner dismisses it with a reason.
func (s *Server) resolveUnrecognised(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "BAD_ID", "that is not a file number")
		return
	}
	var in struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"status": "dismissed", "note": "..."} as JSON`)
		return
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r), "note": in.Note})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "unrecognised-resolve", "--db", s.cfg.DB, "--id", strconv.Itoa(id), "--status", in.Status)
	writeJSON(w, status, out)
}

// removeSample deletes a kept sample file; the audit entry stays.
func (s *Server) removeSample(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "BAD_ID", "that is not a sample number")
		return
	}
	body, err := json.Marshal(map[string]string{"by": actorOf(r)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status, out := s.engineJSON(string(body), "format-sample-discard", "--db", s.cfg.DB, "--id", strconv.Itoa(id))
	writeJSON(w, status, out)
}

// exportFormat sends an office format and its parameters as a bundle file, for another machine.
func (s *Server) exportFormat(w http.ResponseWriter, r *http.Request) {
	id, ok := formatIDOf(w, r)
	if !ok {
		return
	}
	status, out := s.engineJSON("", "format-export", "--db", s.cfg.DB, "--format", id, "--vocab", s.cfg.Vocab)
	if status == http.StatusOK {
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.format.json"`)
	}
	writeJSON(w, status, out)
}

// importFormat checks a bundle and, unless check_only, adds its format (as testing) and
// parameters. The bundle is read as JSON and checked by the engine with the strict loaders.
func (s *Server) importFormat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Bundle    json.RawMessage `json:"bundle"`
		CheckOnly bool            `json:"check_only"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 5<<20))
	if err := dec.Decode(&in); err != nil || len(in.Bundle) == 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"bundle": {...}, "check_only": true} as JSON`)
		return
	}
	args := []string{"format-import", "--db", s.cfg.DB, "--office-profiles", s.cfg.Office, "--profiles", s.cfg.Profiles,
		"--vocab", s.cfg.Vocab, "--by", actorOf(r)}
	if in.CheckOnly {
		args = append(args, "--check-only")
	}
	status, out := s.engineJSON(string(in.Bundle), args...)
	writeJSON(w, status, out)
}

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// The Document Master's endpoints (SPEC-022 part A). Reading the list (it fills the layout wizard)
// needs only sign-in; every change needs manage_documents, which only the Partner holds. Each
// change goes to the engine, which makes it in one transaction with one audit entry, and the
// actor is always the signed-in user (SPEC-012 §6). The engine decides what a valid document is.

// docCodePattern only keeps a path value safe to pass on; the engine applies the real rules.
var docCodePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// fileTypes are the formats a document can accept.
var fileTypes = []string{"json", "xlsx", "csv"}

func docCodeOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := r.PathValue("code")
	if !docCodePattern.MatchString(code) {
		writeError(w, http.StatusBadRequest, "BAD_DOCUMENT_CODE", "that is not a document code")
		return "", false
	}
	return code, true
}

// boolQuery reads a true/false query value: 1 or true, 0 or false, or absent.
func boolQuery(w http.ResponseWriter, r *http.Request, name string) (bool, bool) {
	switch v := r.URL.Query().Get(name); v {
	case "", "0", "false":
		return false, true
	case "1", "true":
		return true, true
	default:
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", name+" is 1 or 0")
		return false, false
	}
}

// documentTypes lists every document (built-in and office). ?active=1 gives the documents a new
// layout or rule may use, and ?file_type=xlsx those that accept that file type: together they are
// the layout wizard's list.
func (s *Server) documentTypes(w http.ResponseWriter, r *http.Request) {
	args := []string{"document-list", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab}
	active, ok := boolQuery(w, r, "active")
	if !ok {
		return
	}
	if active {
		args = append(args, "--active")
	}
	if ft := r.URL.Query().Get("file_type"); ft != "" {
		if !slices.Contains(fileTypes, ft) {
			writeError(w, http.StatusBadRequest, "BAD_FORMATS", "file_type is json, xlsx or csv")
			return
		}
		args = append(args, "--file-type", ft)
	}
	status, body := s.engineJSON("", args...)
	writeJSON(w, status, body)
}

// documentType returns one document with its columns, what uses it and its audit history.
func (s *Server) documentType(w http.ResponseWriter, r *http.Request) {
	code, ok := docCodeOf(w, r)
	if !ok {
		return
	}
	status, body := s.engineJSON("", "document-get", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab, "--code", code)
	writeJSON(w, status, body)
}

// documentBody reads a document definition and returns the engine's standard input: the same
// fields, with "by" always the signed-in user and, when code is given, the code of the path. A key
// the engine does not know is refused there (it reads the body strictly).
func documentBody(w http.ResponseWriter, r *http.Request, code string) (string, bool) {
	var in map[string]json.RawMessage
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<17))
	if err := dec.Decode(&in); err != nil || in == nil || dec.More() {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "send the document as a JSON object: code, name, category, description, applicable_for, status and record_type")
		return "", false
	}
	if code != "" {
		if raw, ok := in["code"]; ok {
			var given string
			if json.Unmarshal(raw, &given) != nil || !strings.EqualFold(strings.TrimSpace(given), code) {
				writeError(w, http.StatusBadRequest, "CODE_MISMATCH", "the code in the body is not the code in the address; a document's code is never changed")
				return "", false
			}
		}
		in["code"], _ = json.Marshal(code)
	}
	in["by"], _ = json.Marshal(actorOf(r))
	b, err := json.Marshal(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return "", false
	}
	return string(b), true
}

// addDocument adds a document: a Draft, or ready when its record structure is valid. A document
// whose own fields are refused is 422 INVALID_DOCUMENT with every problem; a code in use is 409.
func (s *Server) addDocument(w http.ResponseWriter, r *http.Request) {
	body, ok := documentBody(w, r, "")
	if !ok {
		return
	}
	status, out := s.engineJSON(body, "document-save", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab)
	if status == http.StatusOK {
		status = http.StatusCreated
	}
	writeJSON(w, status, out)
}

// editDocument edits an office document within the locking rules of its record structure.
func (s *Server) editDocument(w http.ResponseWriter, r *http.Request) {
	code, ok := docCodeOf(w, r)
	if !ok {
		return
	}
	body, ok := documentBody(w, r, code)
	if !ok {
		return
	}
	status, out := s.engineJSON(body, "document-save", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab, "--edit")
	writeJSON(w, status, out)
}

// validateDocument checks a definition and says what is wrong, without saving it. With ?edit=1 it
// checks an edit of an existing document. A copy's columns are filled in the answer.
func (s *Server) validateDocument(w http.ResponseWriter, r *http.Request) {
	edit, ok := boolQuery(w, r, "edit")
	if !ok {
		return
	}
	body, ok := documentBody(w, r, "")
	if !ok {
		return
	}
	args := []string{"document-validate", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab}
	if edit {
		args = append(args, "--edit")
	}
	status, out := s.engineJSON(body, args...)
	writeJSON(w, status, out)
}

// setDocumentStatus switches a document on or off for new layouts and rules: {"active": true}.
// Switching off is always allowed; the answer lists what still reads the document.
func (s *Server) setDocumentStatus(w http.ResponseWriter, r *http.Request) {
	code, ok := docCodeOf(w, r)
	if !ok {
		return
	}
	var in struct {
		Active *bool `json:"active"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || in.Active == nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", `send {"active": true} or {"active": false}`)
		return
	}
	body, _ := json.Marshal(map[string]any{"by": actorOf(r), "code": code, "active": *in.Active})
	status, out := s.engineJSON(string(body), "document-status", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, out)
}

// deleteDocument deletes an unused office document. A built-in or used one is 409 with the
// formats and rules that use it, so the screen can offer Deactivate.
func (s *Server) deleteDocument(w http.ResponseWriter, r *http.Request) {
	code, ok := docCodeOf(w, r)
	if !ok {
		return
	}
	body, _ := json.Marshal(map[string]string{"by": actorOf(r), "code": code})
	status, out := s.engineJSON(string(body), "document-delete", "--db", s.cfg.DB, "--vocab", s.cfg.Vocab)
	writeJSON(w, status, out)
}

// keepUsage copies the "usage" of a refused answer (what reads a document) into the error body.
func keepUsage(body map[string]any, engineOut []byte) {
	var refused struct {
		Usage json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(bytes.TrimSpace(engineOut), &refused) == nil && len(refused.Usage) > 0 {
		body["usage"] = refused.Usage
	}
}

package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"unicode/utf8"
)

// Canonical JSON is the byte-stable form used for golden files, determinism tests and
// fingerprints: object keys sorted, no insignificant whitespace, integers only, no HTML
// escaping, and an empty list written as [] whether it was nil or empty. A value writes
// only the field its kind names. Invalid kinds, statuses, states and text that is not
// valid UTF-8 are refused rather than written in a lossy form.

// CanonicalJSON returns the canonical encoding of e.
func (e Evaluation) CanonicalJSON() ([]byte, error) {
	obj, err := evaluationObject(e)
	if err != nil {
		return nil, err
	}
	return marshalCanonical(obj)
}

// CanonicalJSON returns the canonical encoding of f, with its latest evaluation nested.
func (f Finding) CanonicalJSON() ([]byte, error) {
	if !f.State.Valid() {
		return nil, fmt.Errorf("domain: finding %q has invalid state %q", f.StableID, f.State)
	}
	ev, err := evaluationObject(f.LatestEvaluation)
	if err != nil {
		return nil, err
	}
	obj := object{
		"stable_id":         f.StableID,
		"value_fingerprint": f.ValueFingerprint,
		"state":             string(f.State),
		"review_document":   f.ReviewDocument,
		"latest_evaluation": ev,
	}
	if err := checkText(obj); err != nil {
		return nil, err
	}
	return marshalCanonical(obj)
}

type object = map[string]any

func evaluationObject(e Evaluation) (object, error) {
	if !e.Status.Valid() {
		return nil, fmt.Errorf("domain: evaluation of rule %q has invalid status %d", e.RuleID, e.Status)
	}
	values := make([]any, 0, len(e.Values))
	for i, v := range e.Values {
		val, err := valueObject(v.Value)
		if err != nil {
			return nil, fmt.Errorf("domain: rule %q value %d (%s): %w", e.RuleID, i, v.Input, err)
		}
		values = append(values, object{
			"input":   v.Input,
			"label":   v.Label,
			"value":   val,
			"locator": locatorObject(v.Locator),
		})
	}
	// MissingFacts is a set of names, so its order carries no meaning and is sorted here.
	// Values and law references keep the rule's declared order.
	missing := make([]any, 0, len(e.MissingFacts))
	for _, name := range slices.Sorted(slices.Values(e.MissingFacts)) {
		missing = append(missing, name)
	}
	laws := make([]any, 0, len(e.LawRefs))
	for _, l := range e.LawRefs {
		laws = append(laws, object{"act": l.Act, "section": l.Section, "reference": l.Reference})
	}
	obj := object{
		"run_id":       e.RunID,
		"rule_id":      e.RuleID,
		"rule_version": e.RuleVersion,
		"subject": object{
			"period":    e.Subject.Period,
			"scope":     e.Subject.Scope,
			"match_key": e.Subject.MatchKey,
		},
		"status":        e.Status.String(),
		"values":        values,
		"difference":    int64(e.Difference),
		"missing_facts": missing,
		"law_refs":      laws,
	}
	if err := checkText(obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func valueObject(v Value) (object, error) {
	switch v.Kind {
	case KindMoney:
		return object{"kind": v.Kind.String(), "paise": int64(v.Money)}, nil
	case KindInt:
		return object{"kind": v.Kind.String(), "int": v.Int}, nil
	case KindText:
		return object{"kind": v.Kind.String(), "text": v.Text}, nil
	case KindDate:
		return object{"kind": v.Kind.String(), "date": v.Date}, nil
	case KindBool:
		return object{"kind": v.Kind.String(), "bool": v.Bool}, nil
	case KindRate:
		return object{"kind": v.Kind.String(), "rate_bp": v.RateBP}, nil
	}
	return nil, fmt.Errorf("invalid value kind %d", v.Kind)
}

func locatorObject(l Locator) object {
	return object{
		"document_version_id": l.DocumentVersionID,
		"file_name":           l.FileName,
		"kind":                l.Kind,
		"path":                l.Path,
	}
}

// checkText refuses any string in the tree that is not valid UTF-8, because encoding/json
// would silently replace the bad bytes and make two different texts encode identically.
func checkText(v any) error {
	switch t := v.(type) {
	case string:
		if !utf8.ValidString(t) {
			return fmt.Errorf("domain: text %q is not valid UTF-8", t)
		}
	case object:
		for _, k := range slices.Sorted(maps.Keys(t)) {
			if err := checkText(t[k]); err != nil {
				return err
			}
		}
	case []any:
		for _, e := range t {
			if err := checkText(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// marshalCanonical writes v with encoding/json, which sorts map keys and adds no
// whitespace; HTML escaping is turned off so "<" and "&" are written as themselves.
func marshalCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("domain: canonical JSON: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

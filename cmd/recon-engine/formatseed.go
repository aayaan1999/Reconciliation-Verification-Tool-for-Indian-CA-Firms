package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The standard formats and parameters (SPEC-016). A format is a profile (content/profiles) and a
// parameter is a fact or record type of the vocabulary (content/vocabulary), so the screens and
// the engine share one model. load-rules stores them, one version per changed file, the way it
// stores rules; loading unchanged files again changes nothing.

const seedAuthor = "recon-engine"

// seedStamp is the publication time written on a standard version: it is part of the tool, not
// published on a day, so a fixed value keeps the database the same on every machine.
const seedStamp = "1970-01-01T00:00:00Z"

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// seedStandard stores every profile as a standard format version and every vocabulary fact and
// record type as a standard parameter version. It returns how many versions it added.
func seedStandard(tx *store.Tx, set *profile.Set, v *vocab.Vocab) (formats, parameters int, err error) {
	for _, p := range set.Profiles() {
		body, err := os.ReadFile(p.File)
		if err != nil {
			return 0, 0, inputErr(err)
		}
		sum := sumHex(body)
		have, err := tx.FormatVersions(p.ID)
		if err != nil {
			return 0, 0, err
		}
		if slices.ContainsFunc(have, func(f store.FormatVersion) bool { return f.SHA256 == sum }) {
			continue
		}
		if _, err := tx.AddFormatVersion(store.FormatVersion{
			FormatID: p.ID, Name: p.ID, Source: store.OriginBuiltin, FileType: p.FileType, Document: p.DocumentType,
			BodyYAML: string(body), SHA256: sum, Note: "Profile version " + p.Version + " (" + p.File + ")",
			Status: store.FormatActive, CreatedBy: seedAuthor, PublishedAt: seedStamp, PublishedBy: seedAuthor, ProfileVersion: p.Version,
		}); err != nil {
			return 0, 0, err
		}
		formats++
	}

	current, err := tx.Parameters()
	if err != nil {
		return 0, 0, err
	}
	have := map[string]string{} // key -> sha256 of the version in use
	for _, p := range current {
		have[p.State.ParameterID] = p.Version.SHA256
	}
	add := func(pv store.ParameterVersion) error {
		pv.SHA256 = sumHex([]byte(pv.BodyYAML))
		if have[pv.ParameterID] == pv.SHA256 {
			return nil
		}
		pv.Source, pv.CreatedBy = store.OriginBuiltin, seedAuthor
		if _, err := tx.AddParameterVersion(pv); err != nil {
			return err
		}
		parameters++
		return nil
	}
	for _, f := range v.Facts() { // sorted by name
		body := paramBody(paramDoc{
			Name: f.Name, Kind: f.Kind.String(), Periodicity: string(f.Periodicity), DocumentType: f.DocumentType,
			Description: f.Description, SinceVocab: f.SinceVocab, DeprecatedIn: f.DeprecatedIn,
		})
		if err := add(store.ParameterVersion{ParameterID: f.Name, Name: f.Name, ParamType: f.Kind.String(), Document: f.DocumentType, BodyYAML: body}); err != nil {
			return 0, 0, err
		}
	}
	for _, r := range v.RecordTypes() { // sorted by name
		body := paramBody(paramDoc{
			Name: r.Name, Kind: "record", DocumentType: r.DocumentType, Description: r.Description,
			SinceVocab: r.SinceVocab, DeprecatedIn: r.DeprecatedIn,
		})
		if err := add(store.ParameterVersion{ParameterID: r.Name, Name: r.Name, ParamType: "record", Document: r.DocumentType, BodyYAML: body}); err != nil {
			return 0, 0, err
		}
	}
	return formats, parameters, nil
}

// paramDoc is what a parameter's body holds. The body is written as a JSON object, which is
// valid YAML, with its keys in this fixed order, so the same parameter always has the same
// hash. Aliases are only written for office parameters.
type paramDoc struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Periodicity  string   `json:"periodicity,omitempty"`
	DocumentType string   `json:"document_type"`
	Description  string   `json:"description"`
	Aliases      []string `json:"aliases,omitempty"`
	SinceVocab   int      `json:"since_vocab"`
	DeprecatedIn int      `json:"deprecated_in"`
}

func paramBody(d paramDoc) string {
	b, _ := json.Marshal(d) // a struct of strings and numbers cannot fail
	return string(b) + "\n"
}

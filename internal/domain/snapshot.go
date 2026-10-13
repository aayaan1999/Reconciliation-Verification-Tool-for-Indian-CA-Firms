package domain

// A RunSnapshot records every versioned thing a run used, so a result can be explained
// years later (RU-53; technical spec §A10): "this finding came from rule RECON-DEP-002
// v1, law row TDS-THR-0193, profile generic.taxsuite.computation v1, engine 1.4.2".
type RunSnapshot struct {
	RunID             int64
	EngagementID      int64
	EngineVersion     string // semver of recon-engine, e.g. "1.4.2"
	VocabularyVersion int64  // fact vocabulary version
	Documents         []DocumentUse
	Rules             []RuleUse
	LawRows           []LawRowUse
}

// A DocumentUse is one document version in the run's current view, with the format
// profile that read it.
type DocumentUse struct {
	DocumentVersionID int64
	DocumentType      string // e.g. "computation", "tar"
	SHA256            string // of the uploaded file, lowercase hex
	ProfileID         string // e.g. "generic.taxsuite.computation"
	ProfileVersion    string
}

// A RuleUse is one rule version evaluated in the run.
type RuleUse struct {
	RuleID  string
	Version string // e.g. "1", or "1-office.2" for an office edit (RU-12)
}

// A LawRowUse is one law-table row looked up during the run.
type LawRowUse struct {
	Table       string // e.g. "tds.section_threshold"
	RowID       string // e.g. "TDS-THR-0001"
	ContentHash string // of the row's content, lowercase hex
}

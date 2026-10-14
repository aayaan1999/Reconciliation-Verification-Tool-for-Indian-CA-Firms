package profile

// These types mirror the profile file (schema recon.profile/1, technical spec §B2.2). They
// are decoded by internal/strictyaml, so every text value must be quoted, unknown keys are
// refused, and an absent text value is "".

// Schema is the value of the "schema" key of every profile file.
const Schema = "recon.profile/1"

type fileYAML struct {
	Schema         string        `yaml:"schema"`
	ProfileID      string        `yaml:"profile_id"`
	ProfileVersion string        `yaml:"profile_version"`
	SourceSystem   string        `yaml:"source_system"`
	DocumentType   string        `yaml:"document_type"`
	FileType       string        `yaml:"file_type"`
	Regime         string        `yaml:"regime"`
	Detect         []detectYAML  `yaml:"detect"`
	Context        contextYAML   `yaml:"context"`
	Facts          []factYAML    `yaml:"facts"`
	Records        []recordsYAML `yaml:"records"`
}

// detectYAML is one detect predicate: exactly one of its fields is set.
type detectYAML struct {
	JSONExists  *pathArg    `yaml:"json_exists"`
	JSONEquals  *equalsArg  `yaml:"json_equals"`
	JSONMatches *matchesArg `yaml:"json_matches"`
}

type pathArg struct {
	Path string `yaml:"path"`
}

type equalsArg struct {
	Path  string `yaml:"path"`
	Value string `yaml:"value"`
}

type matchesArg struct {
	Path    string `yaml:"path"`
	Pattern string `yaml:"pattern"`
}

// contextYAML says where the slot keys of a document come from (IN-11).
type contextYAML struct {
	TaxYear *sourceYAML `yaml:"tax_year"`
	PAN     *sourceYAML `yaml:"pan"`
	Period  *sourceYAML `yaml:"period"`
	Scope   *sourceYAML `yaml:"scope"`
	Part    *sourceYAML `yaml:"part"`
}

// sourceYAML is a value read from the file (json_path) or fixed by the profile (const).
type sourceYAML struct {
	JSONPath string `yaml:"json_path"`
	Const    string `yaml:"const"`
}

type factYAML struct {
	Fact     string `yaml:"fact"`
	JSONPath string `yaml:"json_path"`
	Unit     string `yaml:"unit"`     // money facts only: "rupees" | "paise"
	Required bool   `yaml:"required"` // missing → validation-report error, never a zero
	Sign     string `yaml:"sign"`     // money facts only: "as_is" (default) | "negate"
	Rounding string `yaml:"rounding"` // money facts only: a money rounding name; default exact
}

type recordsYAML struct {
	RecordType string       `yaml:"record_type"`
	Rows       string       `yaml:"rows"` // a JSON path with [*], one cell per row
	Unit       string       `yaml:"unit"` // for the amounts: "rupees" | "paise"
	Fields     []columnYAML `yaml:"fields"`
	Amounts    []columnYAML `yaml:"amounts"`
}

// columnYAML maps one column of a record type to a path inside the row, written from the
// row: ".supplier_gstin" or "['Invoice No']".
type columnYAML struct {
	Name     string `yaml:"name"`
	Path     string `yaml:"path"`
	Required bool   `yaml:"required"`
}

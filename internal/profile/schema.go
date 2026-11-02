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
	Bundle         string        `yaml:"bundle"`  // shared by the profiles of one file that holds several documents
	Section        string        `yaml:"section"` // with bundle: the part of the file this profile reads
	Sheets         []sheetYAML   `yaml:"sheets"`  // xlsx only: the sheets read, each as a table (SPEC-008)
	Detect         []detectYAML  `yaml:"detect"`
	Context        contextYAML   `yaml:"context"`
	Facts          []factYAML    `yaml:"facts"`
	Records        []recordsYAML `yaml:"records"`
}

// sheetYAML is one sheet of an Excel profile, read as a table: its header row is the first
// row holding every label of header (capitals aside), and each row below it with a value
// under the headings, to the end of the sheet, is addressed as "$['<name>'][*]" with its
// columns keyed by header text (".['Voucher No']").
type sheetYAML struct {
	Name   string   `yaml:"name"`
	Header []string `yaml:"header"`
	// AnyName reads the table from whichever one sheet has its header row, so an export
	// whose sheet is called "Sheet1" is read too; name is then how the paths address it.
	AnyName bool `yaml:"any_name"`
	// TotalRows are the texts of total rows ("Grand Total"): such a row is left out.
	TotalRows []string `yaml:"total_rows"`
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
// Format (period only) says how a period is written: "mmyyyy" ("082026") or "yyyymm".
// FromPeriod (tax_year only) works the tax year out from the period. FromGSTIN (pan only)
// reads the PAN from a GSTIN: its characters 3 to 12.
type sourceYAML struct {
	JSONPath   string `yaml:"json_path"`
	Const      string `yaml:"const"`
	Format     string `yaml:"format"`
	FromPeriod bool   `yaml:"from_period"`
	FromGSTIN  bool   `yaml:"from_gstin"`
	FromMonth  bool   `yaml:"from_month"` // tax_year only: the tax year of the month at json_path
}

type factYAML struct {
	Fact     string `yaml:"fact"`
	JSONPath string `yaml:"json_path"` // with rows: the path inside the entry, ".total_payment"
	// rows, where, combine and by read the figure from the entries of a list that meet the
	// conditions, never from a position in the list (SPEC-006 part 2): combine "one" (exactly
	// one entry), "sum" (money, the entries added up) or "latest" (the entry whose by column,
	// a date, is the latest).
	Rows     string      `yaml:"rows"`
	Where    []whereYAML `yaml:"where"`
	Combine  string      `yaml:"combine"`
	By       string      `yaml:"by"`
	Unit     string      `yaml:"unit"`     // money facts only: "rupees" | "paise"
	Required bool        `yaml:"required"` // missing → validation-report error, never a zero
	Sign     string      `yaml:"sign"`     // money facts only: "as_is" (default) | "negate"
	Rounding string      `yaml:"rounding"` // money facts only: a money rounding name; default exact
}

type recordsYAML struct {
	RecordType string       `yaml:"record_type"`
	Rows       string       `yaml:"rows"`  // a JSON path with [*], one cell per row
	Where      []whereYAML  `yaml:"where"` // only rows meeting every condition are read
	Unit       string       `yaml:"unit"`  // for the amounts: "rupees" | "paise"
	Fields     []columnYAML `yaml:"fields"`
	Amounts    []columnYAML `yaml:"amounts"`
}

// whereYAML keeps a row only when the value at path (written from the row) is equals.
type whereYAML struct {
	Path       string   `yaml:"path"`
	Equals     string   `yaml:"equals"`
	EqualsPath string   `yaml:"equals_path"` // instead of equals: the value at this path of the file
	In         []string `yaml:"in"`          // instead of equals: any one of these values (SPEC-007)
}

// columnYAML maps one column of a record type to a path inside the row, written from the
// row: ".supplier_gstin" or "['Invoice No']". A leading "^" starts from the object that
// holds the row's list (".ctin" of the supplier around an invoice); "^^" goes up two. A
// path with [*] adds up an amount over every match (item lines), or joins a field's
// distinct values with ",". DateFormat ("dd-mm-yyyy") turns a date into "yyyy-mm-dd".
type columnYAML struct {
	Name       string `yaml:"name"`
	Path       string `yaml:"path"`
	Required   bool   `yaml:"required"`
	DateFormat string `yaml:"date_format"`
}

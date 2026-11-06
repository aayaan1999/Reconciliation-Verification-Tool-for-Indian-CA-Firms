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
	// Supersedes is the version of this profile that this one replaces (SPEC-015): when a file
	// matches both, this one reads it. The older version stays as published, for the drafts it read.
	Supersedes string `yaml:"supersedes"`
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
	// BlankAsText are header labels whose empty cell, in a row that is read, is read as empty
	// text (a blank UDIN), not left out (SPEC-011 part 2).
	BlankAsText []string `yaml:"blank_as_text"`
	// Headings names a column the table gains: the heading row an indented row sits under
	// (SPEC-011 part 2), so rows with one label under two headings can be told apart.
	Headings string `yaml:"headings"`
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
// Format (period only) says how a period is written: "mmyyyy" ("082026"), "yyyymm", "mon-yyyy" ("Aug-2026"), or
// "yyyy-mm-dd" (a date, such as the first voucher date of a register with no period column: its month is the period; SPEC-018).
// FromPeriod (tax_year only) works the tax year out from the period. FromGSTIN (pan only)
// reads the PAN from a GSTIN: its characters 3 to 12.
type sourceYAML struct {
	JSONPath   string `yaml:"json_path"`
	Const      string `yaml:"const"`
	Format     string `yaml:"format"`
	FromPeriod bool   `yaml:"from_period"`
	FromGSTIN  bool   `yaml:"from_gstin"`
	FromMonth  bool   `yaml:"from_month"` // tax_year only: the tax year of the month at json_path
	// FromRun (tax_year only, SPEC-020): the file holds no tax year; the run's tax year is used.
	FromRun bool `yaml:"from_run"`
	// Rows and Where read the value from the one entry of a list that meets the conditions,
	// with json_path written from the entry (".Value"), never from a position in the list
	// (SPEC-011 part 2: the "Previous Year" row of a Particulars and Value sheet).
	Rows  string      `yaml:"rows"`
	Where []whereYAML `yaml:"where"`
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
	// YesNo (bool facts only) reads the text "Yes" or "No", in any capitals, as yes or no; any
	// other text is refused (SPEC-011 part 2: a workbook that writes Yes and No as text).
	YesNo bool `yaml:"yes_no"`
}

type recordsYAML struct {
	RecordType string `yaml:"record_type"`
	Rows       string `yaml:"rows"` // a JSON path with [*], one cell per row
	// OneRow (SPEC-018): rows names one object, read as one row (a GSTR-3B table is an object, not a list).
	OneRow bool        `yaml:"one_row"`
	Where  []whereYAML `yaml:"where"` // only rows meeting every condition are read
	Unit   string      `yaml:"unit"`  // for the amounts: "rupees" | "paise"
	// Sign "negate" negates the block's amounts, such as credit notes among supplies (SPEC-015).
	Sign    string       `yaml:"sign"`
	Fields  []columnYAML `yaml:"fields"`
	Amounts []columnYAML `yaml:"amounts"`
}

// whereYAML keeps a row only when the value at path (written from the row) is equals.
type whereYAML struct {
	Path       string   `yaml:"path"`
	Equals     string   `yaml:"equals"`
	EqualsPath string   `yaml:"equals_path"` // instead of equals: the value at this path of the file
	In         []string `yaml:"in"`          // instead of equals: any one of these values (SPEC-007)
	// EqualsYear, instead of equals: the file's own year, read with its context: "tax_year"
	// ("2025-26") or "assessment_year" (the year after it, "2026-27") (SPEC-011 part 2).
	EqualsYear string `yaml:"equals_year"`
	// NotEquals, instead of equals: any value but this one, such as a layout's "Total" row
	// kept apart from the rows it totals (SPEC-011 part 2). A missing value does not qualify.
	NotEquals string `yaml:"not_equals"`
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
	// Const, instead of path (fields only): the value of this field for every row of the block,
	// such as the tab a list stands for, which the file shows only by the list's name (SPEC-015).
	Const string `yaml:"const"`
}

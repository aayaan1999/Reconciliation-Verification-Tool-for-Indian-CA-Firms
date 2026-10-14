// Package jsonadapter reads JSON files (mainly tax-suite output) with JSON-path selectors.
// Numbers are read as exact text, never through float64.
//
// # Supported JSON paths
//
// Profiles address JSON with a small, fixed subset of JSON path. Anything else is refused
// by ParsePath when the profile loads, so it is never a run-time surprise.
//
//	$                    the whole file (every path starts with it)
//	.key                 an object member whose name is a letter or _ followed by
//	                     letters, digits or _ ("$.business_income.allowed_depreciation")
//	['key with spaces']  any other member name, in single quotes; inside, write ' as \'
//	                     and \ as \\ ("$['Schedule BP']")
//	[n]                  item n of a list, counting from 0, no leading zeros ("$.rows[3]")
//	[*]                  every item of a list, in file order, for record rows ("$.rows[*]")
//
// Not supported: recursive descent (..), .*, filters ([?(...)]), slices ([a:b]), unions
// ([a,b]), negative indexes and double-quoted keys.
//
// A path that finds nothing returns no cells; it is not an error. Every returned cell has
// a json_path locator holding its full concrete path, with [*] replaced by the actual
// index ("$.rows[3].amount"); concrete paths parse back to the same cell.
//
// # What the adapter refuses
//
// A file over the size limit is FILE_TOO_LARGE. These are PARSE_ERROR, with line, column
// and byte offset: malformed JSON, text that is not UTF-8 (encoding/json would quietly
// replace it), the same key twice in one object (encoding/json would quietly keep the
// last), text after the JSON value, and nesting deeper than 512 levels. A UTF-8
// byte-order mark at the start is allowed.
package jsonadapter

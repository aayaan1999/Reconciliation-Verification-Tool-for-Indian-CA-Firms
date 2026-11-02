// Package csvadapter reads comma-separated files (UTF-8, first row the column names) into
// the same tree the JSON reader gives: a list of rows, each an object keyed by column name
// with the cells as text. CSV formats are then written like JSON ones ("$[*]",
// ".amount_inr"), and every value keeps its place ("$[3].amount_inr" is the fourth data row).
package csvadapter

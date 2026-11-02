// Package xlsx reads Excel workbooks (.xlsx) with excelize (SPEC-008). Opening a workbook
// gives every non-empty cell of every sheet as exact text, with its sheet and cell; a
// profile then reads a sheet as a table through View, which finds the header row by its
// labels and keys each data row by header text, so columns are never found by their
// letter. Old Excel files (.xls) and password-protected workbooks are refused.
package xlsx

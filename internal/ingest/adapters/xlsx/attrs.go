package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// The cell kinds a light pass over a sheet's XML tells apart, from a cell's t attribute.
const (
	kindNumber = 'n' // a number, or no type: a number or a date by its style
	kindBool   = 'b'
	kindDate   = 'd' // an ISO 8601 date stored as text
	kindText   = 'x' // a shared or inline string, a formula's text result or an error value
)

// cellAttrs are what the pass finds for one cell with a value: its kind and its style.
type cellAttrs struct {
	kind  byte
	style int
}

// sheetAttrs are the attributes of one sheet's cells, rows and columns, read in one pass
// over its XML instead of excelize loading the whole sheet a second time for each cell's
// type and style (a 200,000-row sheet took seconds and gigabytes that way).
type sheetAttrs struct {
	cells      map[int64]cellAttrs // by row<<20 | column; only cells holding a value
	hiddenRows map[int]bool
	hiddenCols [][2]int // [min, max] column ranges marked hidden
}

func cellKey(row, col int) int64 { return int64(row)<<20 | int64(col) }

func (a *sheetAttrs) colHidden(c int) bool {
	for _, r := range a.hiddenCols {
		if c >= r[0] && c <= r[1] {
			return true
		}
	}
	return false
}

var errNoSheetPart = errors.New("xlsx: the sheet's part is not found")

// sheetParts maps each sheet's name to its XML part in the zip ("xl/worksheets/sheet1.xml"),
// from xl/workbook.xml and its relationships.
func sheetParts(zr *zip.Reader) (map[string]string, error) {
	ids := map[string]string{} // relationship ID → sheet name
	if err := scan(zr, "xl/workbook.xml", func(se xml.StartElement) {
		if se.Name.Local != "sheet" {
			return
		}
		var name, id string
		for _, a := range se.Attr {
			switch {
			case a.Name.Local == "name":
				name = a.Value
			case a.Name.Local == "id" && a.Name.Space != "":
				id = a.Value
			}
		}
		ids[id] = name
	}); err != nil {
		return nil, err
	}
	out := map[string]string{}
	if err := scan(zr, "xl/_rels/workbook.xml.rels", func(se xml.StartElement) {
		if se.Name.Local != "Relationship" {
			return
		}
		var id, target string
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "Id":
				id = a.Value
			case "Target":
				target = a.Value
			}
		}
		if name, ok := ids[id]; ok && target != "" {
			if strings.HasPrefix(target, "/") {
				out[name] = strings.TrimPrefix(target, "/")
			} else {
				out[name] = path.Clean("xl/" + target)
			}
		}
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// readAttrs reads the attributes of a sheet's cells, rows and columns from its XML part.
// Rows and cells without a position take the next one, as the format allows.
func readAttrs(zr *zip.Reader, part string) (*sheetAttrs, error) {
	a := &sheetAttrs{cells: map[int64]cellAttrs{}, hiddenRows: map[int]bool{}}
	row, col := 0, 0
	var cur cellAttrs
	inCell, hasValue := false, false
	err := scanTokens(zr, part, func(tok xml.Token) {
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				row, col = row+1, 0
				for _, at := range t.Attr {
					switch at.Name.Local {
					case "r":
						if n, err := strconv.Atoi(at.Value); err == nil {
							row = n
						}
					case "hidden":
						if at.Value == "1" || at.Value == "true" {
							a.hiddenRows[row] = true
						}
					}
				}
			case "c":
				col++
				cur, inCell, hasValue = cellAttrs{kind: kindNumber}, true, false
				for _, at := range t.Attr {
					switch at.Name.Local {
					case "r":
						if c, r, err := excelize.CellNameToCoordinates(at.Value); err == nil {
							col, row = c, r
						}
					case "s":
						cur.style, _ = strconv.Atoi(at.Value)
					case "t":
						switch at.Value {
						case "n", "":
						case "b":
							cur.kind = kindBool
						case "d":
							cur.kind = kindDate
						default:
							cur.kind = kindText
						}
					}
				}
			case "v", "is":
				if inCell {
					hasValue = true
				}
			case "col":
				var lo, hi int
				hidden := false
				for _, at := range t.Attr {
					switch at.Name.Local {
					case "min":
						lo, _ = strconv.Atoi(at.Value)
					case "max":
						hi, _ = strconv.Atoi(at.Value)
					case "hidden":
						hidden = at.Value == "1" || at.Value == "true"
					}
				}
				if hidden && lo > 0 && hi >= lo {
					a.hiddenCols = append(a.hiddenCols, [2]int{lo, hi})
				}
			}
		case xml.EndElement:
			if t.Name.Local == "c" {
				if hasValue {
					a.cells[cellKey(row, col)] = cur
				}
				inCell = false
			}
		}
	})
	return a, err
}

// scan calls fn for each start element of a part.
func scan(zr *zip.Reader, part string, fn func(xml.StartElement)) error {
	return scanTokens(zr, part, func(tok xml.Token) {
		if se, ok := tok.(xml.StartElement); ok {
			fn(se)
		}
	})
}

// scanTokens calls fn for each token of a part, without building the document in memory.
func scanTokens(zr *zip.Reader, part string, fn func(xml.Token)) error {
	var zf *zip.File
	for _, f := range zr.File {
		if f.Name == part {
			zf = f
			break
		}
	}
	if zf == nil {
		return errNoSheetPart
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		fn(tok)
	}
}

package currency

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// FileName is the rate file inside the currency folder.
const FileName = "usd-rate.yaml"

// Schema is the value of the "schema" key of the rate file.
const Schema = "recon.currency.usd/1"

// The two reasons a dollar amount cannot be converted. They are the words shown for a figure
// that cannot be checked.
const (
	ReasonNotSet = "dollar rate not set"
	ReasonBad    = "dollar rate not valid"
)

type fileYAML struct {
	Schema        string `yaml:"schema"`
	USDRate       string `yaml:"usd_rate"`
	EffectiveFrom string `yaml:"effective_from"`
	Note          string `yaml:"note"`
}

// A Rate is what the rate file gave. Value is above zero only when the rate is usable; when
// it is not, Problem says why (ReasonNotSet or ReasonBad) and Detail names the cause for the
// operator, never a client figure.
type Rate struct {
	Value     money.USDRate
	Effective string // the date the rate takes effect, as written (yyyy-mm-dd); kept for the CA's records
	Note      string // where the rate came from, as the file's reader decoded it; kept for the CA's records
	Problem   string
	Detail    string
	File      string // the path read, for the audit trail
}

// OK reports whether the rate can convert a dollar amount.
func (r Rate) OK() bool { return r.Problem == "" && r.Value > 0 }

// None is the rate of a run that was given no rate file at all.
func None() Rate { return Rate{Problem: ReasonNotSet, Detail: "no rate file was given"} }

// Load reads the rate from the currency folder. It never fails the run: a file that is
// missing or empty gives ReasonNotSet, and one that cannot be read as a rate gives ReasonBad.
func Load(dir string) Rate {
	path := filepath.Join(dir, FileName)
	r := Rate{File: filepath.ToSlash(path)}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.Problem, r.Detail = ReasonNotSet, FileName+" is missing"
		return r
	case err != nil:
		r.Problem, r.Detail = ReasonBad, FileName+" cannot be read"
		return r
	}
	if strings.TrimSpace(string(data)) == "" {
		r.Problem, r.Detail = ReasonNotSet, FileName+" is empty"
		return r
	}
	var f fileYAML
	if _, err := strictyaml.Decode(FileName, data, &f); err != nil {
		r.Problem, r.Detail = ReasonBad, FileName+" is not a valid rate file"
		return r
	}
	if f.Schema != Schema {
		r.Problem, r.Detail = ReasonBad, "schema must be "+Schema
		return r
	}
	r.Effective, r.Note = f.EffectiveFrom, f.Note
	if strings.TrimSpace(f.USDRate) == "" {
		r.Problem, r.Detail = ReasonNotSet, "usd_rate is empty"
		return r
	}
	v, err := money.ParseUSDRate(f.USDRate)
	if err != nil {
		r.Problem, r.Detail = ReasonBad, "usd_rate is not a rate (digits, up to four decimal places, above zero)"
		return r
	}
	if f.EffectiveFrom != "" {
		if _, err := period.ParseDate(f.EffectiveFrom); err != nil {
			r.Problem, r.Detail = ReasonBad, "effective_from is not a date written yyyy-mm-dd"
			return r
		}
	}
	r.Value = v
	return r
}

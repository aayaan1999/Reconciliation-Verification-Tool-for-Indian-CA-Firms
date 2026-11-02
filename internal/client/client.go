// Package client holds a client's details and checks them (SPEC-004 part 8): the identity
// numbers a CA firm records for a client, each checked for its format, and each GSTIN
// checked for its state code, its check digit and that its PAN part is the client's PAN.
// It has no I/O; the engine stores the details and the server checks a form with it first.
package client

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Details are everything recorded about a client. Text fields are "" when not given.
type Details struct {
	LegalName           string  `json:"legal_name"`
	TradeName           string  `json:"trade_name"`
	PAN                 string  `json:"pan,omitempty"` // in: the full PAN; never sent back out
	PANLast4            string  `json:"pan_last4"`     // out only
	Constitution        string  `json:"constitution"`
	DateOfIncorporation string  `json:"date_of_incorporation"` // YYYY-MM-DD
	CIN                 string  `json:"cin"`                   // CIN of a company or LLPIN of an LLP
	TAN                 string  `json:"tan"`
	GSTINs              []GSTIN `json:"gstins"`
	Address             Address `json:"address"`
	ContactPerson       string  `json:"contact_person"`
	Email               string  `json:"email"`
	Phone               string  `json:"phone"`
	AccountingSoftware  string  `json:"accounting_software"`
	Notes               string  `json:"notes"`
}

// A GSTIN is one GST registration of the client.
type GSTIN struct {
	GSTIN            string `json:"gstin"`
	RegistrationType string `json:"registration_type"`
	Primary          bool   `json:"primary"`
}

// Address is the registered address.
type Address struct {
	Line1 string `json:"line1"`
	Line2 string `json:"line2"`
	City  string `json:"city"`
	State string `json:"state"` // a two-digit GST state code
	PIN   string `json:"pin"`
}

// Constitutions are the kinds of client.
var Constitutions = []string{"company", "llp", "partnership_firm", "proprietorship", "individual", "huf", "trust", "aop_boi", "other"}

// RegistrationTypes are the kinds of GST registration.
var RegistrationTypes = []string{"regular", "composition", "sez_unit", "sez_developer", "isd", "tds_tcs", "non_resident", "casual"}

// States maps GST state codes to state and union-territory names.
var States = map[string]string{
	"01": "Jammu and Kashmir", "02": "Himachal Pradesh", "03": "Punjab", "04": "Chandigarh", "05": "Uttarakhand",
	"06": "Haryana", "07": "Delhi", "08": "Rajasthan", "09": "Uttar Pradesh", "10": "Bihar", "11": "Sikkim",
	"12": "Arunachal Pradesh", "13": "Nagaland", "14": "Manipur", "15": "Mizoram", "16": "Tripura", "17": "Meghalaya",
	"18": "Assam", "19": "West Bengal", "20": "Jharkhand", "21": "Odisha", "22": "Chhattisgarh", "23": "Madhya Pradesh",
	"24": "Gujarat", "26": "Dadra and Nagar Haveli and Daman and Diu", "27": "Maharashtra", "29": "Karnataka",
	"30": "Goa", "31": "Lakshadweep", "32": "Kerala", "33": "Tamil Nadu", "34": "Puducherry",
	"35": "Andaman and Nicobar Islands", "36": "Telangana", "37": "Andhra Pradesh", "38": "Ladakh", "97": "Other Territory",
}

// A Problem is one field that does not pass its check.
type Problem struct {
	Field   string `json:"field"` // "pan", "gstins[1]", "address.pin"
	Code    string `json:"code"`  // "BAD_PAN", "GSTIN_CHECK_DIGIT" ...
	Message string `json:"message"`
}

func (p Problem) String() string { return p.Field + ": " + p.Code + ": " + p.Message }

var (
	panPattern   = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)
	tanPattern   = regexp.MustCompile(`^[A-Z]{4}[0-9]{5}[A-Z]$`)
	cinPattern   = regexp.MustCompile(`^[LU][0-9]{5}[A-Z]{2}[0-9]{4}[A-Z]{3}[0-9]{6}$`)
	llpinPattern = regexp.MustCompile(`^[A-Z]{3}-[0-9]{4}$`)
	gstinPattern = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
	pinPattern   = regexp.MustCompile(`^[1-9][0-9]{5}$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 -]{6,18}[0-9]$`)
	datePattern  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	// A plain pattern, not net/mail: the engine may import no network package (archtest).
	emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
)

// Normalise trims every field and puts identity numbers in capitals.
func (d *Details) Normalise() {
	trim := strings.TrimSpace
	d.LegalName, d.TradeName, d.Constitution = trim(d.LegalName), trim(d.TradeName), trim(d.Constitution)
	d.PAN, d.CIN, d.TAN = strings.ToUpper(trim(d.PAN)), strings.ToUpper(trim(d.CIN)), strings.ToUpper(trim(d.TAN))
	d.DateOfIncorporation, d.ContactPerson, d.Email, d.Phone = trim(d.DateOfIncorporation), trim(d.ContactPerson), trim(d.Email), trim(d.Phone)
	d.AccountingSoftware, d.Notes = trim(d.AccountingSoftware), trim(d.Notes)
	a := &d.Address
	a.Line1, a.Line2, a.City, a.State, a.PIN = trim(a.Line1), trim(a.Line2), trim(a.City), trim(a.State), trim(a.PIN)
	for i := range d.GSTINs {
		d.GSTINs[i].GSTIN = strings.ToUpper(trim(d.GSTINs[i].GSTIN))
		d.GSTINs[i].RegistrationType = trim(d.GSTINs[i].RegistrationType)
	}
}

// Validate returns every field that does not pass its check, in field order. pan is the
// client's PAN, used to check the GSTINs (the details' own PAN when it is given).
func (d Details) Validate(pan string, panRequired bool) []Problem {
	var out []Problem
	bad := func(field, code, format string, a ...any) {
		out = append(out, Problem{Field: field, Code: code, Message: fmt.Sprintf(format, a...)})
	}
	if d.LegalName == "" || len([]rune(d.LegalName)) > 200 || strings.IndexFunc(d.LegalName, unicode.IsControl) >= 0 {
		bad("legal_name", "BAD_CLIENT_NAME", "the legal name is 1 to 200 characters of plain text")
	}
	if len([]rune(d.TradeName)) > 200 || strings.IndexFunc(d.TradeName, unicode.IsControl) >= 0 {
		bad("trade_name", "BAD_TRADE_NAME", "the trade name is at most 200 characters of plain text")
	}
	switch {
	case d.PAN == "" && panRequired:
		bad("pan", "BAD_PAN", "the PAN is required")
	case d.PAN != "" && !panPattern.MatchString(d.PAN):
		bad("pan", "BAD_PAN", "a PAN is five letters, four digits and one letter, such as ABCDE1234F")
	}
	if d.Constitution != "" && !slices.Contains(Constitutions, d.Constitution) {
		bad("constitution", "BAD_CONSTITUTION", "constitution %q is not one of: %s", d.Constitution, strings.Join(Constitutions, ", "))
	}
	if d.DateOfIncorporation != "" && !validDate(d.DateOfIncorporation) {
		bad("date_of_incorporation", "BAD_DATE", "write the date as YYYY-MM-DD")
	}
	if d.CIN != "" && !cinPattern.MatchString(d.CIN) && !llpinPattern.MatchString(d.CIN) {
		bad("cin", "BAD_CIN", "a CIN is 21 characters such as U72200MH2012PTC123456, or an LLPIN such as AAB-1234")
	}
	if d.TAN != "" && !tanPattern.MatchString(d.TAN) {
		bad("tan", "BAD_TAN", "a TAN is four letters, five digits and one letter, such as PNEA12345B")
	}
	primaries := 0
	seen := map[string]bool{}
	for i, g := range d.GSTINs {
		field := fmt.Sprintf("gstins[%d]", i)
		if g.Primary {
			primaries++
		}
		switch {
		case !gstinPattern.MatchString(g.GSTIN):
			bad(field, "BAD_GSTIN", "a GSTIN is 15 characters: state code, PAN, entity number, Z and a check character")
			continue
		case States[g.GSTIN[:2]] == "":
			bad(field, "GSTIN_STATE", "%s does not start with a GST state code", g.GSTIN)
		case CheckChar(g.GSTIN[:14]) != g.GSTIN[14]:
			bad(field, "GSTIN_CHECK_DIGIT", "the check character of %s is wrong; the GSTIN is mistyped", g.GSTIN)
		case pan != "" && g.GSTIN[2:12] != pan:
			bad(field, "GSTIN_PAN_MISMATCH", "%s does not contain the client's PAN (it contains %s)", g.GSTIN, g.GSTIN[2:12])
		case seen[g.GSTIN]:
			bad(field, "GSTIN_TWICE", "%s is listed twice", g.GSTIN)
		}
		seen[g.GSTIN] = true
		if !slices.Contains(RegistrationTypes, g.RegistrationType) {
			bad(field+".registration_type", "BAD_REGISTRATION_TYPE", "registration type %q is not one of: %s", g.RegistrationType, strings.Join(RegistrationTypes, ", "))
		}
	}
	if len(d.GSTINs) > 0 && primaries != 1 {
		bad("gstins", "PRIMARY_GSTIN", "mark exactly one GSTIN as primary")
	}
	if d.Address.State != "" && States[d.Address.State] == "" {
		bad("address.state", "BAD_STATE", "state %q is not a GST state code", d.Address.State)
	}
	if d.Address.PIN != "" && !pinPattern.MatchString(d.Address.PIN) {
		bad("address.pin", "BAD_PIN", "a PIN code is six digits and does not start with 0")
	}
	if d.Email != "" && (!emailPattern.MatchString(d.Email) || len(d.Email) > 254) {
		bad("email", "BAD_EMAIL", "write the email as name@example.com")
	}
	if d.Phone != "" && !phonePattern.MatchString(d.Phone) {
		bad("phone", "BAD_PHONE", "a phone number is digits, with an optional + and spaces or dashes")
	}
	for _, f := range []struct{ name, value string }{{"address.line1", d.Address.Line1}, {"address.line2", d.Address.Line2},
		{"address.city", d.Address.City}, {"contact_person", d.ContactPerson}, {"accounting_software", d.AccountingSoftware}} {
		if len([]rune(f.value)) > 200 || strings.IndexFunc(f.value, unicode.IsControl) >= 0 {
			bad(f.name, "TOO_LONG", "at most 200 characters of plain text")
		}
	}
	if len([]rune(d.Notes)) > 2000 {
		bad("notes", "TOO_LONG", "notes are at most 2000 characters")
	}
	return out
}

func validDate(s string) bool {
	if !datePattern.MatchString(s) {
		return false
	}
	y, m, d := atoi(s[:4]), atoi(s[5:7]), atoi(s[8:])
	if m < 1 || m > 12 || d < 1 || y < 1800 {
		return false
	}
	days := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[m-1]
	if m == 2 && y%4 == 0 && (y%100 != 0 || y%400 == 0) {
		days = 29
	}
	return d <= days
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

const gstinAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// CheckChar returns the check character of the first 14 characters of a GSTIN: the
// weighted sum (weights 1 and 2 alternately) of the characters' values in base 36, each
// product reduced to quotient plus remainder by 36, and the check character is the value
// that makes the total a multiple of 36.
func CheckChar(first14 string) byte {
	sum := 0
	for i := 0; i < len(first14); i++ {
		v := strings.IndexByte(gstinAlphabet, first14[i])
		if v < 0 {
			return 0
		}
		p := v * (1 + i%2)
		sum += p/36 + p%36
	}
	return gstinAlphabet[(36-sum%36)%36]
}

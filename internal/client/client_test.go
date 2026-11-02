package client

import (
	"strings"
	"testing"
)

// The GSTINs of the synthetic GST pack have valid check characters (its README says so).
func TestCheckChar(t *testing.T) {
	for _, g := range []string{"27ZLVCD9565Z1ZF", "27BMSCV1962U1ZB", "29YWSCH4524Q1ZQ", "29KLJCK1232C1ZY", "33UVVCM8633H1Z8"} {
		if c := CheckChar(g[:14]); c != g[14] {
			t.Errorf("%s: check character %c, want %c", g, c, g[14])
		}
	}
}

func good() Details {
	return Details{
		LegalName: "Deccan Byte Systems Private Limited", PAN: "ZLVCD9565Z", Constitution: "company",
		DateOfIncorporation: "2014-03-12", CIN: "U72200MH2014PTC123456", TAN: "PNED12345B",
		GSTINs:  []GSTIN{{GSTIN: "27ZLVCD9565Z1ZF", RegistrationType: "regular", Primary: true}},
		Address: Address{Line1: "Plot 4, Hinjewadi", City: "Pune", State: "27", PIN: "411057"},
		Email:   "accounts@example.com", Phone: "+91 20 1234 5678",
	}
}

func TestValidate(t *testing.T) {
	if p := good().Validate("ZLVCD9565Z", true); len(p) != 0 {
		t.Fatalf("good details refused: %v", p)
	}
	for name, c := range map[string]struct {
		edit func(*Details)
		code string
	}{
		"PAN":               {func(d *Details) { d.PAN = "ZLVCD9565" }, "BAD_PAN"},
		"no name":           {func(d *Details) { d.LegalName = "" }, "BAD_CLIENT_NAME"},
		"TAN":               {func(d *Details) { d.TAN = "PNE12345B" }, "BAD_TAN"},
		"CIN":               {func(d *Details) { d.CIN = "U722" }, "BAD_CIN"},
		"LLPIN accepted":    {func(d *Details) { d.CIN = "AAB-1234" }, ""},
		"check digit":       {func(d *Details) { d.GSTINs[0].GSTIN = "27ZLVCD9565Z1ZG" }, "GSTIN_CHECK_DIGIT"},
		"other PAN":         {func(d *Details) { d.GSTINs[0].GSTIN = "27BMSCV1962U1ZB" }, "GSTIN_PAN_MISMATCH"},
		"state code":        {func(d *Details) { d.GSTINs[0].GSTIN = "99ZLVCD9565Z1Z" + string(CheckChar("99ZLVCD9565Z1Z")) }, "GSTIN_STATE"},
		"no primary":        {func(d *Details) { d.GSTINs[0].Primary = false }, "PRIMARY_GSTIN"},
		"registration type": {func(d *Details) { d.GSTINs[0].RegistrationType = "x" }, "BAD_REGISTRATION_TYPE"},
		"PIN":               {func(d *Details) { d.Address.PIN = "011057" }, "BAD_PIN"},
		"email":             {func(d *Details) { d.Email = "accounts" }, "BAD_EMAIL"},
		"phone":             {func(d *Details) { d.Phone = "call me" }, "BAD_PHONE"},
		"date":              {func(d *Details) { d.DateOfIncorporation = "2014-02-30" }, "BAD_DATE"},
		"constitution":      {func(d *Details) { d.Constitution = "club" }, "BAD_CONSTITUTION"},
	} {
		t.Run(name, func(t *testing.T) {
			d := good()
			c.edit(&d)
			var codes []string
			for _, p := range d.Validate("ZLVCD9565Z", true) {
				codes = append(codes, p.Code)
			}
			if got := strings.Join(codes, ","); got != c.code {
				t.Errorf("codes %q, want %q", got, c.code)
			}
		})
	}
}

func TestNormalise(t *testing.T) {
	d := Details{LegalName: "  X  ", PAN: " zlvcd9565z ", GSTINs: []GSTIN{{GSTIN: " 27zlvcd9565z1zf "}}}
	d.Normalise()
	if d.LegalName != "X" || d.PAN != "ZLVCD9565Z" || d.GSTINs[0].GSTIN != "27ZLVCD9565Z1ZF" {
		t.Errorf("%+v", d)
	}
}

package money

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseAccepted(t *testing.T) {
	cases := []struct {
		in   string
		unit Unit
		want Paise
	}{
		{"1,12,450", UnitRupees, 11245000},
		{"1,12,450.50", UnitRupees, 11245050},
		{"1,12,450.5", UnitRupees, 11245050},
		{"112,450.50", UnitRupees, 11245050},
		{"112450.50", UnitRupees, 11245050},
		{"12,34,56,789", UnitRupees, 123456789 * 100},
		{"1,234,567", UnitRupees, 1234567 * 100},
		{"12,345", UnitRupees, 1234500},
		{"123,456", UnitRupees, 12345600},
		{"-500", UnitRupees, -50000},
		{"(2,750.00)", UnitRupees, -275000},
		{"(2750)", UnitRupees, -275000},
		{"₹1,12,450", UnitRupees, 11245000},
		{"-₹500", UnitRupees, -50000},
		{"(₹2,750)", UnitRupees, -275000},
		{"-0.50", UnitRupees, -50},
		{"0.05", UnitRupees, 5},
		{"0", UnitRupees, 0},
		{"-0", UnitRupees, 0},
		{"007", UnitRupees, 700},
		{"  1,12,450  ", UnitRupees, 11245000},
		{"11245050", UnitPaise, 11245050},
		{"-275000", UnitPaise, -275000},
		{"1,12,45,050", UnitPaise, 11245050},
		{"92233720368547758.07", UnitRupees, MaxPaise},
		{"-92233720368547758.08", UnitRupees, MinPaise},
		{"9223372036854775807", UnitPaise, MaxPaise},
		{"-9223372036854775808", UnitPaise, MinPaise},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := Parse(c.in, c.unit)
			if err != nil || got != c.want {
				t.Fatalf("Parse(%q) = %d, %v; want %d", c.in, got, err, c.want)
			}
		})
	}
}

func TestParseRefused(t *testing.T) {
	cases := []struct {
		in     string
		unit   Unit
		reason string
	}{
		{"", UnitRupees, "empty"},
		{"   ", UnitRupees, "empty"},
		{"12.345", UnitRupees, "more than two decimal places"},
		{"1,1,2", UnitRupees, "commas are not in valid Indian or Western positions"},
		{"1,12,4500", UnitRupees, "commas are not in valid Indian or Western positions"},
		{"123,45,678", UnitRupees, "commas are not in valid Indian or Western positions"},
		{"1,234,56", UnitRupees, "commas are not in valid Indian or Western positions"},
		{"12,34,567,890", UnitRupees, "commas are not in valid Indian or Western positions"},
		{",123", UnitRupees, "commas are not in valid Indian or Western positions"},
		{"0,123", UnitRupees, "commas are not in valid Indian or Western positions"},
		{"1e5", UnitRupees, "contains characters that are not digits"},
		{"abc", UnitRupees, "contains characters that are not digits"},
		{"12 34", UnitRupees, "contains characters that are not digits"},
		{"+500", UnitRupees, "contains characters that are not digits"},
		{"--500", UnitRupees, "contains characters that are not digits"},
		{"(-500)", UnitRupees, "contains characters that are not digits"},
		{"₹-500", UnitRupees, "contains characters that are not digits"},
		{"-(500)", UnitRupees, "both a minus sign and brackets"},
		{"1.2.3", UnitRupees, "more than one decimal point"},
		{".50", UnitRupees, "no digits before the decimal point"},
		{"12.", UnitRupees, "no digits after the decimal point"},
		{"-", UnitRupees, "no digits before the decimal point"},
		{"()", UnitRupees, "no digits before the decimal point"},
		{"100.5", UnitPaise, "a paise amount cannot have decimal places"},
		{"100", Unit(0), "unknown unit"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := Parse(c.in, c.unit)
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Parse(%q) = %d, %v; want a ParseError", c.in, got, err)
			}
			if pe.Reason != c.reason || pe.Input != c.in {
				t.Fatalf("Parse(%q) reason %q; want %q", c.in, pe.Reason, c.reason)
			}
		})
	}
}

func TestParseOverflow(t *testing.T) {
	for _, c := range []struct {
		in   string
		unit Unit
	}{
		{"92233720368547758.08", UnitRupees},
		{"-92233720368547758.09", UnitRupees},
		{"9223372036854775808", UnitPaise},
		{"99999999999999999999999", UnitRupees},
	} {
		if got, err := Parse(c.in, c.unit); !errors.Is(err, ErrOverflow) {
			t.Errorf("Parse(%q) = %d, %v; want ErrOverflow", c.in, got, err)
		}
	}
}

func TestParseJSONNumber(t *testing.T) {
	// A value a float64 cannot hold exactly: it would come back as ...409.94 or similar.
	if got, err := ParseJSONNumber(json.Number("90071992547409.93"), UnitRupees); err != nil || got != 9007199254740993 {
		t.Errorf("exact large value: got %d, %v", got, err)
	}
	if got, err := ParseJSONNumber(json.Number("-2750.5"), UnitRupees); err != nil || got != -275050 {
		t.Errorf("negative: got %d, %v", got, err)
	}
	for _, in := range []string{"1e5", "1E5", "1,000", "(500)", "₹500", "12.345", ""} {
		if got, err := ParseJSONNumber(json.Number(in), UnitRupees); err == nil {
			t.Errorf("ParseJSONNumber(%q) = %d; want an error", in, got)
		}
	}
}

func TestParseFromDecodedJSON(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`{"allowed_depreciation": 115200.00}`))
	dec.UseNumber()
	var doc map[string]json.Number
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if got, err := ParseJSONNumber(doc["allowed_depreciation"], UnitRupees); err != nil || got != 11520000 {
		t.Errorf("got %d, %v; want 11520000", got, err)
	}
}

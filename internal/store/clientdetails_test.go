package store

import (
	"errors"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/client"
)

// The PAN and GSTINs are the synthetic ones of the sample GST pack (testdata/gst_pack_aug2026).
func deccan() client.Details {
	return client.Details{
		LegalName: "Deccan Byte Systems Pvt Ltd", PAN: "ZLVCD9565Z", Constitution: "company", TAN: "PNED12345A",
		GSTINs:  []client.GSTIN{{GSTIN: "27ZLVCD9565Z1ZF", RegistrationType: "regular", Primary: true}},
		Address: client.Address{City: "Pune", State: "27", PIN: "411001"},
	}
}

func TestClientDetailsRoundTrip(t *testing.T) {
	db, _ := open(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	c, err := tx.AddClientDetails(deccan())
	if err != nil {
		t.Fatal(err)
	}
	d, found, err := tx.ClientDetails(c.ID)
	if err != nil || !found {
		t.Fatalf("ClientDetails: found %v, err %v", found, err)
	}
	if d.PAN != "" || d.PANLast4 != "565Z" || d.LegalName != "Deccan Byte Systems Pvt Ltd" || d.TAN != "PNED12345A" ||
		d.Address.PIN != "411001" || len(d.GSTINs) != 1 || !d.GSTINs[0].Primary {
		t.Errorf("details = %+v", d)
	}

	// The PAN inside a GSTIN is checked against the stored hash; the full PAN is not stored.
	if ok, err := tx.GSTINPANMatches("27ZLVCD9565Z1ZF", c.PANHash); err != nil || !ok {
		t.Errorf("GSTINPANMatches(own GSTIN) = %v, %v", ok, err)
	}
	if ok, _ := tx.GSTINPANMatches("27AAACI1234Q1ZX", c.PANHash); ok {
		t.Error("GSTINPANMatches(another PAN's GSTIN) = true")
	}

	// Editing with an empty PAN keeps the PAN and replaces the rest.
	second := "29ZLVCD9565Z1Z" + string(client.CheckChar("29ZLVCD9565Z1Z"))
	e := deccan()
	e.PAN, e.TradeName = "", "Deccan Byte"
	e.GSTINs = append(e.GSTINs, client.GSTIN{GSTIN: second, RegistrationType: "regular"})
	if _, err := tx.UpdateClient(c.ID, e); err != nil {
		t.Fatal(err)
	}
	d, _, _ = tx.ClientDetails(c.ID)
	if d.PANLast4 != "565Z" || d.TradeName != "Deccan Byte" || len(d.GSTINs) != 2 || d.GSTINs[1].GSTIN != second {
		t.Errorf("after update: %+v", d)
	}

	// A GSTIN belongs to one client only.
	other := client.Details{LegalName: "Other", PAN: "AAACI1234Q", GSTINs: []client.GSTIN{{GSTIN: second, RegistrationType: "regular", Primary: true}}}
	var ce *ConflictError
	if _, err := tx.AddClientDetails(other); !errors.As(err, &ce) {
		t.Errorf("second client with the same GSTIN: err %v, want a ConflictError", err)
	}
}

package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// SPEC-022 part B through the server: a rule over the rows of a document the Partner added goes
// through the existing rule endpoints with the existing permissions. Names are invented.

// officeRuleForm is a rule over the sales return register: a return whose net amount is zero is
// reported. It has one pass and one fail fixture, each a row of the office record type.
func officeRuleForm(id string) rules.Form {
	row := func(file, ref, net string) rules.FormRecord {
		return rules.FormRecord{RecordType: "sales_return_row", Period: "2026-08", File: file, Locator: "$.Returns[0]",
			Fields:  []rules.FormNamed{{Name: "ref_no", Value: ref}},
			Amounts: []rules.FormNamed{{Name: "net_amount", Value: net}}}
	}
	return rules.Form{
		ID: id, Title: "Sales returns with no net amount (office test)", RuleText: "Reports a sales return whose net amount is zero.",
		Module: "office", Category: "R1", TaxYears: []string{"2026-27"}, ValidFrom: "2026-04-01",
		Inputs:         []rules.FormInput{{Name: "a", Kind: "record_type", Ref: "sales_return_row", Label: "Sales returns"}},
		Check:          rules.FormCheck{Type: "present_together", When: []rules.FormCondition{{Input: "a", Field: "net_amount", Is: "zero"}}},
		Severity:       "medium",
		ReviewDocument: "sales_return",
		Law:            []rules.FormLaw{{Act: "Central Goods and Services Tax Act, 2017", Section: "34"}},
		Fixtures: []rules.FormFixture{
			{Kind: "pass", Name: "has_net", Description: "A return with a net amount", Records: []rules.FormRecord{row("ok.json", "SR-1", "1000.00")}},
			{Kind: "fail", Name: "zero_net", Description: "A return with no net amount", Records: []rules.FormRecord{row("zero.json", "SR-2", "0.00")}},
		},
	}
}

// AC-17, AC-18, AC-23 through the server.
func TestOfficeRulesThroughServer(t *testing.T) {
	e := newEnv(t)
	staff, manager, readOnly := users(t, e)
	url := e.srv.URL

	// the Partner adds the document; its record structure is ready at once
	if status, out := docReq(t, e.client, "POST", url+"/api/v1/document-types", salesReturnDoc); status != http.StatusCreated {
		t.Fatalf("add the document: %d %s", status, out)
	}

	// every role that may see rules reads the office record types, with labelled columns and the row check types
	for name, c := range map[string]*http.Client{"partner": e.client, "staff": staff, "manager": manager, "read-only": readOnly} {
		status, out := docReq(t, c, "GET", url+"/api/v1/rule-options", "")
		for _, want := range []string{`"origin":"office"`, `"name":"sales_return_row"`, `"label":"Reference"`, `"row_check_types":["match_records","present_together","sum_equals"]`, `"office"`} {
			if status != http.StatusOK || !strings.Contains(out, want) {
				t.Errorf("rule-options as %s lacks %s (status %d)", name, want, status)
			}
		}
	}

	// Staff and Manager draft and test; Read-only cannot
	save := func(c *http.Client, id string) (int, string) {
		return docReqJSON(t, c, "POST", url+"/api/v1/rules", map[string]any{"rule": officeRuleForm(id)})
	}
	if status, out := save(staff, "RECON-OFF-201"); status != http.StatusCreated {
		t.Fatalf("Staff saves an office rule: %d %s", status, out)
	}
	if status, out := save(manager, "RECON-OFF-202"); status != http.StatusCreated {
		t.Fatalf("Manager saves an office rule: %d %s", status, out)
	}
	if status, out := save(readOnly, "RECON-OFF-203"); status != http.StatusForbidden || !strings.Contains(out, "draft_rules") {
		t.Errorf("Read-only saves an office rule: %d %s", status, out)
	}
	// neither can approve or disable it; the Partner can ask (and is told it has not run on a client)
	for name, c := range map[string]*http.Client{"staff": staff, "manager": manager} {
		if status, out := docReqJSON(t, c, "POST", url+"/api/v1/rules/RECON-OFF-201/approve", map[string]string{"reason": "checked"}); status != http.StatusForbidden || !strings.Contains(out, "activate_rules") {
			t.Errorf("%s approves: %d %s", name, status, out)
		}
		if status, out := docReqJSON(t, c, "POST", url+"/api/v1/rules/RECON-OFF-201/disable", map[string]string{"reason": "checked"}); status != http.StatusForbidden || !strings.Contains(out, "disable_rules") {
			t.Errorf("%s disables: %d %s", name, status, out)
		}
	}
	if status, out := docReqJSON(t, e.client, "POST", url+"/api/v1/rules/RECON-OFF-201/approve", map[string]string{"reason": "checked"}); status != http.StatusUnprocessableEntity || !strings.Contains(out, "NOT_RUN_ON_CLIENT") {
		t.Errorf("the Partner approves before a client run: %d %s", status, out)
	}

	// the rules list flags both, as testing rules over an office document
	status, list := e.get(t, "/api/v1/rules")
	if status != http.StatusOK || strings.Count(list, `"reads_office_documents":true`) != 2 || !strings.Contains(list, `"id":"RECON-OFF-201"`) {
		t.Errorf("the rules list: %d, flagged %d", status, strings.Count(list, `"reads_office_documents":true`))
	}

	// refusals name the field and store nothing
	notOffice := officeRuleForm("RECON-OFF-204")
	notOffice.Module = "tieout"
	status, out := docReqJSON(t, staff, "POST", url+"/api/v1/rules", map[string]any{"rule": notOffice})
	if status != http.StatusUnprocessableEntity || !strings.Contains(out, `"field":"module"`) || !strings.Contains(out, "office module") {
		t.Errorf("an office rule in the tieout module: %d %s", status, out)
	}
	equal := officeRuleForm("RECON-OFF-205")
	equal.Check = rules.FormCheck{Type: "equal"}
	equal.Inputs = append(equal.Inputs, rules.FormInput{Name: "b", Kind: "fact", Ref: "computation.allowed_depreciation", Label: "A figure"})
	status, out = docReqJSON(t, staff, "POST", url+"/api/v1/rules", map[string]any{"rule": equal})
	if status != http.StatusUnprocessableEntity || !strings.Contains(out, `"field":"check.type"`) || !strings.Contains(out, "match_records, present_together, sum_equals") {
		t.Errorf("equal over office rows: %d %s", status, out)
	}
	badColumn := officeRuleForm("RECON-OFF-206")
	badColumn.Check.When[0].Field = "vat_amount"
	status, out = docReqJSON(t, staff, "POST", url+"/api/v1/rules", map[string]any{"rule": badColumn})
	if status != http.StatusUnprocessableEntity || !strings.Contains(out, `"field":"check.when[0].field"`) || !strings.Contains(out, "vat_amount") {
		t.Errorf("a column the record type does not have: %d %s", status, out)
	}
	if status, list := e.get(t, "/api/v1/rules"); status != http.StatusOK || strings.Contains(list, "RECON-OFF-204") || strings.Contains(list, "RECON-OFF-205") || strings.Contains(list, "RECON-OFF-206") {
		t.Error("a refused rule was stored")
	}

	// the rule locks the document's structure and keeps it from being deleted
	if status, out := docReq(t, e.client, "DELETE", url+"/api/v1/document-types/sales_return", ""); status != http.StatusConflict || !strings.Contains(out, "IN_USE") || !strings.Contains(out, "RECON-OFF-201") {
		t.Errorf("delete of a document a rule reads: %d %s", status, out)
	}
	dropColumn := strings.Replace(salesReturnDoc, `,
  {"name":"doc_date","label":"Date","type":"date","key":true,"required":false}`, "", 1)
	if status, out := docReq(t, e.client, "PUT", url+"/api/v1/document-types/sales_return", dropColumn); status != http.StatusUnprocessableEntity || !strings.Contains(out, `"code":"LOCKED"`) {
		t.Errorf("removing a column a rule's document has: %d %s", status, out)
	}

	// the audit log names each person who drafted a rule
	st := auditOf(t, e)
	creates := newest(st, "rule.create")
	if len(creates) != 2 {
		t.Fatalf("%d rule.create entries, want 2", len(creates))
	}
	actors := map[string]bool{}
	for _, i := range creates {
		actors[st.Entries[i].Actor] = true
	}
	if !actors["sunil"] || !actors["meera"] {
		t.Errorf("rule.create actors = %v, want Staff and Manager", actors)
	}
}

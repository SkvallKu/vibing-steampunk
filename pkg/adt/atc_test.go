package adt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestParseATCCustomizing(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<atccust:customizing xmlns:atccust="http://www.sap.com/adt/atc/customizing">
  <atccust:properties>
    <atccust:property name="systemCheckVariant" value="STANDARD"/>
    <atccust:property name="transportCheckVariant" value="TRANSPORT_CHECK"/>
  </atccust:properties>
  <atccust:exemption>
    <atccust:reasons>
      <atccust:reason id="FPOS" title="False Positive" justificationMandatory="true"/>
      <atccust:reason id="OTHR" title="Other" justificationMandatory="false"/>
    </atccust:reasons>
  </atccust:exemption>
</atccust:customizing>`

	result, err := parseATCCustomizing([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseATCCustomizing failed: %v", err)
	}

	// Check properties
	if len(result.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(result.Properties))
	}

	var foundVariant bool
	for _, p := range result.Properties {
		if p.Name == "systemCheckVariant" && p.Value == "STANDARD" {
			foundVariant = true
		}
	}
	if !foundVariant {
		t.Error("expected to find systemCheckVariant=STANDARD")
	}

	// Check exemptions
	if len(result.Exemptions) != 2 {
		t.Errorf("expected 2 exemptions, got %d", len(result.Exemptions))
	}

	var foundFPOS bool
	for _, e := range result.Exemptions {
		if e.ID == "FPOS" && e.Title == "False Positive" && e.JustificationMandatory {
			foundFPOS = true
		}
	}
	if !foundFPOS {
		t.Error("expected to find FPOS exemption with JustificationMandatory=true")
	}
}

func TestParseATCRunResult(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<atcworklist:worklistRun xmlns:atcworklist="http://www.sap.com/adt/atc/worklist">
  <atcworklist:worklistId>12345ABCDE</atcworklist:worklistId>
  <atcworklist:worklistTimestamp>2025-12-04T10:30:00Z</atcworklist:worklistTimestamp>
  <atcworklist:infos>
    <atcworklist:info type="info" description="Check completed"/>
  </atcworklist:infos>
</atcworklist:worklistRun>`

	result, err := parseATCRunResult([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseATCRunResult failed: %v", err)
	}

	if result.WorklistID != "12345ABCDE" {
		t.Errorf("expected worklistId '12345ABCDE', got '%s'", result.WorklistID)
	}

	if len(result.Infos) != 1 {
		t.Errorf("expected 1 info, got %d", len(result.Infos))
	}

	if result.Infos[0].Type != "info" || result.Infos[0].Description != "Check completed" {
		t.Errorf("unexpected info: %+v", result.Infos[0])
	}
}

func TestParseATCWorklist(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<atcworklist:worklist xmlns:atcworklist="http://www.sap.com/adt/atc/worklist"
                      xmlns:atcobject="http://www.sap.com/adt/atc/object"
                      xmlns:atcfinding="http://www.sap.com/adt/atc/finding"
                      xmlns:adtcore="http://www.sap.com/adt/core"
                      id="12345ABCDE" timestamp="2025-12-04T10:30:00Z"
                      usedObjectSet="LAST_RUN" objectSetIsComplete="true">
  <atcworklist:objectSets>
    <atcworklist:objectSet name="LAST_RUN" title="Last Run" kind="LAST_RUN"/>
  </atcworklist:objectSets>
  <atcworklist:objects>
    <atcobject:object uri="/sap/bc/adt/oo/classes/ZCL_TEST" type="CLAS/OC"
                      name="ZCL_TEST" packageName="$TMP" author="DEVELOPER">
      <atcfinding:findings>
        <atcfinding:finding uri="/sap/bc/adt/atc/findings/123"
                           location="/sap/bc/adt/oo/classes/ZCL_TEST/source/main#start=10,5"
                           priority="2" checkId="CL_CI_TEST" checkTitle="Test Check"
                           messageId="001" messageTitle="Unused variable"
                           exemptionApproval="" exemptionKind="" quickfixInfo="QF123">
          <atcfinding:link href="/sap/bc/adt/atc/quickfix/123" rel="http://www.sap.com/adt/relations/quickfix" type="application/xml"/>
        </atcfinding:finding>
        <atcfinding:finding uri="/sap/bc/adt/atc/findings/124"
                           location="/sap/bc/adt/oo/classes/ZCL_TEST/source/main#start=25,1"
                           priority="1" checkId="CL_CI_SYNTAX" checkTitle="Syntax Check"
                           messageId="002" messageTitle="Syntax error in statement">
        </atcfinding:finding>
      </atcfinding:findings>
    </atcobject:object>
  </atcworklist:objects>
</atcworklist:worklist>`

	result, err := parseATCWorklist([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseATCWorklist failed: %v", err)
	}

	// Check basic attributes
	if result.ID != "12345ABCDE" {
		t.Errorf("expected id '12345ABCDE', got '%s'", result.ID)
	}
	if result.UsedObjectSet != "LAST_RUN" {
		t.Errorf("expected usedObjectSet 'LAST_RUN', got '%s'", result.UsedObjectSet)
	}
	if !result.ObjectSetIsComplete {
		t.Error("expected objectSetIsComplete to be true")
	}

	// Check object sets
	if len(result.ObjectSets) != 1 {
		t.Errorf("expected 1 object set, got %d", len(result.ObjectSets))
	}

	// Check objects
	if len(result.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(result.Objects))
	}

	obj := result.Objects[0]
	if obj.Name != "ZCL_TEST" {
		t.Errorf("expected object name 'ZCL_TEST', got '%s'", obj.Name)
	}
	if obj.PackageName != "$TMP" {
		t.Errorf("expected package '$TMP', got '%s'", obj.PackageName)
	}

	// Check findings
	if len(obj.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(obj.Findings))
	}

	// First finding (warning)
	f1 := obj.Findings[0]
	if f1.Priority != 2 {
		t.Errorf("expected priority 2 (warning), got %d", f1.Priority)
	}
	if f1.MessageTitle != "Unused variable" {
		t.Errorf("expected message 'Unused variable', got '%s'", f1.MessageTitle)
	}
	if f1.Line != 10 || f1.Column != 5 {
		t.Errorf("expected line=10, column=5, got line=%d, column=%d", f1.Line, f1.Column)
	}
	if f1.QuickfixInfo != "QF123" {
		t.Errorf("expected quickfixInfo 'QF123', got '%s'", f1.QuickfixInfo)
	}

	// Second finding (error)
	f2 := obj.Findings[1]
	if f2.Priority != 1 {
		t.Errorf("expected priority 1 (error), got %d", f2.Priority)
	}
	if f2.Line != 25 || f2.Column != 1 {
		t.Errorf("expected line=25, column=1, got line=%d, column=%d", f2.Line, f2.Column)
	}
}

func TestParseATCWorklistEmpty(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<atcworklist:worklist xmlns:atcworklist="http://www.sap.com/adt/atc/worklist"
                      id="EMPTY123" timestamp="2025-12-04T10:30:00Z"
                      usedObjectSet="" objectSetIsComplete="true">
  <atcworklist:objectSets/>
  <atcworklist:objects/>
</atcworklist:worklist>`

	result, err := parseATCWorklist([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseATCWorklist failed: %v", err)
	}

	if result.ID != "EMPTY123" {
		t.Errorf("expected id 'EMPTY123', got '%s'", result.ID)
	}
	if len(result.Objects) != 0 {
		t.Errorf("expected 0 objects, got %d", len(result.Objects))
	}
}

func TestATCFindingPriorityMapping(t *testing.T) {
	// Test priority interpretation: 1=Error, 2=Warning, 3+=Info
	tests := []struct {
		priority int
		expected string
	}{
		{1, "Error"},
		{2, "Warning"},
		{3, "Info"},
		{4, "Info"},
	}

	for _, tt := range tests {
		var got string
		switch tt.priority {
		case 1:
			got = "Error"
		case 2:
			got = "Warning"
		default:
			got = "Info"
		}
		if got != tt.expected {
			t.Errorf("priority %d: expected %s, got %s", tt.priority, tt.expected, got)
		}
	}
}

// The one-step answer of 7.40 SP06: verdicts inline, an info when the asked
// variant does not exist.
const atcSinglePassResult = `<?xml version="1.0" encoding="utf-8"?><atc:result verdicts="2" adtcore:uri="/sap/bc/adt/atc/runs/0050ABCDEF" xmlns:atc="http://www.sap.com/adt/atc" xmlns:adtcore="http://www.sap.com/adt/core"><verdicts>` +
	`<verdict adtcore:uri="/sap/bc/adt/atc/runs/0050ABCDEF/verdicts/1" priority="3" checkTitle="Extended Program Check (SLIN)" messageTitle="Security check without license" exemptionType="no"><objectAttributes author="TESTUSER" adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo_settings#start=1,0" adtcore:type="CLAS" adtcore:name="ZCL_DEMO_SETTINGS" adtcore:packageName="$ZDEMO"/><atom:link href="/sap/bc/adt/documentation/atc/documents/0050ABCDEF/verdict/1" rel="http://www.sap.com/adt/relations/documentation" type="text/html" xmlns:atom="http://www.w3.org/2005/Atom"/></verdict>` +
	`<verdict adtcore:uri="/sap/bc/adt/atc/runs/0050ABCDEF/verdicts/2" priority="1" checkTitle="Critical Statements" messageTitle="EXEC SQL" exemptionType="db"><objectAttributes author="TESTUSER" adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo_settings#start=42,4" adtcore:type="CLAS" adtcore:name="ZCL_DEMO_SETTINGS" adtcore:packageName="$ZDEMO"/></verdict>` +
	`</verdicts><infos><info description="Check variant NOPE does not exist. The DEFAULT variant was used instead of it." type="info"/></infos></atc:result>`

func TestParseATCResult(t *testing.T) {
	wl, err := parseATCResult([]byte(atcSinglePassResult))
	if err != nil {
		t.Fatalf("parseATCResult: %v", err)
	}
	if wl.ID != "0050ABCDEF" {
		t.Errorf("ID = %q", wl.ID)
	}
	if len(wl.Objects) != 1 {
		t.Fatalf("want the verdicts of one object grouped, got %d objects", len(wl.Objects))
	}
	obj := wl.Objects[0]
	if obj.URI != "/sap/bc/adt/oo/classes/zcl_demo_settings" || obj.Name != "ZCL_DEMO_SETTINGS" || obj.Type != "CLAS" || obj.PackageName != "$ZDEMO" {
		t.Errorf("object = %+v", obj)
	}
	if len(obj.Findings) != 2 {
		t.Fatalf("findings = %d", len(obj.Findings))
	}
	f := obj.Findings[1]
	if f.Priority != 1 || f.CheckTitle != "Critical Statements" || f.MessageTitle != "EXEC SQL" || f.Line != 42 || f.Column != 4 || f.ExemptionKind != "db" {
		t.Errorf("finding = %+v", f)
	}
	if obj.Findings[0].ExemptionKind != "" {
		t.Errorf(`exemptionType "no" must leave ExemptionKind empty, got %q`, obj.Findings[0].ExemptionKind)
	}
	if len(wl.Infos) != 1 || !strings.Contains(wl.Infos[0].Description, "NOPE does not exist") {
		t.Errorf("infos = %+v", wl.Infos)
	}
}

func TestParseATCResultWithoutVerdicts(t *testing.T) {
	wl, err := parseATCResult([]byte(`<?xml version="1.0" encoding="utf-8"?><atc:result verdicts="0" adtcore:uri="/sap/bc/adt/atc/runs/X1" xmlns:atc="http://www.sap.com/adt/atc" xmlns:adtcore="http://www.sap.com/adt/core"><verdicts/></atc:result>`))
	if err != nil {
		t.Fatalf("parseATCResult: %v", err)
	}
	if len(wl.Objects) != 0 || wl.Objects == nil {
		t.Errorf("want an empty, non-nil object list, got %#v", wl.Objects)
	}
}

// Without /atc/worklists in discovery the run is one POST to /atc/runs, the
// variant in the body; with it, the worklist protocol is used as before.
func TestRunATCCheckChoosesTheProtocolByDiscovery(t *testing.T) {
	const withoutWorklists = `<app:service xmlns:app="http://www.w3.org/2007/app"><app:workspace>
<app:collection href="/sap/bc/adt/atc/customizing"/><app:collection href="/sap/bc/adt/atc/runs"/>
</app:workspace></app:service>`
	const withWorklists = `<app:service xmlns:app="http://www.w3.org/2007/app"><app:workspace>
<app:collection href="/sap/bc/adt/atc/runs"/><app:collection href="/sap/bc/adt/atc/worklists"/>
</app:workspace></app:service>`

	for _, tc := range []struct {
		name, discovery string
		discoveryStatus int
		wantSinglePass  bool
	}{
		{"7.40 SP06, no worklists", withoutWorklists, http.StatusOK, true},
		{"worklists advertised", withWorklists, http.StatusOK, false},
		{"discovery unreadable", "", http.StatusInternalServerError, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var seen []string
			var runBody, runQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("x-csrf-token", "test-token")
				if r.Method == http.MethodHead {
					return
				}
				b, _ := io.ReadAll(r.Body)
				mu.Lock()
				seen = append(seen, r.Method+" "+r.URL.Path)
				mu.Unlock()
				switch {
				case r.URL.Path == "/sap/bc/adt/discovery":
					w.WriteHeader(tc.discoveryStatus)
					w.Write([]byte(tc.discovery))
				case r.URL.Path == "/sap/bc/adt/atc/runs" && r.URL.Query().Get("worklistId") == "":
					mu.Lock()
					runBody, runQuery = string(b), r.URL.RawQuery
					mu.Unlock()
					w.Write([]byte(atcSinglePassResult))
				case r.URL.Path == "/sap/bc/adt/atc/worklists" && r.Method == http.MethodPost:
					w.Write([]byte("WL1"))
				case r.URL.Path == "/sap/bc/adt/atc/runs":
					w.Write([]byte(`<atcworklist:worklistRun xmlns:atcworklist="http://www.sap.com/adt/atc/worklist"><atcworklist:worklistId>WL1</atcworklist:worklistId></atcworklist:worklistRun>`))
				case r.URL.Path == "/sap/bc/adt/atc/worklists/WL1":
					w.Write([]byte(`<atcworklist:worklist atcworklist:id="WL1" xmlns:atcworklist="http://www.sap.com/adt/atc/worklist"><atcworklist:objects/></atcworklist:worklist>`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			c := NewClient(srv.URL, "user", "pass")

			wl, err := c.RunATCCheck(context.Background(), "/sap/bc/adt/oo/classes/zcl_demo_settings", "SECURITY_CHECK_ALL", 50)
			if err != nil {
				t.Fatalf("RunATCCheck: %v (requests %v)", err, seen)
			}
			mu.Lock()
			defer mu.Unlock()
			usedWorklists := false
			for _, s := range seen {
				if strings.Contains(s, "/atc/worklists") {
					usedWorklists = true
				}
			}
			if tc.wantSinglePass {
				if usedWorklists {
					t.Errorf("one-step run must not touch worklists: %v", seen)
				}
				if !strings.Contains(runBody, `checkVariant="SECURITY_CHECK_ALL"`) || !strings.Contains(runBody, `maximumVerdicts="50"`) {
					t.Errorf("variant and limit belong in the body: %s", runBody)
				}
				if strings.Contains(runQuery, "checkVariant") || strings.Contains(runQuery, "worklistId") {
					t.Errorf("query = %q: the system does not read it", runQuery)
				}
				if len(wl.Objects) != 1 || len(wl.Infos) != 1 {
					t.Errorf("result not carried over: %+v", wl)
				}
			} else if !usedWorklists {
				t.Errorf("want the worklist protocol: %v", seen)
			}
		})
	}
}

// 7.50 answers a worklist run with the info as child elements.
func TestParseATCRunResultWithInfoElements(t *testing.T) {
	result, err := parseATCRunResult([]byte(`<?xml version="1.0" encoding="utf-8"?><atcworklist:worklistRun xmlns:atcworklist="http://www.sap.com/adt/atc/worklist"><atcworklist:worklistId>WL2</atcworklist:worklistId><atcworklist:infos><atcinfo:info xmlns:atcinfo="http://www.sap.com/adt/atc/info"><atcinfo:type>FINDING_STATS</atcinfo:type><atcinfo:description>0,2,0</atcinfo:description></atcinfo:info></atcworklist:infos></atcworklist:worklistRun>`))
	if err != nil {
		t.Fatalf("parseATCRunResult: %v", err)
	}
	if len(result.Infos) != 1 || result.Infos[0].Type != "FINDING_STATS" || result.Infos[0].Description != "0,2,0" {
		t.Errorf("infos = %+v", result.Infos)
	}
}

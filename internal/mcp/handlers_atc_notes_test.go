package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// On 7.40 a check variant that does not exist is replaced by DEFAULT and the
// system says so only in an info of the run. The answer carries it in the
// summary, beside the counts, where it is read with them.
func TestRunATCCheckSummaryNamesAReplacedVariant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-CSRF-Token", "t")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/atc/runs"):
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><atc:result verdicts="1" adtcore:uri="/sap/bc/adt/atc/runs/0050ABCDEF" xmlns:atc="http://www.sap.com/adt/atc" xmlns:adtcore="http://www.sap.com/adt/core"><verdicts>` +
				`<verdict adtcore:uri="/sap/bc/adt/atc/runs/0050ABCDEF/verdicts/1" priority="3" checkTitle="Extended Program Check (SLIN)" messageTitle="Security check without license" exemptionType="no"><objectAttributes author="TESTUSER" adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo_settings#start=1,0" adtcore:type="CLAS" adtcore:name="ZCL_DEMO_SETTINGS" adtcore:packageName="$ZDEMO"/></verdict>` +
				`</verdicts><infos><info description="Check variant NOPE does not exist. The DEFAULT variant was used instead of it." type="info"/></infos></atc:result>`))
		case strings.Contains(r.URL.Path, "/discovery"):
			_, _ = w.Write([]byte(`<app:service xmlns:app="http://www.w3.org/2007/app"><app:workspace>` +
				`<app:collection href="/sap/bc/adt/atc/customizing"/><app:collection href="/sap/bc/adt/atc/runs"/></app:workspace></app:service>`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()
	s := &Server{config: &Config{}, adtClient: adt.NewClient(srv.URL, "u", "p")}

	res, err := s.handleRunATCCheck(context.Background(), newRequest(map[string]any{
		"object_url": "/sap/bc/adt/oo/classes/zcl_demo_settings", "variant": "NOPE",
	}))
	if err != nil || res.IsError {
		t.Fatalf("RunATCCheck: %v %s", err, resultText(res))
	}
	var out struct {
		Summary struct {
			TotalFindings int      `json:"totalFindings"`
			Notes         []string `json:"notes"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatalf("answer: %v\n%s", err, resultText(res))
	}
	if out.Summary.TotalFindings != 1 || len(out.Summary.Notes) != 1 || !strings.Contains(out.Summary.Notes[0], "DEFAULT variant was used") {
		t.Errorf("summary = %+v", out.Summary)
	}
}

package adt

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// Shapes read off 7.57 (domains, v2) and 7.50 (data elements, v1); the
// documents are the same element for element.
const testDomainXML = `<?xml version="1.0" encoding="utf-8"?><doma:domain adtcore:responsible="TESTUSER" adtcore:masterLanguage="RU" adtcore:name="ZDEMO_KIND" adtcore:type="DOMA/DD" adtcore:version="active" adtcore:description="Demo kind" adtcore:language="RU" xmlns:doma="http://www.sap.com/dictionary/domain" xmlns:adtcore="http://www.sap.com/adt/core">` +
	`<atom:link href="versions" rel="http://www.sap.com/adt/relations/versions" xmlns:atom="http://www.w3.org/2005/Atom"/>` +
	`<adtcore:packageRef adtcore:uri="/sap/bc/adt/packages/%24tmp" adtcore:type="DEVC/K" adtcore:name="$TMP"/>` +
	`<doma:content><doma:typeInformation><doma:datatype>CHAR</doma:datatype><doma:length>000002</doma:length><doma:decimals>000000</doma:decimals></doma:typeInformation>` +
	`<doma:outputInformation><doma:length>000002</doma:length><doma:style>00</doma:style><doma:conversionExit/><doma:signExists>false</doma:signExists><doma:lowercase>false</doma:lowercase><doma:ampmFormat>false</doma:ampmFormat></doma:outputInformation>` +
	`<doma:valueInformation><doma:valueTableRef/><doma:appendExists>false</doma:appendExists><doma:fixValues>` +
	`<doma:fixValue><doma:position>0001</doma:position><doma:low>01</doma:low><doma:high/><doma:text>First</doma:text></doma:fixValue>` +
	`<doma:fixValue><doma:position>0002</doma:position><doma:low>10</doma:low><doma:high>15</doma:high><doma:text>Ten to fifteen</doma:text></doma:fixValue>` +
	`<doma:fixValue><doma:position>0001</doma:position><doma:low>99</doma:low><doma:high/><doma:text>From an append</doma:text><doma:contributingAppendRef adtcore:uri="/sap/bc/adt/ddic/domains/zdemo_kind_app" adtcore:type="DOMA/DD" adtcore:name="ZDEMO_KIND_APP"/></doma:fixValue>` +
	`</doma:fixValues></doma:valueInformation></doma:content></doma:domain>`

const testDataElementXML = `<?xml version="1.0" encoding="utf-8"?><blue:wbobj adtcore:responsible="TESTUSER" adtcore:masterLanguage="RU" adtcore:name="ZDEMO_KIND" adtcore:type="DTEL/DE" adtcore:version="active" adtcore:description="Demo kind" adtcore:language="RU" xmlns:blue="http://www.sap.com/wbobj/dictionary/dtel" xmlns:adtcore="http://www.sap.com/adt/core">` +
	`<adtcore:packageRef adtcore:uri="/sap/bc/adt/vit/wb/object_type/devck/object_name/ZDEMO" adtcore:type="DEVC/K" adtcore:name="ZDEMO"/>` +
	`<dtel:dataElement xmlns:dtel="http://www.sap.com/adt/dictionary/dataelements"><dtel:typeKind>domain</dtel:typeKind><dtel:typeName>ZDEMO_KIND</dtel:typeName><dtel:dataType>CHAR</dtel:dataType><dtel:dataTypeLength>000002</dtel:dataTypeLength><dtel:dataTypeDecimals>000000</dtel:dataTypeDecimals>` +
	`<dtel:shortFieldLabel>Kind</dtel:shortFieldLabel><dtel:shortFieldLength>10</dtel:shortFieldLength><dtel:shortFieldMaxLength>10</dtel:shortFieldMaxLength>` +
	`<dtel:mediumFieldLabel>Demo kind</dtel:mediumFieldLabel><dtel:mediumFieldLength>20</dtel:mediumFieldLength><dtel:mediumFieldMaxLength>20</dtel:mediumFieldMaxLength>` +
	`<dtel:longFieldLabel>Demo kind</dtel:longFieldLabel><dtel:longFieldLength>40</dtel:longFieldLength><dtel:longFieldMaxLength>40</dtel:longFieldMaxLength>` +
	`<dtel:headingFieldLabel>Kind</dtel:headingFieldLabel><dtel:headingFieldLength>04</dtel:headingFieldLength><dtel:headingFieldMaxLength>55</dtel:headingFieldMaxLength>` +
	`<dtel:searchHelp/><dtel:searchHelpParameter/><dtel:setGetParameter>ZKD</dtel:setGetParameter><dtel:defaultComponentName/>` +
	`<dtel:deactivateInputHistory>false</dtel:deactivateInputHistory><dtel:changeDocument>true</dtel:changeDocument><dtel:leftToRightDirection>false</dtel:leftToRightDirection><dtel:deactivateBIDIFiltering>false</dtel:deactivateBIDIFiltering>` +
	`</dtel:dataElement></blue:wbobj>`

func TestParseDomainLeavesAppendValuesOut(t *testing.T) {
	d, err := parseDomain([]byte(testDomainXML))
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "ZDEMO_KIND" || d.Package != "$TMP" || d.DataType != "CHAR" || d.Length != 2 || d.OutputLength != 2 || d.OutputStyle != "00" {
		t.Errorf("domain = %+v", d)
	}
	want := []DomainFixValue{{1, "01", "", "First"}, {2, "10", "15", "Ten to fifteen"}}
	if len(d.FixValues) != len(want) {
		t.Fatalf("fixed values = %+v, want %+v (the append's value is the append's)", d.FixValues, want)
	}
	for i := range want {
		if d.FixValues[i] != want[i] {
			t.Errorf("fixed value %d = %+v, want %+v", i, d.FixValues[i], want[i])
		}
	}
}

// The body is read back by the same parser: what is written is what is read.
func TestDomainBodyRoundTrips(t *testing.T) {
	d, err := parseDomain([]byte(testDomainXML))
	if err != nil {
		t.Fatal(err)
	}
	d.Description = "Demo <kind> & more"
	d.ValueTable = "zdemo_t"
	d.FixValues = append(d.FixValues, DomainFixValue{Low: "20", Text: "Twenty"})
	body := string(domainBody(d, "ru"))
	for _, want := range []string{
		`adtcore:name="ZDEMO_KIND"`, `adtcore:type="DOMA/DD"`, `adtcore:language="RU"`, `adtcore:masterLanguage="RU"`,
		`adtcore:description="Demo &lt;kind&gt; &amp; more"`, `<adtcore:packageRef adtcore:name="$TMP"/>`,
		`<doma:length>000002</doma:length>`, `<doma:conversionExit/>`,
		`<doma:valueTableRef adtcore:uri="/sap/bc/adt/ddic/tables/zdemo_t" adtcore:type="TABL/DT" adtcore:name="ZDEMO_T"/>`,
		`<doma:position>0003</doma:position><doma:low>20</doma:low><doma:high/><doma:text>Twenty</doma:text>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s:\n%s", want, body)
		}
	}
	again, err := parseDomain([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if again.Description != d.Description || again.ValueTable != "ZDEMO_T" || len(again.FixValues) != 3 || again.FixValues[2].Position != 3 {
		t.Errorf("read back %+v", again)
	}
}

func TestDataElementBodyRoundTrips(t *testing.T) {
	e, err := parseDataElement([]byte(testDataElementXML))
	if err != nil {
		t.Fatal(err)
	}
	if e.TypeKind != TypeKindDomain || e.TypeName != "ZDEMO_KIND" || e.HeadingLength != 4 || e.SetGetParameter != "ZKD" || !e.ChangeDocument || e.Package != "ZDEMO" {
		t.Errorf("data element = %+v", e)
	}
	e.ShortLabel = "Art"
	body := string(dataElementBody(e, "RU"))
	for _, want := range []string{
		`<blue:wbobj xmlns:blue="http://www.sap.com/wbobj/dictionary/dtel"`, `adtcore:type="DTEL/DE"`,
		`<dtel:dataElement xmlns:dtel="http://www.sap.com/adt/dictionary/dataelements">`,
		`<dtel:typeKind>domain</dtel:typeKind><dtel:typeName>ZDEMO_KIND</dtel:typeName>`,
		`<dtel:shortFieldLabel>Art</dtel:shortFieldLabel><dtel:shortFieldLength>10</dtel:shortFieldLength><dtel:shortFieldMaxLength>10</dtel:shortFieldMaxLength>`,
		`<dtel:headingFieldLength>04</dtel:headingFieldLength>`, `<dtel:searchHelp/>`, `<dtel:changeDocument>true</dtel:changeDocument>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s:\n%s", want, body)
		}
	}
	again, err := parseDataElement([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	e.contentType, e.Version = "", "" // the PUT names no version
	if *again != *e {
		t.Errorf("read back\n%+v\nwant\n%+v", again, e)
	}
}

func TestDataElementCheck(t *testing.T) {
	for _, tc := range []struct {
		e    DataElement
		fail bool
	}{
		{DataElement{TypeKind: TypeKindDomain, TypeName: "ZDEMO_KIND"}, false},
		{DataElement{TypeKind: TypeKindDomain}, true},
		{DataElement{TypeKind: TypeKindPredefined, DataType: "CHAR", Length: 10}, false},
		{DataElement{TypeKind: TypeKindPredefined}, true},
		{DataElement{TypeKind: "char"}, true},
		{DataElement{TypeKind: TypeKindRefToClassOrIntf, TypeName: "ZCL_DEMO"}, false},
		{DataElement{TypeKind: TypeKindDomain, TypeName: "X", ShortLabel: "Eleven char"}, true},
		{DataElement{TypeKind: TypeKindDomain, TypeName: "X", HeadingLength: 56}, true},
	} {
		if err := tc.e.Check(); (err != nil) != tc.fail {
			t.Errorf("%+v: Check() = %v, want failure %v", tc.e, err, tc.fail)
		}
	}
}

// An update is LOCK, a read under the lock, a PUT of the whole document in
// the version the system served, UNLOCK, then the activation — the PUT
// inside the lock's stateful window (#91).
func TestUpdateDataElementWritesTheWholeDocumentInTheServedVersion(t *testing.T) {
	rec := &adtRecorder{}
	var mu sync.Mutex
	var put, putType string
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = io.WriteString(w, testLockXML)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/ddic/dataelements/"):
			w.Header().Set("Content-Type", "application/vnd.sap.adt.dataelements.v1+xml; charset=utf-8")
			_, _ = io.WriteString(w, testDataElementXML)
		case r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			put, putType = string(b), r.Header.Get("Content-Type")
			mu.Unlock()
		default:
			w.WriteHeader(http.StatusOK)
		}
	}, WithLanguage("RU"))

	w, err := client.UpdateDataElement(context.Background(), "zdemo_kind", "", func(e *DataElement) error {
		e.MediumLabel = "Kind of demo"
		e.MediumLength = 0
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateDataElement: %v", err)
	}
	if w.Activation == nil || !w.Activation.Success {
		t.Errorf("write = %+v", w)
	}
	if putType != "application/vnd.sap.adt.dataelements.v1+xml" {
		t.Errorf("PUT Content-Type %q, want the served v1", putType)
	}
	for _, want := range []string{
		`<dtel:mediumFieldLabel>Kind of demo</dtel:mediumFieldLabel><dtel:mediumFieldLength>20</dtel:mediumFieldLength>`,
		`<dtel:typeName>ZDEMO_KIND</dtel:typeName>`, `<dtel:setGetParameter>ZKD</dtel:setGetParameter>`,
		`<adtcore:packageRef adtcore:name="ZDEMO"/>`,
	} {
		if !strings.Contains(put, want) {
			t.Errorf("PUT lacks %s:\n%s", want, put)
		}
	}

	calls := rec.snapshot()
	lockAt := indexOfCall(calls, isLock)
	putAt := indexOfCall(calls, func(c wireCall) bool { return c.method == http.MethodPut })
	unlockAt := indexOfCall(calls, isUnlock)
	activateAt := indexOfCall(calls, func(c wireCall) bool { return strings.Contains(c.path, "/activation") })
	if lockAt < 0 || putAt < lockAt || unlockAt < putAt || activateAt < unlockAt {
		t.Fatalf("want LOCK, PUT, UNLOCK, activation in that order: lock %d put %d unlock %d activate %d", lockAt, putAt, unlockAt, activateAt)
	}
	assertWindowStateful(t, calls, lockAt, putAt+1)
}

func TestUpdateDomainRefusesBeforeLockingWhenTheEditIsWrong(t *testing.T) {
	rec := &adtRecorder{}
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = io.WriteString(w, testLockXML)
		case r.Method == http.MethodGet:
			_, _ = io.WriteString(w, testDomainXML)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}, WithLanguage("RU"))
	_, err := client.UpdateDomain(context.Background(), "ZDEMO_KIND", "", func(d *Domain) error {
		d.FixValues = append(d.FixValues, DomainFixValue{Low: "ELEVEN_CHAR"})
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "longer than 10") {
		t.Fatalf("err = %v", err)
	}
	for _, c := range rec.snapshot() {
		if c.method == http.MethodPut {
			t.Errorf("a PUT went out: %+v", c)
		}
	}
	if indexOfCall(rec.snapshot(), isUnlock) < 0 {
		t.Errorf("the lock was not released")
	}
}

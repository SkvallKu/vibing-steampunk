package adt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// The minimal working status of vspext-gui-status: BACK/CANCEL/EXIT on
// F3/F12/F15, codes linked through ADM, STA and PFK.
const cuaMain = `<CUA>
 <ADM><ACTCODE>000001</ACTCODE><PFKCODE>000001</PFKCODE></ADM>
 <STA><RSMPE_STAT><CODE>MAIN</CODE><MODAL>D</MODAL><ACTCODE>000001</ACTCODE><PFKCODE>000001</PFKCODE><BUTCODE>0001</BUTCODE><INT_NOTE>Материалы</INT_NOTE></RSMPE_STAT></STA>
 <FUN>
  <RSMPE_FUNT><CODE>BACK</CODE><TEXTNO>001</TEXTNO><TEXT_TYPE>S</TEXT_TYPE><FUN_TEXT>Назад</FUN_TEXT></RSMPE_FUNT>
  <RSMPE_FUNT><CODE>CANCEL</CODE><TEXTNO>001</TEXTNO><TEXT_TYPE>S</TEXT_TYPE><TEXT_NAME>ICON_CANCEL</TEXT_NAME><ICON_ID>@0W@</ICON_ID><FUN_TEXT>Отменить</FUN_TEXT></RSMPE_FUNT>
  <RSMPE_FUNT><CODE>EXIT</CODE><TEXTNO>001</TEXTNO><TEXT_TYPE>S</TEXT_TYPE><FUN_TEXT>Выход</FUN_TEXT></RSMPE_FUNT>
 </FUN>
 <PFK>
  <RSMPE_PFK><CODE>000001</CODE><PFNO>03</PFNO><FUNCODE>BACK</FUNCODE><FUNNO>001</FUNNO></RSMPE_PFK>
  <RSMPE_PFK><CODE>000001</CODE><PFNO>12</PFNO><FUNCODE>CANCEL</FUNCODE><FUNNO>001</FUNNO></RSMPE_PFK>
  <RSMPE_PFK><CODE>000001</CODE><PFNO>15</PFNO><FUNCODE>EXIT</FUNCODE><FUNNO>001</FUNNO></RSMPE_PFK>
 </PFK>
 <SET>
  <RSMPE_STAF><STATUS>MAIN</STATUS><FUNCTION>BACK</FUNCTION></RSMPE_STAF>
  <RSMPE_STAF><STATUS>MAIN</STATUS><FUNCTION>CANCEL</FUNCTION></RSMPE_STAF>
  <RSMPE_STAF><STATUS>MAIN</STATUS><FUNCTION>EXIT</FUNCTION></RSMPE_STAF>
 </SET>
 <TIT><RSMPE_TITT><CODE>MAIN</CODE><TEXT>Материалы</TEXT></RSMPE_TITT></TIT>
</CUA>`

// A whole abapGit .prog.xml: the CUA node is taken out of it.
const cuaProgXML = `<?xml version="1.0" encoding="utf-8"?>
<abapGit version="v1.0.0" serializer="LCL_OBJECT_PROG" serializer_version="v1.0.0">
 <asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
   <PROGDIR><NAME>ZDEMO_CUA</NAME><SUBC>1</SUBC></PROGDIR>
   ` + cuaMain + `
   <TPOOL><item><ID>R</ID><ENTRY>Demo</ENTRY></item></TPOOL>
  </asx:values>
 </asx:abap>
</abapGit>`

func TestCUADocumentTakesTheCUANode(t *testing.T) {
	got, err := CUADocument(cuaProgXML)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "PROGDIR") || strings.Contains(got, "TPOOL") || !strings.Contains(got, "<asx:values><CUA>") {
		t.Errorf("document %q", got)
	}
	if _, err := CUADocument("<PROGDIR/>"); err == nil {
		t.Error("a document without CUA: no error")
	}
	if got, err := CUADocument("<CUA/>"); err != nil || !strings.Contains(got, "<CUA/>") {
		t.Errorf("empty CUA: %q, %v", got, err)
	}
}

func TestParseCUA(t *testing.T) {
	doc, err := parseCUA(cuaProgXML)
	if err != nil {
		t.Fatal(err)
	}
	if doc.adm["PFKCODE"] != "000001" || len(doc.tables["FUN"]) != 3 || len(doc.tables["PFK"]) != 3 {
		t.Fatalf("adm %v, tables %v", doc.adm, doc.tables)
	}
	if got := doc.tables["FUN"][1]["ICON_ID"]; got != "@0W@" {
		t.Errorf("FUN[1].ICON_ID %q", got)
	}
	if got := doc.codes("STA", "CODE"); !reflect.DeepEqual(got, []string{"MAIN"}) {
		t.Errorf("statuses %v", got)
	}
	if w := doc.check(); len(w) != 0 {
		t.Errorf("a working status warns: %v", w)
	}
}

// The mistake vspext-gui-status describes: a key number for the key set
// code, no ADM. SAP writes and activates it, and the status is empty.
func TestCUACheckFindsBrokenLinks(t *testing.T) {
	broken := `<CUA>
 <STA><RSMPE_STAT><CODE>MAIN</CODE><PFKCODE>MAIN</PFKCODE></RSMPE_STAT><RSMPE_STAT><CODE>GREY</CODE></RSMPE_STAT></STA>
 <FUN><RSMPE_FUNT><CODE>BACK</CODE></RSMPE_FUNT></FUN>
 <PFK><RSMPE_PFK><CODE>03</CODE><PFNO>03</PFNO><FUNCODE>BAKC</FUNCODE></RSMPE_PFK></PFK>
 <SET><RSMPE_STAF><STATUS>MAIN</STATUS><FUNCTION>BACK</FUNCTION></RSMPE_STAF><RSMPE_STAF><STATUS>OTHER</STATUS><FUNCTION>EXIT</FUNCTION></RSMPE_STAF></SET>
</CUA>`
	doc, err := parseCUA(broken)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(doc.check(), "\n")
	for _, want := range []string{
		"function BAKC is not in FUN",
		"status MAIN: key set PFKCODE MAIN has no rows in PFK",
		"status GREY: no function is active",
		"ADM has no PFKCODE",
		"SET: status OTHER is not in STA",
		"SET OTHER: function EXIT is not in FUN",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func cuaServer(t *testing.T, features string, calls *[]string, put, activation *string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		mu.Lock()
		defer mu.Unlock()
		switch p := r.URL.Path; {
		case p == "/sap/bc/adt/discovery":
			w.Write([]byte(`<app:collection href="/sap/bc/adt/zvsp/info"/><app:collection href="/sap/bc/adt/zvsp/cua/programs"/>`))
		case p == "/sap/bc/adt/zvsp/info":
			w.Write([]byte("version=0.9.0\nfeatures=" + features + "\n"))
		case p == "/sap/bc/adt/zvsp/cua/programs/ZDEMO_CUA":
			action := r.URL.Query().Get("_action")
			*calls = append(*calls, r.Method+" "+action)
			switch {
			case action == "LOCK":
				w.Write([]byte(`<asx:abap><asx:values><DATA><LOCK_HANDLE>HANDLE-1</LOCK_HANDLE><IS_LOCAL>X</IS_LOCAL></DATA></asx:values></asx:abap>`))
			case r.Method == http.MethodPut:
				if r.URL.Query().Get("lockHandle") != "HANDLE-1" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				b, _ := io.ReadAll(r.Body)
				*put = string(b)
				w.Write(b)
			case r.Method == http.MethodGet:
				w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values>` + cuaMain + `</asx:values></asx:abap>`))
			}
		case p == "/sap/bc/adt/activation":
			b, _ := io.ReadAll(r.Body)
			*activation = string(b)
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><chkl:messages xmlns:chkl="http://www.sap.com/abapxml/checklist"/>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestWriteProgramCUA(t *testing.T) {
	var calls []string
	var put, activation string
	srv := cuaServer(t, "/textelements,/cua", &calls, &put, &activation)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got, err := c.WriteProgramCUA(context.Background(), "zdemo_cua", cuaProgXML, "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Activated || len(got.Warnings) != 0 || !reflect.DeepEqual(got.Statuses, []string{"MAIN"}) || !reflect.DeepEqual(got.Titles, []string{"MAIN"}) {
		t.Errorf("result %+v", got)
	}
	if want := []string{"POST LOCK", "PUT ", "POST UNLOCK"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
	if strings.Contains(put, "PROGDIR") || !strings.Contains(put, "<asx:values><CUA>") {
		t.Errorf("PUT body %q", put)
	}
	if !strings.Contains(activation, `adtcore:uri="/sap/bc/adt/zvsp/cua/programs/ZDEMO_CUA"`) {
		t.Errorf("activation %q", activation)
	}
}

func TestProgramCUANeedsTheSet(t *testing.T) {
	var calls []string
	var put, activation string
	srv := cuaServer(t, "/textelements", &calls, &put, &activation)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	if _, err := c.GetProgramCUA(context.Background(), "ZDEMO_CUA", ""); err == nil || !strings.Contains(err.Error(), "0.9.0") {
		t.Errorf("read: %v", err)
	}
	if _, err := c.WriteProgramCUA(context.Background(), "ZDEMO_CUA", cuaMain, ""); err == nil || !strings.Contains(err.Error(), "0.9.0") {
		t.Errorf("write: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("calls %v", calls)
	}
}

func TestGetProgramCUA(t *testing.T) {
	var calls []string
	var put, activation string
	srv := cuaServer(t, "/cua", &calls, &put, &activation)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got, err := c.GetProgramCUA(context.Background(), "ZDEMO_CUA", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Statuses, []string{"MAIN"}) || !strings.Contains(got.Document, "ICON_CANCEL") {
		t.Errorf("got %+v", got)
	}
	if _, err := c.GetProgramCUA(context.Background(), "ZDEMO_CUA", "newest"); err == nil {
		t.Error("a bad version: no error")
	}
}

func TestWriteProgramCUARefusesAnEmptyCUA(t *testing.T) {
	c := NewClient("http://unused.invalid", "user", "pass")
	if _, err := c.WriteProgramCUA(context.Background(), "ZDEMO_CUA", "<CUA/>", ""); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("err %v", err)
	}
}

// SAP drops a field it does not know; the answer to the PUT shows it.
func TestCUANotStoredAsSent(t *testing.T) {
	sent, _ := parseCUA(`<CUA><ADM><PFKCODE>000001</PFKCODE></ADM><TIT><RSMPE_TITT><CODE>MAIN</CODE><TEXT>Demo</TEXT></RSMPE_TITT>` +
		`<RSMPE_TITT><CODE>SECOND</CODE><TXET>Typo</TXET></RSMPE_TITT></TIT><FUN><RSMPE_FUNT><CODE>BACK</CODE></RSMPE_FUNT></FUN></CUA>`)
	stored, _ := parseCUA(`<CUA><ADM><PFKCODE>000001</PFKCODE></ADM><TIT><RSMPE_TITT><CODE>SECOND</CODE></RSMPE_TITT>` +
		`<RSMPE_TITT><CODE>MAIN</CODE><TEXT>Demo</TEXT></RSMPE_TITT></TIT></CUA>`)
	got := sent.notStoredIn(stored)
	want := []string{
		"FUN row 1 not stored as sent (unknown field or changed value): CODE=BACK",
		"TIT row 2 not stored as sent (unknown field or changed value): TXET=Typo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := sent.notStoredIn(sent); len(got) != 0 {
		t.Errorf("the same document: %v", got)
	}
}

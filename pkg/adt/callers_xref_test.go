package adt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// xrefServer stands in for a 7.40 system: the where-used list answers
// usageStatus, and free SQL answers whichever table the statement names from
// tables. A table missing from tables is refused with 403.
type xrefServer struct {
	usageStatus int
	usageBody   string // default: the status text
	tables      map[string]string

	mu      sync.Mutex
	queries []string
}

func (x *xrefServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.Method == http.MethodHead || r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "usageReferences") {
			w.WriteHeader(x.usageStatus)
			body := x.usageBody
			if body == "" {
				body = "usage references: " + http.StatusText(x.usageStatus)
			}
			w.Write([]byte(body))
			return
		}
		body, _ := io.ReadAll(r.Body)
		sql := string(body)
		x.mu.Lock()
		x.queries = append(x.queries, sql)
		x.mu.Unlock()
		for table, answer := range x.tables {
			if strings.Contains(sql, " FROM "+table+" ") {
				w.Header().Set("Content-Type", "application/xml")
				w.Write([]byte(answer))
				return
			}
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("not authorised"))
	}))
}

func (x *xrefServer) asked(table string) []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	var out []string
	for _, q := range x.queries {
		if strings.Contains(q, " FROM "+table+" ") {
			out = append(out, q)
		}
	}
	return out
}

func callerByName(callers []ExposedCaller, name string) *ExposedCaller {
	for i := range callers {
		if callers[i].Name == name {
			return &callers[i]
		}
	}
	return nil
}

// The case the fallback exists for: a table's users on a system with no
// where-used list, each include mapped to the object anybody would open.
func TestWhereUsedFallsBackToCrossReferenceOn404(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusNotFound, tables: map[string]string{
		"WBCROSSGT": tableXML(
			col("INCLUDE",
				"ZCL_ORDER=====================CM001",
				"ZCL_ORDER=====================CM002",
				"ZCL_ORDER=====================CCAU",
				"LZSALESU03",
				"LZSALESF01",
				"ZREPORT",
				"ZREPORT_TOP",
				"ZMARAX_VIEW",
			),
			col("NAME",
				"ZMARA",
				"ZMARA\\TY:MATNR",
				"ZMARA",
				"ZMARA",
				"ZMARA",
				"ZMARA\\TY:ERSDA",
				"ZMARA",
				// ZMARA\% matches this through the underscore-free
				// LIKE only because of the backslash; ZMARAX is another
				// table and must not be counted.
				"ZMARAX",
			),
		),
		"TFDIR": tableXML(
			col("FUNCNAME", "Z_SALES_READ"),
			col("PNAME", "SAPLZSALES"),
			col("INCLUDE", "3"),
		),
		"TRDIR": tableXML(
			col("NAME", "ZREPORT", "ZREPORT_TOP"),
			col("SUBC", "1", "I"),
		),
		"TADIR": tableXML(
			col("OBJECT", "CLAS", "FUGR", "PROG", "PROG"),
			col("OBJ_NAME", "ZCL_ORDER", "ZSALES", "ZREPORT", "ZREPORT_TOP"),
			col("DEVCLASS", "ZORDERS", "ZSALES", "ZTOOLS", "ZTOOLS"),
		),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	res, err := client.WhereUsedFull(context.Background(), "/sap/bc/adt/ddic/tables/zmara")
	if err != nil {
		t.Fatalf("a 404 from the where-used list should turn to the tables: %v", err)
	}
	if res.Source != WhereUsedSourceXref || res.Why != WhereUsedWhyNoList {
		t.Errorf("the answer must say it came from the tables, got source %q", res.Source)
	}
	if len(res.Unsearched) != 0 || res.Truncated != "" {
		t.Errorf("every table answered in full: unsearched=%v truncated=%q", res.Unsearched, res.Truncated)
	}

	want := map[string]struct{ typ, uri, pkg string }{
		"ZCL_ORDER":    {"CLAS/OC", "/sap/bc/adt/oo/classes/zcl_order", "ZORDERS"},
		"Z_SALES_READ": {"FUGR/FF", "/sap/bc/adt/functions/groups/zsales/fmodules/z_sales_read", "ZSALES"},
		"ZSALES":       {"FUGR/F", "/sap/bc/adt/functions/groups/zsales", "ZSALES"},
		"ZREPORT":      {"PROG/P", "/sap/bc/adt/programs/programs/zreport", "ZTOOLS"},
		"ZREPORT_TOP":  {"PROG/I", "/sap/bc/adt/programs/includes/zreport_top", "ZTOOLS"},
	}
	if len(res.Callers) != len(want) {
		t.Errorf("want one caller per object, %d of them, got %d: %+v", len(want), len(res.Callers), res.Callers)
	}
	for name, w := range want {
		c := callerByName(res.Callers, name)
		if c == nil {
			t.Errorf("%s uses the table and is missing", name)
			continue
		}
		if c.Type != w.typ || c.URI != w.uri || c.Package != w.pkg {
			t.Errorf("%s: got type %q uri %q package %q, want %q %q %q", name, c.Type, c.URI, c.Package, w.typ, w.uri, w.pkg)
		}
	}
	if c := callerByName(res.Callers, "ZMARAX_VIEW"); c != nil {
		t.Error("a reference to ZMARAX is not a reference to ZMARA")
	}
	if c := callerByName(res.Callers, "ZCL_ORDER"); c != nil {
		if !c.IsTest {
			// Three includes of one class collapse into one caller, and
			// the test include among them marks it.
			t.Error("ZCL_ORDER is used from its test include too")
		}
		if c.Component != "MATNR" {
			t.Errorf("the field used should be kept as the component, got %q", c.Component)
		}
	}
}

// Only a 404 means "not on this system". A refusal is the list answering, and
// quietly asking somewhere else would hide it.
func TestWhereUsedDoesNotFallBackOnOtherErrors(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusForbidden, tables: map[string]string{
		"WBCROSSGT": tableXML(col("INCLUDE", "ZREPORT"), col("NAME", "ZCL_ORDER")),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	if _, err := client.WhereUsedFull(context.Background(), "/sap/bc/adt/oo/classes/zcl_order"); err == nil {
		t.Fatal("a 403 from the where-used list must come back as an error")
	}
	if n := len(x.asked("WBCROSSGT")); n != 0 {
		t.Errorf("the tables were asked %d times after a 403", n)
	}
}

// The 500 ERH (7.50) answers for tables and function modules is the list's
// own failure and goes to the tables; a 500 with anything else in it does not.
func TestWhereUsedFallsBackOnTheReferencesConversion500Only(t *testing.T) {
	const conversion = `<?xml version="1.0" encoding="utf-8"?><exc:exception xmlns:exc="http://www.sap.com/abapxml/types/communicationframework">` +
		`<namespace id="com.sap.adt.ris"/><type id="ABAP References Resource Error"/>` +
		`<message lang="EN">Error while converting object references</message><properties/></exc:exception>`
	tables := map[string]string{
		"WBCROSSGT": tableXML(col("INCLUDE", "ZREPORT"), col("NAME", "MARA")),
		"TRDIR":     tableXML(col("NAME", "ZREPORT"), col("SUBC", "1")),
		"TADIR":     tableXML(col("OBJECT"), col("OBJ_NAME"), col("DEVCLASS")),
	}

	x := &xrefServer{usageStatus: http.StatusInternalServerError, usageBody: conversion, tables: tables}
	srv := x.start(t)
	res, err := NewClient(srv.URL, "user", "pass").WhereUsedFull(context.Background(), "/sap/bc/adt/ddic/tables/mara")
	srv.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != WhereUsedSourceXref || res.Why != WhereUsedWhyConversion || callerByName(res.Callers, "ZREPORT") == nil {
		t.Errorf("the conversion 500 should be answered from the tables and say so, got %+v", res)
	}

	x = &xrefServer{usageStatus: http.StatusInternalServerError, tables: tables}
	srv = x.start(t)
	defer srv.Close()
	if _, err := NewClient(srv.URL, "user", "pass").WhereUsedFull(context.Background(), "/sap/bc/adt/ddic/tables/mara"); err == nil {
		t.Fatal("any other 500 must come back as an error")
	}
	if n := len(x.asked("WBCROSSGT")); n != 0 {
		t.Errorf("the tables were asked %d times after a plain 500", n)
	}
}

// A class's own includes are not its users, and neither are those of a class
// whose name the LIKE pattern catches.
func TestCallersFromXrefDropsTheTargetsOwnIncludes(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusNotFound, tables: map[string]string{
		"WBCROSSGT": tableXML(
			col("INCLUDE",
				"ZCL_ORDER=====================CM001",
				"ZCL_ORDER=====================CU",
				"ZCL_ORDER_TEST================CM001",
			),
			col("NAME", "ZCL_ORDER\\ME:RUN", "ZCL_ORDER", "ZCL_ORDER\\ME:RUN"),
		),
		"TADIR": tableXML(col("OBJECT"), col("OBJ_NAME"), col("DEVCLASS")),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	res, err := client.CallersFromXref(context.Background(), "/sap/bc/adt/oo/classes/zcl_order")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Callers) != 1 || res.Callers[0].Name != "ZCL_ORDER_TEST" {
		t.Fatalf("only ZCL_ORDER_TEST uses ZCL_ORDER, got %+v", res.Callers)
	}
	if res.Callers[0].Component != "RUN" {
		t.Errorf("the method called should be the component, got %q", res.Callers[0].Component)
	}
}

// A function module's callers come from CROSS, and a call from its own
// include is recursion, not a caller.
func TestCallersFromXrefReadsFunctionModuleCallsFromCross(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusNotFound, tables: map[string]string{
		"CROSS": tableXML(col("INCLUDE", "ZREPORT", "LZSALESU03")),
		"TFDIR": tableXML(
			col("FUNCNAME", "Z_SALES_READ"),
			col("PNAME", "SAPLZSALES"),
			col("INCLUDE", "03"),
		),
		"TRDIR": tableXML(col("NAME", "ZREPORT"), col("SUBC", "1")),
		"TADIR": tableXML(col("OBJECT"), col("OBJ_NAME"), col("DEVCLASS")),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	res, err := client.CallersFromXref(context.Background(), "/sap/bc/adt/functions/groups/zsales/fmodules/z_sales_read")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Callers) != 1 || res.Callers[0].Name != "ZREPORT" {
		t.Fatalf("ZREPORT calls Z_SALES_READ and the module's own include does not count, got %+v", res.Callers)
	}
	cross := x.asked("CROSS")
	if len(cross) != 1 || !strings.Contains(cross[0], "TYPE = 'F'") || !strings.Contains(cross[0], "NAME = 'Z_SALES_READ'") {
		t.Errorf("CROSS should be asked for calls of this module, got %v", cross)
	}
}

// A program's users are SUBMIT and PERFORM ... IN PROGRAM, two questions; one
// failing makes the answer short, and it must say so.
func TestCallersFromXrefNamesTheQueryItLost(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusNotFound, tables: map[string]string{
		"TRDIR": tableXML(col("NAME", "ZCALLER"), col("SUBC", "1")),
		"TADIR": tableXML(col("OBJECT"), col("OBJ_NAME"), col("DEVCLASS")),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	_, err := client.CallersFromXref(context.Background(), "/sap/bc/adt/programs/programs/zreport")
	if err == nil {
		t.Fatal("CROSS refused every query; an empty list would read as \"nobody uses this\"")
	}
}

// More rows than a query returns is said, not left to be guessed from a
// suspiciously round count.
func TestCallersFromXrefSaysWhenTheRowLimitWasReached(t *testing.T) {
	includes := make([]string, xrefRowLimit)
	names := make([]string, xrefRowLimit)
	for i := range includes {
		includes[i] = "ZCL_ORDER=====================CM001"
		names[i] = "ZMARA"
	}
	x := &xrefServer{usageStatus: http.StatusNotFound, tables: map[string]string{
		"WBCROSSGT": tableXML(col("INCLUDE", includes...), col("NAME", names...)),
		"TADIR":     tableXML(col("OBJECT"), col("OBJ_NAME"), col("DEVCLASS")),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	res, err := client.CallersFromXref(context.Background(), "/sap/bc/adt/ddic/tables/zmara")
	if err != nil {
		t.Fatal(err)
	}
	if res.Truncated == "" {
		t.Error("the query came back at its limit and the answer does not say it may be partial")
	}
}

func TestFindReferencesExplainsA404(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusNotFound}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	_, err := client.FindReferences(context.Background(), "/sap/bc/adt/oo/classes/zcl_order", 10, 5)
	if err == nil || !strings.Contains(err.Error(), "GetCallersOf") {
		t.Fatalf("a 404 should point at what can still answer, got %v", err)
	}
	if !isNotFound(err) {
		t.Error("the wrapped error must still read as a 404 to WhereUsedFull")
	}
}

func TestCallerTargetFromURIKnowsDictionaryTypes(t *testing.T) {
	for uri, want := range map[string]string{
		"/sap/bc/adt/ddic/tables/mara":           "TABL MARA",
		"/sap/bc/adt/ddic/structures/bapiret2":   "TABL BAPIRET2",
		"/sap/bc/adt/ddic/dataelements/matnr":    "DTEL MATNR",
		"/sap/bc/adt/ddic/tabletypes/bapiret2_t": "TTYP BAPIRET2_T",
		"/sap/bc/adt/oo/classes/%2fbobf%2fcl_x":  "CLAS /BOBF/CL_X",
	} {
		got, err := callerTargetFromURI(uri)
		if err != nil {
			t.Errorf("%s: %v", uri, err)
			continue
		}
		if got.Type+" "+got.Name != want {
			t.Errorf("%s: got %s %s, want %s", uri, got.Type, got.Name, want)
		}
	}
	if _, err := callerTargetFromURI("/sap/bc/adt/ddic/ddl/sources/zcds"); err == nil {
		t.Error("a CDS view is not something these tables are searched for, and should be refused by name")
	}
}

// Seen on 7.40: the callers of BAPI_MATERIAL_AVAILABILITY included
// LWSAO_DISPF0A, whose section is not a letter and two digits, and
// /SAPNEA/LMR3_HANDHELD_SALESU30, a module include in a namespace. Both came
// out as includes with no package. LOAD_DATA has the same shape as the first
// and is a report: TLIBG tells them apart.
func TestCallersFromXrefFindsTheGroupOfUnusualIncludes(t *testing.T) {
	x := &xrefServer{usageStatus: http.StatusNotFound, tables: map[string]string{
		"CROSS": tableXML(col("INCLUDE", "LWSAO_DISPF0A", "/SAPNEA/LMR3_HANDHELD_SALESU30", "LOAD_DATA")),
		"TLIBG": tableXML(col("AREA", "WSAO_DISP")),
		"TFDIR": tableXML(
			col("FUNCNAME", "/SAPNEA/HH_SALES_CHECK"),
			col("PNAME", "/SAPNEA/SAPLMR3_HANDHELD_SALES"),
			col("INCLUDE", "30"),
		),
		"TRDIR": tableXML(col("NAME", "LOAD_DATA"), col("SUBC", "1")),
		"TADIR": tableXML(
			col("OBJECT", "FUGR", "FUGR"),
			col("OBJ_NAME", "WSAO_DISP", "/SAPNEA/MR3_HANDHELD_SALES"),
			col("DEVCLASS", "WSAO", "/SAPNEA/HH"),
		),
	}}
	srv := x.start(t)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	res, err := client.CallersFromXref(context.Background(), "/sap/bc/adt/functions/groups/w61v/fmodules/bapi_material_availability")
	if err != nil {
		t.Fatal(err)
	}
	if c := callerByName(res.Callers, "WSAO_DISP"); c == nil || c.Type != "FUGR/F" || c.Package != "WSAO" {
		t.Errorf("LWSAO_DISPF0A belongs to group WSAO_DISP, got %+v", res.Callers)
	}
	if c := callerByName(res.Callers, "/SAPNEA/HH_SALES_CHECK"); c == nil || c.Type != "FUGR/FF" || c.Package != "/SAPNEA/HH" ||
		c.URI != "/sap/bc/adt/functions/groups/%2Fsapnea%2Fmr3_handheld_sales/fmodules/%2Fsapnea%2Fhh_sales_check" {
		t.Errorf("the namespaced module include holds /SAPNEA/HH_SALES_CHECK, got %+v", res.Callers)
	}
	if c := callerByName(res.Callers, "LOAD_DATA"); c == nil || c.Type != "PROG/P" {
		t.Errorf("LOAD_DATA is a report, not group OAD_D, got %+v", res.Callers)
	}
	tfdir := x.asked("TFDIR")
	if len(tfdir) != 1 || !strings.Contains(tfdir[0], "'/SAPNEA/SAPLMR3_HANDHELD_SALES'") {
		t.Errorf("TFDIR should be asked for the namespaced pool, got %v", tfdir)
	}
}

// Sections past U99 are letters, and a group can live in a namespace.
func TestModuleIncludesAndLooseGroups(t *testing.T) {
	if !isModuleInclude("/BEV1/LEM0U07", "/BEV1/EM0") || !isModuleInclude("LZSALESUA0", "ZSALES") || isModuleInclude("LZSALESF01", "ZSALES") {
		t.Error("isModuleInclude: a U section in or out of a namespace is a module include, F01 is not")
	}
	if g, ok := looseGroupOf("LWSAO_DISPF0A"); !ok || g != "WSAO_DISP" {
		t.Errorf("looseGroupOf(LWSAO_DISPF0A) = %q, %v", g, ok)
	}
	if _, ok := looseGroupOf("ZLOAD"); ok {
		t.Error("a name not starting with L is no group candidate")
	}
}

// A chunk that fails is one lost query, said as such: the other chunks are
// still answered, and the failed one is not asked again in pieces, which
// would only hide what went wrong.
func TestQueryInKeepsTheChunksThatAnswer(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.Method != http.MethodPost {
			return
		}
		body, _ := io.ReadAll(r.Body)
		sql := string(body)
		mu.Lock()
		asked = append(asked, sql)
		mu.Unlock()
		if strings.Contains(sql, fmt.Sprintf("'ZPROGRAM_%04d'", xrefInChunk+20)) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`Following "', '" a blank is required`))
			return
		}
		var names []string
		for _, part := range strings.Split(sql, "'")[1:] {
			if strings.HasPrefix(part, "ZPROGRAM_") {
				names = append(names, part)
			}
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(tableXML(col("NAME", names...), col("SUBC", names...))))
	}))
	defer srv.Close()

	var names []string
	for i := 0; i < xrefInChunk+50; i++ {
		names = append(names, fmt.Sprintf("ZPROGRAM_%04d", i))
	}
	kinds, err := NewClient(srv.URL, "user", "pass").programKinds(context.Background(), names)
	if err == nil || !strings.Contains(err.Error(), "a blank is required") {
		t.Fatalf("the refused chunk should come back as its error, got %v", err)
	}
	if len(kinds) != xrefInChunk {
		t.Errorf("the chunk that answered should be kept: %d names, want %d", len(kinds), xrefInChunk)
	}
	if len(asked) != 2 {
		t.Errorf("two chunks, two queries; asked %d times", len(asked))
	}
}

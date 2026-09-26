package adt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// nodeXML is a node structure answer in the shape 7.50 sends: the nodes, then
// the list of kinds with their labels.
func nodeXML(nodes [][3]string, types [][2]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?><asx:abap version="1.0" xmlns:asx="http://www.sap.com/abapxml"><asx:values><DATA><TREE_CONTENT>`)
	for _, n := range nodes {
		b.WriteString("<SEU_ADT_REPOSITORY_OBJ_NODE><OBJECT_TYPE>" + n[0] + "</OBJECT_TYPE><OBJECT_NAME>" + n[1] +
			"</OBJECT_NAME><TECH_NAME>" + n[1] + "</TECH_NAME><OBJECT_URI>" + n[2] + "</OBJECT_URI><EXPANDABLE/></SEU_ADT_REPOSITORY_OBJ_NODE>")
	}
	b.WriteString("</TREE_CONTENT><OBJECT_TYPES>")
	for _, t := range types {
		b.WriteString("<SEU_ADT_OBJECT_TYPE_INFO><OBJECT_TYPE>" + t[0] + "</OBJECT_TYPE><CATEGORY_TAG>source_library</CATEGORY_TAG><OBJECT_TYPE_LABEL>" +
			t[1] + "</OBJECT_TYPE_LABEL></SEU_ADT_OBJECT_TYPE_INFO>")
	}
	b.WriteString("</OBJECT_TYPES></DATA></asx:values></asx:abap>")
	return b.String()
}

// treeServer answers the search from search, the node structure from nodes,
// and a class's objectstructure with one method. It records what was asked.
type treeServer struct {
	search string
	nodes  string

	mu    sync.Mutex
	asked []string
}

func (x *treeServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.Method == http.MethodHead {
			return
		}
		x.mu.Lock()
		x.asked = append(x.asked, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		x.mu.Unlock()
		switch {
		case strings.Contains(r.URL.Path, "informationsystem/search"):
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><adtcore:objectReferences xmlns:adtcore="http://www.sap.com/adt/core">` +
				x.search + `</adtcore:objectReferences>`))
		case strings.HasSuffix(r.URL.Path, "/repository/nodestructure"):
			w.Write([]byte(x.nodes))
		case strings.HasSuffix(r.URL.Path, "/objectstructure"):
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<abapsource:objectStructureElement xmlns:abapsource="http://www.sap.com/adt/abapsource" xmlns:adtcore="http://www.sap.com/adt/core"
    adtcore:name="ZCL_TEST" adtcore:type="CLAS/OC">
  <abapsource:objectStructureElement adtcore:name="RUN" adtcore:type="CLAS/OM" visibility="public"/>
</abapsource:objectStructureElement>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func (x *treeServer) askedFor(part string) []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	var out []string
	for _, a := range x.asked {
		if strings.Contains(a, part) {
			out = append(out, a)
		}
	}
	return out
}

func ref(uri, typ, name string) string {
	return `<adtcore:objectReference adtcore:uri="` + uri + `" adtcore:type="` + typ + `" adtcore:name="` + name + `"/>`
}

// ERP-182: a function group's name went to /oo/classes/W61V/objectstructure
// and came back 400 "class W61V does not exist".
func TestObjectStructureOfAFunctionGroupFoundBySearch(t *testing.T) {
	x := &treeServer{
		search: ref("/sap/bc/adt/functions/groups/w61vx", "FUGR/F", "W61VX") +
			ref("/sap/bc/adt/functions/groups/w61v", "FUGR/F", "W61V (Функциональная группа)"),
		nodes: nodeXML([][3]string{
			{"FUGR/PD", "F1", "/f1"}, {"FUGR/PD", "F2", "/f2"}, {"FUGR/PD", "F3", "/f3"},
			{"FUGR/FF", "W61V_EXIT", "/sap/bc/adt/functions/groups/w61v/fmodules/w61v_exit"},
			{"FUGR/I", "LW61VTOP", "/sap/bc/adt/functions/groups/w61v/includes/lw61vtop/source/main"},
			{"FUGR/I", "", ""}, // a folder row, not an object
			{"FUGR/ZZ", "ODD", "/odd"},
		}, [][2]string{{"FUGR/FF", "Function Modules"}, {"FUGR/PD", "Fields"}, {"FUGR/I", "Includes"}}),
	}
	srv := x.start(t)
	defer srv.Close()

	root, err := NewClient(srv.URL, "user", "pass").GetObjectStructure(context.Background(), "w61v", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.askedFor("/objectstructure")) != 0 {
		t.Error("a function group must not be asked for as a class")
	}
	if ns := x.askedFor("nodestructure"); len(ns) != 1 || !strings.Contains(ns[0], "parent_type=FUGR%2FF") || !strings.Contains(ns[0], "parent_name=W61V") {
		t.Errorf("node structure asked as %v", ns)
	}
	if root.Name != "W61V" || root.Type != "FUGR/F" || root.URI != "/sap/bc/adt/functions/groups/w61v" || root.Note != "" {
		t.Errorf("root = %+v", root)
	}

	var got []string
	for _, f := range root.Children {
		got = append(got, f.Name+":"+f.Type)
	}
	if strings.Join(got, " ") != "Function Modules:FUGR/FF Fields:FUGR/PD Includes:FUGR/I FUGR/ZZ:FUGR/ZZ" {
		t.Errorf("kinds, in SAP's order with an unlisted one last: %v", got)
	}
	if f := root.Children[1]; len(f.Children) != 2 || f.Omitted != 1 {
		t.Errorf("the limit applies per kind: fields %d shown, %d omitted", len(f.Children), f.Omitted)
	}
	if inc := root.Children[2]; len(inc.Children) != 1 || inc.Children[0].URI != "/sap/bc/adt/functions/groups/w61v/includes/lw61vtop/source/main" {
		t.Errorf("includes = %+v", inc.Children)
	}
}

func TestObjectStructureTakesTheTypeItIsGiven(t *testing.T) {
	x := &treeServer{nodes: nodeXML([][3]string{{"SFPI/5I", "ZZ_KUD_TMP_INTF", "/i"}}, [][2]string{{"SFPI/5I", "Interface Used"}})}
	srv := x.start(t)
	defer srv.Close()

	root, err := NewClient(srv.URL, "user", "pass").GetObjectStructure(context.Background(), "ZZZ_KUD_TMP_FORM", "SFPF", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.askedFor("search")) != 0 {
		t.Error("with a type given the repository need not be searched")
	}
	if ns := x.askedFor("nodestructure"); len(ns) != 1 || !strings.Contains(ns[0], "parent_type=SFPF%2F5F") {
		t.Errorf("node structure asked as %v", ns)
	}
	if len(root.Children) != 1 || root.Children[0].Children[0].Name != "ZZ_KUD_TMP_INTF" {
		t.Errorf("tree = %+v", root)
	}
}

func TestObjectStructureOfAClassIsStillItsObjectStructure(t *testing.T) {
	x := &treeServer{search: ref("/sap/bc/adt/oo/classes/zcl_test", "CLAS/OC", "ZCL_TEST")}
	srv := x.start(t)
	defer srv.Close()

	root, err := NewClient(srv.URL, "user", "pass").GetObjectStructureCAI(context.Background(), "ZCL_TEST", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.askedFor("/oo/classes/ZCL_TEST/objectstructure")) != 1 || len(x.askedFor("nodestructure")) != 0 {
		t.Errorf("asked %v", x.asked)
	}
	if len(root.Children) != 1 || root.Children[0].Name != "RUN" {
		t.Errorf("tree = %+v", root)
	}
}

// SBAL_DISPLAY is a program and a function group; a table and its data
// element share a name and have no tree at all.
func TestObjectStructureNamesTheOtherObjectsOfTheName(t *testing.T) {
	x := &treeServer{
		search: ref("/sap/bc/adt/ddic/dataelements/zthing", "DTEL/DE", "ZTHING") +
			ref("/sap/bc/adt/functions/groups/zthing", "FUGR/F", "ZTHING") +
			ref("/sap/bc/adt/programs/programs/zthing", "PROG/P", "ZTHING"),
		nodes: nodeXML(nil, nil),
	}
	srv := x.start(t)
	defer srv.Close()

	root, err := NewClient(srv.URL, "user", "pass").GetObjectStructure(context.Background(), "ZTHING", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if root.Type != "FUGR/F" || !strings.Contains(root.Note, "DTEL/DE, PROG/P.") || !strings.Contains(root.Note, "Pass object_type") ||
		!strings.Contains(root.Note, "no components") {
		t.Errorf("root = %+v", root)
	}

	// RSPARAM is a program and a structure: nothing else to ask for.
	x.search = ref("/sap/bc/adt/programs/programs/rsparam", "PROG/P", "RSPARAM") + ref("/sap/bc/adt/ddic/structures/rsparam", "TABL/DS", "RSPARAM")
	root, err = NewClient(srv.URL, "user", "pass").GetObjectStructure(context.Background(), "RSPARAM", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if root.Type != "PROG/P" || !strings.Contains(root.Note, "TABL/DS") || strings.Contains(root.Note, "Pass object_type") {
		t.Errorf("root = %+v", root)
	}
}

// Asked about a table, the node structure answers with hundreds of unrelated
// objects; that is not the table's structure and is not passed on.
func TestObjectStructureRefusesATypeWithoutATree(t *testing.T) {
	x := &treeServer{
		search: ref("/sap/bc/adt/ddic/tables/tvarvc", "TABL/DT", "TVARVC"),
		nodes:  nodeXML([][3]string{{"CLAS/OC", "CL_UNRELATED", "/c"}}, nil),
	}
	srv := x.start(t)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	for _, typ := range []string{"", "TABL"} {
		_, err := c.GetObjectStructure(context.Background(), "TVARVC", typ, 0)
		if err == nil || !strings.Contains(err.Error(), "TABL/DT") || !strings.Contains(err.Error(), "GetTable") {
			t.Errorf("type %q: %v", typ, err)
		}
	}
	if n := len(x.askedFor("nodestructure")); n != 0 {
		t.Errorf("the node structure was asked %d times", n)
	}
}

func TestObjectStructureOfANameNobodyHas(t *testing.T) {
	x := &treeServer{search: ref("/sap/bc/adt/programs/programs/zabcd", "PROG/P", "ZABCD")}
	srv := x.start(t)
	defer srv.Close()

	_, err := NewClient(srv.URL, "user", "pass").GetObjectStructure(context.Background(), "ZABC", "", 0)
	if err == nil || !strings.Contains(err.Error(), "no object named ZABC") {
		t.Errorf("a prefix hit is not the object: %v", err)
	}
}

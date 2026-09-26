package adt

import (
	"encoding/xml"
	"testing"
)

// The shape both releases return: an ABAP XML envelope whose TREE_CONTENT holds
// one node per row of the object tree — including a header row per category,
// which carries a type and a TECH_NAME but no object name.
const nodeStructureXML = `<?xml version="1.0" encoding="utf-8"?>
<asx:abap version="1.0" xmlns:asx="http://www.sap.com/abapxml"><asx:values><DATA><TREE_CONTENT>
  <SEU_ADT_REPOSITORY_OBJ_NODE>
    <OBJECT_TYPE>FUGR/FF</OBJECT_TYPE><OBJECT_NAME/><TECH_NAME>SAPLZDEMO_FG</TECH_NAME><OBJECT_URI/>
  </SEU_ADT_REPOSITORY_OBJ_NODE>
  <SEU_ADT_REPOSITORY_OBJ_NODE>
    <OBJECT_TYPE>FUGR/I</OBJECT_TYPE><OBJECT_NAME/><TECH_NAME>SAPLZDEMO_FG</TECH_NAME><OBJECT_URI/>
  </SEU_ADT_REPOSITORY_OBJ_NODE>
  <SEU_ADT_REPOSITORY_OBJ_NODE>
    <OBJECT_TYPE>FUGR/FF</OBJECT_TYPE><OBJECT_NAME>ZDEMO_FM_ONE</OBJECT_NAME>
    <TECH_NAME>ZDEMO_FM_ONE</TECH_NAME>
    <OBJECT_URI>/sap/bc/adt/functions/groups/zdemo_fg/fmodules/zdemo_fm_one</OBJECT_URI>
  </SEU_ADT_REPOSITORY_OBJ_NODE>
  <SEU_ADT_REPOSITORY_OBJ_NODE>
    <OBJECT_TYPE>FUGR/I</OBJECT_TYPE><OBJECT_NAME>LZDEMO_FGTOP</OBJECT_NAME>
    <TECH_NAME>LZDEMO_FGTOP</TECH_NAME>
    <OBJECT_URI>/sap/bc/adt/functions/groups/zdemo_fg/includes/lzdemo_fgtop</OBJECT_URI>
  </SEU_ADT_REPOSITORY_OBJ_NODE>
  <SEU_ADT_REPOSITORY_OBJ_NODE>
    <OBJECT_TYPE>FUGR/PX</OBJECT_TYPE><OBJECT_NAME>SAPLZDEMO_FG</OBJECT_NAME>
    <TECH_NAME>SAPLZDEMO_FG</TECH_NAME>
    <OBJECT_URI>/sap/bc/adt/textelements/functiongroups/zdemo_fg</OBJECT_URI>
  </SEU_ADT_REPOSITORY_OBJ_NODE>
  <SEU_ADT_REPOSITORY_OBJ_NODE>
    <OBJECT_TYPE>FUGR/FF</OBJECT_TYPE><OBJECT_NAME>ZDEMO_FM_TWO</OBJECT_NAME>
    <TECH_NAME>ZDEMO_FM_TWO</TECH_NAME>
    <OBJECT_URI>/sap/bc/adt/functions/groups/zdemo_fg/fmodules/zdemo_fm_two</OBJECT_URI>
  </SEU_ADT_REPOSITORY_OBJ_NODE>
</TREE_CONTENT></DATA></asx:values></asx:abap>`

func parseNodes(t *testing.T) []repositoryNode {
	t.Helper()
	var doc repositoryNodeStructure
	if err := xml.Unmarshal([]byte(nodeStructureXML), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Nodes) != 6 {
		t.Fatalf("parsed %d nodes, want 6 — the path into the ABAP envelope is wrong", len(doc.Nodes))
	}
	return doc.Nodes
}

func TestFunctionModulesFromNodes(t *testing.T) {
	modules := functionModulesFromNodes(parseNodes(t))

	if len(modules) != 2 {
		t.Fatalf("got %d modules, want 2: %+v", len(modules), modules)
	}
	for i, want := range []string{"ZDEMO_FM_ONE", "ZDEMO_FM_TWO"} {
		if modules[i].Name != want {
			t.Errorf("module %d = %q, want %q", i, modules[i].Name, want)
		}
		if modules[i].Type != "FUGR/FF" {
			t.Errorf("module %d type = %q, want FUGR/FF", i, modules[i].Type)
		}
		if modules[i].URI == "" {
			t.Errorf("module %d has no URI", i)
		}
	}
}

func TestFunctionModulesDropCategoryHeaders(t *testing.T) {
	// A header row carries the group's main program in TECH_NAME. Falling back
	// to that field invents a module named SAPL<group>, which is a program.
	for _, m := range functionModulesFromNodes(parseNodes(t)) {
		if m.Name == "SAPLZDEMO_FG" {
			t.Error("the category header became a module named after the group's main program")
		}
		if m.Name == "" {
			t.Error("a nameless node became a module")
		}
	}
}

func TestFunctionModulesDropIncludesAndPrograms(t *testing.T) {
	// A group's includes and its text-element program sit in the same tree and
	// are not function modules.
	for _, m := range functionModulesFromNodes(parseNodes(t)) {
		switch m.Name {
		case "LZDEMO_FGTOP":
			t.Error("an include (FUGR/I) was returned as a function module")
		case "SAPLZDEMO_FG":
			t.Error("a program (FUGR/PX) was returned as a function module")
		}
	}
}

func TestFunctionModulesFromNothing(t *testing.T) {
	if got := functionModulesFromNodes(nil); got != nil {
		t.Errorf("got %v, want nil for a group with no nodes", got)
	}
}

// ERP-182: GetFunctionGroup named the modules and not the includes, and the
// includes were in the same answer.
func TestFunctionGroupIncludesFromNodes(t *testing.T) {
	includes := functionGroupIncludesFromNodes(parseNodes(t))
	if len(includes) != 1 || includes[0].Name != "LZDEMO_FGTOP" ||
		includes[0].URI != "/sap/bc/adt/functions/groups/zdemo_fg/includes/lzdemo_fgtop" {
		t.Errorf("includes = %+v; the header row is not one, the text-element program is not one", includes)
	}
}

// The source URIs of a group read from the node structure: 7.40 gives an
// include without /source/main, 7.50 with it; an include from elsewhere is a
// program include in the group's context; a module needs /source/main.
func TestFunctionGroupSourcesFromNodes(t *testing.T) {
	nodes := []repositoryNode{
		{ObjectType: "FUGR/I", ObjectName: "LW61VTOP", ObjectURI: "/sap/bc/adt/functions/groups/w61v/includes/lw61vtop"},
		{ObjectType: "FUGR/I", ObjectName: "LW61VF01", ObjectURI: "/sap/bc/adt/functions/groups/w61v/includes/lw61vf01/source/main"},
		{ObjectType: "FUGR/I", ObjectName: "SBALTYPE", ObjectURI: "/sap/bc/adt/programs/includes/sbaltype/source/main?context=%2fsap%2fbc%2fadt%2ffunctions%2fgroups%2fw61v"},
		{ObjectType: "FUGR/FF", ObjectName: "BAPI_MATERIAL_AVAILABILITY", ObjectURI: "/sap/bc/adt/functions/groups/w61v/fmodules/bapi_material_availability"},
		{ObjectType: "FUGR/PU", ObjectName: "ADD_MENGE", ObjectURI: "/sap/bc/adt/functions/groups/w61v/includes/lw61vf01/source/main#start=5,6"},
		{ObjectType: "FUGR/I", ObjectName: "LW61VTOP", ObjectURI: "/sap/bc/adt/functions/groups/w61v/includes/lw61vtop#type=FUGR%2FI"},
	}
	want := []string{
		"/sap/bc/adt/functions/groups/w61v/source/main",
		"/sap/bc/adt/functions/groups/w61v/includes/lw61vtop/source/main",
		"/sap/bc/adt/functions/groups/w61v/includes/lw61vf01/source/main",
		"/sap/bc/adt/programs/includes/sbaltype/source/main?context=%2fsap%2fbc%2fadt%2ffunctions%2fgroups%2fw61v",
		"/sap/bc/adt/functions/groups/w61v/fmodules/bapi_material_availability/source/main",
	}
	got := functionGroupSourcesFromNodes("W61V", nodes)
	if len(got) != len(want) {
		t.Fatalf("got %d URIs, want %d:\n%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("URI %d = %q, want %q", i, got[i], want[i])
		}
	}
}

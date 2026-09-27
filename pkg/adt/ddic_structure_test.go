package adt

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A DDIC structure has its own resource, /ddic/structures. Before this it
// could be read (GetStructure) but not created or written: create OBJECT
// refused TABL/DS, and edit TABL sent a structure's source to /ddic/tables,
// which serves no structure on any release (upstream #253).

func TestIsStructureSource(t *testing.T) {
	cases := []struct {
		name, src string
		want      bool
	}{
		{"7.57", "@EndUserText.label : 'Return'\n@AbapCatalog.enhancement.category : #NOT_EXTENSIBLE\ndefine structure bapiret2 {\n  type : bapi_mtype;\n}", true},
		{"7.50", "@EndUserText.label : 'demo'\n@AbapCatalog.enhancementCategory : #NOT_CLASSIFIED\ndefine type zsewm_pb_item_row {\n  description : char40;\n}", true},
		{"append", "extend type zs with zs_app {\n  zz_x : char1;\n}", true},
		{"comments first", "// a structure\n/* define table x { } */\n  DEFINE   STRUCTURE zs { a : char1; }", true},
		{"table", "@EndUserText.label : 'x'\n@AbapCatalog.tableCategory : #TRANSPARENT\ndefine table zt {\n  key client : mandt not null;\n}", false},
		{"not DDL", "REPORT ztest.", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		if got := IsStructureSource(c.src); got != c.want {
			t.Errorf("%s: IsStructureSource = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAStructureHasItsOwnAddress(t *testing.T) {
	if got := GetObjectURL(ObjectTypeStructure, "ZDEMO_S", ""); got != "/sap/bc/adt/ddic/structures/zdemo_s" {
		t.Errorf("unexpected structure URL %q", got)
	}
	if got := GetObjectURL(ObjectTypeStructure, "/DMO/S", ""); strings.Contains(got, "//dmo/") {
		t.Errorf("a namespaced name must be escaped: %q", got)
	}
}

func TestCreateObjectSaysWhereATableIsMade(t *testing.T) {
	err := (&Client{}).CreateObject(context.Background(), CreateObjectOptions{
		ObjectType: ObjectTypeTable, Name: "ZT", PackageName: "$TMP"})
	if err == nil || !strings.Contains(err.Error(), "create target TABL") {
		t.Errorf("the refusal should name the table route, got %v", err)
	}
}

func structureStub(exists bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "informationsystem/search"):
			_, _ = io.WriteString(w, searchXMLFor("/sap/bc/adt/ddic/structures/zdemo_s", "ZDEMO_S", "$TMP"))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/ddic/structures/zdemo_s"):
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = io.WriteString(w, "define structure zdemo_s {\n  a : char1;\n}")
		case strings.Contains(r.URL.Path, "/checkruns"):
			w.Header().Set("Content-Type", "application/vnd.sap.adt.checkmessages+xml")
			_, _ = io.WriteString(w, testEmptyCheckXML)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			w.Header().Set("Content-Type", "application/vnd.sap.as+xml")
			_, _ = io.WriteString(w, testLockXML)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}
}

func TestEditTABLWithAStructureSourceWritesTheStructure(t *testing.T) {
	rec := &adtRecorder{}
	client := newStubbedClient(t, rec, structureStub(true))

	src := "@EndUserText.label : 'demo'\ndefine structure zdemo_s {\n  a : char1;\n  b : char2;\n}"
	result, err := client.WriteSource(context.Background(), "TABL", "ZDEMO_S", src, nil)
	if err != nil {
		t.Fatalf("WriteSource: %v", err)
	}
	if result.ObjectURL != "/sap/bc/adt/ddic/structures/zdemo_s" {
		t.Errorf("object URL = %q, want the structure's", result.ObjectURL)
	}
	calls := rec.snapshot()
	putAt := indexOfCall(calls, isSourcePut)
	if putAt < 0 || calls[putAt].path != "/sap/bc/adt/ddic/structures/zdemo_s/source/main" {
		t.Errorf("expected the source PUT on the structure; trace:")
		dumpCalls(t, calls)
	}
	for _, c := range calls {
		if strings.Contains(c.path, "/ddic/tables/") {
			t.Errorf("a structure must not go to the table resource: %s", c)
		}
	}
}

func TestCreateSTRUCTCreatesThenWrites(t *testing.T) {
	rec := &adtRecorder{}
	client := newStubbedClient(t, rec, structureStub(false), WithAllowedPackages("$TMP"))

	src := "define structure zdemo_s {\n  a : char1;\n}"
	_, err := client.WriteSource(context.Background(), "STRUCT", "ZDEMO_S", src,
		&WriteSourceOptions{Mode: WriteModeCreate, Package: "$TMP", Description: "demo"})
	if err != nil {
		t.Fatalf("WriteSource: %v", err)
	}
	calls := rec.snapshot()
	createAt := indexOfCall(calls, func(c wireCall) bool {
		return c.method == http.MethodPost && c.path == "/sap/bc/adt/ddic/structures"
	})
	putAt := indexOfCall(calls, isSourcePut)
	if createAt < 0 || putAt < createAt {
		t.Errorf("expected the create POST on /ddic/structures, then the source PUT; trace:")
		dumpCalls(t, calls)
	}
}

func TestStructureCreateBodyIsTypedDS(t *testing.T) {
	body := buildStructureBody(CreateObjectOptions{Name: "ZDEMO_S", Description: "a & b", PackageName: "$TMP"},
		objectTypes[ObjectTypeStructure], "DEV")
	for _, want := range []string{`adtcore:type="TABL/DS"`, `adtcore:name="ZDEMO_S"`, `a &amp; b`, `adtcore:name="$TMP"`} {
		if !strings.Contains(body, want) {
			t.Errorf("create body lacks %s:\n%s", want, body)
		}
	}
}

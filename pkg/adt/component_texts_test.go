package adt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func textMap(texts []ComponentText) map[string]string {
	m := map[string]string{}
	for _, t := range texts {
		m[t.key()] = t.Text
	}
	return m
}

func TestParseComponentTextsReadsEveryKindOfText(t *testing.T) {
	src := `"! <p class="shorttext synchronized">The class</p>
CLASS zcl_demo_texts DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    "! <p class="shorttext synchronized">Counter</p>
    DATA mv_count TYPE i.
    "! <p class="shorttext synchronized">A line</p>
    TYPES ty_line TYPE string.
    "! <p class="shorttext synchronized">Done</p>
    "! @parameter ev_id | <p class="shorttext synchronized">Event id</p>
    EVENTS done EXPORTING VALUE(ev_id) TYPE i.
    "! <p class="shorttext synchronized">Adds</p>
    "! @parameter iv_a | <p class="shorttext synchronized">First summand</p>
    "! @parameter rv_sum | <p class="shorttext synchronized">Sum</p>
    "! @raising cx_sy_arithmetic_overflow | <p class="shorttext synchronized">Overflow</p>
    METHODS add IMPORTING iv_a TYPE i RETURNING VALUE(rv_sum) TYPE i
      RAISING cx_sy_arithmetic_overflow.
    "! <p class="shorttext synchronized">Classic</p>
    "! @parameter iv_x | <p class="shorttext synchronized">Input X</p>
    "! @exception failed | <p class="shorttext synchronized">Failed</p>
    CLASS-METHODS old IMPORTING iv_x TYPE i EXCEPTIONS failed.
ENDCLASS.

CLASS zcl_demo_texts IMPLEMENTATION.
  METHOD add.
    "! <p class="shorttext synchronized">Not a declaration</p>
    DATA lv_local TYPE i.
  ENDMETHOD.
ENDCLASS.`
	want := map[string]string{
		"MV_COUNT":                      "Counter",
		"TY_LINE":                       "A line",
		"DONE":                          "Done",
		"DONE.EV_ID":                    "Event id",
		"ADD":                           "Adds",
		"ADD.IV_A":                      "First summand",
		"ADD.RV_SUM":                    "Sum",
		"ADD.CX_SY_ARITHMETIC_OVERFLOW": "Overflow",
		"OLD":                           "Classic",
		"OLD.IV_X":                      "Input X",
		"OLD.FAILED":                    "Failed",
	}
	if got := textMap(ParseComponentTexts(src, "ZCL_DEMO_TEXTS")); !reflect.DeepEqual(got, want) {
		t.Errorf("texts:\n got %v\nwant %v", got, want)
	}
}

func TestParseComponentTextsChainsAndStructures(t *testing.T) {
	src := `CLASS zcl_demo_texts DEFINITION PUBLIC.
  PUBLIC SECTION.
    TYPES:
      "! <p class="shorttext synchronized">Pair</p>
      BEGIN OF ty_pair,
        "! <p class="shorttext synchronized">Not a component</p>
        key   TYPE string,
        value TYPE string,
      END OF ty_pair,
      "! <p class="shorttext synchronized">Pairs</p>
      ty_pairs TYPE STANDARD TABLE OF ty_pair WITH DEFAULT KEY.
    "! <p class="shorttext synchronized">Both</p>
    CONSTANTS: c_a TYPE c LENGTH 1 VALUE 'a',
      "! <p class="shorttext synchronized">Second, with a comma</p>
      c_b TYPE string VALUE 'x,y'.
    CLASS-DATA gv_x TYPE i.
  PROTECTED SECTION.
    "! <p class="shorttext synchronized">Redefined</p>
    METHODS run REDEFINITION.
    METHODS
      "! <p class="shorttext synchronized">Multi
      "!   line &amp; escaped</p>
      !go IMPORTING !iv TYPE i.
ENDCLASS.`
	want := map[string]string{
		"TY_PAIR":  "Pair",
		"TY_PAIRS": "Pairs",
		"C_A":      "Both",
		"C_B":      "Second, with a comma",
		"GO":       "Multi line & escaped",
	}
	if got := textMap(ParseComponentTexts(src, "zcl_demo_texts")); !reflect.DeepEqual(got, want) {
		t.Errorf("texts:\n got %v\nwant %v", got, want)
	}
}

func TestParseComponentTextsSkipsWhatIsNotSynchronized(t *testing.T) {
	src := `CLASS zcl_other DEFINITION DEFERRED.
CLASS lcl_local DEFINITION.
  PUBLIC SECTION.
    "! <p class="shorttext synchronized">Local class</p>
    METHODS local.
ENDCLASS.
INTERFACE zif_demo_texts PUBLIC.
  "! Plain documentation, no short text
  METHODS plain.
  "! <p class="shorttext synchronized" lang="en">With a language</p>
  "! <p class="shorttext">Not synchronized</p>
  METHODS with_lang.
  "! <p class="shorttext synchronized">Broken by a comment</p>
  " ordinary comment
  METHODS broken.
ENDINTERFACE.`
	want := map[string]string{"WITH_LANG": "With a language"}
	if got := textMap(ParseComponentTexts(src, "ZIF_DEMO_TEXTS")); !reflect.DeepEqual(got, want) {
		t.Errorf("texts:\n got %v\nwant %v", got, want)
	}
}

func TestParseComponentTextLines(t *testing.T) {
	got := parseComponentTextLines("ADD\t\tAdds\r\nADD\tIV_A\tFirst\n\n")
	want := []ComponentText{{"ADD", "", "Adds"}, {"ADD", "IV_A", "First"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// componentTextsServer is a system with the adt-fallback set: offered says
// whether discovery offers the component texts (7.40) or not (7.50+, where
// saving the source writes them). It records the requests to the stand-in.
func componentTextsServer(t *testing.T, offered bool, calls *[]string, put *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		switch {
		case r.URL.Path == "/sap/bc/adt/discovery":
			doc := `<app:collection href="/sap/bc/adt/zvsp/info"/>`
			if offered {
				doc += `<app:collection href="/sap/bc/adt/zvsp/oo/componenttexts"/>`
			}
			w.Write([]byte(doc))
		case r.URL.Path == "/sap/bc/adt/zvsp/info":
			w.Write([]byte("version=0.8.0\nfeatures=/textelements,/oo/componenttexts\n"))
		case strings.HasPrefix(r.URL.Path, "/sap/bc/adt/zvsp/oo/componenttexts/"):
			action := r.URL.Query().Get("_action")
			*calls = append(*calls, r.Method+" "+action)
			switch {
			case action == "LOCK":
				w.Write([]byte(`<asx:abap><asx:values><DATA><LOCK_HANDLE>HANDLE-1</LOCK_HANDLE><IS_LOCAL>X</IS_LOCAL></DATA></asx:values></asx:abap>`))
			case r.Method == http.MethodPut:
				body, _ := io.ReadAll(r.Body)
				*put = string(body)
			}
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

const componentTextsSource = `CLASS zcl_demo_texts DEFINITION PUBLIC.
  PUBLIC SECTION.
    "! <p class="shorttext synchronized">Adds</p>
    "! @parameter iv_a | <p class="shorttext synchronized">First</p>
    "! @parameter iv_b | <p class="shorttext synchronized">` + "0123456789012345678901234567890123456789012345678901234567890" + `</p>
    METHODS add IMPORTING iv_a TYPE i iv_b TYPE i.
ENDCLASS.`

func TestComponentTextsGoToTheStandInWhereDiscoveryOffersIt(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, true, &calls, &put)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(context.Background(), "CLAS/OC", "ZCL_DEMO_TEXTS", componentTextsSource, "")
	if got == nil || got.Written != 2 || got.Error != "" {
		t.Fatalf("result: %+v", got)
	}
	if len(got.Skipped) != 1 || !strings.HasPrefix(got.Skipped[0], "ADD.IV_B: longer than 60") {
		t.Errorf("skipped: %v", got.Skipped)
	}
	if want := "ADD\t\tAdds\nADD\tIV_A\tFirst\n"; put != want {
		t.Errorf("PUT body %q, want %q", put, want)
	}
	if want := []string{"POST LOCK", "PUT ", "POST UNLOCK"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
}

func TestComponentTextsAreLeftToASystemThatWritesThem(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, false, &calls, &put)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	if got := c.SyncComponentTexts(context.Background(), "CLAS", "ZCL_DEMO_TEXTS", componentTextsSource, ""); got != nil {
		t.Errorf("the set has the feature and does not offer it: nothing to do, got %+v", got)
	}
	if len(calls) != 0 {
		t.Errorf("no request to the stand-in expected, got %v", calls)
	}
}

func TestComponentTextsNeedNothingWithoutABAPDoc(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, true, &calls, &put)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	src := "CLASS zcl_demo_texts DEFINITION PUBLIC.\n  PUBLIC SECTION.\n    METHODS add.\nENDCLASS."
	if got := c.SyncComponentTexts(context.Background(), "CLAS", "ZCL_DEMO_TEXTS", src, ""); got != nil || len(calls) != 0 {
		t.Errorf("got %+v, calls %v", got, calls)
	}
	if got := c.SyncComponentTexts(context.Background(), "PROG", "ZCL_DEMO_TEXTS", componentTextsSource, ""); got != nil {
		t.Errorf("a program has no component texts, got %+v", got)
	}
}

func TestParsePlainComponentTexts(t *testing.T) {
	src := `"! The class
CLASS zcl_demo_texts DEFINITION PUBLIC.
  PUBLIC SECTION.
    "! Counter
    DATA mv_count TYPE i.
    "! Adds two
    "! numbers
    "!
    "! Longer documentation that is not the short text.
    "! @parameter iv_a | First
    "!   summand
    "! @parameter iv_b |
    "! @parameter rv_sum | <em>Sum</em> &amp; more
    "! @raising cx_sy_arithmetic_overflow | Overflow
    "! @see other
    METHODS add IMPORTING iv_a TYPE i iv_b TYPE i RETURNING VALUE(rv_sum) TYPE i
      RAISING cx_sy_arithmetic_overflow.
    "! @parameter iv_x | Only the parameter
    "! @exception failed | Failed
    METHODS old IMPORTING iv_x TYPE i EXCEPTIONS failed.
    "! <p class="shorttext synchronized">Tagged wins</p>
    "! Plain line
    "! @parameter iv_y | Plain parameter
    METHODS mixed IMPORTING iv_y TYPE i.
    TYPES:
      "! Pair
      BEGIN OF ty_pair,
        "! Not a component
        key TYPE string,
      END OF ty_pair,
      "! Pairs
      ty_pairs TYPE STANDARD TABLE OF ty_pair WITH DEFAULT KEY.
    "! Broken by a comment
    " ordinary comment
    METHODS broken.
ENDCLASS.`
	want := map[string]string{
		"MV_COUNT":                      "Counter",
		"ADD":                           "Adds two numbers",
		"ADD.IV_A":                      "First summand",
		"ADD.RV_SUM":                    "Sum & more",
		"ADD.CX_SY_ARITHMETIC_OVERFLOW": "Overflow",
		"OLD.IV_X":                      "Only the parameter",
		"OLD.FAILED":                    "Failed",
		"MIXED":                         "Tagged wins",
		"MIXED.IV_Y":                    "Plain parameter",
		"TY_PAIR":                       "Pair",
		"TY_PAIRS":                      "Pairs",
	}
	if got := textMap(ParsePlainComponentTexts(src, "ZCL_DEMO_TEXTS")); !reflect.DeepEqual(got, want) {
		t.Errorf("texts:\n got %v\nwant %v", got, want)
	}
	// Without plain only the tag counts.
	if got := textMap(ParseComponentTexts(src, "ZCL_DEMO_TEXTS")); !reflect.DeepEqual(got, map[string]string{"MIXED": "Tagged wins"}) {
		t.Errorf("tagged only: %v", got)
	}
}

const plainTextsSource = `CLASS zcl_demo_texts DEFINITION PUBLIC.
  PUBLIC SECTION.
    "! Adds
    "! @parameter iv_a | First
    METHODS add IMPORTING iv_a TYPE i.
ENDCLASS.`

// Where the system does not write the texts, plain ABAP Doc is written.
func TestPlainComponentTextsGoToTheStandIn(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, true, &calls, &put)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(context.Background(), "CLAS", "ZCL_DEMO_TEXTS", plainTextsSource, "")
	if got == nil || got.Written != 2 || got.Error != "" || got.Note != "" {
		t.Fatalf("result: %+v", got)
	}
	if want := "ADD\t\tAdds\nADD\tIV_A\tFirst\n"; put != want {
		t.Errorf("PUT body %q, want %q", put, want)
	}
}

// Where the system writes the texts itself, plain ABAP Doc is documentation
// only: nothing goes to the stand-in.
func TestPlainComponentTextsAreLeftAloneWhereTheSystemWritesTexts(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, false, &calls, &put)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	if got := c.SyncComponentTexts(context.Background(), "CLAS", "ZCL_DEMO_TEXTS", plainTextsSource, ""); got != nil || len(calls) != 0 {
		t.Errorf("got %+v, calls %v", got, calls)
	}
}

// Tags still work on 7.40, with a note that its class check warns on them.
func TestTaggedComponentTextsOn740CarryANote(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, true, &calls, &put)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(context.Background(), "CLAS", "ZCL_DEMO_TEXTS", componentTextsSource, "")
	if got == nil || got.Written != 2 || !strings.Contains(got.Note, "plain ABAP Doc") {
		t.Fatalf("result: %+v", got)
	}
}

func TestComponentTextsFromMap(t *testing.T) {
	got, err := ComponentTextsFromMap(map[string]string{
		"get_limit":          "  Получить\n лимит ",
		"GET_LIMIT.iv_werks": "Завод",
		"C_LIMIT":            "",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []ComponentText{
		{Component: "C_LIMIT"},
		{Component: "GET_LIMIT", Subcomponent: "IV_WERKS", Text: "Завод"},
		{Component: "GET_LIMIT", Text: "Получить лимит"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	for _, bad := range []map[string]string{
		{"": "x"}, {"A.": "x"}, {".B": "x"}, {"A.B.C": "x"}, {"a": "x", "A": "y"},
	} {
		if _, err := ComponentTextsFromMap(bad); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
}

// Given texts go with the source's on 7.40 and win over them for the same
// key; an empty one deletes the text.
func TestGivenComponentTextsWinOverTheSource(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, true, &calls, &put)
	defer srv.Close()

	ctx := WithComponentTexts(context.Background(), []ComponentText{
		{Component: "ADD", Text: "Сложить"},
		{Component: "ADD", Subcomponent: "IV_B"},
	})
	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(ctx, "CLAS", "ZCL_DEMO_TEXTS", plainTextsSource, "")
	if got == nil || got.Written != 2 || got.Deleted != 1 || got.Error != "" {
		t.Fatalf("result: %+v", got)
	}
	if want := "ADD\tIV_A\tFirst\nADD\t\tСложить\nADD\tIV_B\t\n"; put != want {
		t.Errorf("PUT body %q, want %q", put, want)
	}
}

// Where the system takes ABAP Doc itself, given texts still go through the
// set, and only they.
func TestGivenComponentTextsGoWhereTheSystemWritesTexts(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, false, &calls, &put)
	defer srv.Close()

	ctx := WithComponentTexts(context.Background(), []ComponentText{{Component: "ADD", Subcomponent: "IV_A", Text: "Первое"}})
	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(ctx, "INTF/OI", "ZCL_DEMO_TEXTS", componentTextsSource, "")
	if got == nil || got.Written != 1 || got.Error != "" {
		t.Fatalf("result: %+v", got)
	}
	if want := "ADD\tIV_A\tПервое\n"; put != want {
		t.Errorf("PUT body %q, want %q", put, want)
	}
}

func TestGivenComponentTextsNeedTheSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if strings.HasPrefix(r.URL.Path, "/sap/bc/adt/zvsp/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`<app:service/>`))
	}))
	defer srv.Close()

	ctx := WithComponentTexts(context.Background(), []ComponentText{{Component: "ADD", Text: "Adds"}})
	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(ctx, "CLAS", "ZCL_DEMO_TEXTS", "CLASS zcl_demo_texts DEFINITION. ENDCLASS.", "")
	if got == nil || !strings.Contains(got.Error, "adt-fallback") || got.Written != 0 {
		t.Fatalf("result: %+v", got)
	}
}

func TestGivenComponentTextsOnlyForClassesAndInterfaces(t *testing.T) {
	ctx := WithComponentTexts(context.Background(), []ComponentText{{Component: "ADD", Text: "Adds"}})
	c := NewClient("http://unused.invalid", "user", "pass")
	got := c.SyncComponentTexts(ctx, "PROG", "ZDEMO", "REPORT zdemo.", "")
	if got == nil || !strings.Contains(got.Error, "only classes and interfaces") {
		t.Fatalf("result: %+v", got)
	}
}

// 7.40's ABAP Doc check warns on Cyrillic: plain texts in it get a note.
func TestCyrillicABAPDocOn740CarriesANote(t *testing.T) {
	var calls []string
	var put string
	srv := componentTextsServer(t, true, &calls, &put)
	defer srv.Close()

	source := strings.ReplaceAll(plainTextsSource, "Adds", "Сложить")
	c := NewClient(srv.URL, "user", "pass")
	got := c.SyncComponentTexts(context.Background(), "CLAS", "ZCL_DEMO_TEXTS", source, "")
	if got == nil || got.Written != 2 || !strings.Contains(got.Note, "component_texts") {
		t.Fatalf("result: %+v", got)
	}
}

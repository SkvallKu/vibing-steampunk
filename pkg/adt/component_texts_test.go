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

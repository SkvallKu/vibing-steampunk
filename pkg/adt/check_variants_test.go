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

const checkVariantDoc = "description\tLogic checks\n" +
	"check\tCL_CI_TEST_SYSUBRC\n" +
	"param\tCL_CI_TEST_SYSUBRC\tSELECT\tX\n" +
	"param\tCL_CI_TEST_SYSUBRC\tANY\tREAD TABLE\n" +
	"param\tCL_CI_TEST_SYSUBRC\tANY\tASSIGN\n" +
	"# CL_CI_TEST_SYSUBRC REFS: not an elementary value or flat structure, not shown\n" +
	"check\tCL_CI_TEST_NO_ORDER_BY\n"

func TestParseCheckVariant(t *testing.T) {
	v, err := ParseCheckVariant("zvsp_logic", checkVariantDoc)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "ZVSP_LOGIC" || v.Description != "Logic checks" || v.Transported {
		t.Errorf("head %+v", v)
	}
	want := []CheckVariantCheck{
		{Class: "CL_CI_TEST_SYSUBRC", Params: []CheckParam{
			{Name: "SELECT", Values: []string{"X"}},
			{Name: "ANY", Values: []string{"READ TABLE"}},
			{Name: "ANY", Values: []string{"ASSIGN"}},
		}},
		{Class: "CL_CI_TEST_NO_ORDER_BY"},
	}
	if !reflect.DeepEqual(v.Checks, want) {
		t.Errorf("checks %+v", v.Checks)
	}
	if len(v.Notes) != 1 {
		t.Errorf("notes %v", v.Notes)
	}
}

func TestParseCheckVariantRefusesBadLines(t *testing.T) {
	for doc, want := range map[string]string{
		"param\tCL_X\tA\tB\n":        "no check line before it",
		"check\tCL_X\ncheck\tcl_x\n": "named twice",
		"check\tCL_X\nparam\tCL_X\n": "want param TAB check",
		"checks\tCL_X\n":             "want description, check or param",
		"check\t\n":                  "check without a class",
	} {
		if _, err := ParseCheckVariant("ZV", doc); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %v, want %q", doc, err, want)
		}
	}
}

func TestParseCodeInspectorChecks(t *testing.T) {
	v, err := ParseCheckVariant("", "check\tCL_CI_TEST_SYSUBRC\t002\tCL_CI_CATEGORY_ROBUSTNESS\tCheck of SY-SUBRC handling\n"+
		"param\tCL_CI_TEST_SYSUBRC\tANY\t\n")
	if err != nil {
		t.Fatal(err)
	}
	want := CheckVariantCheck{Class: "CL_CI_TEST_SYSUBRC", Version: "002", Category: "CL_CI_CATEGORY_ROBUSTNESS",
		Title: "Check of SY-SUBRC handling", Params: []CheckParam{{Name: "ANY", Values: []string{""}}}}
	if !reflect.DeepEqual(v.Checks, []CheckVariantCheck{want}) {
		t.Errorf("checks %+v", v.Checks)
	}
}

func ciServer(t *testing.T, features string, calls *[]string, body *string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		mu.Lock()
		defer mu.Unlock()
		switch p := r.URL.Path; {
		case p == "/sap/bc/adt/discovery":
			w.Write([]byte(`<app:collection href="/sap/bc/adt/zvsp/info"/><app:collection href="/sap/bc/adt/zvsp/ci/checkvariants"/>`))
		case p == "/sap/bc/adt/zvsp/info":
			w.Write([]byte("version=0.10.0\nfeatures=" + features + "\n"))
		case p == "/sap/bc/adt/zvsp/ci/checkvariants/ZVSP_LOGIC":
			*calls = append(*calls, r.Method)
			switch r.Method {
			case http.MethodPut:
				b, _ := io.ReadAll(r.Body)
				*body = string(b)
				w.Write(b)
			case http.MethodGet:
				w.Write([]byte(checkVariantDoc))
			case http.MethodDelete:
				w.WriteHeader(http.StatusOK)
			}
		case p == "/sap/bc/adt/zvsp/ci/checks":
			*calls = append(*calls, r.Method+" checks "+r.URL.Query().Get("check"))
			w.Write([]byte("check\tCL_CI_TEST_SYSUBRC\t002\tCL_CI_CATEGORY_ROBUSTNESS\tSY-SUBRC\nparam\tCL_CI_TEST_SYSUBRC\tSELECT\t\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestWriteCheckVariant(t *testing.T) {
	var calls []string
	var body string
	srv := ciServer(t, "/textelements,/ci", &calls, &body)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got, err := c.WriteCheckVariant(context.Background(), "zvsp_logic", checkVariantDoc)
	if err != nil {
		t.Fatal(err)
	}
	if body != checkVariantDoc {
		t.Errorf("PUT body %q", body)
	}
	if got.Name != "ZVSP_LOGIC" || len(got.Checks) != 2 {
		t.Errorf("result %+v", got)
	}
	if err := c.DeleteCheckVariant(context.Background(), "zvsp_logic"); err != nil {
		t.Fatal(err)
	}
	if want := []string{http.MethodPut, http.MethodDelete}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
}

func TestListCodeInspectorChecks(t *testing.T) {
	var calls []string
	var body string
	srv := ciServer(t, "/ci", &calls, &body)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	got, err := c.ListCodeInspectorChecks(context.Background(), "cl_ci_test_sysubrc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != "002" || len(got[0].Params) != 1 {
		t.Errorf("checks %+v", got)
	}
	if want := []string{"GET checks CL_CI_TEST_SYSUBRC"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
}

func TestCheckVariantNeedsTheSet(t *testing.T) {
	var calls []string
	var body string
	srv := ciServer(t, "/textelements", &calls, &body)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	if _, err := c.GetCheckVariant(context.Background(), "ZVSP_LOGIC"); err == nil || !strings.Contains(err.Error(), "0.10.0") {
		t.Errorf("get: %v", err)
	}
	if _, err := c.WriteCheckVariant(context.Background(), "ZVSP_LOGIC", checkVariantDoc); err == nil || !strings.Contains(err.Error(), "0.10.0") {
		t.Errorf("write: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("calls %v", calls)
	}
}

func TestCheckVariantWritesPassTheSafetyGate(t *testing.T) {
	var calls []string
	var body string
	srv := ciServer(t, "/ci", &calls, &body)
	defer srv.Close()

	readOnly := NewClient(srv.URL, "user", "pass", WithReadOnly())
	if _, err := readOnly.WriteCheckVariant(context.Background(), "ZVSP_LOGIC", checkVariantDoc); err == nil {
		t.Error("write in read-only mode went through")
	}
	if err := readOnly.DeleteCheckVariant(context.Background(), "ZVSP_LOGIC"); err == nil {
		t.Error("delete in read-only mode went through")
	}
	packages := NewClient(srv.URL, "user", "pass", WithAllowedPackages("$ZDEMO"))
	if _, err := packages.WriteCheckVariant(context.Background(), "ZVSP_LOGIC", checkVariantDoc); err == nil || !strings.Contains(err.Error(), "belongs to no package") {
		t.Errorf("write with allowed packages: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("calls %v", calls)
	}
	// Reading stays allowed.
	if _, err := readOnly.GetCheckVariant(context.Background(), "ZVSP_LOGIC"); err != nil {
		t.Errorf("read in read-only mode: %v", err)
	}
}

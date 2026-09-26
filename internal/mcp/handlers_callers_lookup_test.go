package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// A Russian logon gets the module's name back from the search with its kind
// appended, "BAL_MSG_DISPLAY_ABAP (Функциональный модуль)", and comparing
// names found nothing: GetCallersOf said the module was not in the
// repository. The URI carries the plain name.
func TestCallGraphObjectURIFindsAModuleWhateverTheLogonLanguage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		w.Header().Set("Content-Type", "application/xml")
		if !strings.Contains(r.URL.Path, "informationsystem/search") {
			return
		}
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>` +
			`<adtcore:objectReferences xmlns:adtcore="http://www.sap.com/adt/core">` +
			`<adtcore:objectReference adtcore:uri="/sap/bc/adt/functions/groups/sbal_display_message/fmodules/bal_msg_display_abap_x" adtcore:type="FUGR/FF" adtcore:name="BAL_MSG_DISPLAY_ABAP_X (Функциональный модуль)"/>` +
			`<adtcore:objectReference adtcore:uri="/sap/bc/adt/functions/groups/sbal_display_message/fmodules/bal_msg_display_abap" adtcore:type="FUGR/FF" adtcore:name="BAL_MSG_DISPLAY_ABAP (Функциональный модуль)"/>` +
			`</adtcore:objectReferences>`))
	}))
	defer srv.Close()

	s := &Server{adtClient: adt.NewClient(srv.URL, "user", "pass")}
	uri, err := s.callGraphObjectURI(context.Background(), newRequest(map[string]any{
		"object_type": "FUNC", "object_name": "bal_msg_display_abap",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if uri != "/sap/bc/adt/functions/groups/sbal_display_message/fmodules/bal_msg_display_abap" {
		t.Errorf("uri = %q", uri)
	}
}

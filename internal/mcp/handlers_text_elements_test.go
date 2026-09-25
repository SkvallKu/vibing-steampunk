package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// GetTextElements went through the ZADT_VSP WebSocket, so on a system reached
// only by RFC through a SAProuter it timed out dialling the ICM port, and on
// any system without ZADT_VSP it could not answer at all. It reads the text
// pool over ADT now, with no WebSocket client on the server.

func textElementsServer(t *testing.T, seen *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.Path)
		w.Header().Set("x-csrf-token", "test-token")
		w.Header().Set("Content-Type", "text/plain")
		// The name's case in the path differs between the read paths; SAP
		// does not mind, and neither does this.
		path := strings.ToLower(r.URL.Path)
		switch {
		case strings.HasSuffix(path, "/textelements/programs/zdemo_run/source/selections"):
			w.Write([]byte("P_DEVC  =Package to scan\n"))
		case strings.HasSuffix(path, "/textelements/programs/zdemo_run/source/symbols"):
			w.Write([]byte("@MaxLength:20\n001=Nothing found\n"))
		default:
			// No headings: a 404 is an empty kind, not a missing text pool.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestGetTextElementsReadsOverADT(t *testing.T) {
	var seen []string
	srv := textElementsServer(t, &seen)
	defer srv.Close()

	s := &Server{adtClient: adt.NewClient(srv.URL, "user", "pass")}
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]any{"program": "zdemo_run", "language": "E"}

	result, err := s.handleGetTextElements(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if result.IsError {
		t.Fatalf("the text pool was there to read:\n%s", toolResultText(t, result))
	}
	text := toolResultText(t, result)
	for _, want := range []string{
		"Text Elements for ZDEMO_RUN (Language: E)",
		"P_DEVC: Package to scan",
		"TEXT-001: Nothing found",
		"Heading Texts:\n  (none)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if s.amdpWSClient != nil {
		t.Error("reading texts must not open a WebSocket to ZADT_VSP")
	}
	for _, p := range seen {
		if strings.Contains(p, "/sap/bc/apc/") {
			t.Errorf("request to the APC handler: %s", p)
		}
	}
}

func TestSetTextElementsRefusesBadJSONBeforeTouchingTheSystem(t *testing.T) {
	var seen []string
	srv := textElementsServer(t, &seen)
	defer srv.Close()

	s := &Server{adtClient: adt.NewClient(srv.URL, "user", "pass")}
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]any{"program": "ZDEMO_RUN", "selection_texts": "{not json"}

	result, err := s.handleSetTextElements(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !result.IsError || !strings.Contains(toolResultText(t, result), "selection_texts") {
		t.Fatalf("want an error naming selection_texts, got:\n%s", toolResultText(t, result))
	}
	if len(seen) != 0 {
		t.Errorf("nothing should reach the system, got %v", seen)
	}
}

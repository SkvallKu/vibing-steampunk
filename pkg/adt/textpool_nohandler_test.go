package adt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// On 7.50 there is no /sap/bc/adt/textelements at all: every document
// answers 404 "No application class found for URI". Read as "this kind is
// empty", that came back as a program with no texts — RSUSR002, with a
// hundred of them, read as none. A missing resource is an error; a missing
// kind on a system that has the resource is still an empty kind.

func textPoolServer(noHandler bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		path := strings.ToLower(r.URL.Path)
		switch {
		case noHandler:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No application class found for URI: " + r.URL.Path))
		case strings.HasSuffix(path, "/source/symbols"):
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("001=Nothing found\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Resource does not exist"))
		}
	}))
}

func TestTextPoolWithoutTheResourceIsAnError(t *testing.T) {
	srv := textPoolServer(true)
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	for _, target := range []TextPoolTarget{{Type: "PROG", Name: "RSUSR002"}, {Type: "CLAS", Name: "ZCL_DEMO"}} {
		entries, err := c.TextPool(context.Background(), target, "E")
		if err == nil {
			t.Fatalf("%s: want an error, got %d entries and no error", target, len(entries))
		}
		if !strings.Contains(err.Error(), "no ADT text elements resource") {
			t.Errorf("%s: the error must say the resource is missing, not the texts: %v", target, err)
		}
	}
}

func TestTextPoolMissingKindIsStillEmpty(t *testing.T) {
	srv := textPoolServer(false)
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	entries, err := c.TextPool(context.Background(), TextPoolTarget{Name: "ZDEMO"}, "E")
	if err != nil {
		t.Fatalf("a kind that is not there is an empty kind: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "I" || entries[0].Key != "001" {
		t.Errorf("want the one symbol, got %+v", entries)
	}
}

func TestIsNoHandlerError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{&APIError{StatusCode: 404, Message: "No application class found for URI: /sap/bc/adt/textelements/x"}, true},
		{&APIError{StatusCode: 404, Message: "Resource does not exist"}, false},
		{&APIError{StatusCode: 500, Message: "No application class found"}, false},
		{nil, false},
	} {
		if got := IsNoHandlerError(tc.err); got != tc.want {
			t.Errorf("IsNoHandlerError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

package adt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestSingleTableOf(t *testing.T) {
	for q, want := range map[string]string{
		"SELECT MANDT, MTEXT FROM t000 WHERE MANDT = '100'": "T000",
		"SELECT * FROM /BEV1/LUTRANSP":                      "/BEV1/LUTRANSP",
		"SELECT a~x FROM a INNER JOIN b ON a~k = b~k":       "",
		"SELECT * FROM a WHERE x IN ( SELECT y FROM b )":    "",
		"UPDATE t000 SET mtext = 'x'":                       "",
	} {
		if got := singleTableOf(q); got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
}

// On 7.40 before SP08 a query was a 404 from freestyle, a 400 from the
// classic parser for the commas, and the query again: three requests, each
// data preview one generating a subroutine pool. Once the client has seen
// both, the next query is one request in the form that works.
func TestRunQueryRemembersAnOldDataPreview(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.Method != http.MethodPost {
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		asked = append(asked, r.URL.Path+" "+string(body))
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/freestyle"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No suitable resource found"))
		case strings.Contains(string(body), ","):
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Explicit length specifications are necessary with types C, P, X, N und W in the OO context"))
		default:
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(tableXML(col("MANDT", "100"), col("MTEXT", "Demo"))))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	for i, want := range []int{3, 1} {
		mu.Lock()
		asked = nil
		mu.Unlock()
		res, err := c.RunQuery(context.Background(), "SELECT MANDT, MTEXT FROM T000 WHERE MANDT = '100'", 10)
		if err != nil || len(res.Rows) != 1 {
			t.Fatalf("query %d: %v %+v", i+1, err, res)
		}
		mu.Lock()
		got := append([]string(nil), asked...)
		mu.Unlock()
		if len(got) != want {
			t.Errorf("query %d: %d requests, want %d: %q", i+1, len(got), want, got)
		}
	}

	// A query no single table answers says freestyle is missing without
	// asking for it again.
	mu.Lock()
	asked = nil
	mu.Unlock()
	if _, err := c.RunQuery(context.Background(), "SELECT a~x FROM a INNER JOIN b ON a~k = b~k", 10); err == nil || !strings.Contains(err.Error(), "no freestyle SQL") {
		t.Errorf("join: %v", err)
	}
	if len(asked) != 0 {
		t.Errorf("freestyle was asked again: %q", asked)
	}
}

// A 404 that is an exception document is about the query, not a missing
// resource, and is not remembered as one.
func TestRunQueryForgetsNothingOnAnObjectNotFound(t *testing.T) {
	freestyle := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if strings.HasSuffix(r.URL.Path, "/freestyle") {
			freestyle++
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`<exc:exception xmlns:exc="x"><type id="ExceptionResourceNotFound"/></exc:exception>`))
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(tableXML(col("MANDT", "100"))))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")
	for i := 0; i < 2; i++ {
		_, _ = c.RunQuery(context.Background(), "SELECT MANDT FROM T000", 10)
	}
	if freestyle != 2 {
		t.Errorf("freestyle asked %d times, want 2", freestyle)
	}
}

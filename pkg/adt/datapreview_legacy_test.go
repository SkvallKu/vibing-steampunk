package adt

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeDataPreviewSQL(t *testing.T) {
	tests := map[string]string{
		// The ERP_218 shapes: IN list and a bracketed OR, both tight.
		"SELECT * FROM DD03L WHERE TABNAME IN ('A','B,C','D') AND X = ','": "SELECT * FROM DD03L WHERE TABNAME IN ( 'A' , 'B,C' , 'D' ) AND X = ','",
		"SELECT * FROM T WHERE (A = '(x)' OR B = 'y') AND C = 1":           "SELECT * FROM T WHERE ( A = '(x)' OR B = 'y' ) AND C = 1",
		// Already spaced: unchanged.
		"SELECT * FROM T WHERE A IN ( 'X' , 'Y' )": "SELECT * FROM T WHERE A IN ( 'X' , 'Y' )",
		// A comma touching the literal before it: the service's cut can
		// double that quote (see the top of datapreview_legacy.go).
		"SELECT * FROM T WHERE A IN ( 'X', 'Y' )": "SELECT * FROM T WHERE A IN ( 'X' , 'Y' )",
		"SELECT COUNT(*) FROM T":                  "SELECT COUNT( * ) FROM T",
		"SELECT f() FROM T":                       "SELECT f() FROM T",
		// A query written over several lines: on 7.40 SP06 the break is not
		// white space and "DD03L\nWHERE" is read as a table name. A break
		// inside a literal is the literal's and stays.
		"SELECT *\r\nFROM DD03L\nWHERE TABNAME = 'A\nB'\tAND X = 1": "SELECT *  FROM DD03L WHERE TABNAME = 'A\nB' AND X = 1",
	}
	for in, want := range tests {
		if got := normalizeDataPreviewSQL(in); got != want {
			t.Errorf("%q:\n got  %q\n want %q", in, got, want)
		}
	}
}

func TestLegacyDataPreviewSQL(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{
			// The ERP_218 query: projection plus ORDER BY with commas.
			in:   "SELECT TABNAME, POSITION, FIELDNAME FROM DD03L WHERE TABNAME LIKE 'Z%' ORDER BY TABNAME, POSITION",
			want: "SELECT TABNAME POSITION FIELDNAME FROM DD03L WHERE TABNAME LIKE 'Z%' ORDER BY TABNAME POSITION",
			ok:   true,
		},
		{
			// IN keeps its commas, and so does a literal.
			in:   "SELECT A, B FROM T WHERE A IN ( 'X', 'Y' ) AND B = ','",
			want: "SELECT A B FROM T WHERE A IN ( 'X', 'Y' ) AND B = ','",
			ok:   true,
		},
		{
			in: "SELECT * FROM T000 WHERE MANDT IN ( '100', '200' )",
			ok: false,
		},
	}
	for _, tt := range tests {
		got, ok := legacyDataPreviewSQL(tt.in)
		if ok != tt.ok {
			t.Errorf("%q: ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("%q:\n got  %q\n want %q", tt.in, got, tt.want)
		}
	}
}

// commaSensitivePreview answers the data preview the way 7.40 SP06 does for a
// statement that names a field the table does not have: with commas between
// the columns the generated program fails on them, without commas the
// parser gets as far as the field.
type commaSensitivePreview struct{ bodies []string }

func (m *commaSensitivePreview) Do(req *http.Request) (*http.Response, error) {
	h := http.Header{}
	h.Set("X-CSRF-Token", "t")
	msg := "ok"
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		m.bodies = append(m.bodies, string(b))
		msg = "The field NOSUCHFIELD is unknown"
		if strings.Contains(string(b), ",") {
			msg = "Explicit length specifications are necessary with types C, P, X, and N in the OO context"
		}
	}
	status := http.StatusBadRequest
	if req.Body == nil {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(msg)), Header: h}, nil
}

func TestGetTableContents_RetryErrorIsReported(t *testing.T) {
	mock := &commaSensitivePreview{}
	cfg := NewConfig("https://sap.example.com:44300", "u", "p")
	client := NewClientWithTransport(cfg, NewTransportWithClient(cfg, mock))

	_, err := client.GetTableContents(context.Background(), "T000", 10, "SELECT MANDT, NOSUCHFIELD FROM T000")
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(mock.bodies) != 2 {
		t.Fatalf("expected the statement and its retry without commas, got %q", mock.bodies)
	}
	msg := err.Error()
	field := strings.Index(msg, "NOSUCHFIELD is unknown")
	oo := strings.Index(msg, "OO context")
	if field < 0 || oo < 0 || field > oo {
		t.Errorf("want the retry's error first, then the first attempt's; got: %v", err)
	}
}

package adt

import "testing"

func TestNormalizeDataPreviewSQL(t *testing.T) {
	tests := map[string]string{
		// The ERP_218 shapes: IN list and a bracketed OR, both tight.
		"SELECT * FROM DD03L WHERE TABNAME IN ('A','B,C','D') AND X = ','": "SELECT * FROM DD03L WHERE TABNAME IN ( 'A', 'B,C', 'D' ) AND X = ','",
		"SELECT * FROM T WHERE (A = '(x)' OR B = 'y') AND C = 1":           "SELECT * FROM T WHERE ( A = '(x)' OR B = 'y' ) AND C = 1",
		// Already spaced: unchanged.
		"SELECT * FROM T WHERE A IN ( 'X', 'Y' )": "SELECT * FROM T WHERE A IN ( 'X', 'Y' )",
		"SELECT COUNT(*) FROM T":                  "SELECT COUNT( * ) FROM T",
		"SELECT f() FROM T":                       "SELECT f() FROM T",
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

package saprfc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/open-rfc-go/rfc"
)

func TestSplitWhereClause(t *testing.T) {
	cases := []struct {
		name  string
		where string
		want  int // expected number of OPTIONS rows
	}{
		{"empty", "", 0},
		{"short", "MANDT = '001'", 1},
		{"exactly 72", strings.Repeat("A", 71) + "B", 1},
		{"73 chars", "FUNCNAME LIKE 'RFC_READ%' OR FUNCNAME LIKE 'RFC_PING%' OR FUNCNAME LIKE 'STFC%'", 2},
		{"long", "FUNCNAME LIKE 'RFC_READ%' OR FUNCNAME LIKE 'RFC_PING%' OR FUNCNAME LIKE 'STFC%' OR FUNCNAME LIKE 'BAPI_USER%' AND FMODE = 'R'", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitWhereClause(tc.where)
			if err != nil {
				t.Fatalf("splitWhereClause(%q): %v", tc.where, err)
			}
			if len(got) != tc.want {
				t.Errorf("got %d rows, want %d: %q", len(got), tc.want, got)
			}
			for i, line := range got {
				if len(line) > optionsLineLen {
					t.Errorf("row %d is %d chars (max %d): %q", i, len(line), optionsLineLen, line)
				}
				if line != strings.TrimSpace(line) {
					t.Errorf("row %d has stray edge whitespace: %q", i, line)
				}
			}
			// ABAP joins the OPTIONS rows with a blank; the clause must come
			// back identical (token for token, in order).
			if rejoined := strings.Join(got, " "); rejoined != strings.Join(strings.Fields(tc.where), " ") {
				t.Errorf("rejoined %q != original %q", rejoined, tc.where)
			}
		})
	}
}

// A clause packed with long tokens must still keep every token whole.
func TestSplitWhereClauseNeverSplitsAToken(t *testing.T) {
	// Long, awkward tokens are the point of this case; the names are synthetic
	// on purpose, because a real logon name in a tracked test is a leak.
	where := "BNAME LIKE 'DEVELOPER%' OR BNAME LIKE 'TESTUSER_LONGNAME%' OR BNAME LIKE 'ZDEMO_SVCUSER%' OR BNAME = 'DDIC'"
	rows, err := splitWhereClause(where)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected the clause to be split, got %d row(s)", len(rows))
	}
	var seen []string
	for _, r := range rows {
		if len(r) > optionsLineLen {
			t.Errorf("row too long (%d): %q", len(r), r)
		}
		seen = append(seen, strings.Fields(r)...)
	}
	want := strings.Fields(where)
	if len(seen) != len(want) {
		t.Fatalf("token count %d != %d", len(seen), len(want))
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("token %d: got %q, want %q", i, seen[i], want[i])
		}
	}
}

// A single token wider than the OPTIONS line cannot be expressed; that must be
// an error rather than a silent truncation.
func TestSplitWhereClauseRejectsOversizedToken(t *testing.T) {
	where := "BNAME = '" + strings.Repeat("X", 80) + "' OR MANDT = '001'"
	if _, err := splitWhereClause(where); err == nil {
		t.Fatal("expected an error for a token longer than the OPTIONS line")
	}
}

// fakeReadTable answers RFC_READ_TABLE with errs in turn, then an empty result,
// and keeps what it was asked.
type fakeReadTable struct {
	errs  []error
	calls []rfc.Params
}

func (f *fakeReadTable) Call(_ context.Context, _ string, in rfc.Params) (rfc.Result, error) {
	cp := rfc.Params{}
	for k, v := range in {
		cp[k] = v
	}
	f.calls = append(f.calls, cp)
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return rfc.Result{}, err
	}
	return rfc.Result{}, nil
}

func TestReadTableNoRowsIsEmptyNotNil(t *testing.T) {
	rows, err := ReadTable(context.Background(), &fakeReadTable{}, "TFDIR", "FUNCNAME = 'Z_NONE'", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("rows = %#v, want an empty slice", rows)
	}
}

// On 7.50 a wide row raises DATA_BUFFER_EXCEEDED and RFC_READ_TABLE has no
// USE_ET_DATA_4_RETURN to retry with.
func TestReadTableWideRowWithoutETData(t *testing.T) {
	f := &fakeReadTable{errs: []error{
		&rfc.ABAPException{Kind: rfc.KindException, Key: "DATA_BUFFER_EXCEEDED"},
		fmt.Errorf("%w: RFC_READ_TABLE.USE_ET_DATA_4_RETURN", rfc.ErrUnknownParameter),
	}}
	_, err := ReadTable(context.Background(), f, "BADI_CHAR_COND", "", nil, 1)
	if err == nil || !strings.Contains(err.Error(), "fewer fields") || !strings.Contains(err.Error(), "vsp query") {
		t.Fatalf("err = %v", err)
	}
	if len(f.calls) != 2 || f.calls[1]["USE_ET_DATA_4_RETURN"] != "X" {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestReadTableStringColumnDump(t *testing.T) {
	dump := &rfc.ABAPException{Kind: rfc.KindRuntime, PlainText: "Error with ASSIGN ... CASTING in program SAPLSDTX"}
	_, err := ReadTable(context.Background(), &fakeReadTable{errs: []error{dump}}, "BADI_STRING_COND", "", []string{"VALUE1"}, 1)
	if err == nil || !strings.Contains(err.Error(), "STRING or RAWSTRING") || !errors.Is(err, dump) {
		t.Fatalf("err = %v", err)
	}
	other := &rfc.ABAPException{Kind: rfc.KindException, Key: "TABLE_NOT_AVAILABLE"}
	if _, err := ReadTable(context.Background(), &fakeReadTable{errs: []error{other}}, "ZNONE", "", nil, 1); err != other {
		t.Fatalf("err = %v, want it passed through", err)
	}
}

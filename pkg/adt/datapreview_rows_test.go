package adt

import (
	"strings"
	"testing"
)

// 7.40 SP06 data preview answers a rowNumber above its limit (between 5,000
// and 9,999) with its default of 100 rows, and nothing in the answer says
// so. vsp does not ask again; it marks the result, and only where the mark
// can be right.

func hundredRows() *TableContentsResult {
	return &TableContentsResult{Rows: make([]map[string]interface{}, 100)}
}

func TestExactlyAHundredRowsForALargeRequestIsNoted(t *testing.T) {
	res := noteRowFallback(hundredRows(), UnlimitedRows)
	if !strings.Contains(strings.Join(res.Notes, " "), "90000 rows asked for, exactly 100 returned") {
		t.Errorf("notes: %q", res.Notes)
	}
}

func TestAHundredRowsWithinTheLimitAreJustAHundredRows(t *testing.T) {
	// 7.40 honours up to 5,000: 100 rows back for 300 asked is the answer.
	for _, asked := range []int{100, 300, rowFallbackAbove} {
		if res := noteRowFallback(hundredRows(), asked); len(res.Notes) != 0 {
			t.Errorf("asked %d: no note wanted, got %q", asked, res.Notes)
		}
	}
}

func TestOtherCountsAreNotNoted(t *testing.T) {
	res := noteRowFallback(&TableContentsResult{Rows: make([]map[string]interface{}, 6252)}, 20000)
	if len(res.Notes) != 0 {
		t.Errorf("got %q", res.Notes)
	}
	if noteRowFallback(nil, 20000) != nil {
		t.Error("nil stays nil")
	}
}

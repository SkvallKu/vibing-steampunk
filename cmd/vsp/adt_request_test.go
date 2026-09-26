package main

import (
	"net/url"
	"testing"
)

func TestIsLockRequest(t *testing.T) {
	for _, tc := range []struct {
		path  string
		query url.Values
		want  bool
	}{
		{"/sap/bc/adt/programs/programs/ztest", url.Values{"_action": {"LOCK"}}, true},
		{"/sap/bc/adt/programs/programs/ztest?_action=LOCK&accessMode=MODIFY", nil, true},
		{"/sap/bc/adt/programs/programs/ztest?_action=lock", url.Values{}, true},
		{"/sap/bc/adt/programs/programs/ztest?_action=UNLOCK&lockHandle=x", nil, false},
		{"/sap/bc/adt/programs/programs/ztest/source/main", url.Values{"lockHandle": {"x"}}, false},
	} {
		if got := isLockRequest(tc.path, tc.query); got != tc.want {
			t.Errorf("isLockRequest(%q, %v) = %v, want %v", tc.path, tc.query, got, tc.want)
		}
	}
}

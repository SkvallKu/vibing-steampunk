package adt

import (
	"reflect"
	"testing"
)

// A warning (ERH: "redundant conversion") does not stop a deploy; only E, A
// and X do, as in WriteSource.
func TestSplitSyntaxResults(t *testing.T) {
	errs, warns := splitSyntaxResults([]SyntaxCheckResult{
		{Line: 3, Severity: "W", Text: "Redundant conversion"},
		{Line: 5, Severity: "E", Text: "Field X is unknown"},
		{Line: 7, Severity: "I", Text: "Info"},
		{Line: 9, Severity: "A", Text: "Abort"},
	})
	if want := []string{"Line 5: Field X is unknown", "Line 9: Abort"}; !reflect.DeepEqual(errs, want) {
		t.Errorf("errors = %q, want %q", errs, want)
	}
	if want := []string{"Line 3: Redundant conversion", "Line 7: Info"}; !reflect.DeepEqual(warns, want) {
		t.Errorf("warnings = %q, want %q", warns, want)
	}
	if errs, _ := splitSyntaxResults([]SyntaxCheckResult{{Line: 1, Severity: "W", Text: "w"}}); errs != nil {
		t.Errorf("warnings only: errors = %q, want none", errs)
	}
}

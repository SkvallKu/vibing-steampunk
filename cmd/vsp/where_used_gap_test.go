package main

import (
	"strings"
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// `vsp graph --direction callers` prints the unresolved includes beside the
// table, so a capped or failed lookup does not read as a complete answer.
func TestPrintWhereUsedCallersSaysWhichIncludesStayedUnresolved(t *testing.T) {
	out := captureStdout(t, func() {
		printWhereUsedCallers(&adt.WhereUsedResult{
			Source:     adt.WhereUsedSourceList,
			Callers:    []adt.ExposedCaller{{Name: "ZDEMO_INCL", Type: "PROG/I", Package: "$ZDEMO"}},
			Unresolved: []adt.Unsearched{{Object: "ZDEMO_INCL", Reason: "not resolved to its main program: status 500"}},
		})
	})
	if !strings.Contains(out, "not a complete answer") || !strings.Contains(out, "ZDEMO_INCL: not resolved") {
		t.Errorf("output does not name the unresolved include:\n%s", out)
	}
}

// Callers from the cross-reference tables are coarser than SE84's. The CLI
// used to print them as "from the where-used list"; the source now comes
// first, and the empty answer names it too.
func TestPrintWhereUsedCallersNamesTheTables(t *testing.T) {
	xref := &adt.WhereUsedResult{Source: adt.WhereUsedSourceXref, Why: adt.WhereUsedWhyNoList,
		Callers: []adt.ExposedCaller{{Name: "ZDEMO_CALLER", Type: "CLAS/OC", Package: "$ZDEMO"}}}
	out := captureStdout(t, func() { printWhereUsedCallers(xref) })
	if !strings.HasPrefix(out, "Callers read from the cross-reference tables") || !strings.Contains(out, "no where-used list") {
		t.Errorf("the source is not said first:\n%s", out)
	}
	xref.Callers = nil
	out = captureStdout(t, func() { printWhereUsedCallers(xref) })
	if strings.Contains(out, "The where-used list answered") || !strings.Contains(out, "The cross-reference tables answered and it is empty") {
		t.Errorf("empty answer from the tables:\n%s", out)
	}
	list := &adt.WhereUsedResult{Source: adt.WhereUsedSourceList}
	if out := captureStdout(t, func() { printWhereUsedCallers(list) }); !strings.HasPrefix(out, "The where-used list answered") {
		t.Errorf("empty where-used list:\n%s", out)
	}
}

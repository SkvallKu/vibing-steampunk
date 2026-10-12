package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// The caveat of an answer from the cross-reference tables comes before the
// callers. As a plain map the answer came out in alphabetical order, so it
// stood after the whole list.
func TestCallGraphReplyPutsTheSourceFirst(t *testing.T) {
	reply := callGraphReply{
		"callers":    []string{"ZA", "ZB"},
		"caveat":     callersXrefCaveat,
		"source":     sourceCallersXref + adt.WhereUsedWhyNoList,
		"total":      2,
		"object_uri": "/sap/bc/adt/ddic/tables/zmara",
		"direction":  "callers",
		"zzz_extra":  true,
	}
	out, err := json.MarshalIndent(reply, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	order := []string{"object_uri", "direction", "source", "caveat", "total", "zzz_extra", "callers"}
	last := -1
	for _, k := range order {
		i := strings.Index(text, `"`+k+`":`)
		if i <= last {
			t.Fatalf("%s out of order:\n%s", k, text)
		}
		last = i
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil || len(back) != len(reply) {
		t.Errorf("not the same object back: %v %v", err, back)
	}
}

// Dump impact names the units whose callers came from the tables among its
// notes, not only inside units[].
func TestImpactNotesNameUnitsFromTheTables(t *testing.T) {
	result := &adt.DumpImpactResult{Units: []adt.ImpactUnit{
		{Object: "ZCL_DEMO", Source: adt.WhereUsedSourceXref, Caveat: "read from the cross-reference tables"},
		{Object: "ZDEMO_REPORT"},
	}}
	notes := strings.Join(impactNotes(result), "\n")
	if !strings.Contains(notes, "1 of 2 units") || !strings.Contains(notes, "ZCL_DEMO: read from the cross-reference tables") {
		t.Errorf("notes:\n%s", notes)
	}
}

package adt

import (
	"strings"
	"testing"
)

func TestFormatDDICSource(t *testing.T) {
	row := func(kv ...string) map[string]interface{} {
		m := map[string]interface{}{"DEPTH": "00", "ADMINFIELD": "0"}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	rows := []map[string]interface{}{
		row("FIELDNAME", "MATNR", "POSITION", "0002", "KEYFLAG", "X", "ROLLNAME", "MATNR", "COMPTYPE", "E", "NOTNULL", "X"),
		row("FIELDNAME", "MANDT", "POSITION", "0001", "KEYFLAG", "X", "ROLLNAME", "MANDT", "COMPTYPE", "E", "NOTNULL", "X"),
		row("FIELDNAME", ".INCLUDE", "POSITION", "0003", "PRECFIELD", "EMARA", "COMPTYPE", "S"),
		// An include's field, listed in place: not the table's own line.
		row("FIELDNAME", "ERSDA", "POSITION", "0004", "ROLLNAME", "ERSDA", "COMPTYPE", "E", "ADMINFIELD", "1"),
		row("FIELDNAME", "NOTE", "POSITION", "0005", "DATATYPE", "STRG", "LENG", "000000"),
		row("FIELDNAME", "AMOUNT", "POSITION", "0006", "DATATYPE", "CURR", "LENG", "000015", "DECIMALS", "000002"),
		// A component of a nested structure.
		row("FIELDNAME", "INNER", "POSITION", "0007", "DATATYPE", "CHAR", "LENG", "000010", "DEPTH", "01"),
	}
	got := formatDDICSource("ZMARA", "TRANSP", "Material", rows)
	for _, want := range []string{
		"@EndUserText.label : 'Material'",
		"define table zmara {",
		"key mandt : mandt not null;",
		"key matnr : matnr not null;",
		"include emara;",
		"note      : abap.string(0);",
		"amount    : abap.curr(15,2);",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "ersda") || strings.Contains(got, "inner") {
		t.Errorf("include or nested fields leaked:\n%s", got)
	}
	if strings.Index(got, "mandt") > strings.Index(got, "matnr") {
		t.Errorf("not in POSITION order:\n%s", got)
	}
}

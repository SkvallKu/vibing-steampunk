package adt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

// ddicDoer answers data preview for DD02L and DD03L the way 7.40 SP06 does:
// a request above rowFallbackAbove rows gets the default of 100.
type ddicDoer struct {
	fields int
	asked  []int
}

func (d *ddicDoer) Do(req *http.Request) (*http.Response, error) {
	resp := func(body string) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"X-Csrf-Token": []string{"t"}}}, nil
	}
	if req.URL.Path != "/sap/bc/adt/datapreview/ddic" {
		return resp("")
	}
	switch req.URL.Query().Get("ddicEntityName") {
	case "DD02L":
		return resp(dataPreviewBody([]string{"TABNAME", "TABCLASS"}, [][]string{{"ZWIDE", "TRANSP"}}))
	case "DD03L":
		n, _ := strconv.Atoi(req.URL.Query().Get("rowNumber"))
		d.asked = append(d.asked, n)
		rows := d.fields
		if n > rowFallbackAbove {
			rows = 100
		}
		if rows > n {
			rows = n
		}
		var out [][]string
		for i := 1; i <= rows; i++ {
			out = append(out, []string{fmt.Sprintf("F%04d", i), fmt.Sprintf("%04d", i), "00", "0", "CHAR", "000010"})
		}
		return resp(dataPreviewBody([]string{"FIELDNAME", "POSITION", "DEPTH", "ADMINFIELD", "DATATYPE", "LENG"}, out))
	}
	return resp(dataPreviewBody([]string{"X"}, nil))
}

// On 7.40 SP06 a DD03L read asked for more than rowFallbackAbove rows came
// back as 100, and a table of more fields was written as if it had 100.
func TestDDICSourceFromTablesReadsEveryField(t *testing.T) {
	d := &ddicDoer{fields: 150}
	cfg := NewConfig("https://sap.example.com", "u", "p")
	c := NewClientWithTransport(cfg, NewTransportWithClient(cfg, d))
	src, err := c.ddicSourceFromTables(context.Background(), "ZWIDE")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"f0001", "f0100", "f0150"} {
		if !strings.Contains(src, f) {
			t.Errorf("field %s missing from\n%s", f, src)
		}
	}
	if len(d.asked) != 1 || d.asked[0] > rowFallbackAbove {
		t.Errorf("DD03L asked for %v rows, want one request of at most %d", d.asked, rowFallbackAbove)
	}
}

// A table whose DD03L fills the whole request may have more: refused, not
// written short.
func TestDDICSourceFromTablesRefusesAFullRead(t *testing.T) {
	d := &ddicDoer{fields: rowFallbackAbove + 1}
	cfg := NewConfig("https://sap.example.com", "u", "p")
	c := NewClientWithTransport(cfg, NewTransportWithClient(cfg, d))
	_, err := c.ddicSourceFromTables(context.Background(), "ZWIDE")
	if err == nil || !strings.Contains(err.Error(), "DD03L") {
		t.Fatalf("err = %v, want the refusal", err)
	}
}

package adt

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// --- DDIC definition from the dictionary tables ---

// ddicSourceFromTables writes a table's or structure's definition in the
// shape of the DDL source newer releases serve at .../source/main, from
// DD02L, DD02T and DD03L — for releases that serve no source for DDIC
// tables at all (before 7.52). It reads the active version only.
func (c *Client) ddicSourceFromTables(ctx context.Context, name string) (string, error) {
	lit := strings.ReplaceAll(name, "'", "''")
	head, err := c.GetTableContents(ctx, "DD02L", 5,
		fmt.Sprintf("SELECT * FROM DD02L WHERE TABNAME = '%s' AND AS4LOCAL = 'A'", lit))
	if err != nil {
		return "", err
	}
	if len(head.Rows) == 0 {
		return "", fmt.Errorf("%s is not an active table or structure in DD02L", name)
	}
	tabclass := strings.TrimSpace(asString(head.Rows[0]["TABCLASS"]))
	if tabclass == "VIEW" {
		return "", fmt.Errorf("%s is a view, not a table", name)
	}

	fields, err := c.GetTableContents(ctx, "DD03L", UnlimitedRows,
		fmt.Sprintf("SELECT * FROM DD03L WHERE TABNAME = '%s' AND AS4LOCAL = 'A'", lit))
	if err != nil {
		return "", err
	}

	label := ""
	if texts, err := c.GetTableContents(ctx, "DD02T", 50,
		fmt.Sprintf("SELECT * FROM DD02T WHERE TABNAME = '%s' AND AS4LOCAL = 'A'", lit)); err == nil {
		label = pickText(texts.Rows, "DDLANGUAGE", "DDTEXT", c.config.Language)
	}

	return formatDDICSource(name, tabclass, label, fields.Rows), nil
}

// pickText takes the text in the session language, else English, else any.
func pickText(rows []map[string]interface{}, langCol, textCol, lang string) string {
	// DDLANGUAGE is the one-character key: E, R, D. The session language is
	// usually given as ISO (EN, RU); their first letters match for the
	// common ones.
	want := []string{strings.ToUpper(lang)}
	if len(lang) >= 1 {
		want = append(want, strings.ToUpper(lang[:1]))
	}
	want = append(want, "EN", "E")
	for _, w := range want {
		for _, r := range rows {
			if strings.EqualFold(strings.TrimSpace(asString(r[langCol])), w) {
				return strings.TrimSpace(asString(r[textCol]))
			}
		}
	}
	if len(rows) > 0 {
		return strings.TrimSpace(asString(rows[0][textCol]))
	}
	return ""
}

// ddicNoLength lists the built-in types DDL writes without a length.
var ddicNoLength = map[string]bool{
	"DATS": true, "TIMS": true, "INT1": true, "INT2": true, "INT4": true, "INT8": true,
	"FLTP": true, "LANG": true, "ACCP": true, "PREC": true, "DATN": true, "TIMN": true,
	"UTCL": true, "D16N": true, "D34N": true,
}

// ddicWithDecimals lists the built-in types DDL writes as (length, decimals).
var ddicWithDecimals = map[string]bool{
	"DEC": true, "CURR": true, "QUAN": true, "D16D": true, "D34D": true,
	"D16R": true, "D34R": true, "D16S": true, "D34S": true,
}

func formatDDICSource(name, tabclass, label string, rows []map[string]interface{}) string {
	// DD03L lists the fields of every include in place, with ADMINFIELD
	// counting the include depth, and the components of a nested structure
	// with DEPTH counting the nesting. The table's own lines are the ones at
	// zero on both.
	own := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		if atoiTrim(asString(r["DEPTH"])) != 0 || atoiTrim(asString(r["ADMINFIELD"])) != 0 {
			continue
		}
		own = append(own, r)
	}
	sort.SliceStable(own, func(i, j int) bool {
		return atoiTrim(asString(own[i]["POSITION"])) < atoiTrim(asString(own[j]["POSITION"]))
	})

	kind := "structure"
	switch tabclass {
	case "TRANSP", "POOL", "CLUSTER":
		kind = "table"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// No DDL source on this release: written by vsp from DD02L/DD03L (TABCLASS %s).\n", tabclass)
	if label != "" {
		fmt.Fprintf(&b, "@EndUserText.label : '%s'\n", strings.ReplaceAll(label, "'", "''"))
	}
	fmt.Fprintf(&b, "define %s %s {\n", kind, strings.ToLower(name))

	width := 0
	for _, r := range own {
		fn := strings.TrimSpace(asString(r["FIELDNAME"]))
		if strings.HasPrefix(fn, ".") {
			continue
		}
		w := len(fn)
		if asString(r["KEYFLAG"]) == "X" {
			w += 4
		}
		if w > width {
			width = w
		}
	}

	for _, r := range own {
		fn := strings.TrimSpace(asString(r["FIELDNAME"]))
		if strings.HasPrefix(fn, ".") {
			inc := strings.ToLower(strings.TrimSpace(asString(r["PRECFIELD"])))
			if fn == ".INCLUDE" {
				fmt.Fprintf(&b, "  include %s;\n", inc)
			} else {
				fmt.Fprintf(&b, "  include %s; // %s\n", inc, fn)
			}
			continue
		}
		left := strings.ToLower(fn)
		if asString(r["KEYFLAG"]) == "X" {
			left = "key " + left
		}
		typ := ddicFieldType(r)
		if kind == "table" && asString(r["NOTNULL"]) == "X" {
			typ += " not null"
		}
		fmt.Fprintf(&b, "  %-*s : %s;\n", width, left, typ)
	}
	b.WriteString("}\n")
	return b.String()
}

func ddicFieldType(r map[string]interface{}) string {
	roll := strings.TrimSpace(asString(r["ROLLNAME"]))
	switch strings.TrimSpace(asString(r["COMPTYPE"])) {
	case "R":
		if roll != "" {
			return "reference to " + strings.ToLower(roll)
		}
	case "E", "S", "L":
		if roll != "" {
			return strings.ToLower(roll)
		}
	}
	dt := strings.TrimSpace(asString(r["DATATYPE"]))
	leng := atoiTrim(asString(r["LENG"]))
	dec := atoiTrim(asString(r["DECIMALS"]))
	switch {
	case dt == "STRG":
		return "abap.string(0)"
	case dt == "RSTR":
		return "abap.rawstring(0)"
	case ddicNoLength[dt]:
		return "abap." + strings.ToLower(dt)
	case ddicWithDecimals[dt]:
		return fmt.Sprintf("abap.%s(%d,%d)", strings.ToLower(dt), leng, dec)
	case dt != "":
		return fmt.Sprintf("abap.%s(%d)", strings.ToLower(dt), leng)
	}
	return "abap.char(1)"
}

func atoiTrim(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

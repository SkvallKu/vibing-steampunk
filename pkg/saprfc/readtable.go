package saprfc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oisee/open-rfc-go/rfc"
)

// optionsLineLen is the width of RFC_READ_TABLE's OPTIONS-TEXT field (RFC_DB_OPT
// is a CHAR 72). A longer WHERE clause has to be spread over several rows.
const optionsLineLen = 72

// ReadTable runs RFC_READ_TABLE and returns each row as a column->value map.
//
// Two quirks of the classic FM are handled here:
//
//   - The WHERE clause is split over as many OPTIONS rows as it needs. ABAP
//     joins the rows with a blank when it builds the dynamic condition, so a
//     token must never straddle a row boundary; see splitWhereClause.
//   - The classic DATA output is a TAB512, so wide tables raise
//     DATA_BUFFER_EXCEEDED. On that exception the call is retried with
//     USE_ET_DATA_4_RETURN = 'X', which returns the rows in ET_DATA, whose line
//     type is a STRING and therefore has no width limit. Older releases (7.50
//     among them) have no such parameter, and the answer is then to ask for
//     fewer fields.
//
// A STRING or RAWSTRING column is beyond RFC_READ_TABLE on any release: it
// dumps in an ASSIGN ... CASTING. Both dead ends are reported with the way
// round them — data preview over ADT reads either table.
//
// No rows is an empty slice, not nil.
func ReadTable(ctx context.Context, client rfcCaller, table, where string, fields []string, top int) ([]map[string]string, error) {
	in := rfc.Params{"QUERY_TABLE": table, "DELIMITER": "|"}
	if top > 0 {
		in["ROWCOUNT"] = int64(top)
	}
	if strings.TrimSpace(where) != "" {
		lines, err := splitWhereClause(where)
		if err != nil {
			return nil, err
		}
		opts := make([]map[string]any, 0, len(lines))
		for _, l := range lines {
			opts = append(opts, map[string]any{"TEXT": l})
		}
		in["OPTIONS"] = opts
	}
	if len(fields) > 0 {
		fs := make([]map[string]any, 0, len(fields))
		for _, f := range fields {
			fs = append(fs, map[string]any{"FIELDNAME": f})
		}
		in["FIELDS"] = fs
	}

	r, err := client.Call(ctx, "RFC_READ_TABLE", in)
	if err != nil {
		var exc *rfc.ABAPException
		if !errors.As(err, &exc) || exc.Key != "DATA_BUFFER_EXCEEDED" {
			return nil, readTableError(table, err)
		}
		// The row is wider than the 512-byte DATA work area: ask for ET_DATA,
		// whose line type is a STRING.
		in["USE_ET_DATA_4_RETURN"] = "X"
		r, err = client.Call(ctx, "RFC_READ_TABLE", in)
		if errors.Is(err, rfc.ErrUnknownParameter) {
			return nil, fmt.Errorf("a row of %s is wider than the 512 characters RFC_READ_TABLE returns, and this release has no ET_DATA to return it in: name fewer fields, or read the table with data preview (vsp query, GetTableContents)", table)
		}
		if err != nil {
			return nil, readTableError(table, err)
		}
	}

	var cols []string
	for _, fr := range r.Table("FIELDS") {
		cols = append(cols, strings.TrimSpace(fmt.Sprint(fr["FIELDNAME"])))
	}
	// ET_DATA is filled instead of DATA when the fallback kicked in.
	rows, field := r.Table("DATA"), "WA"
	if len(rows) == 0 {
		if et := r.Table("ET_DATA"); len(et) > 0 {
			rows, field = et, "LINE"
		}
	}
	out := []map[string]string{}
	for _, dr := range rows {
		parts := strings.Split(fmt.Sprint(dr[field]), "|")
		row := map[string]string{}
		for i, col := range cols {
			if i < len(parts) {
				row[col] = strings.TrimRight(parts[i], " ")
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// readTableError explains the ASSIGN ... CASTING dump RFC_READ_TABLE ends in
// on a STRING or RAWSTRING column, which names no column itself; other errors
// pass through.
func readTableError(table string, err error) error {
	var exc *rfc.ABAPException
	if errors.As(err, &exc) && exc.Kind == rfc.KindRuntime &&
		(strings.Contains(exc.RuntimeID, "CASTING") || strings.Contains(exc.PlainText, "CASTING")) {
		return fmt.Errorf("RFC_READ_TABLE dumped on %s, as it does on a STRING or RAWSTRING column: leave those out of the fields, or read the table with data preview (vsp query, GetTableContents): %w", table, err)
	}
	return err
}

// splitWhereClause breaks a WHERE clause into OPTIONS rows of at most 72
// characters, never cutting a token in half.
//
// RFC_READ_TABLE hands OPTIONS to a dynamic SELECT ... WHERE (itab), where ABAP
// concatenates the rows with a blank between them — which is also why a token
// may not span two rows. The separating whitespace is therefore dropped at a
// break (it would be lost anyway: OPTIONS-TEXT is a CHAR 72 and pads with
// blanks), and joining the rows back with a single blank reproduces the clause.
//
// A single token longer than 72 characters cannot be expressed at all; that is
// reported rather than silently truncated.
func splitWhereClause(where string) ([]string, error) {
	if where == "" {
		return nil, nil
	}
	if len(where) <= optionsLineLen {
		return []string{where}, nil
	}
	var lines []string
	var cur string
	for _, tok := range strings.Fields(where) {
		if len(tok) > optionsLineLen {
			return nil, fmt.Errorf("WHERE clause cannot be split: token %q is longer than the %d-character OPTIONS line", tok, optionsLineLen)
		}
		switch {
		case cur == "":
			cur = tok
		case len(cur)+1+len(tok) <= optionsLineLen:
			cur += " " + tok
		default:
			lines = append(lines, cur)
			cur = tok
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines, nil
}

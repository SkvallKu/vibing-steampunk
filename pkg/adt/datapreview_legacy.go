package adt

import (
	"errors"
	"net/http"
	"strings"
)

// Releases before 7.40 SP08 (checked on 7.40 SP06) read tables through
// /datapreview/ddic only, and its parser is the classic Open SQL one:
//
//   - a comma-separated field list fails in the generated program with
//     "explicit length specifications are necessary with types C, P, X and N
//     in the OO context"; the same list separated by blanks answers;
//   - ORDER BY a, b is a syntax error (the classic form is ORDER BY a b);
//   - a long statement can fail with 'Following "', '" a blank is required'
//     or a like error naming a quote, a comma or a parenthesis, for where
//     the service cut it, not for what it says (below).
//
// The cut is in CL_ADT_DP_OPEN_SQL_HANDLER (read on 7.40 SP06). GET_INSTANCE
// condenses the statement and appends a period; REPLACE_INTO_UPTO_CLAUSE puts
// "UP TO n ROWS  INTO  CORRESPONDING FIELDS OF TABLE <ft_dynamic_table>"
// after the table; and once that is 255 characters or more,
// SPLIT_QUERY_STRING cuts it into lines of generated code before tokens
// SCAN found. The last line starts one character early — the character
// before its first token comes out twice — and that is harmless only when
// the character is a blank. SCAN makes a token of a comma too, so when the
// last token before column 250 is the comma of 'A', 'B' the last line starts
// with the quote before it: 'A'', 'B' , and every quote after is paired
// wrongly. Which statements fail depends on where that comma falls, so a
// digit more in rowNumber can make or break the same statement. 7.50 has the
// line "lv_last_line_end = lv_last_line_end - 1" commented out.
//
// vsp can say the same thing in a form the old parser takes: blanks around
// commas and inside parentheses always, so that no token but the first
// follows anything but a blank, and on a 400 one retry with the commas that
// separate columns dropped. COUNT( needs nothing: SCAN reads it as one
// token with the blank before it (checked on 7.40 SP06 with COUNT( on each
// column from 237 to 250).

// normalizeDataPreviewSQL puts a blank before and after every comma and
// inside every parenthesis, outside quotes, and makes a line break or a tab outside
// quotes a blank: 7.40 SP06 does not take a line break for white space, so
// "DD03L\nWHERE ..." reads as the name of a table that does not exist.
// Every release accepts the result.
func normalizeDataPreviewSQL(query string) string {
	var out strings.Builder
	inQuote := false
	blank := func(b byte) bool { return b == ' ' || b == '\n' || b == '\t' || b == '\r' }
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			inQuote = !inQuote
		}
		if !inQuote && blank(ch) {
			ch = ' '
		}
		if !inQuote && ch == ')' && i > 0 && !blank(query[i-1]) && query[i-1] != '(' {
			out.WriteByte(' ')
		}
		if !inQuote && ch == ',' && i > 0 && !blank(query[i-1]) {
			out.WriteByte(' ')
		}
		out.WriteByte(ch)
		if inQuote || i+1 >= len(query) || blank(query[i+1]) {
			continue
		}
		switch {
		case ch == ',':
			out.WriteByte(' ')
		case ch == '(' && query[i+1] != ')':
			out.WriteByte(' ')
		}
	}
	return out.String()
}

// legacyDataPreviewSQL rewrites a statement for the classic parser: the
// commas outside quotes and parentheses — the ones that separate columns in
// the select list, ORDER BY and GROUP BY — become blanks. An IN list keeps
// its commas. ok is false when there is nothing to rewrite.
func legacyDataPreviewSQL(query string) (legacy string, ok bool) {
	var out strings.Builder
	depth := 0
	inQuote := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		switch ch {
		case '\'':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				depth++
			}
		case ')':
			if !inQuote {
				depth--
			}
		case ',':
			if !inQuote && depth == 0 {
				continue
			}
		}
		out.WriteByte(ch)
	}
	legacy = out.String()
	return legacy, legacy != query
}

func isBadRequest(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest
}

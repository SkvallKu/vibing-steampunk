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
//   - a parenthesis that touches what it encloses — IN ('A', 'B'),
//     (A = 'x' OR ...) — fails once the statement is long, with an error that
//     names a quote or a comma. The limit depends on rowNumber too (the
//     service adds UP TO n ROWS to what it generates), so the same statement
//     can pass at rowNumber=1 and fail at 100. Classic ABAP wants blanks
//     inside parentheses; with them there is no such limit (330 characters
//     checked).
//
// vsp can say the same thing in a form the old parser takes: blanks after
// commas and inside parentheses always, and on a 400 one retry with the
// commas that separate columns dropped.

// normalizeDataPreviewSQL puts a blank after every comma and inside every
// parenthesis, outside quotes. Every release accepts the result.
func normalizeDataPreviewSQL(query string) string {
	var out strings.Builder
	inQuote := false
	blank := func(b byte) bool { return b == ' ' || b == '\n' || b == '\t' || b == '\r' }
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			inQuote = !inQuote
		}
		if !inQuote && ch == ')' && i > 0 && !blank(query[i-1]) && query[i-1] != '(' {
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

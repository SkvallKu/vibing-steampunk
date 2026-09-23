package adt

import (
	"regexp"
	"strings"
)

// Releases before 7.40 SP08 (checked on 7.40 SP06) have no
// /datapreview/freestyle at all: it answers 404, and RunQuery — with
// everything built on it, GetSystemInfo and `vsp query` among them — has no
// endpoint. /datapreview/ddic is there on every release, and a SELECT on one
// table is something it can answer.

var (
	singleTableRe = regexp.MustCompile(`(?is)^\s*SELECT\s.+?\sFROM\s+([A-Za-z0-9_/]+)(\s|$)`)
	joinOrSubRe   = regexp.MustCompile(`(?i)\bJOIN\b|\bUNION\b|\(\s*SELECT\b`)
)

// singleTableOf returns the table a SELECT reads when it reads exactly one —
// what /datapreview/ddic can answer in place of /datapreview/freestyle.
func singleTableOf(query string) string {
	if joinOrSubRe.MatchString(query) {
		return ""
	}
	m := singleTableRe.FindStringSubmatch(query)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1])
}

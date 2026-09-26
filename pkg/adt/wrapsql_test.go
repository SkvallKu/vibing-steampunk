package adt

import (
	"strings"
	"testing"
)

func TestWrapSQL(t *testing.T) {
	// A long IN list with a literal that must not be split, and a long
	// ORDER BY that the data preview once cut into "sdlt" and "ime".
	var lits []string
	for i := 0; i < 40; i++ {
		lits = append(lits, "'0aAi4G0H7{2vu4jDCI}OeW'")
	}
	q := "SELECT a, b FROM t WHERE x IN ( " + strings.Join(lits, ", ") + " ) AND y = 'a value with spaces inside' ORDER BY sdldate DESCENDING, sdltime DESCENDING"
	w := wrapSQL(q)
	if strings.Contains(strings.ReplaceAll(w, "\r\n", ""), "\n") {
		t.Error("a line was broken with a bare LF, which the service does not read as a line break")
	}
	for n, line := range strings.Split(w, "\r\n") {
		if len(line) > 200 {
			t.Errorf("line %d is %d characters", n+1, len(line))
		}
	}
	if strings.ReplaceAll(w, "\r\n", " ") != q {
		t.Error("wrapping changed the statement's words")
	}
	if !strings.Contains(w, "'a value with spaces inside'") {
		t.Error("a literal was broken")
	}
	if strings.Count(w, "\r\n") < 4 {
		t.Errorf("expected several lines, got %d", strings.Count(w, "\r\n")+1)
	}
	// A break the caller wrote, bare or not, arrives as CR LF, and a CR LF
	// is not doubled.
	if got := wrapSQL("SELECT a FROM t WHERE x IN (\n'1',\n'2' )"); got != "SELECT a FROM t WHERE x IN (\r\n'1',\r\n'2' )" {
		t.Errorf("a bare LF from the caller was kept: %q", got)
	}
	if got := wrapSQL("SELECT a\r\nFROM t"); got != "SELECT a\r\nFROM t" {
		t.Errorf("a CR LF from the caller changed: %q", got)
	}
	if wrapSQL("SELECT 1 FROM t") != "SELECT 1 FROM t" {
		t.Error("a short statement changed")
	}
}

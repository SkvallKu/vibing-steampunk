package adt

import "testing"

func TestSingleTableOf(t *testing.T) {
	for q, want := range map[string]string{
		"SELECT MANDT, MTEXT FROM t000 WHERE MANDT = '100'": "T000",
		"SELECT * FROM /BEV1/LUTRANSP":                      "/BEV1/LUTRANSP",
		"SELECT a~x FROM a INNER JOIN b ON a~k = b~k":       "",
		"SELECT * FROM a WHERE x IN ( SELECT y FROM b )":    "",
		"UPDATE t000 SET mtext = 'x'":                       "",
	} {
		if got := singleTableOf(q); got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
}

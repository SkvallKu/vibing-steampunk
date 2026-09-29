package main

import "testing"

func TestParseFixValue(t *testing.T) {
	for in, want := range map[string][3]string{
		`01=One`:           {"01", "", "One"},
		`01="One"`:         {"01", "", "One"},
		`10..15="A range"`: {"10", "15", "A range"},
		`02="`:             {"02", "", `"`},
		`03=Say "hi"`:      {"03", "", `Say "hi"`},
	} {
		v, err := parseFixValue(in)
		if err != nil || v.Low != want[0] || v.High != want[1] || v.Text != want[2] {
			t.Errorf("%s: %+v %v", in, v, err)
		}
	}
	if _, err := parseFixValue("One"); err == nil {
		t.Error("no = accepted")
	}
}

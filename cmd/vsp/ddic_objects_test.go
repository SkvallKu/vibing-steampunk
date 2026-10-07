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

func TestParseSecondaryKey(t *testing.T) {
	k, err := parseSecondaryKey("by_value:nonUniqueSorted:value, key:By value: and key")
	if err != nil || k.Name != "BY_VALUE" || k.Access != "nonUniqueSorted" || k.Definition != "keyComponents" ||
		len(k.Components) != 2 || k.Components[1] != "KEY" || k.Description != "By value: and key" {
		t.Errorf("%+v %v", k, err)
	}
	k, err = parseSecondaryKey("ALL:uniqueSorted:rowType")
	if err != nil || k.Definition != "rowType" || k.Components != nil {
		t.Errorf("%+v %v", k, err)
	}
	for _, bad := range []string{"BY_VALUE", "BY_VALUE:uniqueHashed", ":uniqueHashed:A", "X:uniqueHashed: "} {
		if _, err := parseSecondaryKey(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

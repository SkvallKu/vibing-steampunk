package adtfallback

import (
	"strings"
	"testing"
)

func TestReportFitsTheRFCProgramWidth(t *testing.T) {
	for _, mode := range []string{ModeInstall, ModeRemove, ModeCheck, ModePackage, ModeDropPackage} {
		src, err := Report(mode, Package)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if strings.Contains(src, "{{") {
			t.Errorf("%s: a placeholder is left in the report", mode)
		}
		if !strings.Contains(src, "'"+mode+"'") || !strings.Contains(src, "'"+Package+"'") {
			t.Errorf("%s: mode or package not set", mode)
		}
	}
}

func TestReportRefusesWhatWouldBreakTheProgram(t *testing.T) {
	for _, tc := range []struct{ mode, pkg string }{
		{"DROP", Package},
		{ModeInstall, "$X'Y"},
		{ModeInstall, ""},
		{ModeInstall, "$" + strings.Repeat("A", 30)},
	} {
		if _, err := Report(tc.mode, tc.pkg); err == nil {
			t.Errorf("Report(%q, %q): want an error", tc.mode, tc.pkg)
		}
	}
}

func TestVersionIsTheApplicationConstant(t *testing.T) {
	if v := Version(); v == "" || strings.ContainsAny(v, " '") {
		t.Errorf("Version() = %q", v)
	}
}

func TestObjectsComeBeforeWhatUsesThem(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range Objects() {
		if len(o.Description) > 60 {
			t.Errorf("%s: description longer than 60", o.Name)
		}
		src := strings.ToUpper(o.Source)
		for _, other := range Objects() {
			if other.Name == o.Name || seen[other.Name] {
				continue
			}
			// A type used as TYPE REF TO or in a static call must be there already.
			if strings.Contains(src, other.Name+"=>") || strings.Contains(src, "REF TO "+other.Name) ||
				strings.Contains(src, "INTERFACES "+other.Name) || strings.Contains(src, other.Name+"=>TY") {
				t.Errorf("%s uses %s, which comes later", o.Name, other.Name)
			}
		}
		seen[o.Name] = true
	}
}

func TestParseReportOutput(t *testing.T) {
	got := ParseReportOutput([]string{
		"Z$$$XRFC",
		"",
		"OK ZVSP_ADT_FALLBACK_APP ZVSP_ADT_FALLBACK ZCL_VSP_ADT_APP",
		"UNCHANGED ZVSP_ADT_FALLBACK_URI_MAP ZVSP_ADT_FALLBACK_MAP",
		"ERROR ZVSP_ADT_FALLBACK_APP BAdI is for SAP internal use",
		"ABSENT ZVSP_ADT_FALLBACK_URI_MAP",
	})
	want := []ReportLine{
		{"OK", "ZVSP_ADT_FALLBACK_APP", "ZVSP_ADT_FALLBACK", "ZCL_VSP_ADT_APP"},
		{"UNCHANGED", "ZVSP_ADT_FALLBACK_URI_MAP", "ZVSP_ADT_FALLBACK_MAP", ""},
		{"ERROR", "ZVSP_ADT_FALLBACK_APP", "", "BAdI is for SAP internal use"},
		{"ABSENT", "ZVSP_ADT_FALLBACK_URI_MAP", "", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// The report goes over RFC, where the ADT client's mutation checks do not
// reach: a read-only system must be refused before anything is sent.
func TestADTFallbackRefusesAReadOnlySystem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("SAP_URL", "https://sap.example:44300")
	t.Setenv("SAP_USER", "TESTER")
	t.Setenv("SAP_PASSWORD", "secret")
	t.Setenv("SAP_READ_ONLY", "true")

	if _, _, err := adtFallbackClient(installADTFallbackCmd, false); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("want a read-only refusal, got %v", err)
	}
	if _, _, err := adtFallbackClient(installADTFallbackCmd, true); err != nil {
		t.Fatalf("a dry run only reads and must be allowed: %v", err)
	}
}

func TestRemovalOrderDeletesUsersBeforeWhatTheyUse(t *testing.T) {
	got := removalOrder([]adt.PackageObject{
		{Type: "ENHO/XH", Name: "ZVSP_ADT_FALLBACK_APP"},
		{Type: "INTF/OI", Name: "ZIF_VSP_ADT_FEATURE"},
		{Type: "CLAS/OC", Name: "ZCL_VSP_ADT_APP"},
		{Type: "PROG/P", Name: "ZVSP_TEST_TEXTS"},
		{Type: "CLAS/OC", Name: "ZCL_VSP_ADT_TXT_FEATURE"},
		{Type: "CLAS/OC", Name: "ZCL_VSP_ADT_TXT_FORMAT"},
	})
	var names []string
	for _, o := range got {
		names = append(names, o.Name)
	}
	want := "ZVSP_TEST_TEXTS,ZCL_VSP_ADT_TXT_FEATURE,ZCL_VSP_ADT_TXT_FORMAT,ZCL_VSP_ADT_APP,ZIF_VSP_ADT_FEATURE"
	if strings.Join(names, ",") != want {
		t.Errorf("got %s, want %s", strings.Join(names, ","), want)
	}
}

func TestSameABAPIgnoresLineEndsAndTrailingBlanks(t *testing.T) {
	if !sameABAP("CLASS a.\r\n  x = 1.  \r\nENDCLASS.\r\n\r\n", "CLASS a.\n  x = 1.\nENDCLASS.") {
		t.Error("line ends and trailing blanks must not count")
	}
	if sameABAP("x = 1.", "x = 2.") {
		t.Error("a changed line must count")
	}
}

func TestADTFallbackInfoVersion(t *testing.T) {
	if v := adtFallbackInfoVersion("version=0.1.0\nfeatures=/textelements\n"); v != "0.1.0" {
		t.Errorf("got %q", v)
	}
	if v := adtFallbackInfoVersion("<html>"); v != "" {
		t.Errorf("got %q", v)
	}
}

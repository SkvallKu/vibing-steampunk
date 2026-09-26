package adt

import (
	"fmt"
	"testing"
)

// The suffix is base 36: CL_DEP_TREE has CM00A to CM00Z, and TMDIR gives
// CM00Z's method index as 35, on 7.40 and 7.50. Hexadecimal would stop at
// CM00F and read CM010 as 16.
func TestMethodIndexIsBase36(t *testing.T) {
	cases := map[string]int{"CM001": 1, "CM009": 9, "CM00A": 10, "CM00G": 16, "CM00Z": 35, "CM010": 36, "CM0ZZ": 1295}
	for section, want := range cases {
		got, ok := methodIndexFromSection(section)
		if !ok {
			t.Fatalf("%s should decode", section)
		}
		if got != want {
			t.Fatalf("%s decoded to %d, want %d", section, got, want)
		}
	}
}

// A class has sections that are not methods, and reporting one as method zero
// would invent a method that does not exist. Zero is a legitimate index, so the
// answer has to be a second return value rather than a sentinel.
func TestSectionsThatAreNotMethodsDoNotDecode(t *testing.T) {
	for _, section := range []string{"CI", "CU", "CO", "CCDEF", "CCIMP", "CM", "CMxyz", ""} {
		if _, ok := methodIndexFromSection(section); ok {
			t.Fatalf("%q is not a method include", section)
		}
	}
}

func TestClassIncludeSplitsOnThePadding(t *testing.T) {
	class, section, ok := splitClassInclude("ZCL_DEMO=======================CM003")
	if !ok || class != "ZCL_DEMO" || section != "CM003" {
		t.Fatalf("got %q / %q / %v", class, section, ok)
	}
	if _, _, ok := splitClassInclude("ZDEMO_REPORT"); ok {
		t.Fatal("a program include has no padding and is not a class include")
	}
	if _, _, ok := splitClassInclude(""); ok {
		t.Fatal("nothing in, nothing out")
	}
}

// A section that is not a method is a complete answer, not a failure: "this
// reference sits in the class definition" is worth saying.
func TestASectionIsReportedRatherThanDropped(t *testing.T) {
	m := MethodInclude{Include: "X", Class: "ZCL_DEMO", Section: "CI"}
	if got := m.Qualified(); got != "ZCL_DEMO (CI)" {
		t.Fatalf("got %q", got)
	}
	withMethod := MethodInclude{Class: "ZCL_DEMO", Method: "ROUTE_MESSAGE"}
	if got := withMethod.Qualified(); got != "ZCL_DEMO=>ROUTE_MESSAGE" {
		t.Fatalf("got %q", got)
	}
	// A method include whose name could not be looked up still names its class
	// rather than pretending the method is absent.
	unresolved := MethodInclude{Class: "ZCL_DEMO", Index: 3}
	if got := unresolved.Qualified(); got != "ZCL_DEMO" {
		t.Fatalf("got %q", got)
	}
}

// Every pairing of a chunk's classes and indices has to fit the rows data
// preview answers faithfully, or TMDIR would come back short without saying.
func TestTMDIRChunksStayWithinTheRowLimit(t *testing.T) {
	wanted := map[string]map[int]string{}
	var classes []string
	for i := 0; i < 300; i++ {
		class := fmt.Sprintf("ZCL_%03d", i)
		classes = append(classes, class)
		wanted[class] = map[int]string{}
		for j := 0; j < 40; j++ {
			wanted[class][(i+j)%90+1] = "x"
		}
	}
	seen := 0
	for _, chunk := range tmdirChunks(classes, wanted) {
		indices := map[int]bool{}
		for _, class := range chunk {
			for idx := range wanted[class] {
				indices[idx] = true
			}
		}
		if len(chunk)*len(indices) > rowFallbackAbove {
			t.Errorf("a chunk of %d classes and %d indices may bring more than %d rows", len(chunk), len(indices), rowFallbackAbove)
		}
		seen += len(chunk)
	}
	if seen != len(classes) {
		t.Errorf("every class goes into a chunk: %d of %d", seen, len(classes))
	}
}

package adt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GetFunctionGroupAllSources exists to be searched for dependencies. A
// sub-source that failed to load contributes no dependencies, which is
// indistinguishable downstream from a sub-source that has none — and that is
// how a boundary report comes back clean about code nobody read. Skipping the
// failure is right; skipping it silently is the defect.

const fugrStructure = `<?xml version="1.0" encoding="utf-8"?>
<objectStructureElement name="ZVSP_DEMO" type="FUGR/F">
  <link rel="http://www.sap.com/adt/relations/source/definitionIdentifier" href="/sap/bc/adt/functions/groups/zvsp_demo/source/main"/>
  <objectStructureElement name="LZVSP_DEMOTOP" type="FUGR/I">
    <link rel="http://www.sap.com/adt/relations/source/definitionIdentifier" href="/sap/bc/adt/functions/groups/zvsp_demo/includes/lzvsp_demotop/source/main"/>
  </objectStructureElement>
  <objectStructureElement name="Z_VSP_DEMO_CALL" type="FUGR/FF">
    <link rel="http://www.sap.com/adt/relations/source/definitionIdentifier" href="/sap/bc/adt/functions/groups/zvsp_demo/fmodules/z_vsp_demo_call/source/main"/>
  </objectStructureElement>
</objectStructureElement>`

// fugrServer answers the structure request, then serves every sub-source except
// the ones named in broken.
func fugrServer(t *testing.T, broken map[string]int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/objectstructure") {
			w.Header().Set("Content-Type", "application/vnd.sap.adt.objectstructure.v2+xml")
			w.Write([]byte(fugrStructure))
			return
		}
		if status, bad := broken[r.URL.Path]; bad {
			w.WriteHeader(status)
			w.Write([]byte("not authorised"))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("* source of " + r.URL.Path + "\n"))
	}))
}

func TestFunctionGroupNamesTheSubSourcesItCouldNotRead(t *testing.T) {
	const denied = "/sap/bc/adt/functions/groups/zvsp_demo/fmodules/z_vsp_demo_call/source/main"
	srv := fugrServer(t, map[string]int{denied: http.StatusForbidden})
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	source, missed, err := client.GetFunctionGroupAllSources(context.Background(), "ZVSP_DEMO")
	if err != nil {
		t.Fatalf("one dead include must not lose the rest of the group: %v", err)
	}
	if source == "" {
		t.Fatal("the readable sub-sources should still come back")
	}
	if len(missed) != 1 {
		t.Fatalf("the unreadable sub-source must be reported, got %d: %+v", len(missed), missed)
	}
	if missed[0].Object != denied {
		t.Fatalf("the caller needs to know which sub-source, got %q", missed[0].Object)
	}
	if !strings.Contains(missed[0].Reason, "403") {
		t.Fatalf("the failure should survive intact — 403 and a timeout call for different next steps, got %q", missed[0].Reason)
	}
	// The note is the sentence that stops the wrong conclusion.
	if note := UnsearchedNote(missed, 3, "sub-source"); !strings.Contains(note, "not a complete answer") {
		t.Fatalf("the gap should render as a caveat:\n%s", note)
	}
}

// A group that loaded whole reports nothing, or the caveat rides along on every
// successful fetch and stops being read.
func TestAFullyReadFunctionGroupReportsNoGaps(t *testing.T) {
	srv := fugrServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, "user", "pass")
	source, missed, err := client.GetFunctionGroupAllSources(context.Background(), "ZVSP_DEMO")
	if err != nil {
		t.Fatal(err)
	}
	if len(missed) != 0 {
		t.Fatalf("everything was read; the gap list should be empty, got %+v", missed)
	}
	if !strings.Contains(source, "fmodules") || !strings.Contains(source, "includes") {
		t.Fatalf("the concatenation should span includes and modules:\n%s", source)
	}
}

// 7.40 and 7.50 answer 404 for a group's objectstructure, and every caller of
// GetFunctionGroupAllSources — transport analysis, the CR audit — lost the
// group. The node structure lists the same sources.
func TestFunctionGroupSourcesWithoutObjectStructure(t *testing.T) {
	for _, tc := range []struct {
		status int
		ok     bool
	}{{http.StatusNotFound, true}, {http.StatusInternalServerError, false}} {
		var nodestructure int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-csrf-token", "test-token")
			switch {
			case r.Method == http.MethodHead:
			case strings.HasSuffix(r.URL.Path, "/objectstructure"):
				w.WriteHeader(tc.status)
			case strings.HasSuffix(r.URL.Path, "/repository/nodestructure"):
				nodestructure++
				w.Write([]byte(nodeStructureXML))
			default:
				w.Write([]byte("* source of " + r.URL.Path + "\n"))
			}
		}))
		source, missed, err := NewClient(srv.URL, "user", "pass").GetFunctionGroupAllSources(context.Background(), "ZDEMO_FG")
		srv.Close()
		if !tc.ok {
			if err == nil || nodestructure != 0 {
				t.Errorf("a %d is the resource failing, not missing: err=%v, node structure asked %d times", tc.status, err, nodestructure)
			}
			continue
		}
		if err != nil || len(missed) != 0 {
			t.Fatalf("err=%v missed=%v", err, missed)
		}
		for _, want := range []string{
			"/functions/groups/zdemo_fg/source/main",
			"/includes/lzdemo_fgtop/source/main",
			"/fmodules/zdemo_fm_one/source/main",
			"/fmodules/zdemo_fm_two/source/main",
		} {
			if !strings.Contains(source, want) {
				t.Errorf("%s is missing from the group's sources:\n%s", want, source)
			}
		}
	}
}

func TestFunctionGroupNamesItsIncludes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		switch {
		case r.Method == http.MethodHead:
		case strings.HasSuffix(r.URL.Path, "/repository/nodestructure"):
			w.Write([]byte(nodeStructureXML))
		default:
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><group:abapFunctionGroup xmlns:group="http://www.sap.com/adt/functions/groups" xmlns:adtcore="http://www.sap.com/adt/core" adtcore:name="ZDEMO_FG" adtcore:type="FUGR/F"/>`))
		}
	}))
	defer srv.Close()

	fg, err := NewClient(srv.URL, "user", "pass").GetFunctionGroup(context.Background(), "zdemo_fg")
	if err != nil {
		t.Fatal(err)
	}
	if len(fg.Functions) != 2 || len(fg.Includes) != 1 || fg.Includes[0].Name != "LZDEMO_FGTOP" {
		t.Errorf("functions %+v, includes %+v", fg.Functions, fg.Includes)
	}
}

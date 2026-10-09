package adt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const atcDataList = "run\t0A1B2C3D4E5F6071829A39213D9B1D5B\tZVSP_LOGIC\t20261009110842\t20261012110842\t0\t21\t77\tZBC_DEMO_REPORT\n" +
	"worklist\t0A1B2C3D4E5F6071829A39213D9A1D5B\tZVSP_LOGIC\t20261009110842\t20261016110843\t1\tPROG ZBC_DEMO_REPORT\n"

func atcDataServer(t *testing.T, features string, calls *[]string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		mu.Lock()
		defer mu.Unlock()
		switch p := r.URL.Path; {
		case p == "/sap/bc/adt/discovery":
			w.Write([]byte(`<app:collection href="/sap/bc/adt/zvsp/info"/><app:collection href="/sap/bc/adt/zvsp/atcdata"/>`))
		case p == "/sap/bc/adt/zvsp/info":
			w.Write([]byte("version=0.11.0\nfeatures=" + features + "\n"))
		case p == "/sap/bc/adt/zvsp/atcdata":
			q := r.URL.Query()
			*calls = append(*calls, r.Method+" variant="+q.Get("variant")+" from="+q.Get("from")+" to="+q.Get("to"))
			w.Write([]byte(atcDataList))
		case strings.HasPrefix(p, "/sap/bc/adt/zvsp/atcdata/"):
			*calls = append(*calls, r.Method+" "+strings.TrimPrefix(p, "/sap/bc/adt/zvsp/atcdata/"))
			w.Write([]byte("results\t0A1B2C3D4E5F6071829A39213D9B1D5B\nexecution\t0A1B2C3D4E5F6071829A39213D9B1D5B\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestParseATCData(t *testing.T) {
	got, err := ParseATCData(atcDataList)
	if err != nil {
		t.Fatal(err)
	}
	want := []ATCData{
		{Kind: "run", ID: "0A1B2C3D4E5F6071829A39213D9B1D5B", Variant: "ZVSP_LOGIC", On: "20261009110842", Expires: "20261012110842",
			Prio2: 21, Prio3: 77, Title: "ZBC_DEMO_REPORT"},
		{Kind: "worklist", ID: "0A1B2C3D4E5F6071829A39213D9A1D5B", Variant: "ZVSP_LOGIC", On: "20261009110842", Expires: "20261016110843",
			Objects: 1, Title: "PROG ZBC_DEMO_REPORT"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	if _, err := ParseATCData("project\tX\n"); err == nil {
		t.Error("unknown kind went through")
	}
}

func TestListAndDeleteATCData(t *testing.T) {
	var calls []string
	srv := atcDataServer(t, "/ci,/atcdata", &calls)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	list, err := c.ListATCData(context.Background(), ATCDataFilter{Variant: "ZVSP*", From: "20261009000000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("list %+v", list)
	}
	lines, err := c.DeleteATCData(context.Background(), ATCDataRun, strings.ToLower(list[0].ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "results ") {
		t.Errorf("lines %q", lines)
	}
	if _, err := c.DeleteATCData(context.Background(), ATCDataWorklist, list[1].ID); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET variant=ZVSP* from=20261009000000 to=",
		"DELETE runs/0A1B2C3D4E5F6071829A39213D9B1D5B",
		"DELETE worklists/0A1B2C3D4E5F6071829A39213D9A1D5B",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %q, want %q", calls, want)
	}
}

func TestATCDataRefusals(t *testing.T) {
	var calls []string
	srv := atcDataServer(t, "/atcdata", &calls)
	defer srv.Close()
	ctx := context.Background()
	id := "0A1B2C3D4E5F6071829A39213D9B1D5B"

	c := NewClient(srv.URL, "user", "pass")
	if _, err := c.DeleteATCData(ctx, "project", id); err == nil {
		t.Error("kind project went through")
	}
	if _, err := c.DeleteATCData(ctx, ATCDataRun, "ZBC_DEMO_REPORT"); err == nil {
		t.Error("a name for an id went through")
	}
	if _, err := c.ListATCData(ctx, ATCDataFilter{From: "2026-10-09"}); err == nil {
		t.Error("a date for a time stamp went through")
	}
	if _, err := NewClient(srv.URL, "user", "pass", WithReadOnly()).DeleteATCData(ctx, ATCDataRun, id); err == nil {
		t.Error("delete in read-only mode went through")
	}
	packages := NewClient(srv.URL, "user", "pass", WithAllowedPackages("$ZDEMO"))
	if _, err := packages.DeleteATCData(ctx, ATCDataRun, id); err == nil || !strings.Contains(err.Error(), "belongs to no package") {
		t.Errorf("delete with allowed packages: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("calls %v", calls)
	}
	// Reading stays allowed in read-only mode.
	if _, err := NewClient(srv.URL, "user", "pass", WithReadOnly()).ListATCData(ctx, ATCDataFilter{}); err != nil {
		t.Errorf("list in read-only mode: %v", err)
	}
}

func TestATCDataNeedsTheSet(t *testing.T) {
	var calls []string
	srv := atcDataServer(t, "/ci", &calls)
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	if _, err := c.ListATCData(context.Background(), ATCDataFilter{}); err == nil || !strings.Contains(err.Error(), "0.11.0") {
		t.Errorf("list: %v", err)
	}
	if _, err := c.DeleteATCData(context.Background(), ATCDataRun, "0A1B2C3D4E5F6071829A39213D9B1D5B"); err == nil || !strings.Contains(err.Error(), "0.11.0") {
		t.Errorf("delete: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("calls %v", calls)
	}
}

func TestATCTimestamp(t *testing.T) {
	for in, ok := range map[string]bool{
		"20261009110842": true, "2026-10-09": true, "2026-10-09T11:08": true, "2026-10-09 11:08:42": true,
		"": true, "yesterday": false, "2026100911": false,
	} {
		got, err := ATCTimestamp(in)
		if (err == nil) != ok {
			t.Errorf("%q: %v", in, err)
		}
		if err == nil && in != "" && !isTimestamp(got) {
			t.Errorf("%q gave %q", in, got)
		}
	}
}

func TestDeleteATCDataByID(t *testing.T) {
	var calls []string
	srv := atcDataServer(t, "/atcdata", &calls)
	defer srv.Close()
	ctx := context.Background()

	if _, err := NewClient(srv.URL, "user", "pass", WithReadOnly()).DeleteATCDataByID(ctx, []string{"0A1B2C3D4E5F6071829A39213D9B1D5B"}); err == nil {
		t.Error("delete in read-only mode went through")
	}
	if len(calls) != 0 {
		t.Errorf("read-only sent %v", calls)
	}
	got, err := NewClient(srv.URL, "user", "pass").DeleteATCDataByID(ctx, []string{"0a1b2c3d4e5f6071829a39213d9a1d5b", "00000000000000000000000000000000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != ATCDataWorklist || got[0].Error != "" || got[1].Error == "" {
		t.Errorf("results %+v", got)
	}
	want := []string{"GET variant= from= to=", "DELETE worklists/0A1B2C3D4E5F6071829A39213D9A1D5B"}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %q, want %q", calls, want)
	}
}

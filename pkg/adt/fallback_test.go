package adt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const discoveryWithStandIn = `<app:service xmlns:app="http://www.w3.org/2007/app">
<app:workspace><app:collection href="/sap/bc/adt/programs/programs"/></app:workspace>
<app:workspace>
 <app:collection href="/sap/bc/adt/zvsp/info"/>
 <app:collection href="/sap/bc/adt/zvsp/textelements/programs"/>
 <app:collection href="/sap/bc/adt/zvsp/textelements/classes"/>
</app:workspace>
</app:service>`

const discoveryWithBoth = `<app:service xmlns:app="http://www.w3.org/2007/app">
<app:workspace><app:collection href="/sap/bc/adt/textelements/programs"/></app:workspace>
<app:workspace><app:collection href="/sap/bc/adt/zvsp/textelements/programs"/></app:workspace>
</app:service>`

func TestFallbackPathUsesTheStandInOnlyWhenTheStandardIsMissing(t *testing.T) {
	for _, tc := range []struct {
		name, discovery, path, want string
	}{
		{"stand-in only", discoveryWithStandIn,
			"/sap/bc/adt/textelements/programs/rsusr002", "/sap/bc/adt/zvsp/textelements/programs/rsusr002"},
		{"collection itself", discoveryWithStandIn,
			"/sap/bc/adt/textelements/classes", "/sap/bc/adt/zvsp/textelements/classes"},
		{"not covered", discoveryWithStandIn,
			"/sap/bc/adt/programs/programs/rsusr002", "/sap/bc/adt/programs/programs/rsusr002"},
		{"prefix is not a segment", discoveryWithStandIn,
			"/sap/bc/adt/textelements/programsx/a", "/sap/bc/adt/textelements/programsx/a"},
		{"standard wins after an upgrade", discoveryWithBoth,
			"/sap/bc/adt/textelements/programs/rsusr002", "/sap/bc/adt/textelements/programs/rsusr002"},
		{"already a stand-in", discoveryWithStandIn,
			"/sap/bc/adt/zvsp/textelements/programs/a", "/sap/bc/adt/zvsp/textelements/programs/a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("x-csrf-token", "test-token")
				if r.URL.Path == "/sap/bc/adt/discovery" {
					w.Write([]byte(tc.discovery))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer srv.Close()
			c := NewClient(srv.URL, "user", "pass")
			if got := c.FallbackPath(context.Background(), tc.path); got != tc.want {
				t.Errorf("FallbackPath(%s) = %s, want %s", tc.path, got, tc.want)
			}
		})
	}
}

func TestFallbackPathWithoutDiscoveryLeavesThePathAndAsksAgain(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.URL.Path == "/sap/bc/adt/discovery" {
			calls++
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	path := "/sap/bc/adt/textelements/programs/a"
	for i := 0; i < 2; i++ {
		if got := c.FallbackPath(context.Background(), path); got != path {
			t.Fatalf("an unreadable discovery must leave the path: got %s", got)
		}
	}
	if calls < 2 {
		t.Errorf("a failed discovery must be asked again, asked %d times", calls)
	}
}

// The whole write goes to the stand-in: the lock, the PUT, the unlock, and
// the URI inside the activation request, which the stand-in's URI mapper
// resolves. Only the discovery is read at the standard address.
func TestWriteTextPoolGoesToTheStandIn(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	var activationBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		path := strings.ToLower(r.URL.Path)
		switch {
		case path == "/sap/bc/adt/discovery":
			w.Write([]byte(discoveryWithStandIn))
		case strings.HasPrefix(path, "/sap/bc/adt/textelements/"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No application class found for URI: " + r.URL.Path))
		case strings.HasPrefix(path, "/sap/bc/adt/datapreview"):
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><dataPreview:tableData xmlns:dataPreview="http://www.sap.com/adt/dataPreview"><dataPreview:totalRows>1</dataPreview:totalRows><dataPreview:columns><dataPreview:metadata dataPreview:name="MASTERLANG"/><dataPreview:dataSet><dataPreview:data>E</dataPreview:data></dataPreview:dataSet></dataPreview:columns></dataPreview:tableData>`))
		case path == "/sap/bc/adt/zvsp/textelements/classes/zcl_demo" && r.Method == http.MethodPost:
			if r.URL.Query().Get("_action") == "LOCK" {
				w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><asx:abap xmlns:asx="http://www.sap.com/abapxml"><asx:values><DATA><LOCK_HANDLE>H1</LOCK_HANDLE><CORRNR/><IS_LOCAL>X</IS_LOCAL></DATA></asx:values></asx:abap>`))
			}
		case path == "/sap/bc/adt/zvsp/textelements/classes/zcl_demo/source/symbols":
			w.Header().Set("Content-Type", "application/vnd.sap.adt.textelements.symbols.v1")
			w.Write([]byte("@MaxLength:10\r\n001=Old"))
		case path == "/sap/bc/adt/activation":
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			activationBody = string(b)
			mu.Unlock()
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><chkl:messages xmlns:chkl="http://www.sap.com/abapxml/checklist"/>`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Resource does not exist"))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	plan, err := c.WriteTextPool(context.Background(), TextPoolTarget{Type: "CLAS", Name: "ZCL_DEMO"}, "E",
		map[string]map[string]string{"I": {"001": "New"}}, "", TextPoolOptions{})
	if err != nil {
		t.Fatalf("WriteTextPool: %v (requests: %v)", err, seen)
	}
	if !plan.Applied || plan.Written != 1 {
		t.Errorf("want one text written, got %+v", plan)
	}
	for _, s := range seen {
		if strings.Contains(strings.ToLower(s), "/sap/bc/adt/textelements/") {
			t.Errorf("request to the missing standard path: %s", s)
		}
	}
	for _, want := range []string{
		"POST /sap/bc/adt/zvsp/textelements/classes/zcl_demo",
		"PUT /sap/bc/adt/zvsp/textelements/classes/zcl_demo/source/symbols",
	} {
		found := false
		for _, s := range seen {
			found = found || strings.EqualFold(s, want)
		}
		if !found {
			t.Errorf("missing request %s in %v", want, seen)
		}
	}
	if !strings.Contains(activationBody, `adtcore:uri="/sap/bc/adt/zvsp/textelements/classes/zcl_demo"`) {
		t.Errorf("the activation must name the stand-in URI, got %s", activationBody)
	}
}

const discoveryPackagesStandIn = `<app:service xmlns:app="http://www.w3.org/2007/app">
<app:workspace><app:collection href="/sap/bc/adt/packages/settings"/></app:workspace>
<app:workspace><app:collection href="/sap/bc/adt/zvsp/packages"/></app:workspace>
</app:service>`

// 7.50 has /sap/bc/adt/packages/settings but not /packages: the stand-in
// takes the package paths, the settings stay with the standard.
func TestFallbackPathLeavesAMoreSpecificStandardPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.URL.Path == "/sap/bc/adt/discovery" {
			w.Write([]byte(discoveryPackagesStandIn))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")
	ctx := context.Background()
	if got := c.FallbackPath(ctx, "/sap/bc/adt/packages/%24demo"); got != "/sap/bc/adt/zvsp/packages/%24demo" {
		t.Errorf("package path: got %s", got)
	}
	if got := c.FallbackPath(ctx, "/sap/bc/adt/packages/settings"); got != "/sap/bc/adt/packages/settings" {
		t.Errorf("settings must stay standard: got %s", got)
	}
}

// Any request the router cannot place is asked once more at the stand-in,
// so PackageExists and a package create work unchanged. A 404 that is the
// resource's own answer (an exception document) is not asked again.
func TestTransportAsksTheStandInAfterARouter404(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch p := strings.ToLower(r.URL.Path); {
		case p == "/sap/bc/adt/discovery":
			w.Write([]byte(discoveryPackagesStandIn))
		case strings.HasPrefix(p, "/sap/bc/adt/packages"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No suitable resource found"))
		case p == "/sap/bc/adt/zvsp/packages/$demo" || p == "/sap/bc/adt/zvsp/packages/%24demo":
			w.Write([]byte(`<pak:package adtcore:name="$DEMO"/>`))
		case p == "/sap/bc/adt/zvsp/packages" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
		case strings.HasPrefix(p, "/sap/bc/adt/zvsp/packages/"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`<?xml version="1.0"?><exc:exception xmlns:exc="x"><type id="ExceptionResourceNotFound"/></exc:exception>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")
	ctx := context.Background()

	if ok, err := c.PackageExists(ctx, "$DEMO"); err != nil || !ok {
		t.Fatalf("PackageExists($DEMO) = %v, %v; want true (requests %v)", ok, err, seen)
	}
	if ok, err := c.PackageExists(ctx, "$NONE"); err != nil || ok {
		t.Fatalf("PackageExists($NONE) = %v, %v; want false", ok, err)
	}
	if err := c.CreateObject(ctx, CreateObjectOptions{ObjectType: ObjectTypePackage, Name: "$NEW", Description: "d"}); err != nil {
		t.Fatalf("create: %v (requests %v)", err, seen)
	}
	found := false
	for _, s := range seen {
		found = found || s == "POST /sap/bc/adt/zvsp/packages"
	}
	if !found {
		t.Errorf("the create must reach the stand-in: %v", seen)
	}
}

func TestIsRouterNotFound(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{&APIError{StatusCode: 404, Message: "Подходящий ресурс не найден"}, true},
		{&APIError{StatusCode: 404, Message: "No application class found for URI: /x"}, true},
		{&APIError{StatusCode: 404, Message: `<?xml version="1.0"?><exc:exception xmlns:exc="x"/>`}, false},
		{&APIError{StatusCode: 500, Message: "x"}, false},
		{nil, false},
	} {
		if got := IsRouterNotFound(tc.err); got != tc.want {
			t.Errorf("IsRouterNotFound(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

const discoveryTablesStandIn = `<app:service xmlns:app="http://www.w3.org/2007/app">
<app:workspace><app:collection href="/sap/bc/adt/ddic/structures"/></app:workspace>
<app:workspace><app:collection href="/sap/bc/adt/zvsp/ddic/tables"/></app:workspace>
</app:service>`

// An activation names its objects in the body. Once a request of this client
// has been turned away by the router, the table URI in the body is the
// stand-in's; before that the discovery document is not even asked for.
func TestActivationNamesTheStandInOnceItIsKnown(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	discoveries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		switch p := strings.ToLower(r.URL.Path); {
		case p == "/sap/bc/adt/discovery":
			mu.Lock()
			discoveries++
			mu.Unlock()
			w.Write([]byte(discoveryTablesStandIn))
		case p == "/sap/bc/adt/activation":
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, string(b))
			mu.Unlock()
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><chkl:messages xmlns:chkl="http://www.sap.com/abapxml/checklist"/>`))
		case strings.HasPrefix(p, "/sap/bc/adt/ddic/tables"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No suitable resource found"))
		case strings.HasPrefix(p, "/sap/bc/adt/zvsp/ddic/tables"):
			w.Write([]byte("define table zdemo {}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")
	ctx := context.Background()

	if _, err := c.Activate(ctx, "/sap/bc/adt/programs/programs/zdemo", "ZDEMO"); err != nil {
		t.Fatal(err)
	}
	if discoveries != 0 {
		t.Fatalf("an activation alone must not read the discovery document")
	}
	if _, err := c.GetTable(ctx, "ZDEMO"); err != nil {
		t.Fatalf("GetTable: %v", err)
	}
	if _, err := c.Activate(ctx, "/sap/bc/adt/ddic/tables/zdemo", "ZDEMO"); err != nil {
		t.Fatal(err)
	}
	last := bodies[len(bodies)-1]
	if !strings.Contains(last, `adtcore:uri="/sap/bc/adt/zvsp/ddic/tables/zdemo"`) {
		t.Errorf("the activation must name the stand-in: %s", last)
	}
	if !strings.Contains(bodies[0], `adtcore:uri="/sap/bc/adt/programs/programs/zdemo"`) {
		t.Errorf("a standard object keeps its URI: %s", bodies[0])
	}
}

// The transport check runs before the lock, often before any request of the
// session has met the router's 404, so the stand-in is not known yet. The
// check names the standard path of a resource the release lacks, SAP finds
// no mapping for it and the check fails — a transportable object then went
// to SAP's own "Generated Request" (7.40, 2026-09-27; 400 on 7.50).
func TestTransportCheckRetriesWithTheStandIn(t *testing.T) {
	var mu sync.Mutex
	var uris []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		switch r.URL.Path {
		case "/sap/bc/adt/discovery":
			w.Write([]byte(discoveryWithStandIn))
		case "/sap/bc/adt/cts/transportchecks":
			b, _ := io.ReadAll(r.Body)
			body := string(b)
			uri := body[strings.Index(body, "<URI>")+5 : strings.Index(body, "</URI>")]
			mu.Lock()
			uris = append(uris, uri)
			mu.Unlock()
			if !strings.HasPrefix(uri, "/sap/bc/adt/zvsp/") {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`<?xml version="1.0"?><exc:exception xmlns:exc="x"><message lang="EN">No URI-Mapping defined for URI</message></exc:exception>`))
				return
			}
			w.Write([]byte(`<asx:abap xmlns:asx="http://www.sap.com/abapxml"><asx:values><DATA><DEVCLASS>ZPKG</DEVCLASS><RECORDING>X</RECORDING>` +
				`<REQUESTS><CTS_REQUEST><REQ_HEADER><TRKORR>TR-EXAMPLE</TRKORR></REQ_HEADER></CTS_REQUEST></REQUESTS></DATA></asx:values></asx:abap>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	check, err := c.CheckTransport(context.Background(), "/sap/bc/adt/textelements/programs/zdemo", "", "")
	if err != nil {
		t.Fatalf("CheckTransport: %v (uris %v)", err, uris)
	}
	if len(check.Candidates) != 1 || check.Candidates[0].Number != "TR-EXAMPLE" {
		t.Errorf("candidates = %+v", check.Candidates)
	}
	if len(uris) != 2 || uris[1] != "/sap/bc/adt/zvsp/textelements/programs/zdemo" {
		t.Errorf("checked %v; want the standard path, then the stand-in", uris)
	}

	// A check that fails for a path with no stand-in is not repeated.
	uris = nil
	if _, err := c.CheckTransport(context.Background(), "/sap/bc/adt/oo/classes/zcl_demo", "", ""); err == nil {
		t.Error("a path with no stand-in must keep its error")
	}
	if len(uris) != 1 {
		t.Errorf("checked %v; want one attempt", uris)
	}
}

// The ATC protocol check and the stand-in routing share one read of the
// discovery document: on 7.40 it can take tens of seconds.
func TestAdvertisesSharesTheDiscoveryOfTheStandIns(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.URL.Path == "/sap/bc/adt/discovery" {
			calls++
			w.Write([]byte(discoveryWithStandIn))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	c.FallbackPath(context.Background(), "/sap/bc/adt/textelements/programs/a")
	if present, known := c.advertises(context.Background(), "/sap/bc/adt/atc/worklists"); present || !known {
		t.Errorf("advertises = %v, %v; want false, true", present, known)
	}
	if present, _ := c.advertises(context.Background(), "/sap/bc/adt/zvsp/info"); !present {
		t.Error("want /zvsp/info advertised")
	}
	if calls != 1 {
		t.Errorf("discovery read %d times, want once", calls)
	}
}

// On 7.40 SP06 nothing but discovery answers, the data preview for free SQL
// included. With an identity pin the first request of a client is preceded
// by the preflight, and when that first request is the discovery read for
// the stand-ins, the preflight's own router 404 must not ask for the
// stand-ins again: it used to wait for the lock its caller held, for ever.
func TestPinnedFirstDiscoveryReadDoesNotHang(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(c *Client, ctx context.Context)
	}{
		{"FallbackPath", func(c *Client, ctx context.Context) { c.FallbackPath(ctx, "/sap/bc/adt/textelements/programs/a") }},
		{"FallbackInstalled", func(c *Client, ctx context.Context) { c.FallbackInstalled(ctx) }},
		{"advertises", func(c *Client, ctx context.Context) { c.advertises(ctx, "/sap/bc/adt/atc/worklists") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("x-csrf-token", "test-token")
				if r.URL.Path == "/sap/bc/adt/discovery" {
					w.Write([]byte(discoveryWithStandIn))
					return
				}
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("No application class found for URI: " + r.URL.Path))
			}))
			defer srv.Close()
			c := NewClient(srv.URL, "DEVELOPER", "secret", WithClient("001"), WithExpect(mustPin(t, "A4H.001/DEVELOPER")))

			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.call(c, context.Background())
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("no answer in 5s: the preflight waits for the stand-in routes it is part of reading")
			}
		})
	}
}

// The discovery read is made without the lock held, and callers that come
// during it still wait for that one read rather than each making their own.
func TestConcurrentCallersShareOneDiscoveryRead(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-csrf-token", "test-token")
		if r.URL.Path == "/sap/bc/adt/discovery" {
			mu.Lock()
			calls++
			mu.Unlock()
			<-release
			w.Write([]byte(discoveryWithStandIn))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user", "pass")

	var wg sync.WaitGroup
	got := make([]string, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i] = c.FallbackPath(context.Background(), "/sap/bc/adt/textelements/programs/a")
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("discovery read %d times, want once", calls)
	}
	for i, p := range got {
		if p != "/sap/bc/adt/zvsp/textelements/programs/a" {
			t.Errorf("caller %d got %q, want the stand-in", i, p)
		}
	}
}

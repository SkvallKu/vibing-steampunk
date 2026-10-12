package saprfc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oisee/open-rfc-go/rfc"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

func TestAfterTunnelError(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		drop, retry bool
	}{
		{"transport", fmt.Errorf("x: %w", rfc.ErrTransport), true, false},
		{"closed", rfc.ErrClosed, true, false},
		// The dump seen on 7.50 after some thirty-six data preview queries
		// in one session: it carries the text, and on other systems the ID.
		{"subpool text", &rfc.ABAPException{Kind: rfc.KindRuntime, PlainText: "No further temporary subroutine pools can be generated."}, true, true},
		{"subpool id", &rfc.ABAPException{Kind: rfc.KindRuntime, RuntimeID: "GENERATE_SUBPOOL_DIR_FULL"}, true, true},
		// Any other dump may have left work half done: the session goes,
		// the request is not repeated.
		{"other dump", &rfc.ABAPException{Kind: rfc.KindRuntime, RuntimeID: "DBSQL_SQL_ERROR"}, true, false},
		{"declared exception", &rfc.ABAPException{Kind: rfc.KindException, Key: "NOT_FOUND"}, false, false},
		{"other", errors.New("boom"), false, false},
	}
	for _, c := range cases {
		drop, retry := afterTunnelError(fmt.Errorf("call: %w", c.err))
		if drop != c.drop || retry != c.retry {
			t.Errorf("%s: got drop=%v retry=%v, want drop=%v retry=%v", c.name, drop, retry, c.drop, c.retry)
		}
	}
}

// The tunnel gets the path as pkg/adt escaped it: an object in a namespace is
// /sap/bc/adt/oo/classes/%2fsdf%2fcl_x, and with the slashes decoded ADT
// answers 404.
func TestTunnelURIKeepsTheEscapedPath(t *testing.T) {
	cases := []struct{ url, want string }{
		{"http://h/sap/bc/adt/oo/classes/%2fsdf%2fcl_cm_chk_adt_res_app/source/main",
			"/sap/bc/adt/oo/classes/%2fsdf%2fcl_cm_chk_adt_res_app/source/main"},
		{"http://h/sap/bc/adt/oo/classes/%2FSDF%2FCL_X?version=active",
			"/sap/bc/adt/oo/classes/%2FSDF%2FCL_X?version=active"},
		{"http://h/sap/bc/adt/programs/programs/zprog", "/sap/bc/adt/programs/programs/zprog"},
		// The logon parameters stay out (7.40 discovery matches its BAdI
		// filter on the whole URI); the rest is passed on as written.
		{"http://h/sap/bc/adt/discovery?sap-client=100&sap-language=RU", "/sap/bc/adt/discovery"},
		{"http://h/sap/bc/adt/programs/programs/zprog?_action=LOCK&accessMode=MODIFY&sap-client=100",
			"/sap/bc/adt/programs/programs/zprog?_action=LOCK&accessMode=MODIFY"},
		{"http://h/sap/bc/adt/repository/informationsystem/search?sap-client=100&query=Z%2A&maxResults=5",
			"/sap/bc/adt/repository/informationsystem/search?query=Z%2A&maxResults=5"},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", c.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := tunnelURI(req.URL); got != c.want {
			t.Errorf("tunnelURI(%s) = %s, want %s", c.url, got, c.want)
		}
	}
}

// Over RFC the session's language is the logon's, whatever sap-language
// says. A request for another language is refused before anything is sent:
// passed through, a read would answer in the logon language and a write
// would overwrite its texts.
func TestTunnelRefusesAnotherLanguage(t *testing.T) {
	cases := []struct {
		logon, url string
		refused    bool
	}{
		{"R", "http://h/sap/bc/adt/programs/programs/zprog?sap-client=100&sap-language=RU", false},
		{"R", "http://h/sap/bc/adt/programs/programs/zprog?sap-language=ru", false},
		{"R", "http://h/sap/bc/adt/programs/programs/zprog?sap-language=R", false},
		{"R", "http://h/sap/bc/adt/programs/programs/zprog", false},
		{"1", "http://h/sap/bc/adt/discovery?sap-language=ZH", false},
		{"R", "http://h/sap/bc/adt/textelements/programs/zprog?sap-language=DE", true},
		{"E", "http://h/sap/bc/adt/textelements/programs/zprog?sap-language=RU", true},
		// Estonian's key is 9: its first letter would pass it as English.
		{"E", "http://h/sap/bc/adt/textelements/programs/zprog?sap-language=ET", true},
		{"9", "http://h/sap/bc/adt/textelements/programs/zprog?sap-language=ET", false},
		// A code with no key is refused, not guessed at.
		{"E", "http://h/sap/bc/adt/textelements/programs/zprog?sap-language=EO", true},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", c.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := NewTunnelDoer(Params{Language: c.logon}, 0)
		err = d.checkLanguage(req.URL)
		if (err != nil) != c.refused {
			t.Errorf("logon %s, %s: err = %v, refused %v", c.logon, c.url, err, c.refused)
		}
		if !c.refused {
			continue
		}
		// Do refuses too, and before dialling: the doer has no host to dial.
		_, err = d.Do(req)
		if err == nil || !strings.Contains(err.Error(), "logon language") {
			t.Errorf("Do: err = %v, want the language refusal", err)
		}
		if d.shared.line != nil || d.context.line != nil {
			t.Errorf("Do dialled for a refused request")
		}
	}
}

// fakeLine stands for one tunnel line: it keeps the URIs it was sent and
// answers each with err.
type fakeLine struct {
	pinned bool
	err    error

	mu     sync.Mutex
	uris   []string
	closed bool
}

func (f *fakeLine) Call(_ context.Context, _ string, in rfc.Params) (rfc.Result, error) {
	req, _ := in["REQUEST"].(map[string]any)
	line, _ := req["REQUEST_LINE"].(map[string]any)
	uri, _ := line["URI"].(string)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uris = append(f.uris, uri)
	return rfc.Result{}, f.err
}

func (f *fakeLine) close(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// fakeTunnel is a TunnelDoer whose lines are fakes; it keeps every line it
// dialled.
type fakeTunnel struct {
	*TunnelDoer
	mu    sync.Mutex
	lines []*fakeLine
	// sharedErr is what the shared lines answer.
	sharedErr error
}

func newFakeTunnel(sharedErr error) *fakeTunnel {
	ft := &fakeTunnel{sharedErr: sharedErr}
	ft.TunnelDoer = &TunnelDoer{dial: func(_ context.Context, _ Params, pinned bool) (*tunnelLine, error) {
		f := &fakeLine{pinned: pinned, err: errors.New("fake answer")}
		if !pinned && ft.sharedErr != nil {
			f.err = ft.sharedErr
		}
		ft.mu.Lock()
		ft.lines = append(ft.lines, f)
		ft.mu.Unlock()
		return &tunnelLine{caller: f, close: f.close}, nil
	}}
	return ft
}

func (ft *fakeTunnel) send(t *testing.T, method, uri, sessionType string) {
	t.Helper()
	req, err := http.NewRequest(method, "http://h"+uri, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sessionType != "" {
		req.Header.Set("X-sap-adt-sessiontype", sessionType)
	}
	_, _ = ft.Do(req)
}

// A lock lives in the ABAP session that took it. Over HTTP sap-contextid
// keeps LOCK, the write and UNLOCK in it; over the tunnel the pinned session
// does. Through the pool, a read running in parallel could take the session
// the LOCK left behind, and the PUT would go to another, to be refused with
// 423 while the lock stayed where it was.
func TestTunnelKeepsTheContextOnOneSession(t *testing.T) {
	ft := newFakeTunnel(nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ft.send(t, "GET", fmt.Sprintf("/sap/bc/adt/read/%d", i), "stateless")
		}(i)
	}
	chain := []string{"/sap/bc/adt/x?_action=LOCK", "/sap/bc/adt/x/source/main?lockHandle=h", "/sap/bc/adt/x?_action=UNLOCK"}
	ft.send(t, "POST", chain[0], "stateful")
	ft.send(t, "PUT", chain[1], "stateful")
	// A request with no session type belongs to the context too, as it
	// does in adt.Transport.do.
	ft.send(t, "POST", chain[2], "")
	wg.Wait()

	var pinned []*fakeLine
	reads := 0
	for _, l := range ft.lines {
		if l.pinned {
			pinned = append(pinned, l)
			continue
		}
		reads += len(l.uris)
		for _, u := range l.uris {
			if !strings.HasPrefix(u, "/sap/bc/adt/read/") {
				t.Errorf("%s went through the pool", u)
			}
		}
	}
	if len(pinned) != 1 {
		t.Fatalf("%d pinned lines dialled, want 1", len(pinned))
	}
	if got := strings.Join(pinned[0].uris, " "); got != strings.Join(chain, " ") {
		t.Errorf("pinned session got %s, want the chain %s", got, strings.Join(chain, " "))
	}
	if reads != 20 {
		t.Errorf("%d reads through the pool, want 20", reads)
	}
}

// A dump in a read drops the pooled line, not the pinned session that may be
// holding a lock: the chain goes on where it was.
func TestTunnelDumpInReadKeepsTheContext(t *testing.T) {
	ft := newFakeTunnel(&rfc.ABAPException{Kind: rfc.KindRuntime, RuntimeID: "DBSQL_SQL_ERROR"})
	ft.send(t, "POST", "/sap/bc/adt/x?_action=LOCK", "stateful")
	ft.send(t, "GET", "/sap/bc/adt/read", "stateless")
	ft.send(t, "POST", "/sap/bc/adt/x?_action=UNLOCK", "stateful")

	var pinned, shared []*fakeLine
	for _, l := range ft.lines {
		if l.pinned {
			pinned = append(pinned, l)
		} else {
			shared = append(shared, l)
		}
	}
	if len(pinned) != 1 || len(pinned[0].uris) != 2 {
		t.Fatalf("pinned lines %d, want one carrying LOCK and UNLOCK", len(pinned))
	}
	if pinned[0].closed {
		t.Errorf("the dump in a read closed the pinned session")
	}
	if ft.shared.line != nil {
		t.Errorf("the pooled line that dumped is still in use")
	}
	if len(shared) != 1 {
		t.Errorf("%d pooled lines dialled, want 1", len(shared))
	}
}

// A pinned session left idle may have been closed under it. The pool checks
// its sessions before handing one out; the pinned line pings after a pause,
// and a dead session is replaced before the request rather than failing it.
func TestTunnelPingsThePinnedSessionAfterAPause(t *testing.T) {
	ft := newFakeTunnel(nil)
	ft.send(t, "POST", "/sap/bc/adt/x?_action=LOCK", "stateful")
	first := ft.context.line
	pinged := 0
	first.ping = func(context.Context) error { pinged++; return rfc.ErrClosed }

	ft.send(t, "POST", "/sap/bc/adt/x?_action=LOCK", "stateful")
	if pinged != 0 || ft.context.line != first {
		t.Fatalf("pinged %d times right after a request", pinged)
	}
	first.used.Store(time.Now().Add(-2 * pinnedIdleCheck).UnixNano())
	ft.send(t, "POST", "/sap/bc/adt/x?_action=LOCK", "stateful")
	if pinged != 1 {
		t.Fatalf("pinged %d times after a pause, want 1", pinged)
	}
	if ft.context.line == first {
		t.Fatalf("the dead session is still in use")
	}
	if n := len(ft.lines[len(ft.lines)-1].uris); n != 1 {
		t.Errorf("the new session got %d requests, want the one after the pause", n)
	}
}

// A rejected logon is not tried again: each attempt with a wrong password
// counts against login/fails_to_user_lock, and an agent repeating its calls
// would lock the user. Any other dial failure is tried again.
func TestTunnelDoesNotRetryARejectedLogon(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantDials int
	}{
		{"rejected", fmt.Errorf("%w: wrong password", rfc.ErrLogonRejected), 1},
		{"network", errors.New("dial tcp: connection refused"), 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dials := 0
			d := &TunnelDoer{dial: func(context.Context, Params, bool) (*tunnelLine, error) {
				dials++
				return nil, tc.err
			}}
			// Both lines log on with the same user; ChannelSystem is the fifth call.
			for _, st := range []string{"stateless", "stateful", "stateless", ""} {
				req, _ := http.NewRequest("GET", "http://h/sap/bc/adt/x", nil)
				req.Header.Set("X-sap-adt-sessiontype", st)
				if _, err := d.Do(req); !errors.Is(err, tc.err) {
					t.Errorf("Do: err = %v, want %v", err, tc.err)
				}
			}
			if _, err := d.ChannelSystem(context.Background()); !errors.Is(err, tc.err) {
				t.Errorf("ChannelSystem: err = %v, want %v", err, tc.err)
			}
			if dials != tc.wantDials {
				t.Errorf("dialled %d times, want %d", dials, tc.wantDials)
			}
		})
	}
}

// open-rfc-go refuses a request past the compact CPIC limit saying only that
// streaming is disabled; the answer names the setting that enables it.
func TestTunnelNamesTheStreamingSetting(t *testing.T) {
	ft := newFakeTunnel(fmt.Errorf("%w: appc: CPIC streaming is disabled; enable this destination before sending more than 28000 application bytes", rfc.ErrProtocol))
	req, _ := http.NewRequest("PUT", "http://h/sap/bc/adt/x", strings.NewReader("big"))
	req.Header.Set("X-sap-adt-sessiontype", "stateless")
	_, err := ft.Do(req)
	if err == nil || !strings.Contains(err.Error(), `"rfc_cpic_streaming": true`) || !errors.Is(err, rfc.ErrProtocol) {
		t.Errorf("Do: err = %v, want the library's error naming rfc_cpic_streaming", err)
	}

	ft = newFakeTunnel(errors.New("some other failure"))
	if _, err := ft.Do(req); err == nil || strings.Contains(err.Error(), "rfc_cpic_streaming") {
		t.Errorf("Do: err = %v, want no streaming hint", err)
	}
}

// A code vsp has no key for is refused for that reason, not as another
// language than the logon's.
func TestTunnelNamesAnUnknownLanguage(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://h/sap/bc/adt/textelements/programs/zprog?sap-language=EO", nil)
	err := NewTunnelDoer(Params{Language: "E"}, 0).checkLanguage(req.URL)
	if err == nil || !strings.Contains(err.Error(), "no SAP language key") {
		t.Errorf("err = %v, want the unknown-code refusal", err)
	}
}

// deadlineCaller notes the deadline of each call it gets.
type deadlineCaller struct {
	deadline time.Time
	ok       bool
}

func (c *deadlineCaller) Call(ctx context.Context, _ string, _ rfc.Params) (rfc.Result, error) {
	c.deadline, c.ok = ctx.Deadline()
	return rfc.Result{}, errors.New("fake answer")
}

// Over HTTP a request is bounded by the client's Timeout, or by the call's
// deadline when it carries one; the tunnel bounds it the same way. Before, it
// had only the library's 30 s per read, which cut an activation or ATC run off
// while SAP went on with it.
func TestTunnelBoundsARequestLikeHTTP(t *testing.T) {
	caller := &deadlineCaller{}
	d := NewTunnelDoer(Params{}, 90*time.Second)
	d.dial = func(context.Context, Params, bool) (*tunnelLine, error) {
		return &tunnelLine{caller: caller, close: func(context.Context) error { return nil }}, nil
	}
	send := func(ctx context.Context) {
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://h/sap/bc/adt/x", nil)
		req.Header.Set("X-sap-adt-sessiontype", "stateless")
		_, _ = d.Do(req)
	}

	send(context.Background())
	if left := time.Until(caller.deadline); !caller.ok || left > 90*time.Second || left < 80*time.Second {
		t.Errorf("plain request: deadline in %v (set %v), want the doer's 90s", left, caller.ok)
	}

	// A call deadline longer than the doer's timeout governs alone.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	send(adt.WithCallDeadline(ctx))
	if left := time.Until(caller.deadline); !caller.ok || left < 9*time.Minute {
		t.Errorf("call with a deadline: deadline in %v (set %v), want the call's 10m", left, caller.ok)
	}

	// An unmarked deadline is not a call budget: the doer's timeout still applies.
	send(ctx)
	if left := time.Until(caller.deadline); left > 90*time.Second {
		t.Errorf("unmarked deadline: deadline in %v, want the doer's 90s", left)
	}
}

// The line itself must not end a call before the request's context does.
func TestTunnelLineOutlastsTheLibraryDefault(t *testing.T) {
	old := dialRFC
	t.Cleanup(func() { dialRFC = old })
	var got time.Duration
	dialRFC = func(_ context.Context, _ Params, timeout time.Duration) (*rfc.Client, error) {
		got = timeout
		return nil, errors.New("no gateway")
	}
	for _, pinned := range []bool{false, true} {
		got = 0
		_, _ = dialTunnel(context.Background(), Params{}, pinned)
		if got < 10*time.Minute {
			t.Errorf("pinned=%v: line opened with a %v read timeout, want one that outlasts a long call", pinned, got)
		}
	}
}

// The ADT client's own timeout is what the tunnel bounds a request by.
func TestTunneledClientTakesItsTimeout(t *testing.T) {
	old := dialRFC
	t.Cleanup(func() { dialRFC = old })
	var left time.Duration
	dialRFC = func(ctx context.Context, _ Params, _ time.Duration) (*rfc.Client, error) {
		if dl, ok := ctx.Deadline(); ok {
			left = time.Until(dl)
		}
		return nil, errors.New("no gateway")
	}
	c := NewTunneledADTClient("http://h:8000", "u", "p", Params{}, adt.WithTimeout(90*time.Second))
	_, _ = c.SearchObject(context.Background(), "X", 1)
	if left > 90*time.Second || left < 80*time.Second {
		t.Errorf("logon under a deadline in %v, want the client's 90s", left)
	}
}

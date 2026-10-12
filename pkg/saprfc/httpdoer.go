package saprfc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oisee/open-rfc-go/rfc"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// TunnelDoer adapts classic RFC to adt.HTTPDoer, so the general
// ADT client — and every MCP tool and CLI command built on it, not just the
// explicit `rfc adt` command and the CallRFC action — can run over classic
// RFC/SAProuter exactly as it runs over HTTPS. Without this, a system whose
// ADT HTTP port is not reachable (case C in GETTING_STARTED.md: "ABAP
// Development – RFC" in Eclipse) answers every ordinary tool call — GetSource,
// GetClassInfo, SearchObject, and the rest — with a context-deadline timeout,
// because pkg/adt.Client only ever knew how to speak plain HTTP.
//
// It keeps two lines, each dialled lazily on first use and reused across
// calls; a fresh logon per HTTP call would be as wasteful here as it would be
// in handlers_rfc.go. Stateless requests go through a pooled client, side by
// side, the way they go over HTTP. Requests into the context — LOCK, the write
// under it, UNLOCK — go through one session pinned on a connection of its
// own: over RFC the ABAP session is the context, so a lock taken in it can
// only be used and released from it. Sent through the pool, a LOCK could
// leave its session to a parallel read and the PUT after it land in another,
// to be refused with 423 while the lock stayed behind. Over HTTP the
// sap-contextid cookie keeps a chain in its context; here only the pin does.
//
// A failure drops only the line it happened on, so that its next call
// redials instead of failing forever: a dump in a read does not end the
// session that holds somebody's lock.
type TunnelDoer struct {
	dest Params
	// dial opens a line; tests replace it.
	dial func(ctx context.Context, dest Params, pinned bool) (*tunnelLine, error)

	shared  tunnelSlot
	context tunnelSlot

	mu     sync.Mutex
	system *adt.ChannelSystem
}

// tunnelLine is one way through to the endpoint: the pooled client, or a
// session pinned on a connection of its own.
type tunnelLine struct {
	caller rfcCaller
	close  func(ctx context.Context) error
	// ping, on the pinned line, checks the session before a request that
	// follows a pause: the pool does that for its own sessions, a pinned one
	// left idle may have been closed by the gateway or a SAProuter.
	ping func(ctx context.Context) error
	used atomic.Int64 // unix nanoseconds of the last request
}

// pinnedIdleCheck is how long the pinned session may stay unused before its
// next request pings it first.
const pinnedIdleCheck = time.Minute

// tunnelSlot holds the line in use for one kind of request.
type tunnelSlot struct {
	mu   sync.Mutex
	line *tunnelLine
}

// NewTunnelDoer returns an HTTPDoer that tunnels every request through
// classic RFC to dest. Nothing is dialled until the first request.
func NewTunnelDoer(dest Params) *TunnelDoer {
	return &TunnelDoer{dest: dest, dial: dialTunnel}
}

// dialTunnel logs on to dest. The pinned line has a client of its own, so
// dropping the shared one never takes the pinned session with it; it costs a
// second logon, made only when the first request into the context comes.
func dialTunnel(ctx context.Context, dest Params, pinned bool) (*tunnelLine, error) {
	c, err := Open(ctx, dest)
	if err != nil {
		return nil, err
	}
	if !pinned {
		return &tunnelLine{caller: c, close: c.Close}, nil
	}
	s, err := c.Pin(ctx)
	if err != nil {
		_ = c.Close(ctx)
		return nil, err
	}
	return &tunnelLine{caller: s, ping: s.Ping, close: func(ctx context.Context) error {
		_ = s.Close()
		return c.Close(ctx)
	}}, nil
}

func (d *TunnelDoer) conn(ctx context.Context, slot *tunnelSlot, pinned bool) (*tunnelLine, error) {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.line != nil {
		return slot.line, nil
	}
	l, err := d.dial(ctx, d.dest, pinned)
	if err != nil {
		return nil, err
	}
	l.used.Store(time.Now().UnixNano())
	slot.line = l
	return l, nil
}

// NoCSRF marks this doer as having no CSRF concept for pkg/adt (see
// adt.noCSRFDoer): classic RFC authenticates at logon, not per request, and
// SADT_REST_RFC_ENDPOINT answers a "Fetch" probe with no token at all — which
// pkg/adt's ordinary CSRF dance would otherwise surface as a hard failure on
// every POST/PUT/DELETE.
func (d *TunnelDoer) NoCSRF() bool { return true }

// ChannelSystem tells pkg/adt which system the tunnel reaches, from
// RFC_SYSTEM_INFO on the tunnel's connection, asked once. Over ADT a release
// without /sap/bc/adt/core/http/systeminformation has only T000's logical
// system to go by, and that is the source system's after a system copy.
func (d *TunnelDoer) ChannelSystem(ctx context.Context) (*adt.ChannelSystem, error) {
	d.mu.Lock()
	known := d.system
	d.mu.Unlock()
	if known != nil {
		return known, nil
	}
	l, err := d.conn(ctx, &d.shared, false)
	if err != nil {
		return nil, fmt.Errorf("RFC tunnel: %w", err)
	}
	r, err := l.caller.Call(ctx, "RFC_SYSTEM_INFO", nil)
	if err != nil {
		return nil, fmt.Errorf("RFC_SYSTEM_INFO: %w", err)
	}
	m, _ := r.Get("RFCSI_EXPORT").(map[string]any)
	// RFCHOST is eight characters: dev-app-server comes back as dev-app-.
	host := str(m["RFCHOST2"])
	if host == "" {
		host = str(m["RFCHOST"])
	}
	s := &adt.ChannelSystem{
		SID:          str(m["RFCSYSID"]),
		Host:         host,
		Database:     str(m["RFCDBSYS"]),
		DatabaseHost: str(m["RFCDBHOST"]),
		Kernel:       str(m["RFCKERNRL"]),
	}
	d.mu.Lock()
	d.system = s
	d.mu.Unlock()
	return s, nil
}

func (d *TunnelDoer) drop(slot *tunnelSlot, bad *tunnelLine) {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.line == bad {
		slot.line = nil
		// Closed rather than forgotten: after a dump the session is still
		// open on the server, and would stay there until it timed out.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = bad.close(ctx)
		}()
	}
}

// tunnelURI is the request line SADT_REST_RFC_ENDPOINT gets: the path as it
// was sent, still escaped. The decoded Path would turn the %2F of a namespace
// (/SDF/CL_X → %2fsdf%2fcl_x) into a slash, and ADT answers 404 to that.
//
// sap-client and sap-language are left out. They are ICF logon parameters:
// over RFC the client is the connection's and the language is not taken from
// the request at all. Kept in, they break 7.40, where the discovery document
// selects its BAdI implementations by the whole request URI, query included —
// /sap/bc/adt/discovery?sap-client=100 matches no filter and the document
// comes back empty. Every other parameter is passed on as it was written.
func tunnelURI(u *url.URL) string {
	uri := u.EscapedPath()
	var kept []string
	for _, p := range strings.Split(u.RawQuery, "&") {
		name, _, _ := strings.Cut(p, "=")
		if p == "" || strings.EqualFold(name, "sap-client") || strings.EqualFold(name, "sap-language") {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) > 0 {
		uri += "?" + strings.Join(kept, "&")
	}
	return uri
}

// checkLanguage refuses a request for another language than the logon's.
// Over HTTP sap-language logs each request on in its language; over RFC the
// session has the one it logged on with, and tunnelURI drops the parameter.
// Passed through, a read in DE answers in the logon language and looks right,
// and a write in DE overwrites the logon language's texts.
func (d *TunnelDoer) checkLanguage(u *url.URL) error {
	asked := u.Query().Get("sap-language")
	if asked == "" {
		return nil
	}
	logon := adt.SAPLanguageKey(d.dest.Language)
	if adt.SAPLanguageKey(asked) == logon {
		return nil
	}
	return fmt.Errorf("RFC tunnel: this request asks for language %s, but over RFC every request runs in the logon language (%s); "+
		"use a system profile whose language is %s", strings.ToUpper(asked), logon, strings.ToUpper(asked))
}

// Do implements adt.HTTPDoer by tunnelling req through SADT_REST_RFC_ENDPOINT
// and translating the answer back into an *http.Response. Everything above it
// in pkg/adt — CSRF, safety gating, caching, response parsing — is unaware
// its request never touched a socket.
func (d *TunnelDoer) Do(req *http.Request) (*http.Response, error) {
	if err := d.checkLanguage(req.URL); err != nil {
		return nil, err
	}
	ctx := req.Context()
	// Everything but an explicit stateless request belongs to the context,
	// as it does in adt.Transport.do.
	slot, pinned := &d.shared, false
	if req.Header.Get("X-sap-adt-sessiontype") != "stateless" {
		slot, pinned = &d.context, true
	}
	l, err := d.conn(ctx, slot, pinned)
	if err == nil && l.ping != nil && time.Since(time.Unix(0, l.used.Load())) > pinnedIdleCheck {
		// A lock it held is gone with it either way; what the ping saves is
		// the request after the pause failing on a dead connection.
		if l.ping(ctx) != nil {
			d.drop(slot, l)
			l, err = d.conn(ctx, slot, pinned)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("RFC tunnel: %w", err)
	}
	defer func() { l.used.Store(time.Now().UnixNano()) }()

	uri := tunnelURI(req.URL)

	var body []byte
	if req.Body != nil {
		defer req.Body.Close()
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
	}

	headers := make([]ADTHeader, 0, len(req.Header))
	for name, values := range req.Header {
		for _, v := range values {
			headers = append(headers, ADTHeader{Name: name, Value: v})
		}
	}

	adtReq := ADTRequest{Method: req.Method, URI: uri, Headers: headers, Body: body}
	res, err := CallADTOn(ctx, l.caller, adtReq)
	if err != nil {
		drop, retry := afterTunnelError(err)
		if drop {
			d.drop(slot, l)
		}
		if retry {
			if l, err = d.conn(ctx, slot, pinned); err != nil {
				return nil, fmt.Errorf("RFC tunnel: %w", err)
			}
			res, err = CallADTOn(ctx, l.caller, adtReq)
			if err != nil {
				if drop, _ := afterTunnelError(err); drop {
					d.drop(slot, l)
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("RFC tunnel: %w", err)
		}
	}

	header := make(http.Header, len(res.Headers))
	for _, h := range res.Headers {
		header.Add(h.Name, h.Value)
	}
	return &http.Response{
		StatusCode: res.Status,
		Status:     fmt.Sprintf("%d %s", res.Status, res.ReasonPhrase),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(res.Body)),
		Request:    req,
	}, nil
}

// afterTunnelError decides what a failed call does to the connection.
//
// A transport failure or a closed client drops it, so that the next call
// redials. So does an ABAP runtime error: a session that has dumped is not one
// to keep sending requests into.
//
// One runtime error is also retried, once, on the fresh connection. The data
// preview generates a temporary subroutine pool per query, the tunnel's
// sessions live as long as vsp does, and a session holds only a few dozen of
// them: past that every query dumps with GENERATE_SUBPOOL_DIR_FULL, "No
// further temporary subroutine pools can be generated" — seen after some
// thirty-six queries in one MCP session on 7.50. The dump happens while the
// query is being generated, before it runs, so the request did nothing and
// asking it again in a new session is safe. Any other dump may have happened
// halfway through something, and is only reported.
func afterTunnelError(err error) (drop, retry bool) {
	if errors.Is(err, rfc.ErrTransport) || errors.Is(err, rfc.ErrClosed) {
		return true, false
	}
	var abap *rfc.ABAPException
	if errors.As(err, &abap) && abap.Kind == rfc.KindRuntime {
		full := abap.RuntimeID == "GENERATE_SUBPOOL_DIR_FULL" ||
			strings.Contains(abap.PlainText, "GENERATE_SUBPOOL_DIR_FULL") ||
			strings.Contains(strings.ToLower(abap.PlainText), "subroutine pools")
		return true, full
	}
	return false, false
}

// NewTunneledADTClient builds an adt.Client whose every HTTP call actually
// runs over classic RFC/SAProuter via dest. baseURL/user/password and opts are
// otherwise exactly what adt.NewClient takes — CSRF, safety config, caching
// and response parsing all behave the same; only the wire underneath changes.
func NewTunneledADTClient(baseURL, user, password string, dest Params, opts ...adt.Option) *adt.Client {
	cfg := adt.NewConfig(baseURL, user, password, opts...)
	transport := adt.NewTransportWithClient(cfg, NewTunnelDoer(dest))
	return adt.NewClientWithTransport(cfg, transport)
}

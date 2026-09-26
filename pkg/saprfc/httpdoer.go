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
	"time"

	"github.com/oisee/open-rfc-go/rfc"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// TunnelDoer adapts a pooled RFC connection to adt.HTTPDoer, so the general
// ADT client — and every MCP tool and CLI command built on it, not just the
// explicit `rfc adt` command and the CallRFC action — can run over classic
// RFC/SAProuter exactly as it runs over HTTPS. Without this, a system whose
// ADT HTTP port is not reachable (case C in GETTING_STARTED.md: "ABAP
// Development – RFC" in Eclipse) answers every ordinary tool call — GetSource,
// GetClassInfo, SearchObject, and the rest — with a context-deadline timeout,
// because pkg/adt.Client only ever knew how to speak plain HTTP.
//
// The connection is dialled lazily on first use and reused across calls, the
// same pooling handlers_rfc.go does for the shared RFC client: a fresh logon
// per HTTP call would be as wasteful here as it would be there. A transport
// error drops the connection so the next call redials instead of failing
// forever.
type TunnelDoer struct {
	dest Params

	mu     sync.Mutex
	client *rfc.Client
}

// NewTunnelDoer returns an HTTPDoer that tunnels every request through
// classic RFC to dest. Nothing is dialled until the first request.
func NewTunnelDoer(dest Params) *TunnelDoer {
	return &TunnelDoer{dest: dest}
}

func (d *TunnelDoer) conn(ctx context.Context) (*rfc.Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client != nil {
		return d.client, nil
	}
	c, err := Open(ctx, d.dest)
	if err != nil {
		return nil, err
	}
	d.client = c
	return c, nil
}

// NoCSRF marks this doer as having no CSRF concept for pkg/adt (see
// adt.noCSRFDoer): classic RFC authenticates at logon, not per request, and
// SADT_REST_RFC_ENDPOINT answers a "Fetch" probe with no token at all — which
// pkg/adt's ordinary CSRF dance would otherwise surface as a hard failure on
// every POST/PUT/DELETE, verified empirically for a read-shaped POST
// (usageReferences) but not for an actual object mutation.
func (d *TunnelDoer) NoCSRF() bool { return true }

func (d *TunnelDoer) drop(bad *rfc.Client) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client == bad {
		d.client = nil
		// Closed rather than forgotten: after a dump the session is still
		// open on the server, and would stay there until it timed out.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = bad.Close(ctx)
		}()
	}
}

// tunnelURI is the request line SADT_REST_RFC_ENDPOINT gets: the path as it
// was sent, still escaped. The decoded Path would turn the %2F of a namespace
// (/SDF/CL_X → %2fsdf%2fcl_x) into a slash, and ADT answers 404 to that.
func tunnelURI(u *url.URL) string {
	uri := u.EscapedPath()
	if u.RawQuery != "" {
		uri += "?" + u.RawQuery
	}
	return uri
}

// Do implements adt.HTTPDoer by tunnelling req through SADT_REST_RFC_ENDPOINT
// and translating the answer back into an *http.Response. Everything above it
// in pkg/adt — CSRF, safety gating, caching, response parsing — is unaware
// its request never touched a socket.
func (d *TunnelDoer) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	c, err := d.conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("RFC tunnel: %w", err)
	}

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
	res, err := CallADT(ctx, c, adtReq)
	if err != nil {
		drop, retry := afterTunnelError(err)
		if drop {
			d.drop(c)
		}
		if retry {
			if c, err = d.conn(ctx); err != nil {
				return nil, fmt.Errorf("RFC tunnel: %w", err)
			}
			res, err = CallADT(ctx, c, adtReq)
			if err != nil {
				if drop, _ := afterTunnelError(err); drop {
					d.drop(c)
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
// preview generates a temporary subroutine pool per query, the tunnel keeps one
// ABAP session for all its calls, and a session holds only a few dozen of
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

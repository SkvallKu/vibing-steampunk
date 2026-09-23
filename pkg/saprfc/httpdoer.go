package saprfc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

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
	}
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

	uri := req.URL.Path
	if req.URL.RawQuery != "" {
		uri += "?" + req.URL.RawQuery
	}

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

	res, err := CallADT(ctx, c, ADTRequest{Method: req.Method, URI: uri, Headers: headers, Body: body})
	if err != nil {
		if errors.Is(err, rfc.ErrTransport) || errors.Is(err, rfc.ErrClosed) {
			d.drop(c)
		}
		return nil, fmt.Errorf("RFC tunnel: %w", err)
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

// NewTunneledADTClient builds an adt.Client whose every HTTP call actually
// runs over classic RFC/SAProuter via dest. baseURL/user/password and opts are
// otherwise exactly what adt.NewClient takes — CSRF, safety config, caching
// and response parsing all behave the same; only the wire underneath changes.
func NewTunneledADTClient(baseURL, user, password string, dest Params, opts ...adt.Option) *adt.Client {
	cfg := adt.NewConfig(baseURL, user, password, opts...)
	transport := adt.NewTransportWithClient(cfg, NewTunnelDoer(dest))
	return adt.NewClientWithTransport(cfg, transport)
}

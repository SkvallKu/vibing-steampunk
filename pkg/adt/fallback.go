package adt

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

// ADT fallback points. On a release without a standard ADT resource,
// `vsp install adt-fallback` puts a stand-in under FallbackRoot followed by
// the tail of the standard path — /sap/bc/adt/textelements/programs is served
// at /sap/bc/adt/zvsp/textelements/programs — speaking the same protocol. The
// stand-in is a customer class registered through BADI_ADT_REST_RFC_APPLICATION,
// so it is reached the way the standard resource would be, RFC tunnel included.
//
// FallbackPath decides once per client, from the discovery document, which
// path to use: the standard one whenever the system advertises it (after an
// upgrade the stand-in is simply no longer used), the stand-in when only that
// is advertised. The rule is the same for every resource and needs no list:
// whatever the installed set advertises under FallbackRoot is what it covers.
const FallbackRoot = "/sap/bc/adt/zvsp"

const adtRoot = "/sap/bc/adt"

type fallbackRoutes struct {
	mu     sync.Mutex
	loaded bool
	// reading is closed when the discovery read in progress ends; nil when
	// none is. The read is made without mu held: its request may be preceded
	// by the identity preflight, whose own requests come back here.
	reading chan struct{}
	// Collection and template hrefs of the discovery document.
	hrefs map[string]bool
	// Standard paths the stand-ins cover, lower case: /sap/bc/adt/textelements/programs.
	covered []string
}

// FallbackPath returns the path to use for a standard ADT path: the path
// itself, or its stand-in under FallbackRoot when the system advertises the
// stand-in and not the standard collection. A discovery that cannot be read
// leaves the path as it is, and is asked again next time.
func (c *Client) FallbackPath(ctx context.Context, path string) string {
	if !strings.HasPrefix(strings.ToLower(path), adtRoot+"/") || strings.HasPrefix(strings.ToLower(path), FallbackRoot+"/") {
		return path
	}
	hrefs, covered, ok := c.fallback.routes(ctx, c)
	if !ok {
		return path
	}
	lower := strings.ToLower(strings.SplitN(path, "?", 2)[0])
	for _, std := range covered {
		if !underPath(lower, std) {
			continue
		}
		if hrefs[std] {
			return path
		}
		// A standard resource more specific than the stand-in stays where it
		// is: 7.50 has /sap/bc/adt/packages/settings, not /packages.
		for href := range hrefs {
			if len(href) > len(std) && !strings.HasPrefix(href, FallbackRoot+"/") && underPath(lower, href) {
				return path
			}
		}
		return FallbackRoot + path[len(adtRoot):]
	}
	return path
}

func underPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func isDiscoveryPath(path string) bool {
	return strings.HasPrefix(path, "/sap/bc/adt/discovery") || strings.HasPrefix(path, "/sap/bc/adt/core/discovery")
}

// FallbackPathIfKnown is FallbackPath for a URI named inside a request body,
// such as the object references of an activation: the request itself goes
// to a standard path and cannot be retried at the stand-in. It rewrites only
// when this client has already read the discovery document — which it does
// the first time the router turns a request away — and so costs a system
// without stand-ins nothing.
func (c *Client) FallbackPathIfKnown(ctx context.Context, path string) string {
	c.fallback.mu.Lock()
	loaded := c.fallback.loaded
	c.fallback.mu.Unlock()
	if !loaded {
		return path
	}
	return c.FallbackPath(ctx, path)
}

// FallbackInstalled tells whether discovery shows the adt-fallback set
// (its /info), whatever it covers. False also when discovery cannot be read.
func (c *Client) FallbackInstalled(ctx context.Context) bool {
	hrefs, _, ok := c.fallback.routes(ctx, c)
	return ok && hrefs[FallbackRoot+"/info"]
}

// fallbackReadKey marks the context of the discovery read. A request made
// under it — the identity preflight that may precede the read, whose probes
// get a router 404 on 7.40 SP06 — finds no routes instead of waiting for the
// read it is part of.
type fallbackReadKey struct{}

// routes returns the hrefs of the discovery document and the standard paths
// the stand-ins cover, reading the document the first time. Callers that
// come while it is read wait for that read; ok is false when it fails, and
// the next caller reads again. The maps are never changed once stored.
func (r *fallbackRoutes) routes(ctx context.Context, c *Client) (hrefs map[string]bool, covered []string, ok bool) {
	if ctx.Value(fallbackReadKey{}) != nil {
		return nil, nil, false
	}
	for {
		r.mu.Lock()
		if r.loaded {
			hrefs, covered = r.hrefs, r.covered
			r.mu.Unlock()
			return hrefs, covered, true
		}
		if wait := r.reading; wait != nil {
			r.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return nil, nil, false
			}
		}
		done := make(chan struct{})
		r.reading = done
		r.mu.Unlock()

		res, err := c.transport.Request(context.WithValue(ctx, fallbackReadKey{}, r), "/sap/bc/adt/discovery", &RequestOptions{
			Method: http.MethodGet,
			Accept: acceptAny,
		})
		if err == nil && res != nil {
			hrefs, covered = parseFallbackDiscovery(string(res.Body))
		}

		r.mu.Lock()
		if hrefs != nil {
			r.hrefs, r.covered, r.loaded = hrefs, covered, true
		}
		r.reading = nil
		close(done)
		r.mu.Unlock()
		return hrefs, covered, hrefs != nil
	}
}

// parseFallbackDiscovery collects every href of the discovery document and,
// for each stand-in collection under FallbackRoot, the standard path it
// covers. Hrefs are compared in lower case: ADT paths are.
func parseFallbackDiscovery(body string) (map[string]bool, []string) {
	hrefs := parseDiscoveryHrefs(body)
	var covered []string
	seen := map[string]bool{}
	for href := range hrefs {
		if !strings.HasPrefix(href, FallbackRoot+"/") {
			continue
		}
		std := adtRoot + href[len(FallbackRoot):]
		if std == adtRoot+"/info" || seen[std] {
			continue
		}
		seen[std] = true
		covered = append(covered, std)
	}
	return hrefs, covered
}

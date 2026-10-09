package adt

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

// discoveryHrefs holds the collection hrefs of the discovery document, read
// once per client, for the operations whose protocol differs by release.
type discoveryHrefs struct {
	mu     sync.Mutex
	loaded bool
	hrefs  map[string]bool
}

// advertises tells whether the discovery document lists the collection href
// (a standard path, no query). known is false when discovery cannot be read,
// and the caller keeps its usual way; it is asked again next time.
func (c *Client) advertises(ctx context.Context, href string) (present, known bool) {
	d := &c.discovery
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.loaded {
		res, err := c.transport.Request(ctx, "/sap/bc/adt/discovery", &RequestOptions{
			Method: http.MethodGet,
			Accept: acceptAny,
		})
		if err != nil || res == nil {
			return false, false
		}
		d.hrefs = parseDiscoveryHrefs(string(res.Body))
		d.loaded = true
	}
	return d.hrefs[strings.ToLower(href)], true
}

// parseDiscoveryHrefs collects every href of the discovery document, in lower
// case (ADT paths are), without a query or template part.
func parseDiscoveryHrefs(body string) map[string]bool {
	hrefs := map[string]bool{}
	const marker = `href="`
	for {
		i := strings.Index(body, marker)
		if i < 0 {
			break
		}
		rest := body[i+len(marker):]
		j := strings.Index(rest, `"`)
		if j < 0 {
			break
		}
		href := strings.ToLower(rest[:j])
		if k := strings.IndexAny(href, "?{"); k >= 0 {
			href = strings.TrimSuffix(href[:k], "/")
		}
		hrefs[href] = true
		body = rest[j:]
	}
	return hrefs
}

package adt

import (
	"context"
	"strings"
)

// advertises tells whether the discovery document lists the collection href
// (a standard path, no query). known is false when discovery cannot be read,
// and the caller keeps its usual way; it is asked again next time. The
// document is the one FallbackPath reads, so it is read once for both.
func (c *Client) advertises(ctx context.Context, href string) (present, known bool) {
	hrefs, _, ok := c.fallback.routes(ctx, c)
	if !ok {
		return false, false
	}
	return hrefs[strings.ToLower(href)], true
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

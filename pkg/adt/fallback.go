package adt

import (
	"context"
	"errors"
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
	r := &c.fallback
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.loaded {
		if !r.load(ctx, c) {
			return path
		}
	}
	lower := strings.ToLower(strings.SplitN(path, "?", 2)[0])
	for _, std := range r.covered {
		if !underPath(lower, std) {
			continue
		}
		if r.hrefs[std] {
			return path
		}
		// A standard resource more specific than the stand-in stays where it
		// is: 7.50 has /sap/bc/adt/packages/settings, not /packages.
		for href := range r.hrefs {
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

// isRouterNotFound tells a 404 of the ADT router — no application for the
// path, or no resource in the application — from a resource's own 404 for
// an object that does not exist, which comes as an ADT exception document.
// Only the first can mean the resource is served somewhere else.
func isRouterNotFound(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		return false
	}
	return !strings.Contains(apiErr.Message, "<exc:exception") && !strings.Contains(apiErr.Message, ":exception ")
}

func isDiscoveryPath(path string) bool {
	return strings.HasPrefix(path, "/sap/bc/adt/discovery") || strings.HasPrefix(path, "/sap/bc/adt/core/discovery")
}

func (r *fallbackRoutes) load(ctx context.Context, c *Client) bool {
	res, err := c.transport.Request(ctx, "/sap/bc/adt/discovery", &RequestOptions{
		Method: http.MethodGet,
		Accept: acceptAny,
	})
	if err != nil || res == nil {
		return false
	}
	r.hrefs, r.covered = parseFallbackDiscovery(string(res.Body))
	r.loaded = true
	return true
}

// parseFallbackDiscovery collects every href of the discovery document and,
// for each stand-in collection under FallbackRoot, the standard path it
// covers. Hrefs are compared in lower case: ADT paths are.
func parseFallbackDiscovery(body string) (map[string]bool, []string) {
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

package adt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// --- Table Operations ---

// GetTable retrieves the source/definition of a database table.
func (c *Client) GetTable(ctx context.Context, tableName string) (string, error) {
	tableName = strings.ToUpper(tableName)

	// URL encode to handle namespaced objects like /DMO/TRAVEL
	sourcePath := fmt.Sprintf("/sap/bc/adt/ddic/tables/%s/source/main", url.PathEscape(tableName))
	resp, err := c.transport.Request(ctx, sourcePath, &RequestOptions{
		Method: http.MethodGet,
	})
	if err != nil {
		// Before 7.52 there is no source for DDIC tables; the dictionary
		// tables still say what the source would.
		if isNotFound(err) {
			if src, ferr := c.ddicSourceFromTables(ctx, tableName); ferr == nil {
				return src, nil
			}
		}
		return "", fmt.Errorf("getting table source: %w", err)
	}

	return ddicSourceText(resp), nil
}

// ddicSourceText is a table or structure source as served, headed by a note
// when the adt-fallback stand-in served it: on a release without the
// resource it is vsp's ABAP set that wrote the source (with SAP's generator
// where the release has one, its own on 7.40), and without a word it reads
// as the system's own. A comment, so the source still parses when written
// back.
func ddicSourceText(resp *Response) string {
	if resp.StandIn == "" {
		return string(resp.Body)
	}
	return "// No DDL source resource on this release: served by vsp's adt-fallback stand-in (" + FallbackRoot + ").\n" + string(resp.Body)
}

// GetView retrieves the source/definition of a DDIC database view.
// This is for classic DDIC views (SE11), not CDS views (which use GetDDLS).
func (c *Client) GetView(ctx context.Context, viewName string) (string, error) {
	viewName = strings.ToUpper(viewName)

	// URL encode the name to handle namespaced objects like /DMO/...
	sourcePath := fmt.Sprintf("/sap/bc/adt/ddic/views/%s/source/main", url.PathEscape(viewName))
	resp, err := c.transport.Request(ctx, sourcePath, &RequestOptions{
		Method: http.MethodGet,
	})
	if err != nil {
		return "", fmt.Errorf("getting view source: %w", err)
	}

	return string(resp.Body), nil
}

// GetStructure retrieves the source/definition of a data structure.
func (c *Client) GetStructure(ctx context.Context, structName string) (string, error) {
	structName = strings.ToUpper(structName)

	// URL encode to handle namespaced objects like /DMO/...
	sourcePath := fmt.Sprintf("/sap/bc/adt/ddic/structures/%s/source/main", url.PathEscape(structName))
	resp, err := c.transport.Request(ctx, sourcePath, &RequestOptions{
		Method: http.MethodGet,
	})
	if err != nil {
		if isNotFound(err) {
			if src, ferr := c.ddicSourceFromTables(ctx, structName); ferr == nil {
				return src, nil
			}
		}
		return "", fmt.Errorf("getting structure source: %w", err)
	}

	return ddicSourceText(resp), nil
}

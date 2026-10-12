package mcp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// Over the RFC tunnel no deployment makes the WebSocket reachable, so the
// answer does not send the caller to deploy ZADT_VSP.
func TestWSConnectFailedAdvice(t *testing.T) {
	text := func(r *mcp.CallToolResult) string { return r.Content[0].(mcp.TextContent).Text }
	tunnel := text(wsConnectFailed(fmt.Errorf("%w: over the RFC tunnel", adt.ErrNoWebSocket)))
	if strings.Contains(tunnel, "Ensure ZADT_VSP is deployed") || !strings.Contains(tunnel, "RFC tunnel") {
		t.Errorf("tunnel refusal: %q", tunnel)
	}
	if http := text(wsConnectFailed(errors.New("HTTP 404"))); !strings.Contains(http, "Ensure ZADT_VSP is deployed") {
		t.Errorf("HTTP failure lost its advice: %q", http)
	}
}

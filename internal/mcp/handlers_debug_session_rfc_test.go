package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oisee/vibing-steampunk/pkg/saprfc"

	openrfc "github.com/oisee/open-rfc-go/rfc"
)

// DebuggerListen waits 60 s by default and up to 240 s. Over RFC the
// listener is one call held for that long, and a client opened with the
// library's 30 s ended it first.
func TestDebuggerRFCOutlastsTheListener(t *testing.T) {
	s := serverFor(t, "https://prodsys-a.example:44300", "100", "")
	old := rfcOpen
	t.Cleanup(func() { rfcOpen = old })
	var got time.Duration
	rfcOpen = func(_ context.Context, _ saprfc.Params, timeout time.Duration) (*openrfc.Client, error) {
		got = timeout
		return nil, errors.New("no gateway")
	}
	_, _ = s.debugger(context.Background())
	if got <= 240*time.Second {
		t.Errorf("debug session opened with a %v call timeout, want more than the longest listen (240s)", got)
	}
}

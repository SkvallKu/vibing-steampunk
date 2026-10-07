package adt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// tokenlessDoer is a channel with no CSRF concept, as the RFC tunnel is: it
// never hands out a token.
type tokenlessDoer struct{}

func (tokenlessDoer) Do(req *http.Request) (*http.Response, error) { return http.DefaultClient.Do(req) }
func (tokenlessDoer) NoCSRF() bool                                 { return true }

// Over a channel without CSRF a missing token says nothing about the session;
// the discovery answering does.
func TestCheckSessionWithoutCSRFAsksTheDiscovery(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sap/bc/adt/core/discovery" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`<app:service xmlns:app="http://www.w3.org/2007/app"/>`))
	}))
	defer srv.Close()
	cfg := NewConfig(srv.URL, "user", "pass")
	c := NewClientWithTransport(cfg, NewTransportWithClient(cfg, tokenlessDoer{}))

	if err := c.CheckSession(context.Background()); err != nil {
		t.Fatalf("a live session without a token: %v", err)
	}
	status = http.StatusUnauthorized
	if err := c.CheckSession(context.Background()); err == nil {
		t.Fatal("a 401 must still say the session is not usable")
	}
}

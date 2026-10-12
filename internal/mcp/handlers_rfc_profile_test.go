package mcp

import (
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/adt"
	"github.com/oisee/vibing-steampunk/pkg/config"
)

// A server started with -s took its whole connection from that system,
// including its logon. The RFC logon must follow it: not SAP_USER/SAP_PASSWORD,
// which serverFor sets for another user, and not the system's entry looked up
// again through GetSystem, which falls back to those same variables.
func TestRFCDestination_ServerSystemTakesItsOwnLogon(t *testing.T) {
	s := serverFor(t, "https://prodsys-a.example:44300", "200", "prodsys-b")
	s.config.Username, s.config.Password = "READER", "reader-secret"
	s.config.System = &config.SystemConfig{RFCHost: "prod-gw-b.example.local", RFCSysnr: "10"}

	dest, err := s.rfcDestination(map[string]any{})
	if err != nil {
		t.Fatalf("rfcDestination: %v", err)
	}
	if dest.Host != "prod-gw-b.example.local" || dest.Client != "200" {
		t.Errorf("destination = %s client %s, want the server's own system", dest.Host, dest.Client)
	}
	if dest.User != "READER" || string(dest.Password) != "reader-secret" {
		t.Errorf("RFC logon = %q, want the server's own READER, not SAP_USER", dest.User)
	}
}

// The route of a server started with -s is its system's, never SAP_SAPROUTER,
// which belongs to whatever system .env was written for.
func TestRFCDestination_ServerSystemTakesItsOwnRoute(t *testing.T) {
	s := serverFor(t, "https://prodsys-a.example:44300", "200", "prodsys-b")
	t.Setenv("SAP_SAPROUTER", "/H/other-router/S/3299")
	s.config.Username, s.config.Password = "READER", "reader-secret"
	s.config.System = &config.SystemConfig{RFCHost: "10.0.0.2", RFCSysnr: "02", RFCSaprouter: "/H/qas-router/S/3299", RFCCpicStreaming: true}

	dest, err := s.rfcDestination(map[string]any{})
	if err != nil {
		t.Fatalf("rfcDestination: %v", err)
	}
	if dest.Router != "/H/qas-router/S/3299/H/" || !dest.CpicStreaming {
		t.Errorf("router = %q, streaming = %v; want the system's own", dest.Router, dest.CpicStreaming)
	}
	if _, ok := rfcTunnelDest(s.config); !ok {
		t.Error("a server system with a route does not tunnel its ADT client")
	}
}

// The tunnel logs on as rfc_user, which may differ from the HTTP user the ADT
// preflight compares with the pin. The pin goes with the tunnel's logon, so
// saprfc.Open checks that user before dialling and the system after.
func TestRFCTunnelDest_CarriesThePin(t *testing.T) {
	s := serverFor(t, "https://prodsys-a.example:44300", "200", "prodsys-b")
	s.config.Username, s.config.Password = "READER", "reader-secret"
	s.config.System = &config.SystemConfig{RFCHost: "10.0.0.2", RFCSysnr: "02", RFCSaprouter: "/H/qas-router/S/3299", RFCUser: "RFCUSER"}
	pin, err := adt.ParseIdentityPin("QAS.200/READER")
	if err != nil {
		t.Fatal(err)
	}
	s.config.Expect = pin

	dest, ok := rfcTunnelDest(s.config)
	if !ok {
		t.Fatal("a server system with a route does not tunnel its ADT client")
	}
	if dest.Expect != pin {
		t.Fatalf("tunnel destination pin = %v, want %v", dest.Expect, pin)
	}
	if err := dest.Expect.CheckConfigured(dest.User, dest.Client); err == nil {
		t.Errorf("RFC user %s passes a pin for READER", dest.User)
	}
}

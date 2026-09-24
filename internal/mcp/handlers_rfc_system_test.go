package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/vibing-steampunk/pkg/config"
)

// A server started with -s must reach its own system over RFC: not the host of
// the default .vsp.json system, and not the logon in SAP_USER/SAP_PASSWORD.
func TestBaseRFCInput_ServerSystem(t *testing.T) {
	dir := t.TempDir()
	vsp := `{"default":"DEV","systems":{"DEV":{"url":"https://dev:44300","rfc_host":"10.0.0.1","rfc_sysnr":"01","rfc_user":"DEVRFC"}}}`
	if err := os.WriteFile(filepath.Join(dir, ".vsp.json"), []byte(vsp), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	t.Setenv("SAP_USER", "DEVUSER")
	t.Setenv("SAP_PASSWORD", "devsecret")
	t.Setenv("SAP_SAPROUTER", "/H/dev-router/S/3299")

	cfg := &Config{
		BaseURL: "https://qas:44300", Username: "READER", Password: "datasecret", Client: "200", Language: "RU",
		System: &config.SystemConfig{RFCHost: "10.0.0.2", RFCSysnr: "02", RFCSaprouter: "/H/qas-router/S/3299"},
	}
	in := baseRFCInput(cfg)
	if in.RFCHost != "10.0.0.2" || in.RFCSysnr != "02" || in.RFCSaprouter != "/H/qas-router/S/3299" {
		t.Fatalf("destination not taken from the server's system: %+v", in)
	}
	if in.RFCUser != "" || in.RFCPassword != "" || in.User != "READER" || in.Password != "datasecret" {
		t.Fatalf("logon must be the server's own, not SAP_USER/SAP_PASSWORD or the default system's: %+v", in)
	}

	// Without a system the server keeps the old behaviour.
	cfg.System = nil
	in = baseRFCInput(cfg)
	if in.RFCHost != "10.0.0.1" || in.RFCUser != "DEVRFC" || in.RFCPassword != "devsecret" || in.RFCSaprouter != "/H/dev-router/S/3299" {
		t.Fatalf("server without -s changed: %+v", in)
	}
}

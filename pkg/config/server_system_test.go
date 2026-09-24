package config

import "testing"

func serverSystemsConfig() *SystemsConfig {
	return &SystemsConfig{Systems: map[string]SystemConfig{
		"DATA": {URL: "https://sap.example:44300", User: "READER", Client: "101"},
	}}
}

// GetSystem keeps falling back to the shared RFC environment, as the CLI
// always has.
func TestGetSystem_FallsBackToSharedSAPEnv(t *testing.T) {
	t.Setenv("SAP_USER", "DEVUSER")
	t.Setenv("SAP_PASSWORD", "devsecret")
	t.Setenv("SAP_SAPROUTER", "/H/router/S/3299")
	sys, err := serverSystemsConfig().GetSystem("DATA")
	if err != nil {
		t.Fatal(err)
	}
	if sys.RFCUser != "DEVUSER" || sys.RFCPassword != "devsecret" || sys.RFCSaprouter != "/H/router/S/3299" {
		t.Fatalf("shared RFC environment not applied: %+v", sys)
	}
}

// GetServerSystem does not: SAP_* usually belongs to another system.
func TestGetServerSystem_IgnoresSharedSAPEnv(t *testing.T) {
	t.Setenv("SAP_USER", "DEVUSER")
	t.Setenv("SAP_PASSWORD", "devsecret")
	t.Setenv("SAP_SAPROUTER", "/H/router/S/3299")
	t.Setenv("VSP_DATA_PASSWORD", "")
	sys, err := serverSystemsConfig().GetServerSystem("DATA")
	if err != nil {
		t.Fatal(err)
	}
	if sys.Password != "" || sys.RFCUser != "" || sys.RFCPassword != "" || sys.RFCSaprouter != "" {
		t.Fatalf("shared SAP_* leaked into a server system: %+v", sys)
	}
}

func TestGetServerSystem_UsesSystemEnv(t *testing.T) {
	t.Setenv("VSP_DATA_PASSWORD", "datasecret")
	t.Setenv("VSP_DATA_RFC_PASSWORD", "rfcsecret")
	t.Setenv("VSP_DATA_RFC_SAPROUTER", "/H/data-router/S/3299")
	sys, err := serverSystemsConfig().GetServerSystem("DATA")
	if err != nil {
		t.Fatal(err)
	}
	if sys.Password != "datasecret" || sys.RFCPassword != "rfcsecret" || sys.RFCSaprouter != "/H/data-router/S/3299" {
		t.Fatalf("per-system variables not applied: %+v", sys)
	}
	if sys.Client != "101" || sys.Language != "EN" {
		t.Fatalf("defaults not applied: %+v", sys)
	}
}

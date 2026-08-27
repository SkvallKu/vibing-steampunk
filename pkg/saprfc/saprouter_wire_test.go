package saprfc

import "testing"

func TestNormalizeRoutePrefix(t *testing.T) {
	cases := map[string]string{
		"":                          "",
		"   ":                       "",
		"/H/router/S/3299":          "/H/router/S/3299/H/",
		"/H/router/S/3299/":         "/H/router/S/3299/H/",
		"  /H/router/S/3299  ":      "/H/router/S/3299/H/",
		"/H/router/S/3299/H/":       "/H/router/S/3299/H/",
		"/H/r1/S/3299/H/r2/S/3299":  "/H/r1/S/3299/H/r2/S/3299/H/",
		"/H/router/S/3299/W/secret": "/H/router/S/3299/W/secret/H/",
	}
	for in, want := range cases {
		if got := normalizeRoutePrefix(in); got != want {
			t.Errorf("normalizeRoutePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveWiresSaprouter(t *testing.T) {
	base := Input{URL: "https://sap.example:44300", User: "DEV", Password: "pw", Client: "100"}

	// flag beats per-system setting, and both get normalized
	in := base
	in.RFCSaprouter = "/H/from-config/S/3299"
	in.SaprouterFlag = "/H/from-flag/S/3299"
	p, err := Resolve(in)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Router != "/H/from-flag/S/3299/H/" {
		t.Fatalf("Router = %q, want the normalized flag value", p.Router)
	}

	// no router configured -> empty (direct connection)
	if p, _ := Resolve(base); p.Router != "" {
		t.Fatalf("Router = %q, want empty for a direct connection", p.Router)
	}

	// per-system value used when no flag
	in = base
	in.RFCSaprouter = "/H/only-config/S/3299/H/"
	if p, _ := Resolve(in); p.Router != "/H/only-config/S/3299/H/" {
		t.Fatalf("Router = %q, want the per-system value", p.Router)
	}
}

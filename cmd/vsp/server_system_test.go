package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/vibing-steampunk/internal/mcp"
	"github.com/spf13/pflag"
)

// The .vsp.json every test here starts from: a development system that is the
// default, and a data system in another client with its own user.
const serverSystemsJSON = `{
  "default": "DEV",
  "systems": {
    "DEV": {
      "url": "https://dev.example:44300", "user": "DEVUSER", "client": "100",
      "allowed_packages": ["Z*", "$TMP"], "rfc_host": "10.0.0.1", "rfc_sysnr": "01"
    },
    "DATA": {
      "url": "https://dev.example:44300", "user": "READER", "client": "101",
      "language": "RU", "read_only": true, "rfc_host": "10.0.0.1", "rfc_sysnr": "01"
    },
    "NOURL":  { "user": "READER", "client": "101" },
    "NOUSER": { "url": "https://dev.example:44300", "client": "101" },
    "SSO":    { "url": "https://dev.example:44300", "auth": "sso" }
  }
}`

// serverEnv is what vsp would read from a project .env: the development
// system's logon under the shared SAP_* names.
var serverEnv = map[string]string{
	"SAP_USER":          "DEVUSER",
	"SAP_PASSWORD":      "devsecret",
	"SAP_CLIENT":        "100",
	"SAP_SAPROUTER":     "/H/other-router/S/3299",
	"VSP_DATA_PASSWORD": "datasecret",
}

// startServerConfig runs the server's configuration steps, as runServer does
// up to validation, for the given arguments and environment, in a directory
// holding serverSystemsJSON. It returns a copy of the resulting config.
func startServerConfig(t *testing.T, args []string, env map[string]string) (mcp.Config, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".vsp.json"), []byte(serverSystemsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	for _, k := range []string{"SAP_URL", "SAP_USER", "SAP_PASSWORD", "SAP_CLIENT", "SAP_LANGUAGE",
		"SAP_SAPROUTER", "SAP_READ_ONLY", "SAP_ALLOWED_PACKAGES", "SAP_TRANSPORT_ATTRIBUTE",
		"VSP_DATA_PASSWORD", "VSP_NOUSER_PASSWORD", "VSP_NOURL_PASSWORD"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	resetServerFlags(t)
	t.Cleanup(func() { resetServerFlags(t) })
	if err := rootCmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	resolveConfig(rootCmd)
	err := applySystemProfile(rootCmd)
	return *cfg, err
}

// resetServerFlags puts every server flag, and the config behind it, back to
// its default, as a fresh process would have it.
func resetServerFlags(t *testing.T) {
	t.Helper()
	*cfg = mcp.Config{}
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else if err := f.Value.Set(f.DefValue); err != nil {
			t.Fatalf("reset --%s: %v", f.Name, err)
		}
		f.Changed = false
	}
	rootCmd.Flags().VisitAll(reset)
	rootCmd.PersistentFlags().VisitAll(reset)
}

// Without -s nothing about the server's configuration may change, whatever
// .vsp.json holds: servers configured by flags and .env are everywhere.
func TestServerWithoutSystemIsUnchanged(t *testing.T) {
	args := []string{"--url", "https://dev.example:44300", "--client", "100", "--allowed-packages", "$TMP"}
	before, err := startServerConfig(t, args, serverEnv)
	if err != nil {
		t.Fatal(err)
	}
	resetServerFlags(t)
	if err := rootCmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	resolveConfig(rootCmd)
	after := *cfg
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("applySystemProfile changed a server started without -s:\n%+v\n%+v", before, after)
	}
	if before.System != nil || before.Username != "DEVUSER" || before.Client != "100" ||
		strings.Join(before.AllowedPackages, ",") != "$TMP" || before.ReadOnly {
		t.Fatalf("unexpected legacy config: %+v", before)
	}
}

// The point of -s: the system's own logon wins over the shared SAP_* logon
// that .env provides for another system.
func TestServerSystemBeatsSharedEnv(t *testing.T) {
	got, err := startServerConfig(t, []string{"-s", "DATA"}, serverEnv)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "READER" || got.Password != "datasecret" || got.Client != "101" || got.Language != "RU" {
		t.Fatalf("logon not taken from the system: user=%q client=%q lang=%q", got.Username, got.Client, got.Language)
	}
	if got.BaseURL != "https://dev.example:44300" || !got.ReadOnly || got.System == nil {
		t.Fatalf("system not applied: %+v", got)
	}
}

func TestServerFlagsBeatSystem(t *testing.T) {
	got, err := startServerConfig(t,
		[]string{"-s", "DATA", "--client", "102", "--user", "OTHER", "--password", "flagsecret", "--language", "EN"},
		serverEnv)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "OTHER" || got.Password != "flagsecret" || got.Client != "102" || got.Language != "EN" {
		t.Fatalf("flags did not win: user=%q client=%q lang=%q", got.Username, got.Client, got.Language)
	}
}

// A system without its own password must stop the server, not borrow
// SAP_PASSWORD from the environment.
func TestServerSystemWithoutPasswordFails(t *testing.T) {
	env := map[string]string{"SAP_USER": "DEVUSER", "SAP_PASSWORD": "devsecret"}
	_, err := startServerConfig(t, []string{"-s", "DATA"}, env)
	if err == nil || !strings.Contains(err.Error(), "VSP_DATA_PASSWORD") {
		t.Fatalf("want an error naming VSP_DATA_PASSWORD, got %v", err)
	}
}

func TestServerSystemErrors(t *testing.T) {
	env := map[string]string{"SAP_USER": "DEVUSER", "SAP_PASSWORD": "devsecret", "SAP_URL": "https://dev.example:44300",
		"VSP_NOUSER_PASSWORD": "x", "VSP_NOURL_PASSWORD": "x"}
	for _, tc := range []struct{ system, want string }{
		{"MISSING", "not found"},
		{"NOURL", `no "url"`},
		{"NOUSER", `no "user"`},
		{"SSO", "not supported"},
	} {
		t.Run(tc.system, func(t *testing.T) {
			_, err := startServerConfig(t, []string{"-s", tc.system}, env)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestServerSystemSafetyOnlyNarrows(t *testing.T) {
	env := map[string]string{"SAP_PASSWORD": "devsecret", "VSP_DEV_PASSWORD": "devsecret"}

	got, err := startServerConfig(t, []string{"-s", "DEV"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.AllowedPackages, ",") != "Z*,$TMP" {
		t.Fatalf("the system's allowed packages must apply without a flag, got %v", got.AllowedPackages)
	}

	got, err = startServerConfig(t, []string{"-s", "DEV", "--allowed-packages", "$TMP,ZFOO*"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.AllowedPackages, ",") != "$TMP,ZFOO*" {
		t.Fatalf("a narrower flag must apply, got %v", got.AllowedPackages)
	}

	if _, err = startServerConfig(t, []string{"-s", "DEV", "--allowed-packages", "Y*"}, env); err == nil {
		t.Fatal("a flag wider than the system's allowed packages must be refused")
	}

	got, err = startServerConfig(t, []string{"-s", "DATA", "--read-only=false"}, serverEnv)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ReadOnly {
		t.Fatal("a read_only system must stay read-only whatever the flags say")
	}
}

func TestPatternWithin(t *testing.T) {
	for _, tc := range []struct {
		p        string
		patterns []string
		want     bool
	}{
		{"$TMP", []string{"Z*", "$TMP"}, true},
		{"ZFOO*", []string{"Z*"}, true},
		{"zfoo", []string{"Z*"}, true},
		{"Z*", []string{"ZFOO*"}, false},
		{"Z*", []string{"Z"}, false},
		{"Y*", []string{"Z*", "$TMP"}, false},
	} {
		if got := patternWithin(tc.p, tc.patterns); got != tc.want {
			t.Errorf("patternWithin(%q, %v) = %v, want %v", tc.p, tc.patterns, got, tc.want)
		}
	}
}

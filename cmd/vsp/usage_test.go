package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// A command that fails at run time reports the error and nothing else; a
// wrong flag still gets the usage screen. Neither is printed by cobra itself:
// main prints the error, once.
func TestUsageOnlyForUsageMistakes(t *testing.T) {
	failing := &cobra.Command{
		Use:  "usage-test-fail",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return errors.New("status 404") },
	}
	rootCmd.AddCommand(failing)
	defer rootCmd.RemoveCommand(failing)

	for _, c := range []struct {
		args      []string
		wantUsage bool
	}{
		{[]string{"usage-test-fail"}, false},
		{[]string{"usage-test-fail", "--no-such-flag"}, true},
		{[]string{"usage-test-fail", "extra"}, true},
	} {
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.SetArgs(c.args)
		err := rootCmd.Execute()
		failing.SilenceUsage = false
		if err == nil {
			t.Fatalf("%v: no error", c.args)
		}
		if got := strings.Contains(out.String(), "Usage:"); got != c.wantUsage {
			t.Errorf("%v: usage printed = %v, want %v\n%s", c.args, got, c.wantUsage, out.String())
		}
		if strings.Contains(out.String(), "Error:") {
			t.Errorf("%v: cobra printed the error, main prints it too\n%s", c.args, out.String())
		}
	}
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs(nil)
}

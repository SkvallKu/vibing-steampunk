package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oisee/open-rfc-go/rfc"
	"github.com/oisee/vibing-steampunk/embedded/abap/adtfallback"
	installer "github.com/oisee/vibing-steampunk/internal/install"
	"github.com/oisee/vibing-steampunk/pkg/adt"
	"github.com/oisee/vibing-steampunk/pkg/saprfc"
	"github.com/spf13/cobra"
)

// ADT fallback points: stand-ins under /sap/bc/adt/zvsp for ADT resources a
// release does not have (pkg/adt/fallback.go). The classes go over ADT like
// any other; the BAdI implementations that register them cannot, as ADT
// before 7.5x has no enhancements resource, so a generated report creates
// them through RFC_ABAP_INSTALL_AND_RUN, which saves it as Z$$$XRFC, runs it
// and deletes it again.

var installADTFallbackCmd = &cobra.Command{
	Use:   "adt-fallback",
	Short: "Install stand-ins for ADT resources this release lacks (/sap/bc/adt/zvsp)",
	Long: `Install the ADT fallback points into the local package ` + adtfallback.Package + `.

On a release without a standard ADT resource vsp then uses the stand-in under
/sap/bc/adt/zvsp, found through the discovery document; once the system has the
standard resource (after an upgrade) vsp uses that again. Covered now:
  /sap/bc/adt/textelements    (programs, classes) — vsp texts, GetTextElements
  /sap/bc/adt/packages        (read, create, delete) — CreatePackage, GetPackage
  /sap/bc/adt/ddic/tables     (transparent tables as source) — GetTable, WriteSource TABL
  /sap/bc/adt/ddic/structures (structures as source, 7.40) — GetStructure, WriteSource STRUCT
The table and structure source is 7.50's own where the release has it,
and vsp's (the same language, checked against 7.50's) where it does not.

What it does:
  1. creates the package if needed (with the same temporary report: ADT
     before 7.5x cannot create one);
  2. writes and activates the classes over ADT, skipping those already the same;
  3. creates the BAdI implementations (ENHO) with a temporary report run
     through RFC_ABAP_INSTALL_AND_RUN; nothing of the report stays behind;
  4. checks GET /sap/bc/adt/zvsp/info.

Running it again updates what changed and leaves the rest. Without the
authorization for RFC_ABAP_INSTALL_AND_RUN, --print-report gives the report to
run by hand in SE38; run install again afterwards.

Examples:
  vsp -s DEV install adt-fallback --dry-run
  vsp -s DEV install adt-fallback
  vsp install adt-fallback --print-report > zvsp_adt_fallback_enho.abap`,
	RunE: runInstallADTFallback,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove components installed with vsp install",
}

var uninstallADTFallbackCmd = &cobra.Command{
	Use:   "adt-fallback",
	Short: "Remove the ADT fallback points: enhancements, objects, package",
	Long: `Remove the ADT fallback points in reverse order:
  1. the BAdI implementations (ENHO), with the same temporary report — first,
     so that no BAdI points at a class that is gone;
  2. every object in ` + adtfallback.Package + `, as the package holds them
     (so also what an older version or a test left there);
  3. the package, with the report again.

Examples:
  vsp -s DEV uninstall adt-fallback --dry-run
  vsp -s DEV uninstall adt-fallback`,
	RunE: runUninstallADTFallback,
}

func init() {
	for _, c := range []*cobra.Command{installADTFallbackCmd, uninstallADTFallbackCmd} {
		c.Flags().Bool("dry-run", false, "Only report what would be done; nothing is written")
		c.Flags().Bool("print-report", false, "Print the ENHO report for SE38 and exit; nothing is sent")
	}
	installCmd.AddCommand(installADTFallbackCmd)
	uninstallCmd.AddCommand(uninstallADTFallbackCmd)
	rootCmd.AddCommand(uninstallCmd)
}

func runInstallADTFallback(cmd *cobra.Command, _ []string) error {
	if printed, err := printADTFallbackReport(cmd, adtfallback.ModeInstall); printed || err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	params, client, err := adtFallbackClient(cmd, dryRun)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pkg := adtfallback.Package

	fmt.Fprintf(os.Stderr, "ADT fallback points %s → %s (package %s)\n\n", adtfallback.Version(), systemLabel(params), pkg)

	exists, err := adtFallbackPackageExists(ctx, client, pkg)
	if err != nil {
		return fmt.Errorf("checking package %s: %w", pkg, err)
	}
	type step struct {
		obj    adtfallback.Object
		action string
	}
	var steps []step
	for _, o := range adtfallback.Objects() {
		action := "CREATE"
		if exists {
			if cur, gerr := client.GetSource(ctx, o.Type, o.Name, nil); gerr == nil {
				action = "UPDATE"
				if sameABAP(cur, o.Source) {
					action = "UNCHANGED"
				}
			}
		}
		steps = append(steps, step{o, action})
	}

	if !exists {
		fmt.Fprintf(os.Stderr, "  package %-26s CREATE\n", pkg)
	}
	for _, s := range steps {
		fmt.Fprintf(os.Stderr, "  %-4s %-30s %s\n", s.obj.Type, s.obj.Name, s.action)
	}
	enhos, err := adtFallbackEnhancements(ctx, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ENHO: could not read BADI_IMPL (%v); the report decides\n", err)
	}
	for _, e := range adtfallback.Enhancements {
		state := "CREATE"
		if enhos[e] > 0 {
			state = "CHECK (exists; the report changes it only if it differs)"
		}
		fmt.Fprintf(os.Stderr, "  ENHO %-30s %s\n", e, state)
	}
	fmt.Fprintln(os.Stderr)
	if dryRun {
		fmt.Fprintln(os.Stderr, "Dry run - nothing was written.")
		return nil
	}

	if !exists {
		lines, err := runADTFallbackReport(cmd, adtfallback.ModePackage)
		if err != nil {
			return err
		}
		if err := reportFailed(lines); err != nil {
			return fmt.Errorf("package %s: %w", pkg, err)
		}
		if ok, err := adtFallbackPackageExists(ctx, client, pkg); err != nil || !ok {
			return fmt.Errorf("package %s: the report created it, but ADT does not see it (%v)", pkg, err)
		}
	}
	for _, s := range steps {
		if s.action == "UNCHANGED" {
			continue
		}
		fmt.Fprintf(os.Stderr, "  %-30s ... ", s.obj.Name)
		res, err := installer.DeploySource(ctx, client, s.obj.Type, s.obj.Name, s.obj.Source, &adt.WriteSourceOptions{
			Package:     pkg,
			Description: s.obj.Description,
			Mode:        adt.WriteModeUpsert,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAILED")
			if res != nil {
				for _, se := range res.SyntaxErrors {
					fmt.Fprintf(os.Stderr, "      %s line %d: %s\n", se.Severity, se.Line, se.Text)
				}
			}
			return fmt.Errorf("%s: %w; the enhancements were not touched", s.obj.Name, err)
		}
		fmt.Fprintln(os.Stderr, "OK")
	}

	lines, err := runADTFallbackReport(cmd, adtfallback.ModeInstall)
	if err != nil {
		return err
	}
	if err := reportFailed(lines); err != nil {
		return err
	}

	// The BAdI runtime buffer takes a while to see a new implementation —
	// over a minute on a 7.50 after a fresh install, as the buffers of the
	// application servers synchronize, more than three minutes on a 7.40 —
	// and a class load a moment to see a new version: ask for six minutes.
	var got string
	for attempt := 0; ; attempt++ {
		if attempt == 1 {
			fmt.Fprintf(os.Stderr, "  waiting for %s/info (the BAdI buffer synchronizes, up to 6 minutes) ...\n", adt.FallbackRoot)
		}
		info, err := client.RawGet(ctx, adt.FallbackRoot+"/info", "text/plain")
		if err == nil {
			got = adtFallbackInfoVersion(string(info.Body))
			if got == adtfallback.Version() {
				break
			}
			err = fmt.Errorf("it reports version %q, not %q", got, adtfallback.Version())
		}
		if attempt == 72 {
			return fmt.Errorf("installed, but GET %s/info after six minutes: %w; run install again later to check", adt.FallbackRoot, err)
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Fprintf(os.Stderr, "\n%s/info: version %s\nDone.\n", adt.FallbackRoot, got)
	return nil
}

func runUninstallADTFallback(cmd *cobra.Command, _ []string) error {
	if printed, err := printADTFallbackReport(cmd, adtfallback.ModeRemove); printed || err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	params, client, err := adtFallbackClient(cmd, dryRun)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pkg := adtfallback.Package

	fmt.Fprintf(os.Stderr, "Removing ADT fallback points from %s (package %s)\n\n", systemLabel(params), pkg)

	enhos, enhErr := adtFallbackEnhancements(ctx, client)
	if enhErr != nil {
		fmt.Fprintf(os.Stderr, "  ENHO: could not read BADI_IMPL (%v); the report decides\n", enhErr)
	}
	for _, e := range adtfallback.Enhancements {
		state := "absent"
		if enhos[e] > 0 || enhErr != nil {
			state = "DELETE"
		}
		fmt.Fprintf(os.Stderr, "  ENHO %-30s %s\n", e, state)
	}
	exists, err := adtFallbackPackageExists(ctx, client, pkg)
	if err != nil {
		return fmt.Errorf("checking package %s: %w", pkg, err)
	}
	var objects []adt.PackageObject
	if exists {
		content, err := client.GetPackage(ctx, pkg)
		if err != nil {
			return fmt.Errorf("reading package %s: %w", pkg, err)
		}
		objects = removalOrder(content.Objects)
		for _, o := range objects {
			fmt.Fprintf(os.Stderr, "  %-9s %-30s DELETE\n", o.Type, o.Name)
		}
		fmt.Fprintf(os.Stderr, "  package   %-30s DELETE\n", pkg)
	} else {
		fmt.Fprintf(os.Stderr, "  package   %-30s absent\n", pkg)
	}
	fmt.Fprintln(os.Stderr)
	if dryRun {
		fmt.Fprintln(os.Stderr, "Dry run - nothing was deleted.")
		return nil
	}

	anyENHO := enhErr != nil
	for _, e := range adtfallback.Enhancements {
		anyENHO = anyENHO || enhos[e] > 0
	}
	if anyENHO {
		lines, err := runADTFallbackReport(cmd, adtfallback.ModeRemove)
		if err != nil {
			return err
		}
		if err := reportFailed(lines); err != nil {
			return fmt.Errorf("%w; the classes were left in place, as a BAdI may still point at them", err)
		}
	}

	var failed []string
	for _, o := range objects {
		fmt.Fprintf(os.Stderr, "  %-30s ... ", o.Name)
		if err := lockAndDelete(ctx, client, o.URI); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
			failed = append(failed, o.Name)
			continue
		}
		fmt.Fprintln(os.Stderr, "DELETED")
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not delete %s; the package was left", strings.Join(failed, ", "))
	}
	if exists {
		lines, err := runADTFallbackReport(cmd, adtfallback.ModeDropPackage)
		if err != nil {
			return err
		}
		if err := reportFailed(lines); err != nil {
			return fmt.Errorf("package %s: %w", pkg, err)
		}
	}

	_, err = client.RawGet(ctx, adt.FallbackRoot+"/info", "text/plain")
	if err == nil || !adt.IsNoHandlerError(err) {
		return fmt.Errorf("removed, but %s/info still answers (%v)", adt.FallbackRoot, err)
	}
	fmt.Fprintf(os.Stderr, "\n%s is gone.\nDone.\n", adt.FallbackRoot)
	return nil
}

// adtFallbackClient refuses a read-only system before anything is sent: the
// report goes over RFC, past the mutation checks of the ADT client.
func adtFallbackClient(cmd *cobra.Command, dryRun bool) (*systemParams, *adt.Client, error) {
	params, err := resolveSystemParams(cmd)
	if err != nil {
		return nil, nil, err
	}
	if params.ReadOnly && !dryRun {
		return nil, nil, fmt.Errorf("system %s is read-only; installing or removing the ADT fallback points writes to it", systemLabel(params))
	}
	client, err := getClient(params)
	if err != nil {
		return nil, nil, err
	}
	return params, client, nil
}

func systemLabel(p *systemParams) string {
	if p.Name != "" {
		return p.Name
	}
	return p.URL
}

func printADTFallbackReport(cmd *cobra.Command, mode string) (bool, error) {
	if print, _ := cmd.Flags().GetBool("print-report"); !print {
		return false, nil
	}
	src, err := adtfallback.Report(mode, adtfallback.Package)
	if err != nil {
		return true, err
	}
	fmt.Print(src)
	return true, nil
}

// adtFallbackPackageExists reads TDEVC: before 7.5x ADT has no package
// resource, and GET /sap/bc/adt/packages/{name} answers 404 for every package.
func adtFallbackPackageExists(ctx context.Context, client *adt.Client, pkg string) (bool, error) {
	res, err := client.GetTableContents(ctx, "TDEVC", 1,
		"SELECT DEVCLASS FROM TDEVC WHERE DEVCLASS = '"+strings.ReplaceAll(pkg, "'", "''")+"'")
	if err != nil {
		return false, err
	}
	return len(res.Rows) > 0, nil
}

// adtFallbackEnhancements counts the BAdI implementations of each of our
// ENHO objects, read from BADI_IMPL through data preview.
func adtFallbackEnhancements(ctx context.Context, client *adt.Client) (map[string]int, error) {
	res, err := client.GetTableContents(ctx, "BADI_IMPL", 50,
		"SELECT ENHNAME, BADI_IMPL FROM BADI_IMPL WHERE ENHNAME LIKE 'ZVSP_ADT_FALLBACK%'")
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, row := range res.Rows {
		out[strings.TrimSpace(fmt.Sprint(row["ENHNAME"]))]++
	}
	return out, nil
}

func runADTFallbackReport(cmd *cobra.Command, mode string) ([]adtfallback.ReportLine, error) {
	src, err := adtfallback.Report(mode, adtfallback.Package)
	if err != nil {
		return nil, err
	}
	program := []map[string]any{}
	for _, l := range adtfallback.ReportLines(src) {
		program = append(program, map[string]any{"LINE": l})
	}
	var writes []string
	err = withRFCDestTimeout(cmd, 5*time.Minute, func(ctx context.Context, c *rfc.Client, _ saprfc.Params) error {
		r, err := c.Call(ctx, "RFC_ABAP_INSTALL_AND_RUN", rfc.Params{"MODE": "F", "PROGRAM": program})
		if err != nil {
			var exc *rfc.ABAPException
			if errors.As(err, &exc) {
				return fmt.Errorf("RFC_ABAP_INSTALL_AND_RUN refused: %w; without the authorization for it, run the report by hand (--print-report into SE38) and run this command again", err)
			}
			return fmt.Errorf("RFC_ABAP_INSTALL_AND_RUN: %w", err)
		}
		if msg := strings.TrimSpace(fmt.Sprint(r.Get("ERRORMESSAGE"))); msg != "" && msg != "<nil>" {
			return fmt.Errorf("the ENHO report does not compile on this system: %s", msg)
		}
		for _, row := range r.Table("WRITES") {
			for _, v := range row {
				writes = append(writes, strings.TrimRight(fmt.Sprint(v), " "))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	lines := adtfallback.ParseReportOutput(writes)
	for _, l := range lines {
		fmt.Fprintf(os.Stderr, "  report %-30s %-24s %s %s\n", l.Enhancement, l.Implementation, l.Status, l.Text)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("the ENHO report printed nothing (output: %q)", strings.Join(writes, " | "))
	}
	return lines, nil
}

func reportFailed(lines []adtfallback.ReportLine) error {
	var errs []string
	for _, l := range lines {
		if l.Status == "ERROR" {
			errs = append(errs, l.Enhancement+": "+l.Text)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("the ENHO report failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// removalOrder deletes classes before interfaces, and what the installer did
// not put there (a test program, an older class) first of all. Enhancements
// are left out: the report deletes them, and ADT before 7.5x cannot.
func removalOrder(objects []adt.PackageObject) []adt.PackageObject {
	installed := map[string]int{}
	for i, o := range adtfallback.Objects() {
		installed[o.Name] = i
	}
	var foreign, ours []adt.PackageObject
	for _, o := range objects {
		if strings.HasPrefix(strings.ToUpper(o.Type), "ENHO") {
			continue
		}
		if _, ok := installed[strings.ToUpper(o.Name)]; ok {
			ours = append(ours, o)
		} else {
			foreign = append(foreign, o)
		}
	}
	// Reverse install order: users before what they use.
	for i := 0; i < len(ours); i++ {
		for j := i + 1; j < len(ours); j++ {
			if installed[strings.ToUpper(ours[j].Name)] > installed[strings.ToUpper(ours[i].Name)] {
				ours[i], ours[j] = ours[j], ours[i]
			}
		}
	}
	return append(foreign, ours...)
}

func lockAndDelete(ctx context.Context, client *adt.Client, uri string) error {
	lock, err := client.LockObject(ctx, uri, "MODIFY")
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if err := client.DeleteObject(ctx, uri, lock.LockHandle, ""); err != nil {
		_ = client.UnlockObject(ctx, uri, lock.LockHandle)
		return err
	}
	return nil
}

// sameABAP compares two sources as ABAP sees them: line ends and trailing
// blanks do not count.
func sameABAP(a, b string) bool {
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		lines := strings.Split(strings.TrimRight(s, "\n "), "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " \t")
		}
		return strings.Join(lines, "\n")
	}
	return norm(a) == norm(b)
}

func adtFallbackInfoVersion(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "version="); ok {
			return v
		}
	}
	return ""
}

// Package adtfallback holds the ABAP of the ADT fallback points: stand-ins,
// under /sap/bc/adt/zvsp, for ADT resources a release does not have. See
// pkg/adt/fallback.go for how vsp finds them and cmd/vsp/adtfallback.go for
// how they are installed and removed.
package adtfallback

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

// Package is the local package everything is installed into.
const Package = "$VSP_ADT_FALLBACK_POINTS"

// Enhancements are the ENHO objects the report creates, in install order.
var Enhancements = []string{"ZVSP_ADT_FALLBACK_APP", "ZVSP_ADT_FALLBACK_URI_MAP"}

// ReportLineWidth is the width RFC_ABAP_INSTALL_AND_RUN takes a program in.
const ReportLineWidth = 72

//go:embed zif_vsp_adt_feature.intf.abap
var zifVspAdtFeature string

//go:embed zcl_vsp_adt_app.clas.abap
var zclVspAdtApp string

//go:embed zcl_vsp_adt_res_info.clas.abap
var zclVspAdtResInfo string

//go:embed zcl_vsp_adt_uri_mapper.clas.abap
var zclVspAdtURIMapper string

//go:embed zcl_vsp_adt_txt_format.clas.abap
var zclVspAdtTxtFormat string

//go:embed zcl_vsp_adt_txt_lock.clas.abap
var zclVspAdtTxtLock string

//go:embed zcl_vsp_adt_txt_pool.clas.abap
var zclVspAdtTxtPool string

//go:embed zcl_vsp_adt_txt_resource.clas.abap
var zclVspAdtTxtResource string

//go:embed zcl_vsp_adt_txt_feature.clas.abap
var zclVspAdtTxtFeature string

//go:embed zvsp_adt_fallback_enho.prog.abap
var enhoReport string

// Object is one ABAP object of the set.
type Object struct {
	Type        string // INTF or CLAS
	Name        string
	Source      string
	Description string // at most 60 characters: the short text of a class
}

// Objects returns the objects in install order: each is activated on its
// own, so nothing may come before what it uses.
func Objects() []Object {
	return []Object{
		{"INTF", "ZIF_VSP_ADT_FEATURE", zifVspAdtFeature, "vsp ADT fallback: one feature"},
		{"CLAS", "ZCL_VSP_ADT_APP", zclVspAdtApp, "vsp ADT fallback: application"},
		{"CLAS", "ZCL_VSP_ADT_RES_INFO", zclVspAdtResInfo, "vsp ADT fallback: /info"},
		{"CLAS", "ZCL_VSP_ADT_URI_MAPPER", zclVspAdtURIMapper, "vsp ADT fallback: URI mapping"},
		{"CLAS", "ZCL_VSP_ADT_TXT_FORMAT", zclVspAdtTxtFormat, "vsp ADT fallback: text pool formats"},
		{"CLAS", "ZCL_VSP_ADT_TXT_LOCK", zclVspAdtTxtLock, "vsp ADT fallback: text pool lock"},
		{"CLAS", "ZCL_VSP_ADT_TXT_POOL", zclVspAdtTxtPool, "vsp ADT fallback: one text pool"},
		{"CLAS", "ZCL_VSP_ADT_TXT_RESOURCE", zclVspAdtTxtResource, "vsp ADT fallback: text elements"},
		{"CLAS", "ZCL_VSP_ADT_TXT_FEATURE", zclVspAdtTxtFeature, "vsp ADT fallback: text elements feature"},
	}
}

var versionRe = regexp.MustCompile(`(?i)CONSTANTS\s+co_version\s+TYPE\s+string\s+VALUE\s+'([^']+)'`)

// Version is the version of the set, as ZCL_VSP_ADT_APP=>CO_VERSION states it
// and GET /sap/bc/adt/zvsp/info reports it.
func Version() string {
	m := versionRe.FindStringSubmatch(zclVspAdtApp)
	if m == nil {
		return ""
	}
	return m[1]
}

// Report modes.
const (
	ModeInstall = "INSTALL"
	ModeRemove  = "REMOVE"
	ModeCheck   = "CHECK"
	// The local package: ADT before 7.5x can neither create nor delete one.
	ModePackage     = "PACKAGE"
	ModeDropPackage = "DROPPACKAGE"
)

// Report returns the ENHO report for a mode and package, checked to fit the
// 72-character lines RFC_ABAP_INSTALL_AND_RUN takes.
func Report(mode, pkg string) (string, error) {
	switch mode {
	case ModeInstall, ModeRemove, ModeCheck, ModePackage, ModeDropPackage:
	default:
		return "", fmt.Errorf("report mode %q: want INSTALL, REMOVE, CHECK, PACKAGE or DROPPACKAGE", mode)
	}
	if pkg == "" || strings.ContainsAny(pkg, "'\n") || len(pkg) > 30 {
		return "", fmt.Errorf("package %q is not a package name", pkg)
	}
	src := strings.ReplaceAll(enhoReport, "{{MODE}}", mode)
	src = strings.ReplaceAll(src, "{{PACKAGE}}", strings.ToUpper(pkg))
	for i, line := range ReportLines(src) {
		if len(line) > ReportLineWidth {
			return "", fmt.Errorf("report line %d is %d characters, more than %d", i+1, len(line), ReportLineWidth)
		}
	}
	return src, nil
}

// ReportLines splits a report into its program lines.
func ReportLines(src string) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	return strings.Split(strings.TrimRight(src, "\n"), "\n")
}

// ReportLine is one line of the report's output: STATUS ENHO [IMPL] [text],
// or STATUS PACKAGE [text] in the package modes.
type ReportLine struct {
	Status, Enhancement, Implementation, Text string
}

// Statuses a report line can carry.
var reportStatuses = map[string]bool{
	"OK": true, "UNCHANGED": true, "DELETED": true, "ABSENT": true,
	"PRESENT": true, "DIFFERENT": true, "MISSING": true, "ERROR": true,
	"CREATED": true, "EXISTS": true,
}

// ParseReportOutput reads the report's WRITE output. Lines that are not
// report lines — a list header, blank lines — are skipped.
func ParseReportOutput(lines []string) []ReportLine {
	var out []ReportLine
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) < 2 || !reportStatuses[f[0]] {
			continue
		}
		rl := ReportLine{Status: f[0], Enhancement: f[1]}
		rest := f[2:]
		// An implementation name is a ZVSP_ name; anything else is text.
		if len(rest) > 0 && strings.HasPrefix(rest[0], "ZVSP_") {
			rl.Implementation = rest[0]
			rest = rest[1:]
		}
		rl.Text = strings.Join(rest, " ")
		out = append(out, rl)
	}
	return out
}

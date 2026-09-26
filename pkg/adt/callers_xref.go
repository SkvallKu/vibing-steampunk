package adt

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Who calls an object, read from the cross-reference tables rather than from
// the where-used list.
//
// The where-used list (/repository/informationsystem/usageReferences) is what
// SE84 shows, and it is what WhereUsed asks first. Releases that do not have
// it — 7.40 SP06 answers 404 on every URI — would otherwise have no answer to
// "who uses this" at all, and every caller of WhereUsed (GetCallersOf, the call
// graph, the dump impact query) would fail with it.
//
// The tables are the same ones Callees reads, turned round: Callees selects by
// INCLUDE and reads NAME, this selects by NAME and reads INCLUDE. That makes it
// a coarser answer than the list, and the differences are worth knowing:
//
//   - a row is a reference, not a call: a program that only declares a
//     variable TYPE MARA is a user of MARA here;
//   - there is no grading into direct and component references, so the only
//     filter is DIRECT = 'X' (INDIRECT rows are types the code never named);
//   - a call of an inherited method is recorded against the class that
//     defines it, so a superclass is used by everyone calling that method on a
//     subclass: CL_SALV_FORM_UIE_LABEL has 9 users in SE84 on 7.50 and 277
//     here, 273 of them calling SET_LABEL_FOR through CL_SALV_FORM_LABEL;
//   - a dynamic call is recorded nowhere, and neither table sees it;
//   - only active sources: an object whose references are all in an
//     unactivated version has them in WBCROSSGTI, which this does not read.

// Where a WhereUsedResult came from.
const (
	// WhereUsedSourceList is the where-used list SE84 uses.
	WhereUsedSourceList = "where_used"
	// WhereUsedSourceXref is the cross-reference tables CROSS, WBCROSSGT and
	// D010INC, read when the list is not on the system.
	WhereUsedSourceXref = "xref"
)

// xrefRowLimit bounds each cross-reference query. It is the most 7.40 SP06 data
// preview answers faithfully: above about 5,000 rows it silently returns its
// default of 100 (see rowFallbackAbove), which would be worse than a cap that
// says it was reached.
const xrefRowLimit = rowFallbackAbove

// xrefInChunk is how many names go into one IN list. Fewer, larger queries
// matter more than they look: every data preview query costs the session a
// generated subroutine pool, and a session holds only a few dozen. A long
// statement is safe because wrapSQL breaks it into lines the service reads.
//
// Two ceilings bound it. 7.40 data preview keeps the whole statement as one
// line of source, and a line over 32767 characters is a runtime error: 2,500
// names were, 1,500 were not. A name of 40 characters takes 44 in the list, so
// 500 is at most about 22,000. And a chunk may bring several rows a name —
// TADIR is asked for 4n+1 — within the 5,000 data preview answers. Measured
// on 881 includes that use MARA: 100, 250, 500 and all 881 at once give the
// same rows, on 7.40 and on 7.50; one query takes about a ninth of the time
// of nine. The DB's own limit on IN lists did not show at 1,500.
const xrefInChunk = 500

// WhereUsedResult is who uses an object, and what the answer is made of.
type WhereUsedResult struct {
	Callers []ExposedCaller `json:"callers"`
	// Source is WhereUsedSourceList or WhereUsedSourceXref.
	Source string `json:"source"`
	// Why says, for WhereUsedSourceXref, what the where-used list answered
	// that sent the question to the tables.
	Why string `json:"why,omitempty"`
	// Unsearched names the cross-reference tables that could not be read. A
	// list with a table missing from it is short, not complete.
	Unsearched []Unsearched `json:"unsearched,omitempty"`
	// Truncated is set when a table held more rows than one query returns;
	// the callers are then the ones in the rows that came back.
	Truncated string `json:"truncated,omitempty"`
	// Notes are caveats that do not make the list shorter: a package that
	// could not be looked up, an include whose kind is unknown.
	Notes []string `json:"notes,omitempty"`
}

// WhereUsedFull asks the where-used list and, where the system does not have
// it, the cross-reference tables.
//
// Only a 404 turns to the tables. Anything else — an authorisation failure,
// a timeout, a 500 — is the list answering, badly, and a second source would
// hide that the first one failed.
func (c *Client) WhereUsedFull(ctx context.Context, objectURI string) (*WhereUsedResult, error) {
	refs, err := c.FindReferences(ctx, objectURI, 0, 0)
	if err == nil {
		return &WhereUsedResult{
			Callers: exposedCallers(refs, objectNameFromURI(objectURI)),
			Source:  WhereUsedSourceList,
		}, nil
	}
	var why string
	switch {
	case isNotFound(err):
		why = WhereUsedWhyNoList
	default:
		return nil, err
	}
	res, xerr := c.CallersFromXref(ctx, objectURI)
	if xerr != nil {
		return nil, fmt.Errorf("%s, and the cross-reference tables could not answer instead: %w", why, xerr)
	}
	res.Why = why
	return res, nil
}

// Why the tables were read instead of the where-used list.
const (
	WhereUsedWhyNoList = "this system has no where-used list " +
		"(/sap/bc/adt/repository/informationsystem/usageReferences answers 404)"
)

// callerTarget is what the tables are searched for.
type callerTarget struct {
	calleeTarget
	// ddic is set for a dictionary type, which only WBCROSSGT records.
	ddic bool
}

// callerTargetFromURI reads the object out of its URI. The dictionary types
// are the addition over calleeTargetFromURI: a table has callers and no
// callees.
func callerTargetFromURI(objectURI string) (callerTarget, error) {
	if t, err := calleeTargetFromURI(objectURI); err == nil {
		return callerTarget{calleeTarget: t}, nil
	}
	path := strings.TrimSpace(objectURI)
	if i := strings.IndexAny(path, "#?"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(path, "/")
	for marker, typ := range map[string]string{
		"/ddic/tables/":       "TABL",
		"/ddic/structures/":   "TABL",
		"/ddic/dataelements/": "DTEL",
		"/ddic/tabletypes/":   "TTYP",
	} {
		if strings.Contains(strings.ToLower(path), marker) {
			return callerTarget{calleeTarget: calleeTarget{Name: segmentAfter(path, marker), Type: typ}, ddic: true}, nil
		}
	}
	return callerTarget{}, fmt.Errorf("who uses %q cannot be read from the cross-reference tables: they are searched for "+
		"classes, interfaces, programs, includes, function groups, function modules, tables, structures, data elements "+
		"and table types", objectURI)
}

// xrefHit is one include that references the target.
type xrefHit struct {
	include string
	// component is the part of the target the include names — a method of
	// a class, a form of a program — when the row says.
	component string
}

// CallersFromXref answers who uses an object from CROSS, WBCROSSGT and
// D010INC. It is exported so that it can be compared with the where-used list
// on a system that has both.
func (c *Client) CallersFromXref(ctx context.Context, objectURI string) (*WhereUsedResult, error) {
	target, err := callerTargetFromURI(objectURI)
	if err != nil {
		return nil, err
	}
	name := strings.ToUpper(strings.TrimSpace(target.Name))
	if err := checkSQLLiteral(name); err != nil {
		return nil, err
	}
	target.Name = name

	res := &WhereUsedResult{Source: WhereUsedSourceXref}
	var hits []xrefHit
	var failures []error

	// lose names a table that could not be read; keep takes the rows of
	// one that could; ask is one query and whichever of the two it earns.
	lose := func(table string, err error) {
		failures = append(failures, fmt.Errorf("%s: %w", table, err))
		res.Unsearched = append(res.Unsearched, Unsearched{Object: table, Reason: err.Error()})
	}
	keep := func(table string, rows *TableContentsResult, read func(row map[string]interface{}) (xrefHit, bool)) {
		if rows == nil {
			return
		}
		if len(rows.Rows) >= xrefRowLimit {
			res.Truncated = fmt.Sprintf("%s holds more than %d references to %s and only the first %d were read, "+
				"so this is some of the users of %s, not all of them", table, xrefRowLimit, name, xrefRowLimit, name)
		}
		for _, row := range rows.Rows {
			if hit, ok := read(row); ok {
				hits = append(hits, hit)
			}
		}
	}
	ask := func(table, query string, read func(row map[string]interface{}) (xrefHit, bool)) {
		rows, err := c.RunQuery(ctx, query, xrefRowLimit)
		if err != nil {
			lose(table, err)
			return
		}
		keep(table, rows, read)
	}

	switch {
	case target.ddic || target.Type == "CLAS" || target.Type == "INTF":
		// The type itself, and its parts: ZCL_FOO\ME:RUN is a call of one
		// method, MARA\TY:MATNR a use of one field. In LIKE an underscore is
		// any character, so ZCL_FOO\% also matches ZCLXFOO\… and the rows
		// are checked against the name afterwards.
		ask("WBCROSSGT", fmt.Sprintf("SELECT INCLUDE, NAME FROM WBCROSSGT WHERE ( NAME = '%s' OR NAME LIKE '%s\\%%' ) AND DIRECT = 'X'", name, name),
			func(row map[string]interface{}) (xrefHit, bool) {
				object, component := splitCrossName(rowString(row, "NAME"))
				if object != name {
					return xrefHit{}, false
				}
				return xrefHit{include: rowString(row, "INCLUDE"), component: component}, true
			})
	case target.Type == "FUNC":
		ask("CROSS", fmt.Sprintf("SELECT INCLUDE FROM CROSS WHERE TYPE = '%s' AND NAME = '%s'", CrossTypeFunctionModule, name), crossHit(""))
	case target.Type == "FUGR":
		pool := functionPool(name)
		modules, err := c.RunQuery(ctx, fmt.Sprintf("SELECT FUNCNAME FROM TFDIR WHERE PNAME = '%s'", pool), xrefRowLimit)
		if err != nil {
			failures = append(failures, fmt.Errorf("TFDIR: %w", err))
			res.Unsearched = append(res.Unsearched, Unsearched{Object: "TFDIR", Reason: "the function modules of " + name + " could not be listed, so their callers were not searched: " + err.Error()})
		} else if modules != nil {
			var names []string
			for _, row := range modules.Rows {
				if fm := strings.ToUpper(rowString(row, "FUNCNAME")); fm != "" && checkSQLLiteral(fm) == nil {
					names = append(names, fm)
				}
			}
			read := func(row map[string]interface{}) (xrefHit, bool) {
				return xrefHit{include: rowString(row, "INCLUDE"), component: strings.ToUpper(rowString(row, "NAME"))}, true
			}
			err := c.queryIn(ctx, names,
				func(in string) string {
					return fmt.Sprintf("SELECT INCLUDE, NAME FROM CROSS WHERE TYPE = '%s' AND NAME IN ( %s )", CrossTypeFunctionModule, in)
				},
				func(int) int { return xrefRowLimit },
				func(rows *TableContentsResult) { keep("CROSS", rows, read) })
			if err != nil {
				lose("CROSS", err)
			}
		}
		// PERFORM form IN PROGRAM SAPL<group>: the group's forms called from
		// outside it.
		ask("CROSS", fmt.Sprintf("SELECT INCLUDE, NAME FROM CROSS WHERE TYPE = '%s' AND PROG = '%s'", CrossTypeSubroutine, pool), crossHit("NAME"))
	case target.Type == "PROG":
		ask("CROSS", fmt.Sprintf("SELECT INCLUDE FROM CROSS WHERE TYPE = '%s' AND NAME = '%s'", CrossTypeReport, name), crossHit(""))
		ask("CROSS", fmt.Sprintf("SELECT INCLUDE, NAME FROM CROSS WHERE TYPE = '%s' AND PROG = '%s'", CrossTypeSubroutine, name), crossHit("NAME"))
	case target.Type == "INCL":
		// Who uses an include is who includes it, and that is D010INC, not
		// a cross-reference table.
		ask("D010INC", fmt.Sprintf("SELECT MASTER FROM D010INC WHERE INCLUDE = '%s'", name),
			func(row map[string]interface{}) (xrefHit, bool) {
				return xrefHit{include: rowString(row, "MASTER")}, true
			})
	default:
		return nil, fmt.Errorf("who uses a %s cannot be read from the cross-reference tables", target.Type)
	}

	if len(hits) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("the cross-reference tables could not be read for %s (%s); they are read over free SQL, "+
			"so this answers nothing if free SQL is blocked or the user may not read them", objectURI, joinErrors(failures))
	}
	res.Callers, res.Notes = c.xrefCallers(ctx, hits, target)
	return res, nil
}

// xrefCaveat is what an answer from the tables should be read with: what it
// could not look at and where it stopped, when either happened.
func xrefCaveat(res *WhereUsedResult) string {
	parts := []string{"read from the cross-reference tables, not the where-used list: references rather than calls, dynamic calls not seen"}
	if note := UnsearchedNote(res.Unsearched, len(res.Unsearched), "table"); note != "" {
		parts = append(parts, note)
	}
	if res.Truncated != "" {
		parts = append(parts, res.Truncated)
	}
	return strings.Join(parts, "; ")
}

// crossHit reads a CROSS row: the include, and the named column as the
// component when there is one.
func crossHit(componentColumn string) func(row map[string]interface{}) (xrefHit, bool) {
	return func(row map[string]interface{}) (xrefHit, bool) {
		hit := xrefHit{include: rowString(row, "INCLUDE")}
		if componentColumn != "" {
			hit.component = strings.ToUpper(rowString(row, componentColumn))
		}
		return hit, true
	}
}

// callerUnit is the object an include belongs to.
type callerUnit struct {
	name, typ, uri string
	// tadirType and tadirName are how TADIR knows it, for the package.
	tadirType, tadirName string
	group                string
	isTest               bool
}

// xrefCallers turns includes into objects, one caller per object, and drops
// the target's own includes: a class using its own attributes is not a user of
// itself.
func (c *Client) xrefCallers(ctx context.Context, hits []xrefHit, target callerTarget) ([]ExposedCaller, []string) {
	var notes []string

	// A function group include whose section is not a letter and two digits
	// — LWSAO_DISPF0A, or U0A past the ninety-ninth module — looks like any
	// program whose name starts with L. TLIBG says which of them are groups.
	looseGroup := map[string]string{} // include → group, confirmed
	var candidates []string
	candidateOf := map[string]string{}
	asked := map[string]bool{}
	for _, h := range hits {
		inc := strings.ToUpper(strings.TrimSpace(h.include))
		if _, _, isClass := classPoolOf(inc); inc == "" || isClass {
			continue
		}
		if _, ok := groupFromPool(inc); ok {
			continue
		}
		if group, ok := looseGroupOf(inc); ok {
			if !asked[group] {
				asked[group] = true
				candidates = append(candidates, group)
			}
			candidateOf[inc] = group
		}
	}
	groups, err := c.existingGroups(ctx, candidates)
	if err != nil {
		notes = append(notes, "TLIBG could not be read, so an include of a function group whose name does not "+
			"end in a letter and two digits is named as an include, not as its group: "+err.Error())
	}
	for inc, group := range candidateOf {
		if groups[group] {
			looseGroup[inc] = group
		}
	}

	// What the include names alone cannot settle: which function module an
	// L<group>U<nn> include holds, and whether a plain name is a program or an
	// include. Both are one query per chunk of names.
	var pools, plain []string
	seen := map[string]bool{}
	for _, h := range hits {
		inc := strings.ToUpper(strings.TrimSpace(h.include))
		if _, _, isClass := classPoolOf(inc); inc == "" || isClass {
			continue
		}
		if group, ok := groupOfInclude(inc, looseGroup); ok {
			if pool := functionPool(group); isModuleInclude(inc, group) && !seen[pool] {
				seen[pool] = true
				pools = append(pools, pool)
			}
			continue
		}
		if !seen[inc] {
			seen[inc] = true
			plain = append(plain, inc)
		}
	}

	modules, err := c.modulesByInclude(ctx, pools)
	if err != nil {
		notes = append(notes, "which function module each function group include holds could not be read from TFDIR, "+
			"so those callers are named by their function group: "+err.Error())
	}
	subc, err := c.programKinds(ctx, plain)
	if err != nil {
		notes = append(notes, "TRDIR could not be read, so each program-like caller is addressed as a program; "+
			"an include among them has the wrong URI: "+err.Error())
	}

	byKey := map[string]int{}
	var out []ExposedCaller
	var tadirKeys []string // parallel to out
	for _, h := range hits {
		unit, ok := callerUnitOf(h, modules, subc, looseGroup)
		if !ok || isTargetsOwn(unit, target) {
			continue
		}
		key := unit.typ + " " + unit.name
		component := h.component
		if at, seen := byKey[key]; seen {
			out[at].IsTest = out[at].IsTest || unit.isTest
			switch {
			case component == "" || containsItem(out[at].Component, component):
			case out[at].Component == "":
				out[at].Component = component
			default:
				out[at].Component += ", " + component
			}
			continue
		}
		out = append(out, ExposedCaller{
			Name:      unit.name,
			Type:      unit.typ,
			URI:       unit.uri,
			Component: component,
			IsTest:    unit.isTest,
		})
		byKey[key] = len(out) - 1
		tadirKeys = append(tadirKeys, unit.tadirType+" "+unit.tadirName)
	}

	packages, err := c.packagesOf(ctx, tadirKeys)
	if err != nil {
		notes = append(notes, "the callers' packages could not be read from TADIR: "+err.Error())
	}
	for i := range out {
		out[i].Package = packages[tadirKeys[i]]
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, notes
}

// callerUnitOf maps one hit to its object.
func callerUnitOf(h xrefHit, modules, subc, looseGroup map[string]string) (callerUnit, bool) {
	inc := strings.ToUpper(strings.TrimSpace(h.include))
	if inc == "" {
		return callerUnit{}, false
	}

	if _, section, ok := classPoolOf(inc); ok {
		unit, ok := unitForFrame(DumpFrame{Program: inc})
		if !ok {
			return callerUnit{}, false
		}
		typ, tadir := "CLAS/OC", "CLAS"
		if unit.Type == "INTF" {
			typ, tadir = "INTF/OI", "INTF"
		}
		// Test classes live in the CCAU include.
		isTest := section == "CCAU"
		return callerUnit{name: unit.Object, typ: typ, uri: unit.URI, tadirType: tadir, tadirName: unit.Object, isTest: isTest}, true
	}

	if group, ok := groupOfInclude(inc, looseGroup); ok {
		base := "/sap/bc/adt/functions/groups/" + adtSegment(group)
		if fm := modules[inc]; fm != "" {
			return callerUnit{name: fm, typ: "FUGR/FF", uri: base + "/fmodules/" + adtSegment(fm),
				tadirType: "FUGR", tadirName: group, group: group}, true
		}
		return callerUnit{name: group, typ: "FUGR/F", uri: base, tadirType: "FUGR", tadirName: group, group: group}, true
	}

	if subc[inc] == "I" {
		return callerUnit{name: inc, typ: "PROG/I", uri: "/sap/bc/adt/programs/includes/" + adtSegment(inc), tadirType: "PROG", tadirName: inc}, true
	}
	return callerUnit{name: inc, typ: "PROG/P", uri: "/sap/bc/adt/programs/programs/" + adtSegment(inc), tadirType: "PROG", tadirName: inc}, true
}

// isTargetsOwn is true for the target's own code. For a function module the
// rest of its group is somebody else: a module calling its neighbour is a
// caller like any other.
func isTargetsOwn(unit callerUnit, target callerTarget) bool {
	switch target.Type {
	case "FUNC":
		return strings.EqualFold(unit.name, target.Name) && unit.typ == "FUGR/FF"
	case "FUGR":
		return strings.EqualFold(unit.group, target.Name)
	case "CLAS", "INTF":
		return strings.EqualFold(unit.tadirName, target.Name) && (unit.tadirType == "CLAS" || unit.tadirType == "INTF")
	case "PROG", "INCL":
		return strings.EqualFold(unit.name, target.Name)
	}
	return false
}

// isModuleInclude is true for L<group>U<nn>, the include a function module's
// body is in. Past U99 the sections go on in letters, so only the U is
// checked; TFDIR says which module, if any, the include holds.
func isModuleInclude(include, group string) bool {
	prefix := strings.TrimSuffix(poolIncludeFor(group, "00"), "U00")
	tail := strings.TrimPrefix(include, prefix)
	return len(tail) == 3 && tail != include && tail[0] == 'U'
}

// groupOfInclude is the function group an include belongs to: by its name
// when the name says so, or by TLIBG when it only might.
func groupOfInclude(include string, looseGroup map[string]string) (string, bool) {
	if group, ok := groupFromPool(include); ok {
		return group, true
	}
	group, ok := looseGroup[include]
	return group, ok
}

// looseGroupOf reads L<group><xyz> with any letter-led three-character
// section. That is also the shape of a report called LOAD_DATA, so the answer
// is a candidate for TLIBG to confirm, never a group by itself.
func looseGroupOf(include string) (string, bool) {
	ns, rest := splitNamespace(include)
	if len(rest) <= 4 || rest[0] != 'L' {
		return "", false
	}
	tail := rest[len(rest)-3:]
	if tail[0] < 'A' || tail[0] > 'Z' {
		return "", false
	}
	for _, ch := range tail[1:] {
		if !(ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return "", false
		}
	}
	return ns + rest[1:len(rest)-3], true
}

// existingGroups answers which of the names are function groups.
func (c *Client) existingGroups(ctx context.Context, names []string) (map[string]bool, error) {
	out := map[string]bool{}
	err := c.queryIn(ctx, names,
		func(in string) string { return "SELECT AREA FROM TLIBG WHERE AREA IN ( " + in + " )" },
		func(n int) int { return n + 1 },
		func(res *TableContentsResult) {
			for _, row := range res.Rows {
				out[strings.ToUpper(rowString(row, "AREA"))] = true
			}
		})
	return out, err
}

// modulesByInclude reads TFDIR for the function pools named, and answers which
// module each L<group>U<nn> include holds.
func (c *Client) modulesByInclude(ctx context.Context, pools []string) (map[string]string, error) {
	out := map[string]string{}
	err := c.queryIn(ctx, pools,
		func(in string) string {
			return "SELECT FUNCNAME, PNAME, INCLUDE FROM TFDIR WHERE PNAME IN ( " + in + " )"
		},
		func(int) int { return xrefRowLimit },
		func(res *TableContentsResult) {
			for _, row := range res.Rows {
				group := groupOfPool(rowString(row, "PNAME"))
				section := rowString(row, "INCLUDE")
				if group == "" || section == "" {
					continue
				}
				out[poolIncludeFor(group, section)] = strings.ToUpper(rowString(row, "FUNCNAME"))
			}
		})
	return out, err
}

// programKinds reads TRDIR's SUBC for the names: I is an include, anything
// else something that runs.
func (c *Client) programKinds(ctx context.Context, names []string) (map[string]string, error) {
	out := map[string]string{}
	err := c.queryIn(ctx, names,
		func(in string) string { return "SELECT NAME, SUBC FROM TRDIR WHERE NAME IN ( " + in + " )" },
		func(n int) int { return n + 1 },
		func(res *TableContentsResult) {
			for _, row := range res.Rows {
				out[strings.ToUpper(rowString(row, "NAME"))] = strings.ToUpper(rowString(row, "SUBC"))
			}
		})
	return out, err
}

// packagesOf reads TADIR for objects keyed "<TADIR type> <name>" and answers
// the package of each, under the same key.
func (c *Client) packagesOf(ctx context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	var names []string
	seen := map[string]bool{}
	for _, key := range keys {
		_, name, _ := strings.Cut(key, " ")
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	err := c.queryIn(ctx, names,
		func(in string) string {
			return "SELECT OBJECT, OBJ_NAME, DEVCLASS FROM TADIR WHERE PGMID = 'R3TR' AND OBJ_NAME IN ( " + in + " )"
		},
		func(n int) int { return 4*n + 1 },
		func(res *TableContentsResult) {
			for _, row := range res.Rows {
				out[strings.ToUpper(rowString(row, "OBJECT"))+" "+strings.ToUpper(rowString(row, "OBJ_NAME"))] = strings.ToUpper(rowString(row, "DEVCLASS"))
			}
		})
	return out, err
}

// queryIn runs a statement over names, one IN list per chunk of
// xrefInChunk: build makes the statement from the quoted list, rows says how
// many rows a list of n may bring, and each takes every answer. A chunk that
// fails is not retried in pieces: the answer then says which query was lost,
// and the error is there to be read. (On 7.40 SP06 a long list could fail for
// where data preview cut it into lines; normalizeDataPreviewSQL now writes
// it so that the cut is harmless — see datapreview_legacy.go.)
func (c *Client) queryIn(ctx context.Context, names []string, build func(in string) string,
	rows func(n int) int, each func(*TableContentsResult)) error {
	var errs []error
	for _, chunk := range chunkNames(names, xrefInChunk) {
		res, err := c.RunQuery(ctx, build(sqlInList(chunk)), rows(len(chunk)))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if res != nil {
			each(res)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", joinErrors(errs))
	}
	return nil
}

// chunkNames splits names into lists of at most n, skipping any that could
// not be quoted safely. The names come from the system's own tables, but they
// go into free SQL, and the check costs nothing.
func chunkNames(names []string, n int) [][]string {
	var out [][]string
	var cur []string
	for _, name := range names {
		if checkSQLLiteral(name) != nil {
			continue
		}
		cur = append(cur, name)
		if len(cur) == n {
			out = append(out, cur)
			cur = nil
		}
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// sqlInList quotes names for an IN list, a blank after each comma: the 7.40
// SP06 parser wants them.
func sqlInList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "'" + sqlQuote(n) + "'"
	}
	return strings.Join(quoted, ", ")
}

func containsItem(list, item string) bool {
	for _, p := range strings.Split(list, ", ") {
		if p == item {
			return true
		}
	}
	return false
}

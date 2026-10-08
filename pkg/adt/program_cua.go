package adt

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// GUI statuses and titles of a program (its CUA, what SE41 edits). No
// release has an ADT resource for them, so they go through the adt-fallback
// set (0.9.0 or later): /sap/bc/adt/zvsp/cua/programs/{name}. The document
// is the CUA node of an abapGit .prog.xml, as asXML:
//
//	<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values>
//	  <CUA><ADM><ACTCODE>000001</ACTCODE><PFKCODE>000001</PFKCODE></ADM>
//	       <STA><RSMPE_STAT><CODE>MAIN</CODE>...</RSMPE_STAT></STA> ...</CUA>
//	</asx:values></asx:abap>
//
// A write replaces the whole CUA of the program and activates it.

// programCUAPath is the stand-in's collection.
const programCUAPath = FallbackRoot + "/cua/programs"

const programCUAContentType = "application/vnd.sap.as+xml; charset=utf-8; dataname=com.sap.adt.zvsp.cua"

func programCUAURL(program string) string {
	return programCUAPath + "/" + url.PathEscape(strings.ToUpper(strings.TrimSpace(program)))
}

// ProgramCUA is the CUA of a program.
type ProgramCUA struct {
	Program string `json:"program"`
	// Document is the CUA as asXML, root CUA, the way abapGit has it.
	Document string   `json:"document"`
	Statuses []string `json:"statuses,omitempty"`
	Titles   []string `json:"titles,omitempty"`
}

// ProgramCUAWrite reports a write of a program's CUA.
type ProgramCUAWrite struct {
	Program    string            `json:"program"`
	Transport  string            `json:"transport,omitempty"`
	Statuses   []string          `json:"statuses,omitempty"`
	Titles     []string          `json:"titles,omitempty"`
	Activated  bool              `json:"activated"`
	Activation *ActivationResult `json:"activation,omitempty"`
	// Warnings are links between the tables that do not hold: the write
	// goes through, but SAP GUI shows a status without keys, or grey.
	Warnings []string `json:"warnings,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

// errNoCUAFeature is returned where the set does not serve GUI statuses.
var errNoCUAFeature = fmt.Errorf("GUI statuses need the adt-fallback set 0.9.0 or later (vsp install adt-fallback): no release has an ADT resource for them")

func (c *Client) requireCUAFeature(ctx context.Context) error {
	if !c.fallbackHasFeature(ctx, "/cua") {
		return errNoCUAFeature
	}
	return nil
}

// GetProgramCUA reads the GUI statuses and titles of a program. version is
// "active", "inactive" or empty for the newest the caller may see.
func (c *Client) GetProgramCUA(ctx context.Context, program, version string) (*ProgramCUA, error) {
	if err := c.requireCUAFeature(ctx); err != nil {
		return nil, err
	}
	opts := &RequestOptions{Method: http.MethodGet, Accept: programCUAContentType}
	if v := strings.ToLower(strings.TrimSpace(version)); v != "" {
		if v != "active" && v != "inactive" {
			return nil, fmt.Errorf("version %q: want active or inactive", version)
		}
		opts.Query = url.Values{"version": {v}}
	}
	resp, err := c.transport.Request(ctx, programCUAURL(program), opts)
	if err != nil {
		return nil, err
	}
	return newProgramCUA(program, string(resp.Body))
}

func newProgramCUA(program, document string) (*ProgramCUA, error) {
	cua, err := parseCUA(document)
	if err != nil {
		return nil, err
	}
	return &ProgramCUA{
		Program:  strings.ToUpper(strings.TrimSpace(program)),
		Document: document,
		Statuses: cua.codes("STA", "CODE"),
		Titles:   cua.codes("TIT", "CODE"),
	}, nil
}

// WriteProgramCUA replaces the GUI statuses and titles of a program with
// those of document and activates them. document is the CUA node in any
// wrapping: a whole abapGit .prog.xml, the asXML GetProgramCUA gives, or
// the bare <CUA> element.
func (c *Client) WriteProgramCUA(ctx context.Context, program, document, transport string) (*ProgramCUAWrite, error) {
	program = strings.ToUpper(strings.TrimSpace(program))
	if program == "" {
		return nil, fmt.Errorf("program name is required")
	}
	body, err := CUADocument(document)
	if err != nil {
		return nil, err
	}
	cua, err := parseCUA(body)
	if err != nil {
		return nil, err
	}
	if len(cua.adm) == 0 && len(cua.tables["STA"]) == 0 && len(cua.tables["TIT"]) == 0 {
		return nil, fmt.Errorf("the CUA is empty: no ADM, no statuses, no titles (deleting a CUA is not served)")
	}
	programURL := "/sap/bc/adt/programs/programs/" + url.PathEscape(program)
	if err := c.checkMutation(ctx, MutationContext{Op: OpUpdate, OpName: "WriteProgramCUA", ObjectURL: programURL, Transport: transport}); err != nil {
		return nil, err
	}
	if err := c.requireCUAFeature(ctx); err != nil {
		return nil, err
	}
	result := &ProgramCUAWrite{
		Program:  program,
		Statuses: cua.codes("STA", "CODE"),
		Titles:   cua.codes("TIT", "CODE"),
		Warnings: cua.check(),
	}

	resource := programCUAURL(program)
	trPlan := c.planTransport(ctx, transport, programURL, "")
	lock, err := c.LockObject(ctx, resource, "MODIFY", trPlan.lockCorrNr(transport))
	if err != nil {
		return result, fmt.Errorf("locking the GUI statuses of %s: %w", program, err)
	}
	unlockCtx := context.WithoutCancel(ctx)
	unlocked := false
	unlock := func() {
		if !unlocked {
			unlocked = true
			_ = c.UnlockObject(unlockCtx, resource, lock.LockHandle)
		}
	}
	defer unlock()
	var trNote string
	if result.Transport, trNote, err = c.resolveWriteTransportFor(trPlan, transport, lock.CorrNr, "WriteProgramCUA"); err != nil {
		return result, err
	}
	if trNote != "" {
		result.Notes = append(result.Notes, trNote)
	}

	params := url.Values{}
	params.Set("lockHandle", lock.LockHandle)
	if result.Transport != "" {
		params.Set("corrNr", result.Transport)
	}
	resp, err := c.transport.Request(ctx, resource, &RequestOptions{
		Method:      http.MethodPut,
		Query:       params,
		Body:        []byte(body),
		ContentType: programCUAContentType,
		Accept:      programCUAContentType,
		Stateful:    true,
	})
	if err != nil {
		return result, fmt.Errorf("writing the GUI statuses of %s: %w", program, err)
	}
	// SAP drops a field it does not know without a word: a misspelt one
	// is lost. The answer is the CUA as stored, so compare.
	if stored, perr := parseCUA(string(resp.Body)); perr == nil {
		result.Warnings = append(result.Warnings, cua.notStoredIn(stored)...)
	}

	// The PUT leaves an inactive version, activated as its own object
	// (CUAD) after the unlock, as SE41 does.
	unlock()
	res, err := c.Activate(unlockCtx, resource, program)
	result.Activation = res
	switch {
	case err != nil:
		result.Notes = append(result.Notes, "written but not activated: "+err.Error())
	case res != nil && !res.Success:
		result.Notes = append(result.Notes, "written but not activated: "+strings.Join(activationMessages(res), "; "))
	default:
		result.Activated = true
	}
	return result, nil
}

var cuaElementRe = regexp.MustCompile(`(?s)<CUA(?:\s[^>]*)?(?:/>|>.*?</CUA>)`)

// CUADocument finds the CUA node in a document (a whole abapGit .prog.xml,
// the asXML of GetProgramCUA, or the bare element) and wraps it as the
// asXML the stand-in reads.
func CUADocument(document string) (string, error) {
	m := cuaElementRe.FindString(document)
	if m == "" {
		return "", fmt.Errorf("no <CUA> element in the document: want the CUA node of an abapGit .prog.xml")
	}
	return `<?xml version="1.0" encoding="utf-8"?><asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values>` +
		m + `</asx:values></asx:abap>`, nil
}

// cuaDoc is a CUA read field by field: ADM, and each table as rows of
// field values.
type cuaDoc struct {
	adm    map[string]string
	tables map[string][]map[string]string
}

// parseCUA reads the CUA element of a document: <CUA><ADM>fields</ADM>
// <STA><RSMPE_STAT>fields</RSMPE_STAT>...</STA>...</CUA>.
func parseCUA(document string) (*cuaDoc, error) {
	doc := &cuaDoc{adm: map[string]string{}, tables: map[string][]map[string]string{}}
	dec := xml.NewDecoder(strings.NewReader(document))
	var path []string
	var row map[string]string
	var text strings.Builder
	found := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading the CUA document: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			path = append(path, t.Name.Local)
			text.Reset()
			if t.Name.Local == "CUA" {
				found = true
			}
			// CUA/<table>/<row>
			if n := len(path); n >= 3 && path[n-3] == "CUA" && path[n-2] != "ADM" {
				row = map[string]string{}
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			n := len(path)
			switch {
			case n >= 3 && path[n-3] == "CUA" && path[n-2] == "ADM":
				doc.adm[path[n-1]] = strings.TrimSpace(text.String())
			case n >= 4 && path[n-4] == "CUA" && row != nil:
				row[path[n-1]] = strings.TrimSpace(text.String())
			case n >= 3 && path[n-3] == "CUA" && row != nil:
				doc.tables[path[n-2]] = append(doc.tables[path[n-2]], row)
				row = nil
			}
			path = path[:n-1]
			text.Reset()
		}
	}
	if !found {
		return nil, fmt.Errorf("no <CUA> element in the document")
	}
	return doc, nil
}

// codes lists the distinct values of a field of a table, sorted.
func (d *cuaDoc) codes(table, field string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range d.tables[table] {
		if v := r[field]; v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// check finds links between the tables that do not hold. Each of them is
// written and activated without a word from SAP, and the status then shows
// no keys, or grey functions.
func (d *cuaDoc) check() []string {
	var out []string
	funs := map[string]bool{}
	for _, f := range d.tables["FUN"] {
		funs[f["CODE"]] = true
	}
	keySets := map[string]bool{}
	for _, p := range d.tables["PFK"] {
		keySets[p["CODE"]] = true
		if p["FUNCODE"] != "" && !funs[p["FUNCODE"]] {
			out = append(out, fmt.Sprintf("PFK %s key %s: function %s is not in FUN", p["CODE"], p["PFNO"], p["FUNCODE"]))
		}
	}
	statuses := map[string]bool{}
	active := map[string]bool{}
	for _, s := range d.tables["SET"] {
		active[s["STATUS"]] = true
	}
	for _, s := range d.tables["STA"] {
		code := s["CODE"]
		statuses[code] = true
		if pfk := s["PFKCODE"]; pfk != "" && !keySets[pfk] {
			out = append(out, fmt.Sprintf("status %s: key set PFKCODE %s has no rows in PFK (a key set code, e.g. 000001, not a key number)", code, pfk))
		}
		if !active[code] {
			out = append(out, fmt.Sprintf("status %s: no function is active in it (no SET rows), all are grey", code))
		}
	}
	if len(d.tables["STA"]) > 0 && d.adm["PFKCODE"] == "" && len(d.tables["PFK"]) > 0 {
		out = append(out, "ADM has no PFKCODE: SE41 keeps the highest key set code there, as in ADM.PFKCODE = STA.PFKCODE = PFK.CODE")
	}
	for _, s := range d.tables["SET"] {
		if !statuses[s["STATUS"]] {
			out = append(out, fmt.Sprintf("SET: status %s is not in STA", s["STATUS"]))
		}
		if !funs[s["FUNCTION"]] {
			out = append(out, fmt.Sprintf("SET %s: function %s is not in FUN", s["STATUS"], s["FUNCTION"]))
		}
	}
	return out
}

// notStoredIn names the rows and fields of d that stored does not hold as
// sent: a field SAP does not know, or a value it changed.
func (d *cuaDoc) notStoredIn(stored *cuaDoc) []string {
	var out []string
	if diff := missingFields(d.adm, stored.adm); len(diff) > 0 {
		out = append(out, "ADM not stored as sent: "+strings.Join(diff, ", "))
	}
	tables := make([]string, 0, len(d.tables))
	for t := range d.tables {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		for i, row := range d.tables[t] {
			best := missingFields(row, nil)
			for _, s := range stored.tables[t] {
				if diff := missingFields(row, s); len(diff) < len(best) {
					best = diff
				}
			}
			if len(best) > 0 {
				out = append(out, fmt.Sprintf("%s row %d not stored as sent (unknown field or changed value): %s", t, i+1, strings.Join(best, ", ")))
			}
		}
	}
	return out
}

// missingFields lists the non-empty fields of sent whose value got is not.
func missingFields(sent, got map[string]string) []string {
	var out []string
	for k, v := range sent {
		if v != "" && got[k] != v {
			out = append(out, k+"="+v)
		}
	}
	sort.Strings(out)
	return out
}

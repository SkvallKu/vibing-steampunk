package adt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// What ATC keeps of the logged-on user's runs. A run from ADT leaves its
// results (kept 3 days), its execution (6 days) and its project (17 days),
// and from 7.50 on the worklist it was merged into (7 days). ATC's garbage
// collector removes only what has expired, and not on every system; ADT
// has no DELETE for any of it. The adt-fallback set (0.11.0 or later)
// lists them at /sap/bc/adt/zvsp/atcdata and removes one at
// /sap/bc/adt/zvsp/atcdata/{runs|worklists}/{id}: only the user's own,
// never a run of a run series or a central run. Nothing removes them on
// its own: RunATCCheck leaves ATC to work as it does.

const atcDataPath = FallbackRoot + "/atcdata"

var errNoATCDataFeature = fmt.Errorf("listing and deleting ATC runs needs the adt-fallback set 0.11.0 or later (vsp install adt-fallback): ADT has no DELETE for them")

// ATC data kinds.
const (
	ATCDataRun      = "run"
	ATCDataWorklist = "worklist"
)

// ATCData is one run or worklist of the user.
type ATCData struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Variant string `json:"variant"`
	// When it was scheduled (run) or last changed (worklist), and when ATC
	// lets it expire: UTC, YYYYMMDDhhmmss.
	On      string `json:"on"`
	Expires string `json:"expires"`
	// Findings of a run by priority.
	Prio1 int `json:"prio1,omitempty"`
	Prio2 int `json:"prio2,omitempty"`
	Prio3 int `json:"prio3,omitempty"`
	// Objects of a worklist.
	Objects int `json:"objects,omitempty"`
	// The run's title (the object, for a run of one), or a worklist's first
	// object (TYPE NAME).
	Title string `json:"title"`
}

// ATCDataFilter narrows the list. Variant and Title are patterns with *;
// a worklist matches Title when one of its objects does. From and To are
// UTC time stamps, YYYYMMDDhhmmss.
type ATCDataFilter struct {
	Variant string
	Title   string
	From    string
	To      string
}

func (c *Client) requireATCDataFeature(ctx context.Context) error {
	if !c.fallbackHasFeature(ctx, "/atcdata") {
		return errNoATCDataFeature
	}
	return nil
}

// ListATCData lists the user's ATC runs and worklists, oldest first.
func (c *Client) ListATCData(ctx context.Context, f ATCDataFilter) ([]ATCData, error) {
	q := url.Values{}
	for name, v := range map[string]string{"variant": f.Variant, "title": f.Title, "from": f.From, "to": f.To} {
		if v = strings.TrimSpace(v); v != "" {
			q.Set(name, v)
		}
	}
	for _, ts := range []string{f.From, f.To} {
		if ts != "" && !isTimestamp(ts) {
			return nil, fmt.Errorf("time stamp %q: want YYYYMMDDhhmmss (UTC)", ts)
		}
	}
	if err := c.requireATCDataFeature(ctx); err != nil {
		return nil, err
	}
	resp, err := c.transport.Request(ctx, atcDataPath, &RequestOptions{
		Method: http.MethodGet,
		Accept: "text/plain",
		Query:  q,
	})
	if err != nil {
		return nil, err
	}
	return ParseATCData(string(resp.Body))
}

// DeleteATCData removes one run (its results, its execution and, when
// nothing else refers to it, its project) or one worklist of the user.
// It returns the lines the system answers: what went, and # notes on what
// was left to ATC's garbage collector.
func (c *Client) DeleteATCData(ctx context.Context, kind, id string) ([]string, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	var coll string
	switch kind {
	case ATCDataRun:
		coll = "runs"
	case ATCDataWorklist:
		coll = "worklists"
	default:
		return nil, fmt.Errorf("ATC data kind %q: want run or worklist", kind)
	}
	if len(id) != 32 || strings.Trim(id, "0123456789ABCDEF") != "" {
		return nil, fmt.Errorf("ATC %s id %q: want 32 hex digits", kind, id)
	}
	if err := c.atcDataDeleteAllowed(ctx); err != nil {
		return nil, err
	}
	if err := c.requireATCDataFeature(ctx); err != nil {
		return nil, err
	}
	resp, err := c.transport.Request(ctx, atcDataPath+"/"+coll+"/"+id, &RequestOptions{
		Method: http.MethodDelete,
		Accept: "text/plain",
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(string(resp.Body), "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, strings.ReplaceAll(line, "\t", " "))
		}
	}
	return out, nil
}

// atcDataDeleteAllowed passes a delete through the safety settings. A run
// belongs to no package, so with allowed packages set it is refused.
func (c *Client) atcDataDeleteAllowed(ctx context.Context) error {
	if err := c.checkSafety(OpDelete, "DeleteATCData"); err != nil {
		return err
	}
	if len(c.config.Safety.AllowedPackages) > 0 {
		return fmt.Errorf("DeleteATCData is refused while allowed packages are set: an ATC run belongs to no package")
	}
	return c.checkMutation(ctx, MutationContext{Op: OpDelete, OpName: "DeleteATCData"})
}

// ATCDataDeletion is what became of one ID of DeleteATCDataByID.
type ATCDataDeletion struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind,omitempty"`
	Lines []string `json:"lines,omitempty"`
	Error string   `json:"error,omitempty"`
}

// DeleteATCDataByID deletes runs and worklists of the user named by ID
// alone: the list tells which is which. An ID not in the list is not sent.
func (c *Client) DeleteATCDataByID(ctx context.Context, ids []string) ([]ATCDataDeletion, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no ATC run or worklist ID given")
	}
	if err := c.atcDataDeleteAllowed(ctx); err != nil {
		return nil, err
	}
	list, err := c.ListATCData(ctx, ATCDataFilter{})
	if err != nil {
		return nil, err
	}
	kinds := map[string]string{}
	for _, d := range list {
		kinds[d.ID] = d.Kind
	}
	out := make([]ATCDataDeletion, 0, len(ids))
	for _, id := range ids {
		r := ATCDataDeletion{ID: strings.ToUpper(strings.TrimSpace(id))}
		r.Kind = kinds[r.ID]
		if r.Kind == "" {
			r.Error = "not among your ATC runs and worklists"
		} else if r.Lines, err = c.DeleteATCData(ctx, r.Kind, r.ID); err != nil {
			r.Error = err.Error()
		}
		out = append(out, r)
	}
	return out, nil
}

// ParseATCData reads the list the system answers.
func ParseATCData(body string) ([]ATCData, error) {
	list := []ATCData{}
	for i, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		for len(f) < 9 {
			f = append(f, "")
		}
		d := ATCData{Kind: f[0], ID: f[1], Variant: strings.TrimSpace(f[2]), On: f[3], Expires: f[4]}
		switch d.Kind {
		case ATCDataRun:
			d.Prio1, _ = strconv.Atoi(strings.TrimSpace(f[5]))
			d.Prio2, _ = strconv.Atoi(strings.TrimSpace(f[6]))
			d.Prio3, _ = strconv.Atoi(strings.TrimSpace(f[7]))
			d.Title = strings.TrimSpace(f[8])
		case ATCDataWorklist:
			d.Objects, _ = strconv.Atoi(strings.TrimSpace(f[5]))
			d.Title = strings.TrimSpace(f[6])
		default:
			return nil, fmt.Errorf("line %d: want run or worklist, not %q", i+1, f[0])
		}
		list = append(list, d)
	}
	return list, nil
}

// ATCTimestamp turns a time given on the command line into the UTC time
// stamp ATC keeps: YYYYMMDDhhmmss as is, or a local date or date and time
// (2006-01-02, 2006-01-02T15:04, 2006-01-02 15:04:05).
func ATCTimestamp(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || isTimestamp(s) {
		return s, nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.UTC().Format("20060102150405"), nil
		}
	}
	return "", fmt.Errorf("time %q: want a date (2006-01-02), a date and time (2006-01-02T15:04) or YYYYMMDDhhmmss in UTC", s)
}

func isTimestamp(s string) bool {
	if len(s) != 14 {
		return false
	}
	_, err := time.Parse("20060102150405", s)
	return err == nil
}

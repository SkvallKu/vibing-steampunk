package adt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Code Inspector check variants: what ATC runs on releases without ATC
// check variant objects. There a run names a global SCI variant
// (checkVariant), and the variants are kept by SCI, not by the repository:
// no ADT resource reads or writes them. The adt-fallback set (0.10.0 or
// later) serves them at /sap/bc/adt/zvsp/ci/checkvariants/{name}, and the
// checks that go into them at /sap/bc/adt/zvsp/ci/checks. 7.5x and later
// edit CHKV objects at /sap/bc/adt/atc/checkvariants, made of CHKO check
// objects: a different thing, not served here.
//
// The document is text, one line per item, fields separated by TAB:
//
//	description	Logic checks
//	check	CL_CI_TEST_SYSUBRC
//	param	CL_CI_TEST_SYSUBRC	SELECT	X
//	param	CL_CI_TEST_SYSUBRC	ANY	READ TABLE
//	check	CL_CI_TEST_NO_ORDER_BY
//
// A param line is one row of a parameter: one value for an elementary
// parameter, the components in order for a flat structure, one line per
// row for a table (a single line with an empty value: an empty table). A
// check's parameters not named keep the check's own value. # lines are
// comments. The system writes only variants in the customer namespace that
// are not transported.

const ciPath = FallbackRoot + "/ci"

const checkVariantContentType = "text/plain; charset=utf-8"

var errNoCIFeature = fmt.Errorf("check variants need the adt-fallback set 0.10.0 or later (vsp install adt-fallback): releases without ATC check variant objects have no ADT resource for them")

// CheckVariant is a Code Inspector check variant.
type CheckVariant struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Transported bool                `json:"transported,omitempty"`
	Checks      []CheckVariantCheck `json:"checks"`
	// Notes are the comment lines of the system: parameters it does not
	// show, a transported variant.
	Notes []string `json:"notes,omitempty"`
	// Document is the variant in the line format, as read.
	Document string `json:"document"`
}

// CheckVariantCheck is one check of a variant, or one check of the system
// with its default parameters.
type CheckVariantCheck struct {
	Class    string       `json:"class"`
	Version  string       `json:"version,omitempty"`
	Category string       `json:"category,omitempty"`
	Title    string       `json:"title,omitempty"`
	Params   []CheckParam `json:"params,omitempty"`
}

// CheckParam is one row of a check parameter.
type CheckParam struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func checkVariantURL(name string) string {
	return ciPath + "/checkvariants/" + url.PathEscape(strings.ToUpper(strings.TrimSpace(name)))
}

func (c *Client) requireCIFeature(ctx context.Context) error {
	if !c.fallbackHasFeature(ctx, "/ci") {
		return errNoCIFeature
	}
	return nil
}

// checkVariantWriteAllowed passes a write or delete of a check variant
// through the safety settings. A variant belongs to no package, so it
// cannot be held against --allowed-packages: with that set, it is refused.
func (c *Client) checkVariantWriteAllowed(ctx context.Context, op OperationType, opName string) error {
	if err := c.checkSafety(op, opName); err != nil {
		return err
	}
	if len(c.config.Safety.AllowedPackages) > 0 {
		return fmt.Errorf("%s is refused while allowed packages are set: a check variant belongs to no package", opName)
	}
	return c.checkMutation(ctx, MutationContext{Op: op, OpName: opName})
}

// GetCheckVariant reads a global Code Inspector check variant.
func (c *Client) GetCheckVariant(ctx context.Context, name string) (*CheckVariant, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("check variant name is required")
	}
	if err := c.requireCIFeature(ctx); err != nil {
		return nil, err
	}
	resp, err := c.transport.Request(ctx, checkVariantURL(name), &RequestOptions{
		Method: http.MethodGet,
		Accept: "text/plain",
	})
	if err != nil {
		return nil, err
	}
	return ParseCheckVariant(name, string(resp.Body))
}

// WriteCheckVariant creates a global check variant, or replaces the checks
// of one, from a document in the line format. It answers the variant as
// the system has saved it.
func (c *Client) WriteCheckVariant(ctx context.Context, name, document string) (*CheckVariant, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		return nil, fmt.Errorf("check variant name is required")
	}
	if err := c.checkVariantWriteAllowed(ctx, OpUpdate, "WriteCheckVariant"); err != nil {
		return nil, err
	}
	parsed, err := ParseCheckVariant(name, document)
	if err != nil {
		return nil, err
	}
	if len(parsed.Checks) == 0 {
		return nil, fmt.Errorf("check variant %s: the document names no check (want lines: check TAB CLASS)", name)
	}
	if err := c.requireCIFeature(ctx); err != nil {
		return nil, err
	}
	resp, err := c.transport.Request(ctx, checkVariantURL(name), &RequestOptions{
		Method:      http.MethodPut,
		Body:        []byte(document),
		ContentType: checkVariantContentType,
		Accept:      "text/plain",
	})
	if err != nil {
		return nil, err
	}
	return ParseCheckVariant(name, string(resp.Body))
}

// DeleteCheckVariant deletes a global check variant.
func (c *Client) DeleteCheckVariant(ctx context.Context, name string) error {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("check variant name is required")
	}
	if err := c.checkVariantWriteAllowed(ctx, OpDelete, "DeleteCheckVariant"); err != nil {
		return err
	}
	if err := c.requireCIFeature(ctx); err != nil {
		return err
	}
	_, err := c.transport.Request(ctx, checkVariantURL(name), &RequestOptions{
		Method: http.MethodDelete,
	})
	return err
}

// ListCodeInspectorChecks lists the checks registered in Code Inspector
// with their default parameters; check names one.
func (c *Client) ListCodeInspectorChecks(ctx context.Context, check string) ([]CheckVariantCheck, error) {
	if err := c.requireCIFeature(ctx); err != nil {
		return nil, err
	}
	opts := &RequestOptions{Method: http.MethodGet, Accept: "text/plain"}
	if check = strings.ToUpper(strings.TrimSpace(check)); check != "" {
		opts.Query = url.Values{"check": {check}}
	}
	resp, err := c.transport.Request(ctx, ciPath+"/checks", opts)
	if err != nil {
		return nil, err
	}
	v, err := ParseCheckVariant("", string(resp.Body))
	if err != nil {
		return nil, err
	}
	return v.Checks, nil
}

// ParseCheckVariant reads a document in the line format. A check line may
// carry version, category and title after the class, as the list of checks
// has them.
func ParseCheckVariant(name, document string) (*CheckVariant, error) {
	v := &CheckVariant{Name: strings.ToUpper(strings.TrimSpace(name)), Checks: []CheckVariantCheck{}, Document: document}
	index := map[string]int{}
	lines := strings.Split(strings.ReplaceAll(document, "\r\n", "\n"), "\n")
	for i, line := range lines {
		n := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			note := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.HasPrefix(note, "transported") {
				v.Transported = true
			}
			v.Notes = append(v.Notes, note)
			continue
		}
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		switch strings.ToLower(strings.TrimSpace(fields[0])) {
		case "description":
			if len(fields) > 1 {
				v.Description = strings.TrimSpace(fields[1])
			}
		case "check":
			if len(fields) < 2 || strings.TrimSpace(fields[1]) == "" {
				return nil, fmt.Errorf("line %d: check without a class", n)
			}
			ch := CheckVariantCheck{Class: strings.ToUpper(strings.TrimSpace(fields[1]))}
			if _, dup := index[ch.Class]; dup {
				return nil, fmt.Errorf("line %d: %s is named twice", n, ch.Class)
			}
			if len(fields) > 2 {
				ch.Version = strings.TrimSpace(fields[2])
			}
			if len(fields) > 3 {
				ch.Category = strings.TrimSpace(fields[3])
			}
			if len(fields) > 4 {
				ch.Title = strings.TrimSpace(strings.Join(fields[4:], " "))
			}
			index[ch.Class] = len(v.Checks)
			v.Checks = append(v.Checks, ch)
		case "param":
			if len(fields) < 3 {
				return nil, fmt.Errorf("line %d: want param TAB check TAB name TAB value(s)", n)
			}
			class := strings.ToUpper(strings.TrimSpace(fields[1]))
			at, ok := index[class]
			if !ok {
				return nil, fmt.Errorf("line %d: %s has no check line before it", n, class)
			}
			p := CheckParam{Name: strings.ToUpper(strings.TrimSpace(fields[2])), Values: fields[3:]}
			if len(p.Values) == 0 {
				p.Values = []string{""}
			}
			v.Checks[at].Params = append(v.Checks[at].Params, p)
		default:
			return nil, fmt.Errorf("line %d: want description, check or param, not %q", n, fields[0])
		}
	}
	return v, nil
}

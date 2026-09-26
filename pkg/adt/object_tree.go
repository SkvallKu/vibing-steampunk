package adt

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetObjectStructure returns an object's components as a tree.
//
// objectType is a short form (FUGR), an ADT type (FUGR/F), or empty, in which
// case the repository is searched for an object of exactly that name. A class
// is read from its objectstructure, as it always was. Every other type used to
// be sent there too, and answered 400 "class W61V does not exist" for a
// function group and ExceptionResourceMessageInvalid for a form; they are read
// from the repository node structure, the tree Eclipse's project explorer
// shows, which answers for any type on 7.40 and 7.50 alike.
//
// The node structure answers for other types too, but not with their
// components: asked about the table TVARVC on 7.50 it lists some three hundred
// classes, programs and data elements. So only the types in treeTypes are
// read; anything else is refused with a pointer to the tool that answers it.
//
// For those the tree has one level per kind of component, in SAP's own order
// and with SAP's label ("Includes", "Screens"), and maxResults applies to each
// kind: a function group lists a hundred fields before its includes, and one
// limit over all of them would cut the includes off.
func (c *Client) GetObjectStructure(ctx context.Context, objectName, objectType string, maxResults int) (*ObjectExplorerNode, error) {
	if maxResults <= 0 {
		maxResults = 100
	}
	name := strings.ToUpper(strings.TrimSpace(objectName))
	if name == "" {
		return nil, fmt.Errorf("object name is required")
	}

	typ := treeParentType(objectType)
	var found *SearchResult
	var note string
	if typ == "" {
		var err error
		found, note, err = c.findObjectOfName(ctx, name)
		if err != nil {
			return nil, err
		}
		typ = found.Type
	}
	if !treeTypes[typ] {
		return nil, fmt.Errorf("%s is of type %s, and GetObjectStructure answers for %s only; "+
			"a dictionary object is read with GetTable or GetStructure, and a function module with GetSource",
			name, typ, treeTypeList)
	}

	var root *ObjectExplorerNode
	var err error
	if typ == "CLAS/OC" {
		root, err = c.classObjectTree(ctx, name, maxResults)
	} else {
		root, err = c.repositoryObjectTree(ctx, name, typ, maxResults)
	}
	if err != nil || root == nil {
		return root, err
	}
	if found != nil && found.URI != "" {
		root.URI = found.URI
	}
	if note != "" {
		root.Note = strings.TrimSpace(note + " " + root.Note)
	}
	return root, nil
}

// treeTypes are the types whose repository tree is their components.
var treeTypes = map[string]bool{
	"CLAS/OC": true, "INTF/OI": true, "PROG/P": true, "FUGR/F": true, "SFPF/5F": true, "SFPI/5I": true,
}

const treeTypeList = "classes, interfaces, programs, function groups and forms (CLAS, INTF, PROG, FUGR, SFPF, SFPI)"

// treeParentType turns what a caller may pass as a type into the ADT type the
// node structure takes. Forms are not among the search's short forms.
func treeParentType(objectType string) string {
	switch t := strings.ToUpper(strings.TrimSpace(objectType)); t {
	case "SFPF":
		return "SFPF/5F"
	case "SFPI":
		return "SFPI/5I"
	case "DEVC":
		return "DEVC/K"
	default:
		return CanonicalObjectType(t)
	}
}

// findObjectOfName is the repository's answer to "what is W61V". The search
// matches a prefix, so only a hit with exactly the name counts; its Name may
// carry the kind after it in the logon language ("W61V (Функциональная
// группа)"), so the name is compared up to the first blank.
//
// A name can belong to several objects: SBAL_DISPLAY is a program and a
// function group, a table and its data element often share one. The first
// hit of a type with a tree is taken, in the search's order, and the others
// are named in the note, so that the caller can ask for the one it meant.
// When no hit has a tree, the first one is returned and refused by the caller
// with its type named.
func (c *Client) findObjectOfName(ctx context.Context, name string) (*SearchResult, string, error) {
	results, err := c.SearchObject(ctx, name, 50)
	if err != nil {
		return nil, "", fmt.Errorf("looking up the type of %s: %w", name, err)
	}
	var hits []SearchResult
	for _, r := range results {
		n, _, _ := strings.Cut(strings.TrimSpace(r.Name), " ")
		if strings.EqualFold(n, name) {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return nil, "", fmt.Errorf("no object named %s is in the repository; check the name, "+
			"or pass object_type if the search cannot find it", name)
	}
	chosen := 0
	for i, h := range hits {
		if treeTypes[h.Type] {
			chosen = i
			break
		}
	}
	var note string
	if len(hits) > 1 {
		var others []string
		withTree := false
		for i, h := range hits {
			if i != chosen {
				others = append(others, h.Type)
				withTree = withTree || treeTypes[h.Type]
			}
		}
		note = fmt.Sprintf("%s is also the name of an object of type %s.", name, strings.Join(others, ", "))
		if withTree {
			note += " Pass object_type to see that one."
		}
	}
	return &hits[chosen], note, nil
}

// repositoryObjectTree reads the node structure of one object. See
// ListFunctionModules for why the Accept header is */*.
func (c *Client) repositoryObjectTree(ctx context.Context, name, typ string, maxResults int) (*ObjectExplorerNode, error) {
	query := url.Values{}
	query.Set("parent_type", typ)
	query.Set("parent_name", name)
	query.Set("withShortDescriptions", "true")

	resp, err := c.transport.Request(ctx, "/sap/bc/adt/repository/nodestructure", &RequestOptions{
		Method: http.MethodPost,
		Query:  query,
		Accept: "*/*",
	})
	if err != nil {
		return nil, fmt.Errorf("reading the repository tree of %s %s: %w", typ, name, err)
	}
	var doc repositoryNodeStructure
	if err := xml.Unmarshal(resp.Body, &doc); err != nil {
		return nil, fmt.Errorf("parsing the repository tree of %s %s: %w", typ, name, err)
	}
	return objectTreeFromNodes(name, typ, doc, maxResults), nil
}

// objectTreeFromNodes groups the flat node list by kind. The kinds come in the
// order of the type list, which is the order Eclipse shows them in; a node of
// a kind the list does not name goes last, under its type code.
//
// An entry with a type and no name stands for a folder, not an object (see
// functionModulesFromNodes), and is skipped.
func objectTreeFromNodes(name, typ string, doc repositoryNodeStructure, maxResults int) *ObjectExplorerNode {
	root := &ObjectExplorerNode{Name: name, Type: typ}

	var order []string
	folders := map[string]*ObjectExplorerNode{}
	folder := func(t, label string) *ObjectExplorerNode {
		if f, ok := folders[t]; ok {
			return f
		}
		if label == "" {
			label = t
		}
		folders[t] = &ObjectExplorerNode{Name: label, Type: t}
		order = append(order, t)
		return folders[t]
	}
	for _, t := range doc.Types {
		folder(strings.TrimSpace(t.ObjectType), strings.TrimSpace(t.Label))
	}
	for _, n := range doc.Nodes {
		objName := strings.TrimSpace(n.ObjectName)
		if objName == "" {
			continue
		}
		f := folder(strings.TrimSpace(n.ObjectType), "")
		if len(f.Children) >= maxResults {
			f.Omitted++
			continue
		}
		f.Children = append(f.Children, ObjectExplorerNode{
			Name: objName,
			Type: strings.TrimSpace(n.ObjectType),
			URI:  strings.TrimSpace(n.ObjectURI),
		})
	}
	for _, t := range order {
		if f := folders[t]; len(f.Children) > 0 {
			root.Children = append(root.Children, *f)
		}
	}
	if len(root.Children) == 0 {
		root.Note = "The repository tree lists no components for this object; " +
			"a name that does not exist as this type answers the same way."
	}
	return root
}

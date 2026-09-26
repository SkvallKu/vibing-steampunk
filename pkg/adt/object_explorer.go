package adt

import (
	"context"
	"strings"
)

// ObjectExplorerNode represents a node in the object explorer tree.
type ObjectExplorerNode struct {
	URI         string               `json:"uri"`
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Description string               `json:"description,omitempty"`
	Children    []ObjectExplorerNode `json:"children,omitempty"`
	// Omitted counts the children left out by maxResults.
	Omitted int `json:"omitted,omitempty"`
	// Note, on the root, says what the tree is not: another object of the
	// same name, or a tree with nothing in it.
	Note string `json:"note,omitempty"`
}

// GetObjectStructureCAI returns an object's components as a tree, finding the
// object's type in the repository first. See GetObjectStructure.
//
// It used to ask /sap/bc/adt/cai/objectexplorer/objects, and that resource does
// not exist. Not "not on older releases" — it is advertised in the discovery
// document of none of 7.50, 7.57 or 7.58 and answers 404 on all of them, along
// with the rest of the /cai/ namespace, which also took the call graph down
// with it. So this had never returned anything, and its three callers —
// analyze type=object_structure among them — had never worked.
func (c *Client) GetObjectStructureCAI(ctx context.Context, objectName string, maxResults int) (*ObjectExplorerNode, error) {
	return c.GetObjectStructure(ctx, objectName, "", maxResults)
}

// classObjectTree is a class's components from /sap/bc/adt/oo/classes/{name}/
// objectstructure, which GetClassObjectStructure already spoke. The resource
// returns a whole class in one answer, so maxResults only cuts it.
func (c *Client) classObjectTree(ctx context.Context, objectName string, maxResults int) (*ObjectExplorerNode, error) {
	structure, err := c.GetClassObjectStructure(ctx, objectName)
	if err != nil {
		return nil, err
	}
	if structure == nil {
		return nil, nil
	}

	root := &ObjectExplorerNode{
		Name: structure.Name,
		Type: structure.Type,
		URI:  "/sap/bc/adt/oo/classes/" + strings.ToLower(objectName),
	}
	for i, el := range structure.Elements {
		if i >= maxResults {
			root.Omitted = len(structure.Elements) - maxResults
			break
		}
		root.Children = append(root.Children, ObjectExplorerNode{
			Name: el.Name,
			Type: el.Type,
		})
	}
	return root, nil
}

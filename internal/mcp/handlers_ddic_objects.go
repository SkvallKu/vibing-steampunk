package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

func (s *Server) handleGetDomain(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, _ := request.GetArguments()["name"].(string)
	if strings.TrimSpace(name) == "" {
		return newToolResultError("name is required"), nil
	}
	d, err := s.adtClient.GetDomain(ctx, name)
	if err != nil {
		return newToolResultError(fmt.Sprintf("GetDomain failed: %v", err)), nil
	}
	out, _ := json.MarshalIndent(d, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func (s *Server) handleGetDataElement(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, _ := request.GetArguments()["name"].(string)
	if strings.TrimSpace(name) == "" {
		return newToolResultError("name is required"), nil
	}
	e, err := s.adtClient.GetDataElement(ctx, name)
	if err != nil {
		return newToolResultError(fmt.Sprintf("GetDataElement failed: %v", err)), nil
	}
	out, _ := json.MarshalIndent(e, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// ddicWriteArgs are the arguments WriteDomain and WriteDataElement share.
type ddicWriteArgs struct {
	name, pkg, description, transport string
	create, hasDescription            bool
	properties                        []byte
}

func readDDICWriteArgs(request mcp.CallToolRequest) (*ddicWriteArgs, error) {
	args := request.GetArguments()
	a := &ddicWriteArgs{}
	a.name, _ = args["name"].(string)
	if strings.TrimSpace(a.name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	a.pkg, _ = args["package"].(string)
	a.description, a.hasDescription = args["description"].(string)
	a.transport, _ = args["transport"].(string)
	a.create, _ = args["create"].(bool)
	switch p := args["properties"].(type) {
	case string:
		if strings.TrimSpace(p) != "" {
			a.properties = []byte(p)
		}
	case map[string]any:
		a.properties, _ = json.Marshal(p)
	}
	if a.properties != nil {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(a.properties, &probe); err != nil {
			return nil, fmt.Errorf("properties is not a JSON object: %v", err)
		}
		for _, k := range []string{"name", "package"} {
			if _, ok := probe[k]; ok {
				return nil, fmt.Errorf("properties names %q: give it as the argument of its own", k)
			}
		}
	}
	if a.create && (strings.TrimSpace(a.pkg) == "" || strings.TrimSpace(a.description) == "") {
		return nil, fmt.Errorf("create needs package and description")
	}
	return a, nil
}

func ddicWriteResult(w *adt.DDICWrite, err error, tool string) (*mcp.CallToolResult, error) {
	if err != nil {
		msg := fmt.Sprintf("%s failed: %v", tool, err)
		if w != nil {
			out, _ := json.MarshalIndent(w, "", "  ")
			msg += "\n" + string(out)
			if w.Created {
				msg += "\nThe object was created but not written through; fix the properties and call again without create, or delete it."
			}
		}
		return newToolResultError(msg), nil
	}
	out, _ := json.MarshalIndent(w, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func (s *Server) handleWriteDomain(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := readDDICWriteArgs(request)
	if err != nil {
		return newToolResultError(err.Error()), nil
	}
	apply := func(d *adt.Domain) error {
		if a.properties != nil {
			if err := json.Unmarshal(a.properties, d); err != nil {
				return fmt.Errorf("properties: %v", err)
			}
		}
		if a.hasDescription {
			d.Description = a.description
		}
		return nil
	}
	if a.create {
		d := &adt.Domain{}
		if err := apply(d); err != nil {
			return newToolResultError(err.Error()), nil
		}
		d.Name, d.Package, d.Description = a.name, a.pkg, a.description
		w, err := s.adtClient.CreateDomain(ctx, d, a.transport)
		return ddicWriteResult(w, err, "WriteDomain")
	}
	w, err := s.adtClient.UpdateDomain(ctx, a.name, a.transport, apply)
	return ddicWriteResult(w, err, "WriteDomain")
}

func (s *Server) handleWriteDataElement(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := readDDICWriteArgs(request)
	if err != nil {
		return newToolResultError(err.Error()), nil
	}
	apply := func(e *adt.DataElement) error {
		if a.properties != nil {
			if err := json.Unmarshal(a.properties, e); err != nil {
				return fmt.Errorf("properties: %v", err)
			}
		}
		if a.hasDescription {
			e.Description = a.description
		}
		return nil
	}
	if a.create {
		e := &adt.DataElement{}
		if err := apply(e); err != nil {
			return newToolResultError(err.Error()), nil
		}
		e.Name, e.Package, e.Description = a.name, a.pkg, a.description
		w, err := s.adtClient.CreateDataElement(ctx, e, a.transport)
		return ddicWriteResult(w, err, "WriteDataElement")
	}
	w, err := s.adtClient.UpdateDataElement(ctx, a.name, a.transport, apply)
	return ddicWriteResult(w, err, "WriteDataElement")
}

// Package mcp provides the MCP server implementation for ABAP ADT tools.
// handlers_atc.go contains handlers for ATC (ABAP Test Cockpit) operations.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// routeATCAction routes "test" with type=atc.
func (s *Server) routeATCAction(ctx context.Context, action, objectType, objectName string, params map[string]any) (*mcp.CallToolResult, bool, error) {
	if action != "test" {
		return nil, false, nil
	}
	analysisType := getStringParam(params, "type")
	if analysisType == "" {
		// Target form: SAP(action="test", target="ATC", params={"object_uri": ...})
		switch objectType {
		case "ATC":
			analysisType = "atc"
		case "ATC_CUSTOMIZING":
			analysisType = "atc_customizing"
		}
	}
	switch analysisType {
	case "atc":
		// The handler expects object_url; object_uri is the documented alias.
		return s.callHandler(ctx, s.handleRunATCCheck, paramsWithAlias(params, "object_url", "object_uri"))
	case "atc_customizing":
		return s.callHandler(ctx, s.handleGetATCCustomizing, params)
	}
	return nil, false, nil
}

// --- ATC Handlers ---

func (s *Server) handleRunATCCheck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	objectURL, ok := request.GetArguments()["object_url"].(string)
	if !ok || objectURL == "" {
		return newToolResultError("object_url is required"), nil
	}

	variant := ""
	if v, ok := request.GetArguments()["variant"].(string); ok {
		variant = v
	}

	maxResults := 100
	if mr, ok := request.GetArguments()["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	result, err := s.adtClient.RunATCCheck(ctx, objectURL, variant, maxResults)
	if err != nil {
		return newToolResultError(fmt.Sprintf("ATC check failed: %v", err)), nil
	}

	// Format output with summary
	type summary struct {
		TotalObjects  int `json:"totalObjects"`
		TotalFindings int `json:"totalFindings"`
		Errors        int `json:"errors"`
		Warnings      int `json:"warnings"`
		Infos         int `json:"infos"`
	}
	type output struct {
		Summary  summary          `json:"summary"`
		Worklist *adt.ATCWorklist `json:"worklist"`
	}

	sum := summary{TotalObjects: len(result.Objects)}
	for _, obj := range result.Objects {
		sum.TotalFindings += len(obj.Findings)
		for _, f := range obj.Findings {
			switch f.Priority {
			case 1:
				sum.Errors++
			case 2:
				sum.Warnings++
			default:
				sum.Infos++
			}
		}
	}

	out := output{Summary: sum, Worklist: result}
	outputJSON, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(outputJSON)), nil
}

func (s *Server) handleGetATCCustomizing(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result, err := s.adtClient.GetATCCustomizing(ctx)
	if err != nil {
		return newToolResultError(fmt.Sprintf("Failed to get ATC customizing: %v", err)), nil
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

// --- Code Inspector check variants ---

func (s *Server) handleGetCheckVariant(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, _ := request.GetArguments()["name"].(string)
	if name == "" {
		return newToolResultError("name is required"), nil
	}
	v, err := s.adtClient.GetCheckVariant(ctx, name)
	if err != nil {
		return newToolResultError(fmt.Sprintf("GetCheckVariant failed: %v", err)), nil
	}
	return mcp.NewToolResultText(v.Document), nil
}

func (s *Server) handleWriteCheckVariant(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, _ := request.GetArguments()["name"].(string)
	document, _ := request.GetArguments()["document"].(string)
	if name == "" || document == "" {
		return newToolResultError("name and document are required"), nil
	}
	v, err := s.adtClient.WriteCheckVariant(ctx, name, document)
	if err != nil {
		return newToolResultError(fmt.Sprintf("WriteCheckVariant failed: %v", err)), nil
	}
	return mcp.NewToolResultText(v.Document), nil
}

func (s *Server) handleDeleteCheckVariant(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, _ := request.GetArguments()["name"].(string)
	if name == "" {
		return newToolResultError("name is required"), nil
	}
	if err := s.adtClient.DeleteCheckVariant(ctx, name); err != nil {
		return newToolResultError(fmt.Sprintf("DeleteCheckVariant failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Deleted check variant %s", name)), nil
}

func (s *Server) handleListCodeInspectorChecks(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	check, _ := request.GetArguments()["check"].(string)
	list, err := s.adtClient.ListCodeInspectorChecks(ctx, check)
	if err != nil {
		return newToolResultError(fmt.Sprintf("ListCodeInspectorChecks failed: %v", err)), nil
	}
	out, _ := json.MarshalIndent(list, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// --- Own ATC runs and worklists ---

func (s *Server) handleListATCRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	var f adt.ATCDataFilter
	f.Variant, _ = args["variant"].(string)
	f.Title, _ = args["object"].(string)
	f.From, _ = args["from"].(string)
	f.To, _ = args["to"].(string)
	list, err := s.adtClient.ListATCData(ctx, f)
	if err != nil {
		return newToolResultError(fmt.Sprintf("ListATCRuns failed: %v", err)), nil
	}
	out, _ := json.MarshalIndent(list, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func (s *Server) handleDeleteATCRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, _ := request.GetArguments()["ids"].(string)
	ids := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	if len(ids) == 0 {
		return newToolResultError("ids is required"), nil
	}
	results, err := s.adtClient.DeleteATCDataByID(ctx, ids)
	if err != nil {
		return newToolResultError(fmt.Sprintf("DeleteATCRuns failed: %v", err)), nil
	}
	var b strings.Builder
	failed := 0
	for _, r := range results {
		if r.Error != "" {
			failed++
			fmt.Fprintf(&b, "FAILED %s %s: %s\n", r.Kind, r.ID, r.Error)
			continue
		}
		fmt.Fprintf(&b, "deleted %s %s\n", r.Kind, r.ID)
		for _, line := range r.Lines {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	if failed > 0 {
		return newToolResultError(b.String()), nil
	}
	return mcp.NewToolResultText(b.String()), nil
}

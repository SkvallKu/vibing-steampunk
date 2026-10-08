package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGetProgramCUA(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	program := getStringParam(request.GetArguments(), "program")
	if program == "" {
		return newToolResultError("program is required"), nil
	}
	cua, err := s.adtClient.GetProgramCUA(ctx, program, getStringParam(request.GetArguments(), "version"))
	if err != nil {
		return newToolResultError(fmt.Sprintf("GetProgramCUA failed: %v", err)), nil
	}
	output, _ := json.MarshalIndent(cua, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func (s *Server) handleWriteProgramCUA(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	program := getStringParam(args, "program")
	if program == "" {
		return newToolResultError("program is required"), nil
	}
	document := getStringParam(args, "cua")
	if path := getStringParam(args, "file_path"); path != "" {
		if document != "" {
			return newToolResultError("give cua or file_path, not both"), nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return newToolResultError(fmt.Sprintf("reading %s: %v", path, err)), nil
		}
		document = string(b)
	}
	if document == "" {
		return newToolResultError("cua or file_path is required"), nil
	}
	w, err := s.adtClient.WriteProgramCUA(ctx, program, document, getStringParam(args, "transport"))
	if err != nil {
		if w == nil {
			return newToolResultError(fmt.Sprintf("WriteProgramCUA failed: %v", err)), nil
		}
		payload, _ := json.MarshalIndent(w, "", "  ")
		return newToolResultError(fmt.Sprintf("WriteProgramCUA failed: %v\n\n%s", err, payload)), nil
	}
	output, _ := json.MarshalIndent(w, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

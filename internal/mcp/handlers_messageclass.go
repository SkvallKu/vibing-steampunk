package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// messageEditArgs reads the messages to set and the numbers to delete.
// messages is {"001":"text"} or [{"number":"001","text":"text"}]; delete is
// a list of numbers or one comma-separated string.
func messageEditArgs(args map[string]any) (map[string]string, []string, error) {
	set := map[string]string{}
	switch m := args["messages"].(type) {
	case nil:
	case map[string]any:
		for k, v := range m {
			s, ok := v.(string)
			if !ok {
				return nil, nil, fmt.Errorf("messages[%s]: text is not a string", k)
			}
			set[k] = s
		}
	case []any:
		for i, e := range m {
			o, _ := e.(map[string]any)
			n, _ := o["number"].(string)
			t, ok := o["text"].(string)
			if n == "" || !ok {
				return nil, nil, fmt.Errorf("messages[%d]: want {\"number\":\"001\",\"text\":\"...\"}", i)
			}
			if _, dup := set[n]; dup {
				return nil, nil, fmt.Errorf("messages: %s named twice", n)
			}
			set[n] = t
		}
	default:
		return nil, nil, fmt.Errorf("messages: want an object or a list")
	}
	var del []string
	switch d := args["delete"].(type) {
	case nil:
	case string:
		for _, n := range strings.Split(d, ",") {
			if n = strings.TrimSpace(n); n != "" {
				del = append(del, n)
			}
		}
	case []any:
		for i, e := range d {
			n, ok := e.(string)
			if !ok {
				return nil, nil, fmt.Errorf("delete[%d]: want a message number as a string", i)
			}
			del = append(del, n)
		}
	default:
		return nil, nil, fmt.Errorf("delete: want a list of message numbers")
	}
	return set, del, nil
}

func (s *Server) handleEditMessageClass(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return newToolResultError("name is required"), nil
	}
	set, del, err := messageEditArgs(args)
	if err != nil {
		return newToolResultError(err.Error()), nil
	}
	transport, _ := args["transport"].(string)
	dryRun, _ := args["dry_run"].(bool)
	edit, err := s.adtClient.EditMessageClass(ctx, name, set, del, transport, dryRun)
	if err != nil {
		msg := fmt.Sprintf("EditMessageClass failed: %v", err)
		if edit != nil {
			out, _ := json.MarshalIndent(edit, "", "  ")
			msg += "\n" + string(out)
		}
		return newToolResultError(msg), nil
	}
	out, _ := json.MarshalIndent(edit, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

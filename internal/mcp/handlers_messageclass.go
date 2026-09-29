package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// createMessageClass creates a message class and, when "messages" are given,
// writes them in "language" (default: the session language):
// SAP(action="create", target="OBJECT", params={"object_type": "MSAG/N", "name": "ZDEMO",
//
//	"description": "Demo", "package_name": "$TMP", "messages": {"001": "Order & not found"}}).
func (s *Server) createMessageClass(ctx context.Context, opts adt.CreateObjectOptions, args map[string]any) (*mcp.CallToolResult, error) {
	lang := strings.ToUpper(getStringParam(args, "language"))
	opts.MasterLanguage = lang
	messages, err := messageTexts(args["messages"])
	if err != nil {
		return newToolResultError(err.Error()), nil
	}
	if err := s.adtClient.CreateObject(ctx, opts); err != nil {
		return newToolResultError(fmt.Sprintf("Failed to create message class: %v", err)), nil
	}
	result := map[string]any{
		"status":     "created",
		"object_url": adt.GetObjectURL(opts.ObjectType, opts.Name, ""),
	}
	if len(messages) > 0 {
		// The class was created in the session language; its messages go
		// in the same one unless a language was named.
		if lang == "" {
			lang = strings.ToUpper(s.adtClient.Language())
		}
		if lang == "" {
			lang = "EN"
		}
		if err := s.adtClient.WriteMessageClassTexts(ctx, opts.Name, lang, messages, "", opts.Transport); err != nil {
			return newToolResultError(fmt.Sprintf("message class %s created, but its messages were not written: %v", strings.ToUpper(opts.Name), err)), nil
		}
		result["messages"] = len(messages)
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// messageTexts reads messages as an object {"001": "text"} or as a list of
// {"number": "001", "text": "..."}.
func messageTexts(v any) ([]adt.MessageClassMessage, error) {
	var out []adt.MessageClassMessage
	switch m := v.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		for no, text := range m {
			out = append(out, adt.MessageClassMessage{Number: no, Text: fmt.Sprint(text)})
		}
	case []any:
		raw, _ := json.Marshal(m)
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("messages: %v", err)
		}
	default:
		return nil, fmt.Errorf(`messages must be {"001": "text"} or [{"number": "001", "text": "..."}]`)
	}
	for i, msg := range out {
		no := strings.TrimSpace(msg.Number)
		if len(no) > 3 || strings.Trim(no, "0123456789") != "" || no == "" {
			return nil, fmt.Errorf("message number %q: three digits", msg.Number)
		}
		out[i].Number = fmt.Sprintf("%03s", no)
		if len([]rune(msg.Text)) > 73 {
			return nil, fmt.Errorf("message %s is %d characters long; T100 holds 73", out[i].Number, len([]rune(msg.Text)))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

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

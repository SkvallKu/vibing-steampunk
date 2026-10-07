package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestWithComponentTextsArg(t *testing.T) {
	for name, arg := range map[string]any{
		"object":      map[string]any{"get_limit": "Получить лимит", "GET_LIMIT.IV_WERKS": "Завод"},
		"JSON string": `{"get_limit":"Получить лимит","GET_LIMIT.IV_WERKS":"Завод"}`,
	} {
		ctx, errRes := withComponentTextsArg(context.Background(), map[string]any{"component_texts": arg})
		if errRes != nil {
			t.Fatalf("%s: %v", name, errRes.Content)
		}
		if ctx == context.Background() {
			t.Errorf("%s: texts not put on the context", name)
		}
	}

	if ctx, errRes := withComponentTextsArg(context.Background(), map[string]any{}); errRes != nil || ctx != context.Background() {
		t.Errorf("no argument: ctx changed or error %v", errRes)
	}

	for name, arg := range map[string]any{
		"list":       []any{"x"},
		"not text":   map[string]any{"A": 1.0},
		"bad key":    map[string]any{"A.B.C": "x"},
		"bad string": "not json",
	} {
		_, errRes := withComponentTextsArg(context.Background(), map[string]any{"component_texts": arg})
		if errRes == nil || !errRes.IsError {
			t.Errorf("%s: no error", name)
			continue
		}
		if !strings.Contains(errRes.Content[0].(mcp.TextContent).Text, "component") {
			t.Errorf("%s: message %q", name, errRes.Content[0].(mcp.TextContent).Text)
		}
	}
}

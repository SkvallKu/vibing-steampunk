package mcp

import "testing"

func TestMessageEditArgs(t *testing.T) {
	set, del, err := messageEditArgs(map[string]any{
		"messages": map[string]any{"001": "Order & created"},
		"delete":   "003, 4",
	})
	if err != nil || set["001"] != "Order & created" || len(del) != 2 || del[1] != "4" {
		t.Errorf("object: %v %v %v", set, del, err)
	}
	set, del, err = messageEditArgs(map[string]any{
		"messages": []any{map[string]any{"number": "2", "text": "Two"}},
		"delete":   []any{"005"},
	})
	if err != nil || set["2"] != "Two" || len(del) != 1 || del[0] != "005" {
		t.Errorf("list: %v %v %v", set, del, err)
	}
	for name, args := range map[string]map[string]any{
		"text not a string":  {"messages": map[string]any{"001": 1}},
		"entry without text": {"messages": []any{map[string]any{"number": "001"}}},
		"number twice": {"messages": []any{
			map[string]any{"number": "001", "text": "a"},
			map[string]any{"number": "001", "text": "b"},
		}},
		"delete not strings": {"delete": []any{1}},
		"messages a string":  {"messages": "001=a"},
	} {
		if _, _, err := messageEditArgs(args); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

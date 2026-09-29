package mcp

import "testing"

func TestMessageTexts(t *testing.T) {
	got, err := messageTexts(map[string]any{"2": "second", "001": "first"})
	if err != nil || len(got) != 2 || got[0].Number != "001" || got[1].Number != "002" {
		t.Fatalf("object form: %+v %v", got, err)
	}
	got, err = messageTexts([]any{map[string]any{"number": "10", "text": "ten"}})
	if err != nil || got[0].Number != "010" {
		t.Fatalf("list form: %+v %v", got, err)
	}
	for name, bad := range map[string]any{
		"four digits": map[string]any{"1000": "x"},
		"not digits":  map[string]any{"A1": "x"},
		"too long":    map[string]any{"001": "0123456789012345678901234567890123456789012345678901234567890123456789ABCD"},
		"wrong shape": "text",
	} {
		if _, err := messageTexts(bad); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

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

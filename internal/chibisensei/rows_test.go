package chibisensei

import (
	"reflect"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

func TestValidateRowInput(t *testing.T) {
	ok := []Message{{"user", "hi"}, {"assistant", "ho"}}
	many := make([]Message, 65)
	for i := range many {
		many[i] = Message{"user", "a"}
	}
	cases := []struct {
		name string
		in   RowInput
		want string
	}{
		{"ok", RowInput{Messages: ok}, ""},
		{"empty", RowInput{}, "row.shape"},
		{"both", RowInput{Messages: ok, Text: ptr("x")}, "row.shape"},
		{"many", RowInput{Messages: many}, "row.too_many_messages"},
		{"role", RowInput{Messages: []Message{{"tool", "x"}}}, "row.bad_role"},
		{"long", RowInput{Messages: []Message{{"user", strings.Repeat("a", 32001)}}}, "row.too_long"},
		{"blank", RowInput{Text: ptr("   ")}, "row.empty_text"},
		{"split", RowInput{Messages: ok, Split: "val"}, "row.bad_split"},
		{"tag", RowInput{Messages: ok, Tags: []string{"a,b"}}, "row.bad_tag"},
	}
	for _, c := range cases {
		is := ValidateRowInput(c.in)
		if c.want == "" {
			if len(is) != 0 {
				t.Errorf("%s: unexpected issues %v", c.name, is)
			}
			continue
		}
		if len(is) == 0 || is[0].MessageKey != "chibi_sensei."+c.want {
			t.Errorf("%s: got %v, want %s", c.name, is, c.want)
		}
	}
}

func TestStructureProblems(t *testing.T) {
	m := func(roles ...string) []Message {
		out := make([]Message, len(roles))
		for i, r := range roles {
			out[i] = Message{r, "x"}
		}
		return out
	}
	cases := []struct {
		name   string
		kind   string
		msgs   []Message
		recipe string
		want   []string
	}{
		{"sys", "chat", m("system", "user", "assistant"), RecipeAssistant, nil},
		{"two turns", "chat", m("user", "assistant", "user", "assistant"), RecipeAssistant, nil},
		{"assistant first", "chat", m("assistant"), RecipeAssistant, []string{"role_order"}},
		{"user only", "chat", m("user"), RecipeAssistant, []string{"missing_assistant"}},
		{"empty assistant", "chat", []Message{{"user", "q"}, {"assistant", "  "}}, RecipeAssistant, []string{"empty_assistant"}},
		{"system middle", "chat", m("user", "system", "assistant"), RecipeAssistant, []string{"role_order"}},
		{"text assistant", "text", nil, RecipeAssistant, []string{"text_outside_free"}},
		{"text free", "text", nil, RecipeFree, nil},
	}
	for _, c := range cases {
		if got := StructureProblems(c.kind, c.msgs, c.recipe); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHashesNormalize(t *testing.T) {
	u := func(c string) []Message { return []Message{{"user", c}} }
	if ContentHash("chat", u("Hello  World"), "") != ContentHash("chat", u(" hello world\n"), "") {
		t.Error("case and whitespace should normalize")
	}
	if ContentHash("chat", u("café"), "") != ContentHash("chat", u("café"), "") {
		t.Error("NFC should normalize")
	}
	if ContentHash("chat", u("x"), "") == ContentHash("chat", []Message{{"system", "x"}}, "") {
		t.Error("role must matter")
	}
	a := []Message{{"user", "q"}, {"assistant", "1"}}
	b := []Message{{"user", "q"}, {"assistant", "2"}}
	if PromptHash("chat", a) != PromptHash("chat", b) || ContentHash("chat", a, "") == ContentHash("chat", b, "") {
		t.Error("prompt hash equal, content hash different expected")
	}
	if PromptHash("text", nil) != "" {
		t.Error("text rows have no prompt hash")
	}
}

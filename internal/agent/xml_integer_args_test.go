package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/codegraph"
	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestXMLIntegerParamsReachTypedAndBuiltinConsumers(t *testing.T) {
	for _, input := range []string{"3", "0", "-1", " 3 "} {
		t.Run(input, func(t *testing.T) {
			want, err := strconv.Atoi(strings.TrimSpace(input))
			if err != nil {
				t.Fatal(err)
			}
			content := `<tool_call><function=web_scraper><parameter=limit>` + input + `</parameter><parameter=count>` + input + `</parameter><parameter=depth>3</parameter></function></tool_call>`
			tc := prepareToolCall(ParseToolCall(content), &DispatchContext{Cfg: &config.Config{}})
			if !tc.IsTool || tc.Limit != want || tc.Count != want || tc.Depth != 3 {
				t.Fatalf("typed values: action=%q limit=%d count=%d depth=%d", tc.Action, tc.Limit, tc.Count, tc.Depth)
			}
			if got := decodeWebScraperArgs(builtinArgsFromToolCall(tc)).Limit; got != want {
				t.Fatalf("builtin limit = %d, want %d", got, want)
			}
			if got := toolArgInt(tc.Params, 50, "limit"); got != want {
				t.Fatalf("raw limit = %d, want %d", got, want)
			}
			encoded, err := json.Marshal(tc.Params)
			if err != nil {
				t.Fatal(err)
			}
			var request codegraph.Request
			if err := json.Unmarshal(encoded, &request); err != nil || request.Limit != want || request.Depth != 3 {
				t.Fatalf("code graph request limit=%d depth=%d err=%v", request.Limit, request.Depth, err)
			}
		})
	}
	tc := ParseToolCall(`<tool_call><function=demo><parameter=entry_id>9223372036854775807</parameter></function></tool_call>`)
	if tc.EntryID != int64(^uint64(0)>>1) || tc.Params["entry_id"] != tc.EntryID {
		t.Fatal("large integer lost precision")
	}
}

func TestXMLIntegerParamsPreserveInvalidValuesAndParameterPriority(t *testing.T) {
	for _, input := range []string{"invalid", "1.5", "1e3", "9223372036854775808"} {
		t.Run(input, func(t *testing.T) {
			tc := ParseToolCall(`<tool_call><function=web_scraper><parameter=limit>` + input + `</parameter></function></tool_call>`)
			if tc.Limit != 0 || tc.Params["limit"] != input {
				t.Fatalf("invalid value changed: limit=%d raw=%#v", tc.Limit, tc.Params["limit"])
			}
		})
	}
	for _, fixture := range []struct {
		body string
		want int
	}{
		{`<parameter=limit>3</parameter><parameter=limit>9</parameter>`, 3},
		{`<parameter=params>{"limit":9}</parameter><parameter=limit>3</parameter>`, 0},
		{`<parameter=limit>3</parameter><parameter=params>{"limit":9}</parameter>`, 9},
	} {
		tc := ParseToolCall(`<tool_call><function=web_scraper>` + fixture.body + `</function></tool_call>`)
		if tc.Limit != fixture.want {
			t.Fatalf("parameter priority: limit=%d want=%d body=%s", tc.Limit, fixture.want, fixture.body)
		}
	}
	tc := ParseToolCall(`<tool_call><function=execute_skill><parameter=skill>demo</parameter><parameter=code>00042</parameter><parameter=event_id>00042</parameter><parameter=draft>false</parameter><parameter=body>00042</parameter><parameter=limit>3</parameter></function></tool_call>`)
	if tc.Code != "00042" || tc.Params["event_id"] != "00042" || tc.Params["body"] != "00042" || tc.Params["draft"] != "false" {
		t.Fatal("numeric normalization changed string or boolean arguments")
	}
}

func TestXMLIntegerParamsPreserveJSONNumericConsumers(t *testing.T) {
	for _, body := range []string{
		`<parameter=limit>9</parameter>`,
		`<parameter=limit>3</parameter><parameter=params>{"limit":9}</parameter>`,
		`<parameter=limit>3</parameter><parameter=skill_args>{"limit":9}</parameter>`,
		`<parameter=params>{"limit":9}</parameter><parameter=limit>3</parameter>`,
		`<parameter=skill_args>{"limit":9}</parameter><parameter=limit>3</parameter>`,
	} {
		t.Run(body, func(t *testing.T) {
			tc := ParseToolCall(`<tool_call><function=sip_phone>` + body + `</function></tool_call>`)
			if got := decodeSIPPhoneToolArgs(tc).Limit; got != 9 {
				t.Fatalf("SIP limit = %d, want 9", got)
			}
			// Game Maker asset search reads this JSON numeric representation directly.
			if value, ok := tc.Params["limit"].(float64); !ok || value != 9 {
				t.Fatalf("JSON numeric argument = %v (%T), want float64(9)", tc.Params["limit"], tc.Params["limit"])
			}
			if strings.Contains(body, "skill_args") && tc.SkillArgs["limit"] != float64(9) {
				t.Fatal("skill_args and Params numeric values diverged")
			}
		})
	}
}

func TestXMLIntegerParamsKeepLargeAliasValuesExact(t *testing.T) {
	for _, container := range []string{"params", "skill_args"} {
		for _, value := range []int64{-(1 << 53) - 1, 1 << 53, 1<<53 + 1, int64(^uint64(0) >> 1)} {
			body := fmt.Sprintf(`<parameter=%s>{"label":"kept"}</parameter><parameter=entry_id>%d</parameter>`, container, value)
			tc := ParseToolCall(`<tool_call><function=demo>` + body + `</function></tool_call>`)
			if tc.EntryID != value || toolArgInt64(tc.Params, "entry_id") != value {
				t.Fatalf("integer lost precision: typed=%d raw=%v want=%d", tc.EntryID, tc.Params["entry_id"], value)
			}
			if tc.Params["label"] != "kept" || tc.SkillArgs["entry_id"] != tc.Params["entry_id"] {
				t.Fatal("numeric normalization changed alias contents")
			}
		}
	}
}

func TestXMLIntegerLimitReachesQueryMemory(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	for i := 0; i < 6; i++ {
		if _, err := stm.InsertMessage("default", "user", fmt.Sprintf("integer fixture %d", i), false, false); err != nil {
			t.Fatal(err)
		}
	}
	tc := ParseToolCall(`<tool_call><function=query_memory><parameter=query>integer fixture</parameter><parameter=limit>3</parameter></function></tool_call>`)
	tc.Sources = []string{"conversation"}
	output, err := executeQueryMemoryContext(t.Context(), tc, "default", stm, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Results []struct {
			Source string `json:"source"`
			Count  int    `json:"count"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Source != "conversation" || response.Results[0].Count != 3 {
		t.Fatalf("conversation limit was not retained: %s", output)
	}
}

func TestToolArgIntAcceptsBoundedDecimalText(t *testing.T) {
	for _, fixture := range []struct {
		value any
		want  int
	}{
		{" 3 ", 3}, {"0", 0}, {"-1", -1}, {"0003", 3}, {json.Number("3"), 3},
		{"1e3", 50}, {"1.5", 50}, {"invalid", 50}, {"9223372036854775808", 50},
		{json.Number("9223372036854775808"), 50}, {true, 50}, {nil, 50}, {float64(3), 3},
	} {
		if got := toolArgInt(map[string]interface{}{"value": fixture.value}, 50, "value"); got != fixture.want {
			t.Errorf("value %v (%T): got %d want %d", fixture.value, fixture.value, got, fixture.want)
		}
	}
	for _, tc := range []ToolCall{
		NativeToolCallToToolCall(openai.ToolCall{Function: openai.FunctionCall{Name: "web_scraper", Arguments: `{"limit":"3"}`}}, nil),
		toolCallFromInvokeArgs("web_scraper", map[string]interface{}{"limit": "3"}),
	} {
		if tc.NativeArgsMalformed || decodeWebScraperArgs(builtinArgsFromToolCall(tc)).Limit != 3 {
			t.Fatal("existing native or invoke integer decoding changed")
		}
	}
}

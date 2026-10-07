package agent

import "testing"

// ff1EffectiveSkillArgs mirrors execute_skill's argument lookup (agent_dispatch_comm.go):
// SkillArgs, then Params, then the arguments synthesized from the call's fields.
func ff1EffectiveSkillArgs(tc ToolCall) map[string]interface{} {
	args := tc.SkillArgs
	if args == nil {
		args = tc.Params
	}
	if len(args) == 0 {
		args = synthesizeExecuteSkillArgs(tc)
	}
	return args
}

// FF1 re-review: recording which name filled FilePath must not put anything into Params.
// A text-format execute_skill (or a skill__x shortcut) whose Params stay empty gets its
// arguments from the call's fields, so path, file_path and the other fields (query,
// prompt) all reach the skill, as before.
func TestFF1XMLSkillCallsKeepTheirArguments(t *testing.T) {
	for name, c := range map[string]struct {
		content string
		want    map[string]string
	}{
		"execute_skill path+query": {
			`<tool_call><function=execute_skill><parameter=skill>pdf_tool</parameter><parameter=path>/a.pdf</parameter><parameter=query>find x</parameter></function></tool_call>`,
			map[string]string{"query": "find x", "path": "/a.pdf", "file_path": "/a.pdf"},
		},
		"skill shortcut path+prompt": {
			`<tool_call><function=skill__pdf_tool><parameter=path>/a.pdf</parameter><parameter=prompt>sum it</parameter></function></tool_call>`,
			map[string]string{"prompt": "sum it", "path": "/a.pdf", "file_path": "/a.pdf"},
		},
		"execute_skill file_path+query": {
			`<tool_call><function=execute_skill><parameter=skill>pdf_tool</parameter><parameter=file_path>/b.pdf</parameter><parameter=query>find y</parameter></function></tool_call>`,
			map[string]string{"query": "find y", "path": "/b.pdf", "file_path": "/b.pdf"},
		},
	} {
		tc := normalizeParsedToolShortcut(ParseToolCall(c.content))
		if tc.Action != "execute_skill" || tc.Skill != "pdf_tool" {
			t.Fatalf("%s: parsed %q / %q", name, tc.Action, tc.Skill)
		}
		args := ff1EffectiveSkillArgs(tc)
		for key, want := range c.want {
			if got, _ := args[key].(string); got != want {
				t.Errorf("%s: args[%q] = %v, want %q (args %v)", name, key, args[key], want, args)
			}
		}
		if _, leaked := args["FilePathParam"]; leaked {
			t.Errorf("%s: the parser's bookkeeping reached the skill: %v", name, args)
		}
	}
}

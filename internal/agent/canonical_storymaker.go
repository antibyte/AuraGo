package agent

import (
	"strings"

	"aurago/internal/storymaker"
)

// canonicalizeStorymakerOperationCall repairs the two ways weak models misaddress
// the job tools: an operation called as a tool, or an operation passed to the
// tool that does not own it. It is a no-op outside a run scoped to storymaker_*
// tools. It never widens the scope, and it leaves an explicit contradictory
// operation and a builtin name untouched. Argument repair belongs to Studio.
func canonicalizeStorymakerOperationCall(tc ToolCall, dc *DispatchContext) ToolCall {
	if !storymakerJobScope(dc) {
		return tc
	}
	owners := storymaker.OperationOwners()
	name := tc.Action
	explicit := toolArgString(tc.Params, "operation")
	operation := firstNonEmptyToolString(explicit, tc.Operation)
	// toolCallSnapshotOperation falls back to the op argument, but op is the
	// dry_run target, not an operation. Without an operation argument the call
	// names none, so the snapshot copy is cleared as well.
	if target := toolArgString(tc.Params, "op"); explicit == "" && target != "" && strings.EqualFold(operation, target) {
		operation = ""
		tc.Operation = ""
	}
	owner := ""
	if name == storymaker.ToolStory || name == storymaker.ToolCatalog || name == storymaker.ToolCheck {
		if owner = owners[operation]; owner == "" {
			owner = name
		}
	} else {
		if owner = owners[name]; owner == "" {
			return tc
		}
		if _, builtin := allBuiltinToolNameSet()[name]; builtin {
			return tc
		}
		if operation != "" && operation != name && owners[operation] != owner {
			return tc
		}
		if operation == "" {
			operation = name
		}
	}
	if owner == name || !dispatchToolAllowed(dc, owner) {
		return tc
	}
	args := map[string]interface{}{}
	if tc.Params != nil {
		args = cloneJSONSchemaValue(tc.Params).(map[string]interface{})
	}
	args["operation"] = operation
	if dc.Logger != nil {
		dc.Logger.Info("Storymaker call normalized", "from", name, "tool", owner, "operation", operation)
	}
	tc.normalizedFrom, tc.Params, tc.Operation, tc.Action = name, args, operation, owner
	return tc
}

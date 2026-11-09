package agent

import "aurago/internal/security"

func storedFileGuardianCheck(call ToolCall, guardian *security.Guardian) (security.GuardianCheck, security.ScanResult, security.ScanResult, bool) {
	args := make(map[string]interface{}, len(call.Params)+3)
	for key, value := range call.Params {
		args[key] = value
	}
	args["operation"] = call.Operation
	if call.SubOperation != "" {
		args["sub_operation"] = call.SubOperation
	}
	if _, present := args["content"]; !present && call.Content != "" {
		args["content"] = call.Content
	}
	return security.StoredFileGuardianCheck(security.GuardianCheck{
		Operation: call.Action, Parameters: toolCallParams(call),
	}, args, guardian)
}

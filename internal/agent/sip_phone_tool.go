package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aurago/internal/sipphone"
)

type sipPhoneToolArgs struct {
	Operation string
	Target    string
	CallID    string
	Digits    string
	Limit     int
}

func decodeSIPPhoneToolArgs(tc ToolCall) sipPhoneToolArgs {
	limit := 50
	if raw, ok := tc.Params["limit"]; ok {
		switch value := raw.(type) {
		case float64:
			limit = int(value)
		case int:
			limit = value
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	return sipPhoneToolArgs{
		Operation: strings.ToLower(firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))),
		Target:    toolArgString(tc.Params, "target", "uri"),
		CallID:    toolArgString(tc.Params, "call_id", "id"),
		Digits:    toolArgString(tc.Params, "digits", "digit"),
		Limit:     limit,
	}
}

func dispatchSIPPhone(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if dc == nil || dc.Cfg == nil {
		return sipPhoneToolResult(nil, fmt.Errorf("SIP phone configuration is unavailable"))
	}
	manager := sipphone.DefaultManager()
	if manager == nil {
		return sipPhoneToolResult(nil, sipphone.ErrDisabled)
	}
	request := decodeSIPPhoneToolArgs(tc)
	switch request.Operation {
	case "status":
		return sipPhoneToolResult(map[string]any{"phone": manager.Status()}, nil)
	case "list_calls":
		calls, err := manager.ListCalls(ctx, request.Limit)
		return sipPhoneToolResult(map[string]any{"calls": calls}, err)
	case "dial":
		call, err := manager.Dial(ctx, request.Target)
		return sipPhoneToolResult(map[string]any{"call": call}, err)
	case "answer":
		return sipPhoneToolResult(map[string]any{"call_id": request.CallID}, manager.Answer(request.CallID))
	case "reject":
		return sipPhoneToolResult(map[string]any{"call_id": request.CallID}, manager.Reject(request.CallID))
	case "hangup":
		return sipPhoneToolResult(map[string]any{"call_id": request.CallID}, manager.Hangup(ctx, request.CallID))
	case "send_dtmf":
		if len([]rune(request.Digits)) != 1 {
			return sipPhoneToolResult(nil, fmt.Errorf("exactly one DTMF digit is required"))
		}
		return sipPhoneToolResult(map[string]any{"call_id": request.CallID}, manager.SendDTMF(request.CallID, []rune(request.Digits)[0]))
	default:
		return sipPhoneToolResult(nil, fmt.Errorf("unknown sip_phone operation"))
	}
}

func sipPhoneToolResult(payload map[string]any, err error) string {
	if payload == nil {
		payload = make(map[string]any)
	}
	if err == nil {
		payload["status"] = "ok"
	} else {
		payload["status"] = "error"
		payload["code"] = sipPhoneErrorCode(err)
		payload["message"] = sipPhonePublicError(err)
	}
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return `Tool Output: {"status":"error","message":"Could not encode SIP phone result."}`
	}
	return "Tool Output: " + string(encoded)
}

func sipPhoneErrorCode(err error) string {
	switch {
	case errors.Is(err, sipphone.ErrDisabled):
		return "disabled"
	case errors.Is(err, sipphone.ErrReadOnly):
		return "read_only"
	case errors.Is(err, sipphone.ErrBusy):
		return "busy"
	case errors.Is(err, sipphone.ErrPermissionDenied):
		return "permission_denied"
	case errors.Is(err, sipphone.ErrCallNotFound):
		return "call_not_found"
	default:
		return "operation_failed"
	}
}

func sipPhonePublicError(err error) string {
	switch sipPhoneErrorCode(err) {
	case "disabled":
		return "The SIP phone is unavailable."
	case "read_only":
		return "The SIP phone is in read-only mode."
	case "busy":
		return "The SIP phone already has an active call."
	case "permission_denied":
		return "The SIP operation is not permitted."
	case "call_not_found":
		return "The SIP call was not found."
	default:
		return "The SIP phone operation failed."
	}
}

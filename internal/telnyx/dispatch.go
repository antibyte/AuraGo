package telnyx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"aurago/internal/config"
)

// DispatchSMS handles telnyx_sms tool calls from the agent.
func DispatchSMS(ctx context.Context, operation, to, message, messageID string, mediaURLs []string, cfg *config.Config, logger *slog.Logger) string {
	client := NewClient(cfg.Telnyx.APIKey, logger)

	switch operation {
	case "send":
		if to == "" {
			return encodeResult("error", "parameter 'to' is required for send operation")
		}
		if message == "" {
			return encodeResult("error", "parameter 'message' is required for send operation")
		}
		resp, err := client.SendSMS(ctx, cfg.Telnyx.PhoneNumber, to, message, cfg.Telnyx.MessagingProfileID)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to send SMS: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":     "success",
			"message_id": resp.Data.ID,
			"to":         to,
			"sms_status": resp.Data.Status,
		})

	case "send_mms":
		if to == "" {
			return encodeResult("error", "parameter 'to' is required for send_mms operation")
		}
		if len(mediaURLs) == 0 {
			return encodeResult("error", "parameter 'media_urls' is required for send_mms operation")
		}
		resp, err := client.SendMMS(ctx, cfg.Telnyx.PhoneNumber, to, message, mediaURLs, cfg.Telnyx.MessagingProfileID)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to send MMS: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":     "success",
			"message_id": resp.Data.ID,
			"to":         to,
			"sms_status": resp.Data.Status,
			"type":       "MMS",
		})

	case "status":
		if messageID == "" {
			return encodeResult("error", "parameter 'message_id' is required for status operation")
		}
		resp, err := client.GetMessage(ctx, messageID)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to get message status: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":     "success",
			"message_id": resp.Data.ID,
			"sms_status": resp.Data.Status,
			"direction":  resp.Data.Direction,
			"type":       resp.Data.Type,
			"parts":      resp.Data.Parts,
			"created_at": resp.Data.CreatedAt,
		})

	default:
		return encodeResult("error", fmt.Sprintf("unknown telnyx_sms operation: %s. Use send, send_mms, or status", operation))
	}
}

// DispatchManage handles telnyx_manage tool calls from the agent.
func DispatchManage(ctx context.Context, operation string, limit, page int, cfg *config.Config, logger *slog.Logger) string {
	client := NewClient(cfg.Telnyx.APIKey, logger)

	switch operation {
	case "list_numbers":
		resp, err := client.ListPhoneNumbers(ctx)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to list phone numbers: %v", err))
		}
		numbers := make([]map[string]interface{}, 0, len(resp.Data))
		for _, n := range resp.Data {
			numbers = append(numbers, map[string]interface{}{
				"id":            n.ID,
				"phone_number":  n.PhoneNumber,
				"status":        n.Status,
				"connection_id": n.ConnectionID,
			})
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"numbers": numbers,
			"total":   resp.Meta.TotalResults,
		})

	case "check_balance":
		resp, err := client.GetBalance(ctx)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to check balance: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":           "success",
			"balance":          resp.Data.Balance,
			"currency":         resp.Data.Currency,
			"available_credit": resp.Data.AvailableCredit,
		})

	case "message_history":
		if limit <= 0 {
			limit = 20
		}
		if page <= 0 {
			page = 1
		}
		resp, err := client.ListMessages(ctx, page, limit)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to get message history: %v", err))
		}
		messages := make([]map[string]interface{}, 0, len(resp.Data))
		for _, m := range resp.Data {
			to := ""
			if len(m.To) > 0 {
				to = m.To[0].PhoneNumber
			}
			messages = append(messages, map[string]interface{}{
				"id":         m.ID,
				"direction":  m.Direction,
				"from":       m.From.PhoneNumber,
				"to":         to,
				"text":       m.Text,
				"status":     m.Status,
				"type":       m.Type,
				"created_at": m.CreatedAt,
			})
		}
		return encodeJSON(map[string]interface{}{
			"status":   "success",
			"messages": messages,
			"page":     resp.Meta.PageNumber,
			"total":    resp.Meta.TotalResults,
		})

	case "call_history":
		// Call history uses the same message listing for now; Telnyx v2 doesn't have a separate call history endpoint
		return encodeResult("error", "call_history is only available for completed calls via the call events webhook log")

	default:
		return encodeResult("error", fmt.Sprintf("unknown telnyx_manage operation: %s. Use list_numbers, check_balance, or message_history", operation))
	}
}

// encodeResult creates a simple status/message JSON string.
func encodeResult(status, message string) string {
	return encodeJSON(map[string]interface{}{"status": status, "message": message})
}

// encodeJSON marshals a map to JSON string.
func encodeJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

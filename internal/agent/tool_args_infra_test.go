package agent

import "testing"

func TestDecodeSQLQueryArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "sql_query",
		Params: map[string]interface{}{
			"operation":       "describe",
			"connection_name": "analytics",
			"sql_query":       "select 1",
			"table_name":      "events",
		},
	}

	req := decodeSQLQueryArgs(tc)
	if req.Operation != "describe" {
		t.Fatalf("Operation = %q, want describe", req.Operation)
	}
	if req.ConnectionName != "analytics" {
		t.Fatalf("ConnectionName = %q, want analytics", req.ConnectionName)
	}
	if req.SQLQuery != "select 1" {
		t.Fatalf("SQLQuery = %q, want select 1", req.SQLQuery)
	}
	if req.TableName != "events" {
		t.Fatalf("TableName = %q, want events", req.TableName)
	}
}

func TestDecodeMQTTArgsUsesPayloadFallbacks(t *testing.T) {
	tc := ToolCall{
		Action: "mqtt_publish",
		Params: map[string]interface{}{
			"topic":  "home/test",
			"qos":    float64(2),
			"retain": true,
			"limit":  float64(15),
		},
		Message: "hello",
	}

	req := decodeMQTTArgs(tc)
	if req.Topic != "home/test" {
		t.Fatalf("Topic = %q, want home/test", req.Topic)
	}
	if req.Payload != "hello" {
		t.Fatalf("Payload = %q, want hello", req.Payload)
	}
	if req.QoS != 2 {
		t.Fatalf("QoS = %d, want 2", req.QoS)
	}
	if !req.Retain {
		t.Fatal("expected Retain to be true")
	}
	if req.Limit != 15 {
		t.Fatalf("Limit = %d, want 15", req.Limit)
	}
}

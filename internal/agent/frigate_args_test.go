package agent

import "testing"

func TestDecodeFrigateArgsReadsOffset(t *testing.T) {
	req := decodeFrigateArgs(ToolCall{
		Action: "frigate",
		Params: map[string]interface{}{
			"operation": "events",
			"offset":    float64(75),
		},
	})
	if req.Offset != 75 {
		t.Fatalf("Offset = %d, want 75", req.Offset)
	}
}

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

func TestDecodeFrigateArgsReadsCurrentReviewFilters(t *testing.T) {
	req := decodeFrigateArgs(ToolCall{
		Action: "frigate",
		Params: map[string]interface{}{
			"operation": "reviews",
			"reviewed":  false,
			"severity":  "alert",
		},
	})
	if req.Reviewed == nil || *req.Reviewed {
		t.Fatalf("Reviewed = %#v, want false pointer", req.Reviewed)
	}
	if req.Severity != "alert" {
		t.Fatalf("Severity = %q, want alert", req.Severity)
	}
}

package tools

import "encoding/json"

// The outer boundary covers early failures and all operations, including open.
// Never emit an invalid JSON prefix or silently present incomplete numbers.
func ecalcBoundOutput(result VirtualDesktopExecution, budget int) VirtualDesktopExecution {
	if len(result.Output) <= budget {
		return result
	}
	var fields map[string]any
	_ = json.Unmarshal([]byte(result.Output), &fields)
	if fields["status"] == "error" || fields["status"] == "policy_denied" {
		out := ecalcFailureOut{}
		_ = json.Unmarshal([]byte(result.Output), &out)
		out.Sets = nil
		message := []rune(out.Message)
		lo, hi := 0, len(message)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			out.Message = string(message[:mid]) + "…"
			encoded, _ := ecalcEncode(out)
			if len(encoded) <= budget {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		out.Message = string(message[:lo]) + "…"
		result.Output, _ = ecalcEncode(out)
	} else if result.Event != nil {
		result.Output, _ = ecalcEncode(map[string]any{"status": "ok", "operation": "open", "page": fields["page"], "truncated": true, "message": "Asked the desktop to open ECalc with the supplied values."})
	} else {
		result.Output = `{"status":"error","code":"output_budget_exceeded","message":"The result exceeds the output budget. Request a larger budget or a narrower query."}`
	}
	return result
}

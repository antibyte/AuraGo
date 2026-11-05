package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

var batchCodePattern = regexp.MustCompile(`^FLOW_[A-Z0-9_]{1,60}$`)

func blockEachDef() *NodeDef {
	min, max, serial := float64(1), float64(maxBatchItems), float64(1)
	return &NodeDef{
		Type: blockEachType, Version: 1, Category: "logic", Icon: "repeat", UntrustedOutput: true,
		LabelKey: "easydrag.node.component_each.label", DescriptionKey: "easydrag.node.component_each.description",
		DefaultTimeout: time.Duration(MaxRunSecondsLimit) * time.Second,
		Params: []ParamSpec{
			{Name: "flow_id", Kind: ParamText, Required: true}, {Name: "revision", Kind: ParamNumber, Required: true},
			{Name: "items", Kind: ParamJSON, Templatable: true, Required: true}, {Name: "item_field", Kind: ParamText, Required: true},
			{Name: "inputs", Kind: ParamJSON, Templatable: true, Default: map[string]any{}},
			{Name: "max_items", Kind: ParamNumber, Required: true, Min: &min, Max: &max},
			{Name: "concurrency", Kind: ParamNumber, Required: true, Min: &serial, Max: &serial},
			{Name: "on_item_error", Kind: ParamSelect, Required: true, Options: []Option{{Value: "stop"}, {Value: "collect"}}},
		},
		OutputFields: []FieldSpec{{Name: "items", Type: "list", Primary: true}, {Name: "count", Type: "number"}, {Name: "succeeded", Type: "number"}, {Name: "failed", Type: "number"}},
		ErrorModes:   []ErrorMode{ErrorStop, ErrorPort},
		Validate: func(n *Node, _ ValidateContext) []Issue {
			if n == nil {
				return nil
			}
			if _, err := blockReference(*n); err != nil || n.Settings.Retry.Count != 0 {
				return []Issue{{Code: IssueParamInvalid, Severity: SeverityError, NodeID: n.ID, Message: "a batch needs a fixed block revision, serial execution and no retries"}}
			}
			return nil
		},
		Execute: executeBlockEach,
	}
}

func batchResourceFailure(code string) bool {
	switch code {
	case "FLOW_CANCELLED", "FLOW_RUN_TIMEOUT", "FLOW_NODE_TIMEOUT", "FLOW_OUTPUT_TOO_LARGE", "FLOW_RUN_OUTPUT_TOO_LARGE", "FLOW_BUDGET_EXCEEDED", "FLOW_TOO_LARGE":
		return true
	}
	return false
}

func safeBatchErrorCode(code string) string {
	if !batchCodePattern.MatchString(code) {
		return "FLOW_NODE_FAILED"
	}
	return code
}

// batchEncodedSize counts the exact JSON envelope and comma bytes before the
// next item is retained. The final output uses the same keys and values.
func batchEncodedSize(itemBytes, count, succeeded, failed int) int {
	return len(fmt.Sprintf(`{"count":%d,"failed":%d,"items":[],"succeeded":%d}`, count, failed, succeeded)) + itemBytes + max(0, count-1)
}

// checkBatchItems bounds traversal without resolving or copying values. Arrays
// supplied by a prior node obey the same limits as literal arrays.
func checkBatchItems(ctx context.Context, items []any) error {
	const maxValues, maxDepth = 200000, 64
	type entry struct {
		value any
		depth int
	}
	stack := []entry{{items, 0}}
	count := 0
	for len(stack) != 0 {
		if count%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		count++
		if count > maxValues || current.depth > maxDepth {
			return NewNodeError("FLOW_PARAM_INVALID", "batch items exceed the value or nesting limit")
		}
		switch value := current.value.(type) {
		case []any:
			if len(value) > maxValues-count-len(stack) {
				return NewNodeError("FLOW_PARAM_INVALID", "batch items exceed the value or nesting limit")
			}
			for _, child := range value {
				stack = append(stack, entry{child, current.depth + 1})
			}
		case map[string]any:
			if len(value) > maxValues-count-len(stack) {
				return NewNodeError("FLOW_PARAM_INVALID", "batch items exceed the value or nesting limit")
			}
			for _, child := range value {
				stack = append(stack, entry{child, current.depth + 1})
			}
		}
	}
	return nil
}

func executeBlockEach(ctx context.Context, in ExecInput) (ExecResult, error) {
	if in.block == nil || in.scope == nil || in.Node == nil || in.Services == nil {
		return ExecResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the block execution plan is unavailable")
	}
	if _, err := blockReference(*in.Node); err != nil || in.Node.Settings.Retry.Count != 0 {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "invalid batch configuration")
	}
	if checkBlockBindings(*in.Node, in.block.Contract) != nil {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "invalid batch input bindings")
	}
	items, ok := in.Params["items"].([]any)
	if !ok || items == nil {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "batch items must be a list")
	}
	limit := int(in.Node.Params["max_items"].(float64))
	if len(items) > limit {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "the batch contains more items than its limit")
	}
	if err := checkBatchItems(ctx, items); err != nil {
		return ExecResult{}, err
	}
	field, ok := in.Node.Params["item_field"].(string)
	if !ok {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "the batch item field is invalid")
	}
	shared := map[string]any{}
	if raw, present := in.Params["inputs"]; present {
		shared, ok = raw.(map[string]any)
		if !ok || shared == nil {
			return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "block inputs must be an object")
		}
	}
	if _, exists := shared[field]; exists {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "the batch item field cannot be a shared input")
	}
	sharedFields := make([]blockField, 0, len(in.block.Contract.Inputs))
	for _, decl := range in.block.Contract.Inputs {
		if decl.Name != field {
			sharedFields = append(sharedFields, decl)
		}
	}
	if checkBlockValues(sharedFields, shared) != nil {
		return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "shared block inputs do not match the declared contract")
	}
	policy := in.Node.Params["on_item_error"].(string)
	results := make([]any, 0, len(items))
	succeeded, failed, itemBytes := 0, 0, 0
	services := &Services{Clock: in.Services.Clock, Location: in.Services.Location}
	for index, item := range items {
		if err := ctx.Err(); err != nil {
			return ExecResult{}, err
		}
		inputs := make(map[string]any, len(shared)+1)
		for name, value := range shared {
			inputs[name] = value
		}
		inputs[field] = item
		code := ""
		var output map[string]any
		if checkBlockValues(in.block.Contract.Inputs, inputs) != nil {
			code = "FLOW_PARAM_INVALID"
		} else {
			result := executePureBlock(ctx, in.block.Document, inputs, services, in.Logger, in.Run.Mode, in.block.Ref.Revision, in.Run.ID, in.scope)
			if err := ctx.Err(); err != nil {
				return ExecResult{}, err
			}
			if result.Status != RunSuccess {
				code = safeBatchErrorCode(result.ErrorCode)
			} else {
				for _, node := range in.block.Document.Nodes {
					if node.Type == blockOutputType {
						output = result.Outputs[node.Key]
						break
					}
				}
				if output == nil {
					code = "FLOW_PARAM_INVALID"
				}
			}
		}
		entry := map[string]any{"index": index}
		if code != "" {
			if batchResourceFailure(code) {
				return ExecResult{}, NewNodeError(code, "the batch stopped because a resource or time limit was reached")
			}
			if policy == "stop" {
				return ExecResult{}, NewNodeError("FLOW_BATCH_ITEM_FAILED", "batch item %d failed (%s)", index, code)
			}
			entry["status"], entry["error_code"] = "error", code
			failed++
		} else {
			entry["status"], entry["output"] = "success", output
			succeeded++
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return ExecResult{}, NewNodeError("FLOW_OUTPUT_INVALID", "batch result is invalid")
		}
		if len(encoded) > MaxOutputBytes || batchEncodedSize(itemBytes+len(encoded), len(results)+1, succeeded, failed) > MaxOutputBytes {
			return ExecResult{}, NewNodeError("FLOW_OUTPUT_TOO_LARGE", "the batch results exceed the output limit")
		}
		itemBytes += len(encoded)
		results = append(results, entry)
	}
	return ExecResult{Output: map[string]any{"items": results, "count": len(results), "succeeded": succeeded, "failed": failed}, ItemCount: len(results)}, nil
}

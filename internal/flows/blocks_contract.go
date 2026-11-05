package flows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
)

// Boundary nodes and nested calls use the isolated pure block registry.
const (
	blockInputType     = "block.input"
	blockOutputType    = "block.output"
	blockCallType      = "component.call"
	blockEachType      = "component.each"
	maxBlockDepth      = 8
	maxBlockVersions   = 32
	maxBlockReferences = 4096
	maxBlockFields     = 64
	maxBlockBytes      = 16 << 20
	maxBatchItems      = 1000
	maxBatchWorkers    = 1
)

var (
	errBlockContract = errors.New("invalid reusable block contract")
	blockFieldName   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	blockFlowID      = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

type blockField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Nullable bool   `json:"nullable"`
}

type blockContract struct {
	Inputs  []blockField `json:"inputs"`
	Outputs []blockField `json:"outputs"`
}

// BlockField and BlockContract describe validated named reusable-block boundaries.
type BlockField = blockField
type BlockContract = blockContract

// BlockVersion pairs an immutable published document with its verified contract.
type BlockVersion struct {
	Doc      *Flow         `json:"doc"`
	Contract BlockContract `json:"contract"`
}

// BlockDraftContract returns checked boundary declarations when they are complete.
// It does not imply that a draft graph is publishable.
func BlockDraftContract(doc *Flow) (*BlockContract, error) {
	if !blockDocumentAllowed(doc) {
		return nil, errBlockContract
	}
	contract, err := readBlockContract(doc)
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

type blockRef struct {
	FlowID   string
	Revision int
}

type resolvedBlock struct {
	Ref      blockRef
	Document *Flow
	Contract blockContract
	Hash     [32]byte
	plan     *blockPlan
}

// blockFields accepts only a bounded static declaration. Inputs and outputs
// distinguish an absent field from an explicit null.
func blockFields(raw any) ([]blockField, error) {
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) > 32<<10 {
		return nil, errBlockContract
	}
	var fields []blockField
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fields); err != nil || fields == nil || len(fields) > maxBlockFields {
		return nil, errBlockContract
	}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if !blockFieldName.MatchString(field.Name) || seen[field.Name] {
			return nil, errBlockContract
		}
		seen[field.Name] = true
		switch field.Type {
		case "string", "number", "boolean", "object", "array":
		default:
			return nil, errBlockContract
		}
	}
	return fields, nil
}

func readBlockContract(doc *Flow) (blockContract, error) {
	var contract blockContract
	if doc == nil || doc.Kind != KindBlock || len(doc.Nodes) > MaxNodes || len(doc.Edges) > MaxEdges {
		return contract, errBlockContract
	}
	inputs, outputs := 0, 0
	var outputNode Node
	for _, node := range doc.Nodes {
		switch node.Type {
		case blockInputType:
			inputs++
			fields, err := blockFields(node.Params["fields"])
			if err != nil || node.TypeVersion != 1 || node.Settings.Disabled {
				return blockContract{}, errBlockContract
			}
			contract.Inputs = fields
		case blockOutputType:
			outputs++
			outputNode = node
			fields, err := blockFields(node.Params["fields"])
			if err != nil || node.TypeVersion != 1 || node.Settings.Disabled {
				return blockContract{}, errBlockContract
			}
			contract.Outputs = fields
		}
	}
	if inputs != 1 || outputs != 1 {
		return blockContract{}, errBlockContract
	}
	bindings := map[string]any{}
	if raw, present := outputNode.Params["values"]; present {
		var ok bool
		bindings, ok = raw.(map[string]any)
		if !ok || bindings == nil {
			return blockContract{}, errBlockContract
		}
	}
	if err := checkBlockBindings(Node{Type: blockCallType, Params: map[string]any{"inputs": bindings}}, blockContract{Inputs: contract.Outputs}); err != nil {
		return blockContract{}, err
	}
	return contract, nil
}

// checkBlockValues accepts JSON values only; it never coerces types or drops
// unknown fields, because either would change the called block's meaning.
func checkBlockValues(fields []blockField, values map[string]any) error {
	known := make(map[string]blockField, len(fields))
	for _, field := range fields {
		known[field.Name] = field
		value, present := values[field.Name]
		if !present {
			if field.Required {
				return errBlockContract
			}
			continue
		}
		if value == nil {
			if !field.Nullable {
				return errBlockContract
			}
			continue
		}
		valid := false
		switch field.Type {
		case "string":
			_, valid = value.(string)
		case "number":
			n, ok := value.(float64)
			valid = ok && !math.IsNaN(n) && !math.IsInf(n, 0)
		case "boolean":
			_, valid = value.(bool)
		case "object":
			_, valid = value.(map[string]any)
		case "array":
			_, valid = value.([]any)
		}
		if !valid {
			return errBlockContract
		}
	}
	for key := range values {
		if _, ok := known[key]; !ok {
			return errBlockContract
		}
	}
	return nil
}

func blockReference(node Node) (blockRef, error) {
	if node.TypeVersion != 1 || (node.Type != blockCallType && node.Type != blockEachType) {
		return blockRef{}, errBlockContract
	}
	id, ok := node.Params["flow_id"].(string)
	if !ok || !blockFlowID.MatchString(id) {
		return blockRef{}, errBlockContract
	}
	number, ok := node.Params["revision"].(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 1 || number > math.MaxInt32 || math.Trunc(number) != number {
		return blockRef{}, errBlockContract
	}
	if node.Type == blockEachType {
		items, itemOK := node.Params["max_items"].(float64)
		workers, workerOK := node.Params["concurrency"].(float64)
		policy, policyOK := node.Params["on_item_error"].(string)
		if !itemOK || !workerOK || math.IsNaN(items) || math.IsInf(items, 0) || math.Trunc(items) != items || math.Trunc(workers) != workers || items < 1 || items > maxBatchItems || workers != maxBatchWorkers || !policyOK || (policy != "stop" && policy != "collect") {
			return blockRef{}, errBlockContract
		}
		if _, exists := node.Params["index_field"]; exists {
			return blockRef{}, errBlockContract
		}
	}
	return blockRef{FlowID: id, Revision: int(number)}, nil
}

// checkBlockBindings validates the static names before any input expression is
// evaluated. Values are resolved and checked against their types at runtime.
func checkBlockBindings(node Node, contract blockContract) error {
	bindings := map[string]any{}
	if raw, present := node.Params["inputs"]; present {
		var ok bool
		bindings, ok = raw.(map[string]any)
		if !ok || bindings == nil {
			return errBlockContract
		}
	}
	fields := make(map[string]blockField, len(contract.Inputs))
	for _, field := range contract.Inputs {
		fields[field.Name] = field
	}
	bound := make(map[string]bool, len(bindings)+2)
	for name := range bindings {
		if _, ok := fields[name]; !ok {
			return errBlockContract
		}
		bound[name] = true
	}
	if node.Type == blockEachType {
		if _, ok := node.Params["items"]; !ok {
			return errBlockContract
		}
		item, ok := node.Params["item_field"].(string)
		if !ok || bound[item] {
			return errBlockContract
		}
		if _, ok := fields[item]; !ok {
			return errBlockContract
		}
		bound[item] = true
		if raw, present := node.Params["index_field"]; present {
			index, ok := raw.(string)
			field, exists := fields[index]
			if !ok || !exists || bound[index] || field.Type != "number" {
				return errBlockContract
			}
			bound[index] = true
		}
	}
	for _, field := range contract.Inputs {
		if field.Required && !bound[field.Name] {
			return errBlockContract
		}
	}
	return nil
}

// blockDependencyError identifies the call in the root document, not a node
// with a potentially identical ID in a nested block.
type blockDependencyError struct {
	nodeID string
	err    error
}

func (e *blockDependencyError) Error() string { return e.err.Error() }
func (e *blockDependencyError) Unwrap() error { return e.err }

func blockDependencyIssue(err error) Issue {
	issue := Issue{Code: IssueParamInvalid, Severity: SeverityError, Message: "a referenced block revision is invalid or unavailable"}
	var dependency *blockDependencyError
	if errors.As(err, &dependency) {
		issue.NodeID, issue.Param = dependency.nodeID, "flow_id"
	}
	return issue
}

// resolveBlockDependencies walks even disabled or unreachable nodes. The
// returned child-before-parent list is deterministic for a fixed document.
// The caller must pin these exact versions atomically before publication or use.
func resolveBlockDependencies(ctx context.Context, root *Flow, load func(context.Context, string, int) (*Flow, error)) ([]resolvedBlock, error) {
	if root == nil || root.ID == "" || load == nil {
		return nil, errBlockContract
	}
	active := map[string]bool{root.ID: true}
	seen := make(map[blockRef]bool)
	resolved := make(map[blockRef]resolvedBlock)
	var result []resolvedBlock
	var totalBytes int
	var references int
	var rootNodeID string
	var walk func(*Flow, int) error
	walk = func(doc *Flow, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > maxBlockDepth || len(doc.Nodes) > MaxNodes || len(doc.Edges) > MaxEdges {
			return errBlockContract
		}
		for _, node := range doc.Nodes {
			if node.Type != blockCallType && node.Type != blockEachType {
				continue
			}
			if depth == 0 {
				rootNodeID = node.ID
			}
			references++
			if references > maxBlockReferences || depth == maxBlockDepth {
				return errBlockContract
			}
			ref, err := blockReference(node)
			if err != nil {
				return err
			}
			if active[ref.FlowID] {
				return fmt.Errorf("%w: recursive dependency", errBlockContract)
			}
			if seen[ref] {
				block := resolved[ref]
				if err := checkBlockBindings(node, block.Contract); err != nil {
					return err
				}
				// A cached subtree can contain another revision of an active
				// ancestor, so it must still be checked along this path.
				active[ref.FlowID] = true
				if err := walk(block.Document, depth+1); err != nil {
					return err
				}
				delete(active, ref.FlowID)
				continue
			}
			if len(seen) >= maxBlockVersions {
				return errBlockContract
			}
			child, err := load(ctx, ref.FlowID, ref.Revision)
			if err != nil {
				return err
			}
			if child == nil || child.ID != ref.FlowID {
				return errBlockContract
			}
			data, err := json.Marshal(child)
			if err != nil || len(data) > MaxDocumentBytes {
				return errBlockContract
			}
			// Own the snapshot; neither caller mutation nor a later lookup can
			// change what was checked. Pins must verify this canonical hash.
			child, err = ParseFlow(data)
			if err != nil {
				return errBlockContract
			}
			contract, err := readBlockContract(child)
			if err != nil {
				return err
			}
			if err := checkBlockBindings(node, contract); err != nil {
				return err
			}
			data, err = json.Marshal(child)
			if err != nil || len(data) > MaxDocumentBytes {
				return errBlockContract
			}
			totalBytes += len(data)
			if totalBytes > maxBlockBytes {
				return errBlockContract
			}
			hash := sha256.Sum256(data)
			seen[ref] = true
			active[ref.FlowID] = true
			if err := walk(child, depth+1); err != nil {
				return err
			}
			delete(active, ref.FlowID)
			block := resolvedBlock{Ref: ref, Document: child, Contract: contract, Hash: hash}
			resolved[ref] = block
			result = append(result, block)
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		if rootNodeID != "" {
			return nil, &blockDependencyError{nodeID: rootNodeID, err: err}
		}
		return nil, err
	}
	return result, nil
}

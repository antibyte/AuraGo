package flows

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

type extractionProposal struct {
	parent *Flow
	block  *Flow
	review ExtractionReview
}

var extractionBlockID = regexp.MustCompile(`^flow_[a-z2-7]{26}$`)
var extractionEdgeID = regexp.MustCompile(`^e_[a-z2-7]{8}$`)

func extractionFailure(reason string) error { return &ExtractionError{Reason: reason} }

func extractionIDs() ExtractionIDs {
	var random [16]byte
	_, _ = rand.Read(random[:])
	id := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random[:]))
	return ExtractionIDs{BlockID: "flow_" + id, CallID: NewNodeID(), InputID: NewNodeID(), OutputID: NewNodeID(), EntryEdgeID: NewEdgeID(), ExitEdgeID: NewEdgeID()}
}

func extractionHash(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func validExtractionRequest(req ExtractionRequest) bool {
	return req.BaseRevision > 0 && len(req.NodeIDs) > 0 && len(req.NodeIDs) <= MaxNodes &&
		strings.TrimSpace(req.Name) == req.Name && req.Name != "" && validGroupText(req.Name, 80, 320, false)
}

// rewriteExtractionValue uses the existing parser for validation and replaces
// only root spans in the original source; escaped text and filters stay intact.
func rewriteExtractionValue(value any, selected map[string]bool, external map[string]bool, depth int) (any, error) {
	if depth > maxParamDepth {
		return nil, extractionFailure("reference")
	}
	switch v := value.(type) {
	case string:
		if !strings.Contains(v, "{{") {
			return v, nil
		}
		if _, err := ParseTemplate(v); err != nil {
			return nil, extractionFailure("reference")
		}
		var out strings.Builder
		for i := 0; i < len(v); {
			if v[i] == '\\' && strings.HasPrefix(v[i+1:], "{{") {
				out.WriteString(v[i : i+3])
				i += 3
				continue
			}
			if !strings.HasPrefix(v[i:], "{{") {
				out.WriteByte(v[i])
				i++
				continue
			}
			end, err := findExprEnd(v, i+2)
			if err != nil {
				return nil, extractionFailure("reference")
			}
			body := v[i+2 : end]
			expr, err := parseExpr(body, i+2)
			if err != nil {
				return nil, extractionFailure("reference")
			}
			if selected[expr.Root] {
				out.WriteString(v[i : end+2])
				i = end + 2
				continue
			}
			if !external[expr.Root] || !blockFieldName.MatchString(expr.Root) {
				return nil, extractionFailure("reference")
			}
			start := len(body) - len(strings.TrimLeft(body, " \t\r\n"))
			if !strings.HasPrefix(body[start:], expr.Root) {
				return nil, extractionFailure("reference")
			}
			out.WriteString("{{")
			out.WriteString(body[:start])
			out.WriteString("block_input.data.")
			out.WriteString(body[start:])
			out.WriteString("}}")
			i = end + 2
		}
		return out.String(), nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			x, err := rewriteExtractionValue(item, selected, external, depth+1)
			if err != nil {
				return nil, err
			}
			out[key] = x
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			x, err := rewriteExtractionValue(item, selected, external, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	default:
		return v, nil
	}
}

func extractionProposalFor(doc *Flow, req ExtractionRequest, ids ExtractionIDs, reg *Registry) (*extractionProposal, error) {
	if doc == nil || doc.Kind != KindFlow || !validExtractionRequest(req) || !extractionBlockID.MatchString(ids.BlockID) || ids.BlockID == doc.ID ||
		!ValidNodeID(ids.CallID) || !ValidNodeID(ids.InputID) || !ValidNodeID(ids.OutputID) || !extractionEdgeID.MatchString(ids.EntryEdgeID) || !extractionEdgeID.MatchString(ids.ExitEdgeID) {
		return nil, ErrExtractionInvalid
	}
	if ids.CallID == ids.InputID || ids.CallID == ids.OutputID || ids.InputID == ids.OutputID || ids.EntryEdgeID == ids.ExitEdgeID {
		return nil, ErrExtractionInvalid
	}
	selected := make(map[string]bool, len(req.NodeIDs))
	byID := make(map[string]*Node, len(doc.Nodes))
	byKey := make(map[string]*Node, len(doc.Nodes))
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		byID[n.ID] = n
		byKey[n.Key] = n
		if n.ID == ids.CallID || n.ID == ids.InputID || n.ID == ids.OutputID {
			return nil, ErrExtractionConflict
		}
	}
	for _, id := range req.NodeIDs {
		if selected[id] || byID[id] == nil {
			return nil, extractionFailure("selection")
		}
		selected[id] = true
	}
	for _, e := range doc.Edges {
		if e.ID == ids.EntryEdgeID || e.ID == ids.ExitEdgeID {
			return nil, ErrExtractionConflict
		}
	}
	var entry, exit *Edge
	incoming := map[string]int{}
	outgoing := map[string]int{}
	next := map[string]string{}
	for i := range doc.Edges {
		e := &doc.Edges[i]
		a, b := selected[e.Source.Node], selected[e.Target.Node]
		if (a && e.Source.Port != PortOut) || (b && e.Target.Port != PortIn) {
			return nil, extractionFailure("boundary")
		}
		if a && b {
			incoming[e.Target.Node]++
			outgoing[e.Source.Node]++
			next[e.Source.Node] = e.Target.Node
		} else if !a && b {
			if entry != nil {
				return nil, extractionFailure("boundary")
			}
			entry = e
			incoming[e.Target.Node]++
		} else if a && !b {
			if exit != nil {
				return nil, extractionFailure("boundary")
			}
			exit = e
			outgoing[e.Source.Node]++
		}
	}
	if entry == nil {
		return nil, extractionFailure("boundary")
	}
	order := make([]string, 0, len(selected))
	for id := entry.Target.Node; id != ""; id = next[id] {
		if len(order) >= len(selected) {
			return nil, extractionFailure("branch")
		}
		order = append(order, id)
	}
	if len(order) != len(selected) {
		return nil, extractionFailure("branch")
	}
	for i, id := range order {
		if incoming[id] != 1 || outgoing[id] > 1 || (i < len(order)-1 && outgoing[id] != 1) {
			return nil, extractionFailure("branch")
		}
	}
	last := byID[order[len(order)-1]]
	if exit != nil && exit.Source.Node != last.ID {
		return nil, extractionFailure("boundary")
	}
	selectedKeys := map[string]bool{}
	for _, id := range order {
		n := byID[id]
		if n.Type != TypeSet || n.TypeVersion != 1 || n.Settings.Disabled || n.Parent != "" || (n.Settings.OnError != "" && n.Settings.OnError != ErrorStop) {
			return nil, extractionFailure("node")
		}
		keep, present := n.Params["keep_input"]
		if present && keep != false {
			return nil, extractionFailure("node")
		}
		if n.Key == "block_input" || n.Key == "block_output" {
			return nil, extractionFailure("node")
		}
		selectedKeys[n.Key] = true
	}
	if exit != nil {
		n := byID[exit.Target.Node]
		if n == nil || n.Type != TypeSet || n.Params["keep_input"] != false {
			return nil, extractionFailure("boundary")
		}
	}
	fieldsRaw, ok := last.Params["fields"].([]any)
	if !ok || len(fieldsRaw) > maxBlockFields {
		return nil, extractionFailure("fields")
	}
	outputs := make([]BlockField, 0, len(fieldsRaw))
	values := map[string]any{}
	seen := map[string]bool{}
	for _, raw := range fieldsRaw {
		f, ok := raw.(map[string]any)
		if !ok {
			return nil, extractionFailure("fields")
		}
		name, ok := f["name"].(string)
		if !ok || !blockFieldName.MatchString(name) || seen[name] {
			return nil, extractionFailure("fields")
		}
		seen[name] = true
		typ, _ := f["type"].(string)
		bt := map[string]string{"text": "string", "number": "number", "bool": "boolean", "object": "object", "list": "array"}[typ]
		if bt == "" {
			return nil, extractionFailure("fields")
		}
		outputs = append(outputs, BlockField{Name: name, Type: bt, Required: true})
		values[name] = fmt.Sprintf("{{%s.%s}}", last.Key, name)
	}
	_, beforeTaint := lintUntrusted(doc, reg)
	before := beforeTaint.env()
	external := map[string]bool{"trigger": true}
	for _, n := range doc.Nodes {
		if !selected[n.ID] {
			external[n.Key] = true
		}
	}
	used := map[string]bool{}
	available := map[string]bool{}
	entrySource := byID[entry.Source.Node]
	for _, id := range order {
		refs, problems := CollectTemplateRefs(byID[id].Params)
		if len(problems) > 0 {
			return nil, extractionFailure("reference")
		}
		for _, ref := range refs {
			root := ref.Expr.Root
			if selectedKeys[root] {
				if !available[root] {
					return nil, extractionFailure("reference")
				}
				continue
			}
			if root == "run" || root == "flow" || !external[root] {
				return nil, extractionFailure("reference")
			}
			// A distant graph ancestor can have been skipped on another branch.
			// The immediate delivering source and trigger object are guaranteed.
			if root != "trigger" && (entrySource == nil || byKey[root] != entrySource) {
				return nil, extractionFailure("reference")
			}
			used[root] = true
		}
		available[byID[id].Key] = true
	}
	for _, n := range doc.Nodes {
		if selected[n.ID] {
			continue
		}
		refs, problems := CollectTemplateRefs(nodeTemplateParams(&n))
		if len(problems) > 0 {
			return nil, extractionFailure("reference")
		}
		for _, ref := range refs {
			if selectedKeys[ref.Expr.Root] && ref.Expr.Root != last.Key {
				return nil, extractionFailure("reference")
			}
		}
	}
	inputNames := make([]string, 0, len(used))
	for name := range used {
		inputNames = append(inputNames, name)
	}
	sort.Strings(inputNames)
	if len(inputNames) > maxBlockFields {
		return nil, extractionFailure("fields")
	}
	inputs := make([]BlockField, 0, len(inputNames))
	bindings := map[string]any{}
	for _, name := range inputNames {
		inputs = append(inputs, BlockField{Name: name, Type: "object", Required: true})
		bindings[name] = "{{" + name + "}}"
	}
	parent, err := doc.Clone()
	if err != nil {
		return nil, err
	}
	block := &Flow{Schema: SchemaVersion, ID: ids.BlockID, Kind: KindBlock, Name: req.Name, Settings: FlowSettings{MaxRunSeconds: MaxRunSecondsLimit}}
	minX, maxX := byID[order[0]].Position.X, byID[order[0]].Position.X
	for _, id := range order {
		p := byID[id].Position
		if math.IsNaN(p.X) || math.IsInf(p.X, 0) || math.IsNaN(p.Y) || math.IsInf(p.Y, 0) {
			return nil, extractionFailure("node")
		}
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
	}
	inputX, outputX := minX-320, maxX+320
	if math.IsInf(inputX, 0) || math.IsInf(outputX, 0) || inputX >= minX || outputX <= maxX {
		return nil, extractionFailure("node")
	}
	block.Nodes = append(block.Nodes, Node{ID: ids.InputID, Key: "block_input", Type: blockInputType, TypeVersion: 1, Position: Point{X: inputX, Y: byID[order[0]].Position.Y}, Params: map[string]any{"fields": inputs}})
	for _, id := range order {
		node := *parent.NodeByID(id)
		p, err := rewriteExtractionValue(node.Params, selectedKeys, external, 0)
		if err != nil {
			return nil, err
		}
		node.Params = p.(map[string]any)
		block.Nodes = append(block.Nodes, node)
	}
	block.Nodes = append(block.Nodes, Node{ID: ids.OutputID, Key: "block_output", Type: blockOutputType, TypeVersion: 1, Position: Point{X: outputX, Y: last.Position.Y}, Params: map[string]any{"fields": outputs, "values": values}})
	block.Edges = append(block.Edges, Edge{ID: ids.EntryEdgeID, Source: PortRef{Node: ids.InputID, Port: PortOut}, Target: PortRef{Node: order[0], Port: PortIn}})
	var edges []Edge
	for _, e := range parent.Edges {
		a, b := selected[e.Source.Node], selected[e.Target.Node]
		if a && b {
			block.Edges = append(block.Edges, e)
			continue
		}
		if b {
			e.Target.Node = ids.CallID
		}
		if a {
			e.Source.Node = ids.CallID
		}
		edges = append(edges, e)
	}
	parent.Edges = edges
	block.Edges = append(block.Edges, Edge{ID: ids.ExitEdgeID, Source: PortRef{Node: last.ID, Port: PortOut}, Target: PortRef{Node: ids.OutputID, Port: PortIn}})
	var nodes []Node
	for _, n := range parent.Nodes {
		if !selected[n.ID] {
			nodes = append(nodes, n)
		}
	}
	nodes = append(nodes, Node{ID: ids.CallID, Key: last.Key, Type: blockCallType, TypeVersion: 1, Position: last.Position, Params: map[string]any{"flow_id": ids.BlockID, "revision": float64(1), "inputs": bindings}, Settings: NodeSettings{TimeoutSeconds: MaxNodeTimeoutSeconds}})
	parent.Nodes = nodes
	review := ExtractionReview{Inputs: inputs, Outputs: outputs, ExtraNodes: 3, ExtraEdges: 2, BudgetNote: true}
	if parent.Groups != nil {
		kept := make([]VisualGroup, 0, len(*parent.Groups))
		var moved []VisualGroup
		for _, g := range *parent.Groups {
			var in, out []string
			for _, id := range g.Members {
				if selected[id] {
					in = append(in, id)
				} else {
					out = append(out, id)
				}
			}
			if len(in) == len(g.Members) {
				moved = append(moved, g)
				review.GroupsMoved++
			} else {
				if len(in) > 0 {
					g.Members = out
					review.GroupsShrunk++
				}
				kept = append(kept, g)
			}
		}
		parent.Groups = &kept
		if len(moved) > 0 {
			block.Groups = &moved
		}
	}
	parent.Normalize()
	block.Normalize()
	block, err = block.Clone()
	if err != nil {
		return nil, err
	}
	if HasErrors(Validate(block, blockRegistry(), ValidateContext{Mode: ModePublish})) || HasErrors(Validate(parent, reg, ValidateContext{Mode: ModeDraft})) {
		return nil, extractionFailure("node")
	}
	_, afterTaint := lintUntrusted(parent, reg)
	after := afterTaint.env()
	for _, n := range parent.Nodes {
		if n.ID == ids.CallID {
			continue
		}
		refs, problems := CollectTemplateRefs(nodeTemplateParams(&n))
		if len(problems) > 0 {
			return nil, extractionFailure("reference")
		}
		def, known := reg.Lookup(n.Type)
		for _, ref := range refs {
			if before[ref.Expr.Root] == after[ref.Expr.Root] {
				continue
			}
			if !known {
				return nil, extractionFailure("taint")
			}
			found := false
			for _, spec := range def.Params {
				if spec.Name != ref.RootParam {
					continue
				}
				found = true
				if spec.FenceUntrusted || spec.SensitiveSink || spec.URLTarget || spec.SecretTarget {
					return nil, extractionFailure("taint")
				}
				break
			}
			if !found {
				return nil, extractionFailure("taint")
			}
		}
	}
	if len(parent.Nodes)+len(block.Nodes) > MaxNodes || len(parent.Edges)+len(block.Edges) > MaxEdges {
		return nil, extractionFailure("budget")
	}
	pb, err := parent.Marshal()
	if err != nil {
		return nil, err
	}
	bb, err := block.Marshal()
	if err != nil {
		return nil, err
	}
	if len(pb)+len(bb) > MaxDocumentBytes {
		return nil, extractionFailure("budget")
	}
	return &extractionProposal{parent: parent, block: block, review: review}, nil
}

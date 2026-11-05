package flows

import "context"

// blockRegistry is a trusted, isolated registry: production replacements and
// effectful definitions are never inherited by a reusable block.
func blockRegistry() *Registry {
	r := NewRegistry()
	for _, def := range []*NodeDef{ifNodeDef(), switchNodeDef(), mergeNodeDef(), setNodeDef(),
		{Type: blockInputType, Version: 1, Category: "logic", Icon: "upload", Trigger: true, UntrustedOutput: true,
			LabelKey: "easydrag.node.block_input.label", DescriptionKey: "easydrag.node.block_input.description",
			Params: []ParamSpec{{Name: "fields", Kind: ParamJSON}}, OutputFields: []FieldSpec{{Name: "data", Type: "object", Primary: true}}},
		{Type: blockOutputType, Version: 1, Category: "logic", Icon: "download",
			LabelKey: "easydrag.node.block_output.label", DescriptionKey: "easydrag.node.block_output.description",
			Params: []ParamSpec{{Name: "fields", Kind: ParamJSON}, {Name: "values", Kind: ParamJSON, Templatable: true}},
			Execute: func(_ context.Context, in ExecInput) (ExecResult, error) {
				fields, err := blockFields(in.Params["fields"])
				if err != nil {
					return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "block output contract is invalid")
				}
				raw, present := in.Params["values"]
				values := map[string]any{}
				if present {
					var ok bool
					values, ok = raw.(map[string]any)
					if !ok || values == nil {
						return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "block output values must be an object")
					}
				}
				if checkBlockValues(fields, values) != nil {
					return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "block outputs do not match the declared contract")
				}
				return ExecResult{Output: values}, nil
			}},
	} {
		if err := r.Register(def); err != nil {
			panic(err)
		}
	}
	return r
}

// blockCallDef executes only a validated, pinned snapshot supplied by the run.
func blockCallDef() *NodeDef {
	return &NodeDef{Type: blockCallType, Version: 1, Category: "logic", Icon: "grid", UntrustedOutput: true,
		LabelKey: "easydrag.node.component_call.label", DescriptionKey: "easydrag.node.component_call.description",
		Params: []ParamSpec{{Name: "flow_id", Kind: ParamText, Required: true}, {Name: "revision", Kind: ParamNumber, Required: true},
			{Name: "inputs", Kind: ParamJSON, Templatable: true}},
		ErrorModes: []ErrorMode{ErrorStop, ErrorPort},
		Validate: func(n *Node, _ ValidateContext) []Issue {
			if n == nil {
				return nil
			}
			if _, err := blockReference(*n); err != nil || n.Settings.Retry.Count != 0 {
				return []Issue{{Code: IssueParamInvalid, Severity: SeverityError, NodeID: n.ID, Message: "a block call needs a fixed revision and no retries"}}
			}
			return nil
		},
		Execute: func(ctx context.Context, in ExecInput) (ExecResult, error) {
			if in.block == nil || in.scope == nil {
				return ExecResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the block execution plan is unavailable")
			}
			inputs := map[string]any{}
			if raw, present := in.Params["inputs"]; present {
				var ok bool
				inputs, ok = raw.(map[string]any)
				if !ok || inputs == nil {
					return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "block inputs must be an object")
				}
			}
			if checkBlockValues(in.block.Contract.Inputs, inputs) != nil {
				return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "block inputs do not match the declared contract")
			}
			services := &Services{Clock: in.Services.Clock, Location: in.Services.Location}
			result := executePureBlock(ctx, in.block.Document, inputs, services, in.Logger, in.Run.Mode, in.block.Ref.Revision, in.Run.ID, in.scope)
			if ctx.Err() != nil {
				return ExecResult{}, ctx.Err()
			}
			if result.Status != RunSuccess {
				code := result.ErrorCode
				if code == "" {
					code = "FLOW_NODE_FAILED"
				}
				return ExecResult{}, NewNodeError(code, "block execution failed")
			}
			for _, node := range in.block.Document.Nodes {
				if node.Type == blockOutputType {
					if output, ok := result.Outputs[node.Key]; ok {
						return ExecResult{Output: output}, nil
					}
					break
				}
			}
			return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "the block produced no output")
		},
	}
}

// BlockCatalog exposes only definitions that are safe inside a reusable block.
func (s *Service) BlockCatalog(translate func(string) string) []NodeTypeInfo {
	return DescribeNodeTypes(blockRegistry(), translate)
}

func blockDocumentAllowed(doc *Flow) bool {
	if doc == nil || doc.Kind != KindBlock || len(doc.Nodes) > MaxNodes || len(doc.Edges) > MaxEdges {
		return false
	}
	for _, n := range doc.Nodes {
		if n.TypeVersion != 1 {
			return false
		}
		switch n.Type {
		case blockInputType, blockOutputType, TypeIf, TypeSwitch, TypeMerge, TypeSet:
		default:
			return false
		}
	}
	return true
}

// ValidateBlock checks untrusted drafts without production registry callbacks.
func (s *Service) ValidateBlock(doc *Flow, mode ValidationMode) []Issue {
	if !blockDocumentAllowed(doc) {
		return []Issue{{Code: IssueNodeTypeUnknown, Severity: SeverityError, Message: "the block contains an unsupported node"}}
	}
	issues := Validate(doc, blockRegistry(), s.vc(mode))
	if _, err := readBlockContract(doc); err != nil {
		severity := SeverityError
		if mode == ModeDraft {
			severity = SeverityWarning
		}
		issues = append(issues, Issue{Code: IssueParamInvalid, Severity: severity, Message: "the block needs valid input and output declarations"})
	}
	return issues
}

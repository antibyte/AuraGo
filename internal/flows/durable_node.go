package flows

import "context"

// durableWaitDef is metadata only in the ordinary engine. The isolated durable
// coordinator is the only implementation allowed to suspend this node.
func durableWaitDef(enabled bool) *NodeDef {
	def := logicDef(durableWaitType, "hourglass")
	min, max := float64(1), float64(MaxRunSecondsLimit)
	def.Params = []ParamSpec{{Name: "seconds", Kind: ParamNumber, LabelKey: "easydrag.param.wait_seconds", Required: true, Templatable: true, Min: &min, Max: &max}}
	def.OutputFields = []FieldSpec{{Name: "waited_seconds", Type: "number", Primary: true}}
	def.ErrorModes = []ErrorMode{ErrorStop}
	def.AvailabilityFunc = func() Availability {
		if enabled {
			return Availability{State: AvailableState}
		}
		return Availability{State: NeedsSetupState, Code: "FLOW_DURABLE_DISABLED", ConfigSection: "flows"}
	}
	def.Execute = func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "durable waiting requires the durable coordinator")
	}
	return def
}

func durableApprovalDef(durableEnabled bool, approvalAuthAvailable func() bool) *NodeDef {
	def := logicDef(durableApprovalType, "shield-check")
	def.approvalAuthAvailable = approvalAuthAvailable
	def.LabelKey = "easydrag.node.logic_durable_approval.label"
	def.DescriptionKey = "easydrag.node.logic_durable_approval.description"
	def.SummaryKey = "easydrag.node.logic_durable_approval.summary"
	def.Params = []ParamSpec{
		{Name: "summary", Kind: ParamTextarea, LabelKey: "easydrag.param.approval_summary", Required: true, Templatable: true, OutputIndependent: true},
		{Name: "review_data", Kind: ParamJSON, LabelKey: "easydrag.param.approval_review_data", Templatable: true, Default: map[string]any{}, OutputIndependent: true},
	}
	def.OutputFields = []FieldSpec{{Name: "approved", Type: "boolean", Primary: true}}
	def.ErrorModes = []ErrorMode{ErrorStop}
	def.AvailabilityFunc = func() Availability {
		if !durableEnabled {
			return Availability{State: NeedsSetupState, Code: "FLOW_DURABLE_DISABLED", ConfigSection: "flows"}
		}
		if approvalAuthAvailable == nil || !approvalAuthAvailable() {
			return Availability{State: NeedsSetupState, Code: "FLOW_APPROVAL_AUTH_UNAVAILABLE", ConfigSection: "auth"}
		}
		return Availability{State: AvailableState}
	}
	def.Execute = func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "durable approval requires the durable coordinator")
	}
	return def
}

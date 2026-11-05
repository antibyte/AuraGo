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

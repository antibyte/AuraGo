package flows

import (
	"errors"
	"fmt"
	"sort"
)

var effectOrder = []Effect{
	EffectSendsMessage, EffectWritesFiles, EffectControlsDevices,
	EffectRunsCode, EffectDeletes, EffectSystemChange,
}

// IsRisky reports whether the effects include running code, deleting or changing the system.
func IsRisky(effects []Effect) bool {
	for _, e := range effects {
		switch e {
		case EffectRunsCode, EffectDeletes, EffectSystemChange:
			return true
		}
	}
	return false
}

// EffectSummary lists the enabled nodes that have one effect.
type EffectSummary struct {
	Effect  Effect   `json:"effect"`
	NodeIDs []string `json:"node_ids"`
}

// CollectEffects summarises the outward effects of all enabled nodes for the publish dialog.
//
// An EffectsFunc that panics is recovered, and its node then contributes no
// effects: there is no way to report the problem here. Validate probes the same
// hook and reports the crash, as an error when publishing, so publish flows that
// validate first never show a summary that silently leaves a node out.
func CollectEffects(f *Flow, reg *Registry) []EffectSummary {
	byEffect := map[Effect][]string{}
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Settings.Disabled {
			continue
		}
		def, ok := reg.Lookup(n.Type)
		if !ok {
			continue
		}
		var nodeEffects []Effect
		if catchPanic(func() { nodeEffects = def.EffectsOf(n) }) != nil {
			continue
		}
		seen := map[Effect]bool{}
		for _, e := range nodeEffects {
			if !seen[e] {
				seen[e] = true
				byEffect[e] = append(byEffect[e], n.ID)
			}
		}
	}
	var out []EffectSummary
	known := make(map[Effect]bool, len(effectOrder))
	for _, e := range effectOrder {
		known[e] = true
		if ids := byEffect[e]; len(ids) > 0 {
			out = append(out, EffectSummary{Effect: e, NodeIDs: ids})
		}
	}
	// A confirmation dialog must never drop an effect it does not know: list any
	// effect missing from effectOrder after the known ones, sorted by name.
	var extra []Effect
	for e := range byEffect {
		if !known[e] {
			extra = append(extra, e)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })
	for _, e := range extra {
		out = append(out, EffectSummary{Effect: e, NodeIDs: byEffect[e]})
	}
	return out
}

// hasTooManyRefs reports whether CollectTemplateRefs gave up on a node's
// parameters because they hold more than maxTemplateRefs expressions.
func hasTooManyRefs(problems []TemplateProblem) bool {
	for _, p := range problems {
		if errors.Is(p.Err, errTooManyTemplateRefs) {
			return true
		}
	}
	return false
}

// passesInputs reports whether node n copies the outputs of its inputs into its
// own output, so that what it outputs is as tainted as what it receives even
// though no template reference says so. logic.merge always does; logic.set does
// when keep_input is on, which for a template string is assumed, because its value
// is only known at run time.
//
// Any node type whose Execute copies in.Inputs into its output must be listed
// here; otherwise untrusted data can pass through it unnoticed.
func passesInputs(n *Node) bool {
	switch n.Type {
	case TypeMerge:
		return true
	case TypeSet:
		raw := n.Params["keep_input"]
		if s, ok := raw.(string); ok && HasTemplate(s) {
			return true
		}
		return truthy(raw)
	}
	return false
}

// LintUntrustedData warns when data from an untrusted source (untrusted triggers,
// nodes with UntrustedOutput, and anything derived from them) reaches a parameter
// flagged SensitiveSink. Taint flows through template references in topological
// order, and through nodes that copy their inputs (see passesInputs) from any
// tainted node on an incoming edge. Disabled nodes neither warn nor taint, as they
// do not run.
//
// The finding is an error (IssueUntrustedSecretTarget, whose message asks for a fixed
// address) instead of a warning for a parameter flagged SecretTarget while the node has
// a secret_ref parameter set: that data decides where the vault secret is sent. The
// caller turns an error into a warning for a draft, as it does for every publish rule.
//
// A reference in a parameter flagged OutputIndependent (it reaches only the node's
// effect, such as the content of a PDF or the text of a message, never its output)
// does not taint the node's output. It still warns if that parameter is a sink.
// This keeps "untrusted text goes into a document, the document's path goes on"
// from warning at every step; a parameter that picks the document (its name) is
// not independent and taints as before.
//
// CollectTemplateRefs drops the references past its cap in sorted key order, and
// a flow author controls that order. When a node hits the cap while an untrusted
// source exists, the lint cannot tell what the dropped references read, so it
// assumes the worst: the node counts as tainted and every one of its sink
// parameters gets a warning. Parse problems are reported by the validator and are
// ignored here.
//
// The result follows topological order, then the node's references in sorted
// parameter order, then the cap warnings in the definition's parameter order. The
// flow and the registry's definitions are only read. The lint reads the Trigger,
// UntrustedOutput and Params fields of a definition (of a parameter spec the Kind,
// SensitiveSink, OutputIndependent and SecretTarget) and calls none of its hooks,
// so a crashing hook cannot affect it.
func LintUntrustedData(f *Flow, reg *Registry) []Issue {
	g := buildGraph(f)
	order, cyclic := g.topoOrder()
	if len(cyclic) > 0 {
		return nil
	}
	triggerUntrusted := false
	for i := range f.Nodes {
		def, ok := reg.Lookup(f.Nodes[i].Type)
		if ok && def.Trigger && def.UntrustedOutput && !f.Nodes[i].Settings.Disabled {
			triggerUntrusted = true
		}
	}
	tainted := map[string]bool{}
	// anyTainted is true once a node earlier in the order is tainted.
	anyTainted := false
	var issues []Issue
	for _, id := range order {
		n := g.nodes[id]
		if n.Settings.Disabled {
			tainted[n.Key] = false
			continue
		}
		def, ok := reg.Lookup(n.Type)
		if !ok {
			continue
		}
		nodeTainted := def.UntrustedOutput
		sinks := map[string]bool{}
		independent := map[string]bool{}
		secretSet := false
		for _, spec := range def.Params {
			if spec.SensitiveSink {
				sinks[spec.Name] = true
			}
			if spec.OutputIndependent {
				independent[spec.Name] = true
			}
			// A template counts as set: the value is only known at run time.
			if spec.Kind == ParamSecretRef && !isEmptyValue(n.Params[spec.Name]) {
				secretSet = true
			}
		}
		// The sinks that decide where the node's secret goes, while it has one.
		secretTargets := map[string]bool{}
		if secretSet {
			for _, spec := range def.Params {
				if spec.SensitiveSink && spec.SecretTarget {
					secretTargets[spec.Name] = true
				}
			}
		}
		reported := map[string]bool{}
		warned := map[string]bool{}
		refs, problems := CollectTemplateRefs(n.Params)
		// The node's key, cut like an error echo, is part of every message.
		nodeKey := echoKey(n.Key)
		for _, ref := range refs {
			root := ref.Expr.Root
			if !(root == "trigger" && triggerUntrusted) && !tainted[root] {
				continue
			}
			param := TopParam(ref.Param)
			// A param that only feeds the node's effect (OutputIndependent) does not make
			// what the node returns untrusted; it still warns when it is a sink.
			if !independent[param] {
				nodeTainted = true
			}
			key := param + "|" + root
			if sinks[param] && !reported[key] {
				reported[key] = true
				warned[param] = true
				if secretTargets[param] {
					issues = append(issues, Issue{
						Code: IssueUntrustedSecretTarget, Severity: SeverityError, NodeID: n.ID, Param: param,
						Message: fmt.Sprintf("unchecked data from %s decides where the secret of %s is sent through %s; use a fixed address",
							truncateForError(root), nodeKey, echoKey(param)),
					})
					continue
				}
				issues = append(issues, Issue{
					Code: IssueUntrustedData, Severity: SeverityWarning, NodeID: n.ID, Param: param,
					Message: fmt.Sprintf("unchecked data from %s flows into %s.%s", truncateForError(root), nodeKey, echoKey(param)),
				})
			}
		}
		if (triggerUntrusted || anyTainted) && hasTooManyRefs(problems) {
			nodeTainted = true
			for _, spec := range def.Params {
				if !spec.SensitiveSink || warned[spec.Name] {
					continue
				}
				warned[spec.Name] = true
				if secretTargets[spec.Name] {
					issues = append(issues, Issue{
						Code: IssueUntrustedSecretTarget, Severity: SeverityError, NodeID: n.ID, Param: spec.Name,
						Message: fmt.Sprintf("%s has more than %d template expressions, so they are not checked; assuming unchecked data decides where the secret is sent through %s; use a fixed address",
							nodeKey, maxTemplateRefs, echoKey(spec.Name)),
					})
					continue
				}
				issues = append(issues, Issue{
					Code: IssueUntrustedData, Severity: SeverityWarning, NodeID: n.ID, Param: spec.Name,
					Message: fmt.Sprintf("%s has more than %d template expressions, so they are not checked; assuming unchecked data flows into %s.%s",
						nodeKey, maxTemplateRefs, nodeKey, echoKey(spec.Name)),
				})
			}
		}
		if !nodeTainted && passesInputs(n) {
			for _, e := range g.incoming[id] {
				if src := g.nodes[e.Source.Node]; src != nil && tainted[src.Key] {
					nodeTainted = true
					break
				}
			}
		}
		tainted[n.Key] = nodeTainted
		if nodeTainted {
			anyTainted = true
		}
	}
	return issues
}

// secretTargetRefusals returns, by node id, the failure a run gives each node that
// LintUntrustedData reports as IssueUntrustedSecretTarget: unchecked data decides where
// the node's vault secret is sent. The rule is a publish error, but a live revision
// published under older rules is never validated again and a test run validates with
// draft rules, so the engine enforces it itself, once per run (checkFlow), before the
// node's Execute runs. The error is FLOW_PARAM_INVALID, which no retry repeats, and
// carries the lint's message, which names the data and asks for a fixed address. A node
// with several findings gets the first. nil when no node is refused. The taint rules
// are the lint's alone; this only reads its result.
func secretTargetRefusals(f *Flow, reg *Registry) map[string]*NodeError {
	var refusals map[string]*NodeError
	for _, is := range LintUntrustedData(f, reg) {
		if is.Code != IssueUntrustedSecretTarget || is.NodeID == "" || refusals[is.NodeID] != nil {
			continue
		}
		if refusals == nil {
			refusals = map[string]*NodeError{}
		}
		refusals[is.NodeID] = NewNodeError("FLOW_PARAM_INVALID", "%s", is.Message)
	}
	return refusals
}

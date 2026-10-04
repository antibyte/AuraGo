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
		seen := map[Effect]bool{}
		for _, e := range def.EffectsOf(n) {
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
// CollectTemplateRefs drops the references past its cap in sorted key order, and
// a flow author controls that order. When a node hits the cap while an untrusted
// source exists, the lint cannot tell what the dropped references read, so it
// assumes the worst: the node counts as tainted and every one of its sink
// parameters gets a warning. Parse problems are reported by the validator and are
// ignored here.
//
// The result follows topological order, then the node's references in sorted
// parameter order, then the cap warnings in the definition's parameter order. The
// flow and the registry's definitions are only read.
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
		for _, spec := range def.Params {
			if spec.SensitiveSink {
				sinks[spec.Name] = true
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
			nodeTainted = true
			param := TopParam(ref.Param)
			key := param + "|" + root
			if sinks[param] && !reported[key] {
				reported[key] = true
				warned[param] = true
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

package flows

import (
	"fmt"
	"strings"
)

// Validate checks f. Structural problems are always errors. Publish rules are
// errors in ModePublish and warnings in ModeDraft, so the editor can show them
// while the user is still building. A draft may be saved when it has no errors.
//
// The work is bounded by MaxNodes and MaxEdges, not by the document size: a flow
// over either limit gets one error per exceeded limit (after the schema and name
// checks) and nothing else. Issues come out in a stable order: the schema, name
// and limit issues, then the structure issues in document order of nodes and
// then edges, then per node in document order its publish-rule issues (template
// references in the order CollectTemplateRefs returns them), then the flow-wide
// issues and the cycle issues, then the untrusted-data warnings. Validate only
// reads f and the registry's definitions.
func Validate(f *Flow, reg *Registry, vc ValidateContext) []Issue {
	v := newValidator(f, reg, vc)
	v.run()
	return v.issues
}

type validator struct {
	f      *Flow
	reg    *Registry
	vc     ValidateContext
	issues []Issue
	// byKey maps a node key to the first node with that key (as Flow.NodeByKey
	// finds it), so template references resolve in O(1). It is filled on first use
	// and only ever looked up; never iterate it to emit issues.
	byKey map[string]*Node
	// ancestorWalks counts the ancestor sets computed. templates computes one per
	// node that references another node, never one per reference; the tests check
	// that.
	ancestorWalks int
}

func newValidator(f *Flow, reg *Registry, vc ValidateContext) *validator {
	if vc.Flow == nil {
		vc.Flow = f
	}
	return &validator{f: f, reg: reg, vc: vc}
}

func (v *validator) run() {
	if !v.documentChecks() {
		return
	}
	v.structure()
	v.publishRules()
}

func (v *validator) add(code string, sev Severity, nodeID, edgeID, param, msg string) {
	v.issues = append(v.issues, Issue{Code: code, Severity: sev, NodeID: nodeID, EdgeID: edgeID, Param: param, Message: msg})
}

func (v *validator) publishSeverity() Severity {
	if v.vc.Mode == ModePublish {
		return SeverityError
	}
	return SeverityWarning
}

// nodeByKey returns the first node with the given key, like Flow.NodeByKey but
// without a scan of all nodes per call.
func (v *validator) nodeByKey(key string) *Node {
	if v.byKey == nil {
		v.byKey = make(map[string]*Node, len(v.f.Nodes))
		for i := range v.f.Nodes {
			n := &v.f.Nodes[i]
			if _, dup := v.byKey[n.Key]; !dup {
				v.byKey[n.Key] = n
			}
		}
	}
	return v.byKey[key]
}

// documentChecks reports the problems that cost O(1) to find: the schema
// version, the name and the size limits. It returns false when the flow is over
// a limit, in which case nothing else is checked: every other check works per
// node or per edge, and a document of at most MaxDocumentBytes can still hold
// tens of thousands of them.
func (v *validator) documentChecks() bool {
	f := v.f
	if f.Schema != SchemaVersion {
		v.add(IssueSchema, SeverityError, "", "", "", fmt.Sprintf("unsupported schema version %d", f.Schema))
	}
	if strings.TrimSpace(f.Name) == "" {
		v.add(IssueNameRequired, SeverityError, "", "", "", "the flow needs a name")
	}
	within := true
	if len(f.Nodes) > MaxNodes {
		v.add(IssueTooManyNodes, SeverityError, "", "", "", fmt.Sprintf("a flow can have at most %d nodes", MaxNodes))
		within = false
	}
	if len(f.Edges) > MaxEdges {
		// Same code as the node limit, so the UI needs no extra translation.
		v.add(IssueTooManyNodes, SeverityError, "", "", "", fmt.Sprintf("a flow can have at most %d connections", MaxEdges))
		within = false
	}
	return within
}

// structure checks node ids and keys and the edges. It assumes the flow is
// within MaxNodes and MaxEdges. Every user-controlled value a message repeats
// goes through quoteForError, truncateForError or echoKey, so a message stays
// short whatever the document holds.
func (v *validator) structure() {
	f := v.f
	ids := make(map[string]bool, len(f.Nodes))
	keys := make(map[string]bool, len(f.Nodes))
	for i := range f.Nodes {
		n := &f.Nodes[i]
		switch {
		case !ValidNodeID(n.ID):
			v.add(IssueNodeIDInvalid, SeverityError, n.ID, "", "", fmt.Sprintf("invalid node id %s", quoteForError(n.ID)))
		case ids[n.ID]:
			v.add(IssueNodeIDDuplicate, SeverityError, n.ID, "", "", fmt.Sprintf("duplicate node id %s", quoteForError(n.ID)))
		}
		ids[n.ID] = true
		switch {
		case IsReservedKey(n.Key):
			v.add(IssueNodeKeyReserved, SeverityError, n.ID, "", "", fmt.Sprintf("%s is reserved and cannot be a node key", quoteForError(n.Key)))
		case !ValidKey(n.Key):
			v.add(IssueNodeKeyInvalid, SeverityError, n.ID, "", "", fmt.Sprintf("invalid node key %s", quoteForError(n.Key)))
		case keys[n.Key]:
			v.add(IssueNodeKeyDuplicate, SeverityError, n.ID, "", "", fmt.Sprintf("duplicate node key %s", quoteForError(n.Key)))
		}
		keys[n.Key] = true
	}
	edgeIDs := make(map[string]bool, len(f.Edges))
	seen := make(map[string]bool, len(f.Edges))
	for i := range f.Edges {
		e := &f.Edges[i]
		if e.ID == "" || edgeIDs[e.ID] {
			v.add(IssueEdgeIDDuplicate, SeverityError, "", e.ID, "", fmt.Sprintf("edge id %s is empty or used twice", quoteForError(e.ID)))
		}
		edgeIDs[e.ID] = true
		src, dst := f.NodeByID(e.Source.Node), f.NodeByID(e.Target.Node)
		if src == nil || dst == nil {
			v.add(IssueEdgeNodeMissing, SeverityError, "", e.ID, "", "the connection points to a node that does not exist")
			continue
		}
		if src.ID == dst.ID {
			v.add(IssueEdgeSelf, SeverityError, src.ID, e.ID, "", "a node cannot connect to itself")
			continue
		}
		if def, ok := v.reg.Lookup(src.Type); ok && !containsString(def.OutputPorts(src), e.Source.Port) {
			v.add(IssueEdgePortInvalid, SeverityError, src.ID, e.ID, "", fmt.Sprintf("node %s has no output %s", echoKey(src.Key), quoteForError(e.Source.Port)))
		}
		if def, ok := v.reg.Lookup(dst.Type); ok && !containsString(def.InputPorts(), e.Target.Port) {
			v.add(IssueEdgePortInvalid, SeverityError, dst.ID, e.ID, "", fmt.Sprintf("node %s has no input %s", echoKey(dst.Key), quoteForError(e.Target.Port)))
		}
		k := e.Source.Node + "\x00" + e.Source.Port + "\x00" + e.Target.Node + "\x00" + e.Target.Port
		if seen[k] {
			v.add(IssueEdgeDuplicate, SeverityError, "", e.ID, "", "the same connection exists twice")
		}
		seen[k] = true
	}
}

// publishRules checks what a flow needs to be published. It builds the graph
// once and walks the nodes in document order; the maps inside the graph are only
// used for lookups.
func (v *validator) publishRules() {
	sev := v.publishSeverity()
	g := buildGraph(v.f)
	hasTrigger := false
	for i := range v.f.Nodes {
		n := &v.f.Nodes[i]
		def, ok := v.reg.Lookup(n.Type)
		if !ok {
			v.add(IssueNodeTypeUnknown, sev, n.ID, "", "", fmt.Sprintf("node type %s is not available", quoteForError(n.Type)))
			continue
		}
		if def.Trigger {
			if !n.Settings.Disabled {
				hasTrigger = true
			}
		} else if len(g.incoming[n.ID]) == 0 {
			v.add(IssueNodeUnreachable, SeverityWarning, n.ID, "", "", fmt.Sprintf("node %s has no incoming connection and never runs", echoKey(n.Key)))
		}
		if n.Settings.Disabled {
			continue
		}
		if a := def.Availability(); a.State != AvailableState {
			v.add(IssueNodeUnavailable, sev, n.ID, "", "", fmt.Sprintf("node %s is not available (%s)", echoKey(n.Key), a.State))
		}
		v.requiredParams(n, def, sev)
		if def.Validate != nil {
			for _, is := range def.Validate(n, v.vc) {
				if is.Severity == SeverityError && v.vc.Mode == ModeDraft {
					is.Severity = SeverityWarning
				}
				if is.NodeID == "" {
					is.NodeID = n.ID
				}
				v.issues = append(v.issues, is)
			}
		}
		v.templates(n, g, sev)
	}
	if !hasTrigger {
		v.add(IssueNoTrigger, sev, "", "", "", "the flow needs at least one active trigger")
	}
	// cyclic is in document order. The onCycle walk per cyclic node is bounded by
	// MaxNodes and MaxEdges.
	if _, cyclic := g.topoOrder(); len(cyclic) > 0 {
		for _, id := range cyclic {
			if g.onCycle(id) {
				v.add(IssueCycle, sev, id, "", "", fmt.Sprintf("node %s is part of a loop", echoKey(g.nodes[id].Key)))
			}
		}
	}
	v.issues = append(v.issues, LintUntrustedData(v.f, v.reg)...)
}

func (v *validator) requiredParams(n *Node, def *NodeDef, sev Severity) {
	for _, spec := range def.Params {
		if !spec.Required || !paramVisible(spec, n, def) {
			continue
		}
		val, present := n.Params[spec.Name]
		if !present {
			val = spec.Default
		}
		if paramEmpty(spec, val) {
			v.add(IssueParamRequired, sev, n.ID, "", spec.Name, fmt.Sprintf("%s needs a value for %s", echoKey(n.Key), spec.Name))
		}
	}
}

func paramVisible(spec ParamSpec, n *Node, def *NodeDef) bool {
	if spec.VisibleIf == nil {
		return true
	}
	val, present := n.Params[spec.VisibleIf.Param]
	if !present {
		for _, s := range def.Params {
			if s.Name == spec.VisibleIf.Param {
				val = s.Default
			}
		}
	}
	return containsString(spec.VisibleIf.Equals, Stringify(val))
}

func paramEmpty(spec ParamSpec, v any) bool {
	if spec.Kind == ParamConditionGroup {
		if v == nil {
			return true
		}
		g, err := DecodeConditionGroup(v)
		if err != nil {
			return false
		}
		return len(g.Rows) == 0
	}
	return isEmptyValue(v)
}

func (v *validator) templates(n *Node, g *graph, sev Severity) {
	refs, problems := CollectTemplateRefs(n.Params)
	for _, p := range problems {
		v.add(IssueTemplateSyntax, sev, n.ID, "", p.Param, p.Err.Error())
	}
	// The ancestor set is computed at most once per node, on the first reference
	// that points at another node.
	var ancestors map[string]bool
	for _, ref := range refs {
		root := ref.Expr.Root
		switch root {
		case "trigger", "run", "flow":
			continue
		case "item", "index":
			v.add(IssueTemplateRootUnavailable, sev, n.ID, "", ref.Param, fmt.Sprintf("{{%s}} is only available inside a loop", truncateForError(ref.Expr.Source)))
			continue
		}
		src := v.nodeByKey(root)
		if src == nil {
			v.add(IssueTemplateUnknownRoot, sev, n.ID, "", ref.Param, fmt.Sprintf("{{%s}} refers to an unknown node %s", truncateForError(ref.Expr.Source), quoteForError(root)))
			continue
		}
		if ancestors == nil {
			ancestors = g.ancestors(n.ID)
			v.ancestorWalks++
		}
		if !ancestors[src.ID] {
			v.add(IssueTemplateNotUpstream, sev, n.ID, "", ref.Param, fmt.Sprintf("{{%s}}: node %s does not run before %s", truncateForError(ref.Expr.Source), echoKey(root), echoKey(n.Key)))
			continue
		}
		v.fieldCheck(n, ref, src)
	}
}

func (v *validator) fieldCheck(n *Node, ref TemplateRef, src *Node) {
	def, ok := v.reg.Lookup(src.Type)
	if !ok || def.Trigger || len(ref.Expr.Path) == 0 || ref.Expr.Path[0].IsIndex {
		return
	}
	fields := def.FieldsOf(src)
	if len(fields) == 0 {
		return
	}
	field := ref.Expr.Path[0].Field
	if field == "error" {
		return
	}
	for _, fs := range fields {
		if fs.Name == field {
			return
		}
	}
	v.add(IssueTemplateUnknownField, SeverityWarning, n.ID, "", ref.Param, fmt.Sprintf("{{%s}}: %s has no output field %s", truncateForError(ref.Expr.Source), echoKey(src.Key), quoteForError(field)))
}

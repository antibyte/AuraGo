package flows

import (
	"fmt"
	"strings"
)

// Output limits. Validate's result goes straight into API responses, so every
// field of every issue and the number of issues are bounded, whatever the
// document holds.
const (
	// maxIssues caps the issues Validate returns, not counting the summary issue
	// that reports how many more there were.
	maxIssues = 500
	// maxIssueIDRunes cuts NodeID and EdgeID; real ids are far shorter.
	maxIssueIDRunes = 64
	// maxIssueParamRunes cuts Param.
	maxIssueParamRunes = 200
	// maxIssueMessageRunes cuts Message. Messages built here are shorter already;
	// this covers the ones node validators build.
	maxIssueMessageRunes = 300
	// maxHookPanicRunes cuts the panic text in the issue about a crashing definition
	// hook; finish cuts the whole message to maxIssueMessageRunes in any case.
	maxHookPanicRunes = 200
)

// Validate checks f. Structural problems are always errors. Publish rules are
// errors in ModePublish and warnings in ModeDraft, so the editor can show them
// while the user is still building. A draft may be saved when it has no errors.
//
// The cost is roughly linear in the size of the document. A flow over MaxNodes
// nodes or MaxEdges edges gets one error per exceeded limit (after the schema and
// name checks) and nothing else, which bounds the node and edge counts and with
// them the graph work. The work per node (parameters, template references, node
// validators) still grows with the document size, and the node definitions'
// callbacks (output ports, declared fields) run once per source node, not once
// per edge or reference.
//
// The result is bounded too. NodeID and EdgeID are cut to 64 runes, Param to 200
// and Message to 300 (each cut ends in an ellipsis). At most maxIssues (500)
// issues are returned: all errors up to that number, the rest of the slots going
// to the earliest warnings, in their original order. When anything was dropped,
// one more issue with code IssueTooManyIssues says how many; it is an error when
// a dropped issue was an error, so a caller that gates on HasErrors keeps
// failing.
//
// Issues come out in a stable order: the schema, name and limit issues, then the
// structure issues in document order of nodes and then edges, then per node in
// document order its publish-rule issues (template references in the order
// CollectTemplateRefs returns them), then the flow-wide issues and the cycle
// issues, then the untrusted-data warnings. Validate only reads f and the
// registry's definitions.
//
// A node definition's hooks (OutputsFunc, OutputFieldsFunc, AvailabilityFunc,
// Validate, and EffectsFunc, which is probed once per enabled node so that a
// crash is not left to CollectEffects) are called under a recover. A panic gives
// one IssueParamInvalid issue for that node, "the node definition crashed: ...",
// and the node's remaining hooks are skipped; validation goes on with the other
// nodes. The issue is a publish rule: a warning in ModeDraft, so the draft can
// still be saved, and an error in ModePublish.
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
	// errCount and warnCount count the kept issues by class; dropped counts the
	// ones emit or finish discarded and droppedError records that one of them was
	// an error. See emit.
	errCount, warnCount int
	dropped             int
	droppedError        bool
	// byKey maps a node key to the first node with that key (as Flow.NodeByKey
	// finds it), so template references resolve in O(1). It is filled on first use
	// and only ever looked up; never iterate it to emit issues.
	byKey map[string]*Node
	// outPorts and fieldSets memoize the definition callbacks per node, so a node
	// with many edges or references costs one call, not one per edge or
	// reference. outPorts holds the output port names of a node; fieldSets holds
	// its declared output fields, with a nil set meaning none are declared. Both
	// are only ever looked up.
	outPorts  map[*Node]map[string]bool
	fieldSets map[*Node]map[string]bool
	// crashed holds the nodes whose definition hooks panicked. Each is reported
	// once and none of its hooks is called again. Only ever looked up.
	crashed map[*Node]bool
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
	if v.documentChecks() {
		v.structure()
		v.publishRules()
	}
	v.finish()
}

func (v *validator) add(code string, sev Severity, nodeID, edgeID, param, msg string) {
	v.emit(Issue{Code: code, Severity: sev, NodeID: nodeID, EdgeID: edgeID, Param: param, Message: msg})
}

// emit records an issue. Every issue goes through here, whatever its source
// (structure, publish rules, node validators, the untrusted-data lint). It keeps
// at most maxIssues errors and maxIssues warnings and only counts the rest, so a
// document that produces hundreds of thousands of issues never holds them all in
// memory. That is enough to select the final list in finish: the errors it keeps
// are the first maxIssues errors, and the warnings it keeps are the earliest
// ones, at most maxIssues of them.
func (v *validator) emit(is Issue) {
	if is.Severity == SeverityError {
		if v.errCount >= maxIssues {
			v.dropped++
			v.droppedError = true
			return
		}
		v.errCount++
	} else {
		if v.warnCount >= maxIssues {
			v.dropped++
			return
		}
		v.warnCount++
	}
	v.issues = append(v.issues, is)
}

// finish bounds the result. When more than maxIssues issues are kept it drops
// the latest warnings until all errors and the earliest warnings fit in
// maxIssues, preserving the order of the rest; it then cuts the long fields of
// every issue and appends a summary issue when anything was dropped.
func (v *validator) finish() {
	if v.errCount+v.warnCount > maxIssues {
		slots := maxIssues - v.errCount // the room errors leave for warnings
		kept := v.issues[:0]
		for _, is := range v.issues {
			if is.Severity != SeverityError {
				if slots == 0 {
					v.dropped++
					continue
				}
				slots--
			}
			kept = append(kept, is)
		}
		clear(v.issues[len(kept):])
		v.issues = kept
	}
	for i := range v.issues {
		is := &v.issues[i]
		is.NodeID = truncateRunes(is.NodeID, maxIssueIDRunes)
		is.EdgeID = truncateRunes(is.EdgeID, maxIssueIDRunes)
		is.Param = truncateRunes(is.Param, maxIssueParamRunes)
		is.Message = truncateRunes(is.Message, maxIssueMessageRunes)
	}
	if v.dropped > 0 {
		sev := SeverityWarning
		if v.droppedError {
			sev = SeverityError
		}
		v.issues = append(v.issues, Issue{
			Code: IssueTooManyIssues, Severity: sev,
			Message: fmt.Sprintf("%d more problems are not shown", v.dropped),
		})
	}
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

// callDefHook runs fn, a call into one of n's definition hooks, under a recover
// and reports whether it returned. A node whose hook panicked is reported once as
// an IssueParamInvalid issue, with the publish-rule severity so that a draft with
// a crashing node can still be saved, and none of its hooks is called again.
func (v *validator) callDefHook(n *Node, fn func()) bool {
	if v.crashed[n] {
		return false
	}
	r := catchPanic(fn)
	if r == nil {
		return true
	}
	if v.crashed == nil {
		v.crashed = make(map[*Node]bool)
	}
	v.crashed[n] = true
	v.add(IssueParamInvalid, v.publishSeverity(), n.ID, "", "",
		"the node definition crashed: "+truncateRunes(fmt.Sprint(r), maxHookPanicRunes))
	return false
}

// outputPorts returns the output ports of node n as a set, calling the
// definition once per node however many edges leave it. A logic.switch builds a
// port per case on every OutputPorts call. known is false when the definition
// crashed; the ports are then not known and edges leaving n are not checked.
func (v *validator) outputPorts(n *Node, def *NodeDef) (map[string]bool, bool) {
	if cached, ok := v.outPorts[n]; ok {
		return cached, true
	}
	var ports []string
	if !v.callDefHook(n, func() { ports = def.OutputPorts(n) }) {
		return nil, false
	}
	set := make(map[string]bool, len(ports))
	for _, p := range ports {
		set[p] = true
	}
	if v.outPorts == nil {
		v.outPorts = make(map[*Node]map[string]bool)
	}
	v.outPorts[n] = set
	return set, true
}

// declaredFields returns the names of the output fields node n declares, or nil
// when it declares none (then references into it are not checked, and neither
// are they when the definition crashed). It calls the definition once per node
// however many references read it; OutputFieldsFunc can build the whole field
// list on every call.
func (v *validator) declaredFields(n *Node, def *NodeDef) map[string]bool {
	if set, ok := v.fieldSets[n]; ok {
		return set
	}
	var fields []FieldSpec
	if !v.callDefHook(n, func() { fields = def.FieldsOf(n) }) {
		return nil
	}
	var set map[string]bool
	if len(fields) > 0 {
		set = make(map[string]bool, len(fields))
		for _, fs := range fields {
			set[fs.Name] = true
		}
	}
	if v.fieldSets == nil {
		v.fieldSets = make(map[*Node]map[string]bool)
	}
	v.fieldSets[n] = set
	return set
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
		if def, ok := v.reg.Lookup(src.Type); ok {
			if ports, known := v.outputPorts(src, def); known && !ports[e.Source.Port] {
				v.add(IssueEdgePortInvalid, SeverityError, src.ID, e.ID, "", fmt.Sprintf("node %s has no output %s", echoKey(src.Key), quoteForError(e.Source.Port)))
			}
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
		var avail Availability
		if v.callDefHook(n, func() { avail = def.Availability() }) && avail.State != AvailableState {
			v.add(IssueNodeUnavailable, sev, n.ID, "", "", fmt.Sprintf("node %s is not available (%s)", echoKey(n.Key), avail.State))
		}
		// CollectEffects leaves out the effects of a node whose hook panics, so the
		// crash must not go unnoticed here: publishing is blocked until it is fixed.
		v.callDefHook(n, func() { _ = def.EffectsOf(n) })
		v.requiredParams(n, def, sev)
		if def.Validate != nil {
			var found []Issue
			if v.callDefHook(n, func() { found = def.Validate(n, v.vc) }) {
				for _, is := range found {
					if is.Severity == SeverityError && v.vc.Mode == ModeDraft {
						is.Severity = SeverityWarning
					}
					if is.NodeID == "" {
						is.NodeID = n.ID
					}
					v.emit(is)
				}
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
	for _, is := range LintUntrustedData(v.f, v.reg) {
		v.emit(is)
	}
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
			v.add(IssueTemplateRootUnavailable, sev, n.ID, "", ref.Param, fmt.Sprintf("{{%s}} is only available inside a loop", echoKey(ref.Expr.Source)))
			continue
		}
		src := v.nodeByKey(root)
		if src == nil {
			v.add(IssueTemplateUnknownRoot, sev, n.ID, "", ref.Param, fmt.Sprintf("{{%s}} refers to an unknown node %s", echoKey(ref.Expr.Source), quoteForError(root)))
			continue
		}
		if ancestors == nil {
			ancestors = g.ancestors(n.ID)
			v.ancestorWalks++
		}
		if !ancestors[src.ID] {
			v.add(IssueTemplateNotUpstream, sev, n.ID, "", ref.Param, fmt.Sprintf("{{%s}}: node %s does not run before %s", echoKey(ref.Expr.Source), echoKey(root), echoKey(n.Key)))
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
	fields := v.declaredFields(src, def)
	if fields == nil {
		return
	}
	field := ref.Expr.Path[0].Field
	if field == "error" || fields[field] {
		return
	}
	v.add(IssueTemplateUnknownField, SeverityWarning, n.ID, "", ref.Param, fmt.Sprintf("{{%s}}: %s has no output field %s", echoKey(ref.Expr.Source), echoKey(src.Key), quoteForError(field)))
}

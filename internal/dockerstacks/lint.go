package dockerstacks

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	sevError   = "error"
	sevWarning = "warning"
	sevHint    = "hint"

	// Bounds of one Lint call, so that a hostile or mistaken file cannot make the answer grow without limit:
	maxLintDiagnostics   = 10000 // diagnostics kept; later findings are dropped
	maxLintedEntries     = 16384 // port and storage entries examined, over all services (aliased lists count every use)
	maxPortRegistrations = 8192  // host port intervals remembered for the duplicate check
)

// portReg is one host port interval (one binding of a service) that the duplicate check remembers.
type portReg struct {
	ip     string
	lo, hi int
	line   int
}

type varUse struct {
	preview     VariablePreview
	allDefault  bool
	missing     bool // some use has no default and no value
	missingReq  bool // ... and one of those uses is required
	missingLine int
	missingCol  int
}

type linter struct {
	*pairWalker // mapPairs, mapGet and mapValue, with one merge budget and one cache for the whole call
	env         map[string]string
	stack       string
	published   []PublishedPort
	diags       []Diagnostic
	preview     Preview
	lines       []string // the compose file by line, to find where a variable of a folded scalar was written

	vars      map[string]*varUse
	varOrder  []string
	ports     map[string][]portReg // by protocol, in the order the bindings were read
	entries   int                  // port and storage entries still allowed (maxLintedEntries)
	usedNamed []string
	usedSet   map[string]bool
	declared  map[string]bool // top-level volumes: name -> created by compose (not external)
}

// Lint checks a compose file and its .env without touching an engine. It never blocks anything: a YAML error stops
// the structural checks but still returns the syntax diagnostic with its line. published are the ports other
// containers publish on the host; the ports of project in.Stack are not conflicts.
func Lint(in LintInput, published []PublishedPort) LintResult {
	env, diags := parseEnvFile(in.Env)
	l := &linter{
		pairWalker: newPairWalker(),
		env:        env, stack: in.Stack, published: published, diags: diags,
		preview:  Preview{Services: []ServicePreview{}, NamedVolumes: []string{}, Variables: []VariablePreview{}},
		vars:     map[string]*varUse{},
		ports:    map[string][]portReg{},
		entries:  maxLintedEntries,
		usedSet:  map[string]bool{},
		declared: map[string]bool{},
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(in.Compose), &doc); err != nil {
		line, detail := yamlErrorDetails(err)
		if last := strings.Count(in.Compose, "\n") + 1; line > last {
			line = last // an error at the end of the stream may name the line after the last one
		}
		l.push(Diagnostic{
			Line: line, Severity: sevError, Code: "yaml_syntax",
			Message: "YAML syntax error: " + detail, Params: map[string]string{"detail": detail},
		})
	} else {
		l.lines = strings.Split(strings.ReplaceAll(in.Compose, "\r\n", "\n"), "\n")
		l.checkDocument(&doc)
	}
	sort.SliceStable(l.diags, func(i, j int) bool {
		a, b := l.diags[i], l.diags[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra < rb
		}
		return a.Column < b.Column
	})
	return LintResult{Diagnostics: l.diags, Preview: l.preview}
}

func severityRank(s string) int {
	switch s {
	case sevError:
		return 0
	case sevWarning:
		return 1
	}
	return 2
}

var yamlErrorLine = regexp.MustCompile(`^line (\d+):\s*(.*)$`)

// yamlErrorDetails splits a yaml.v3 error ("yaml: line 3: did not find expected key") into line and text.
func yamlErrorDetails(err error) (line int, detail string) {
	msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(err.Error(), "yaml: "), "unmarshal errors:"))
	msg, _, _ = strings.Cut(msg, "\n")
	if m := yamlErrorLine.FindStringSubmatch(strings.TrimSpace(msg)); m != nil {
		line, _ = strconv.Atoi(m[1])
		return line, m[2]
	}
	return 0, msg
}

// add records a diagnostic positioned at node n (nil = whole file).
func (l *linter) add(n *yaml.Node, svc, severity, code, message string, params map[string]string) {
	d := Diagnostic{Severity: severity, Code: code, Message: message, Params: params, Service: svc}
	if n != nil {
		d.Line, d.Column = n.Line, n.Column
	}
	l.push(d)
}

// push keeps a diagnostic unless maxLintDiagnostics are there already.
func (l *linter) push(d Diagnostic) {
	if !l.full() {
		l.diags = append(l.diags, d)
	}
}

// full reports that no more diagnostics are kept; callers may skip building the next one.
func (l *linter) full() bool { return len(l.diags) >= maxLintDiagnostics }

// spend takes one port or storage entry from the budget of the call; false means the budget is gone and the
// entry is not examined.
func (l *linter) spend() bool {
	if l.entries <= 0 {
		return false
	}
	l.entries--
	return true
}

func nonNull(n *yaml.Node) bool { return n != nil && !(n.Kind == yaml.ScalarNode && n.Tag == "!!null") }

func (l *linter) checkDocument(doc *yaml.Node) {
	var top *yaml.Node // an empty file or one with only comments decodes to a zero node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		top = resolveAlias(doc.Content[0])
	}
	if !nonNull(top) {
		l.add(nil, "", sevError, "no_services", "The compose file has no services.", nil)
		return
	}
	if top.Kind != yaml.MappingNode {
		l.add(top, "", sevError, "not_a_mapping", "The compose file must be a mapping with a services key.",
			map[string]string{"what": "document"})
		return
	}
	l.collectVariables(top)
	l.collectDeclaredVolumes(top)
	sk, sv := l.mapGet(top, "services")
	switch {
	case sk == nil:
		l.add(nil, "", sevError, "no_services", "The compose file has no services.", nil)
	case !nonNull(sv) || (sv.Kind == yaml.MappingNode && len(sv.Content) == 0):
		l.add(sk, "", sevError, "no_services", "The services section is empty.", nil)
	case sv.Kind != yaml.MappingNode:
		l.add(sv, "", sevError, "not_a_mapping", "services must be a mapping of service names.",
			map[string]string{"what": "services"})
	default:
		for _, p := range l.mapPairs(sv) {
			l.checkService(p.Key, p.Value)
		}
	}
	l.finishPreview()
}

func (l *linter) collectDeclaredVolumes(top *yaml.Node) {
	_, vols := l.mapGet(top, "volumes")
	for _, p := range l.mapPairs(vols) {
		_, ext := l.mapGet(p.Value, "external")
		l.declared[p.Key.Value] = !nonNull(ext) || !(ext.Kind == yaml.MappingNode || scalarBool(ext))
	}
}

// collectVariables finds every ${VAR} use in the document's scalars (comments are not nodes, so they are skipped).
func (l *linter) collectVariables(top *yaml.Node) {
	walkScalars(top, func(n *yaml.Node) {
		if !strings.Contains(n.Value, "$") {
			return
		}
		if n.Style&yaml.LiteralStyle != 0 {
			for i, text := range strings.Split(n.Value, "\n") { // a literal block's text starts on the line after the header
				interpolate(text, l.env, l.recordVar(n.Line+1+i, 0))
			}
			return
		}
		// Folding (a ">" block, or a quoted or plain scalar that continues on the next lines) joins source lines, so the
		// value no longer tells where a variable was written: find each use in the source text, in order.
		cur := varCursor{line: n.Line, off: runeToByte(l.sourceLine(n.Line), n.Column)}
		if n.Style&yaml.FoldedStyle != 0 {
			cur = varCursor{line: n.Line + 1} // the header line carries no text
		}
		interpolate(n.Value, l.env, func(r varRef) {
			line, col := l.locateVar(&cur, r.Name)
			switch {
			case line == 0, line == n.Line: // not found, or on the line the scalar starts: where the scalar starts
				line, col = n.Line, n.Column
			}
			l.recordVar(line, col)(r)
		})
	})
}

// maxVarSearchLines is how far below the start of a scalar locateVar looks for a variable.
const maxVarSearchLines = 100

// varCursor is the position in the source after the last variable found in the current scalar: a 1-based line and a
// byte offset in it.
type varCursor struct{ line, off int }

func (l *linter) sourceLine(line int) string {
	if line < 1 || line > len(l.lines) {
		return ""
	}
	return l.lines[line-1]
}

// locateVar finds the next use of the variable name at or after the cursor and moves the cursor past it. line is 0
// when there is none within maxVarSearchLines.
func (l *linter) locateVar(c *varCursor, name string) (line, col int) {
	for ln := c.line; ln < c.line+maxVarSearchLines && ln <= len(l.lines); ln++ {
		text, from := l.lines[ln-1], 0
		if ln == c.line {
			from = c.off
		}
		if idx := findVarToken(text, from, name); idx >= 0 {
			c.line, c.off = ln, idx+1
			return ln, utf8.RuneCountInString(text[:idx]) + 1
		}
	}
	return 0, 0
}

// findVarToken returns the index of the first "$name" or "${name" in text at or after from, skipping the "$$" that
// stands for a literal dollar sign; -1 when there is none.
func findVarToken(text string, from int, name string) int {
	for i := from; i < len(text); i++ {
		if text[i] != '$' {
			continue
		}
		if i+1 < len(text) && text[i+1] == '$' {
			i++
			continue
		}
		rest := strings.TrimPrefix(text[i+1:], "{")
		if strings.HasPrefix(rest, name) && (len(rest) == len(name) || !isNameChar(rest[len(name)])) {
			return i
		}
	}
	return -1
}

// runeToByte converts a 1-based column counted in characters into a byte offset in s.
func runeToByte(s string, col int) int {
	n := 0
	for i := range s {
		if n >= col-1 {
			return i
		}
		n++
	}
	return len(s)
}

func (l *linter) recordVar(line, col int) func(varRef) {
	return func(r varRef) {
		u := l.vars[r.Name]
		if u == nil {
			_, inEnv := l.env[r.Name]
			u = &varUse{preview: VariablePreview{Name: r.Name, Line: line, InEnv: inEnv}, allDefault: true}
			l.vars[r.Name] = u
			l.varOrder = append(l.varOrder, r.Name)
		}
		u.allDefault = u.allDefault && r.Default
		u.preview.Required = u.preview.Required || r.Required
		if r.Default || r.Set {
			return
		}
		if !u.missing {
			u.missing, u.missingLine, u.missingCol = true, line, col
		}
		u.missingReq = u.missingReq || r.Required
	}
}

// finishPreview turns the collected variables and named volumes into the preview and the variable diagnostics.
func (l *linter) finishPreview() {
	for _, name := range l.varOrder {
		u := l.vars[name]
		u.preview.HasDefault = u.allDefault
		l.preview.Variables = append(l.preview.Variables, u.preview)
		if !u.missing {
			continue
		}
		d := Diagnostic{
			Line: u.missingLine, Column: u.missingCol, Severity: sevWarning, Code: "variable_missing",
			Message: fmt.Sprintf("Variable %s is not set in .env and has no default.", name),
			Params:  map[string]string{"variable": name},
		}
		if !u.missingReq {
			d.Fix = &Fix{Kind: "env_random", Variable: name}
		}
		l.push(d)
	}
	for _, name := range l.usedNamed {
		if l.declared[name] {
			l.preview.NamedVolumes = append(l.preview.NamedVolumes, name)
		}
	}
}

func (l *linter) checkService(k, v *yaml.Node) {
	name := k.Value
	sp := ServicePreview{Name: name, Line: k.Line, Ports: []PortPreview{}, Volumes: []VolumePreview{}}
	defer func() { l.preview.Services = append(l.preview.Services, sp) }()
	if v == nil || v.Kind != yaml.MappingNode {
		l.add(k, name, sevError, "not_a_mapping", fmt.Sprintf("Service %s must be a mapping.", name),
			map[string]string{"what": "service"})
		return
	}
	_, imgNode := l.mapGet(v, "image")
	image, _ := scalarText(imgNode)
	image = strings.TrimSpace(image)
	_, buildNode := l.mapGet(v, "build")
	_, extends := l.mapGet(v, "extends")
	sp.Image, sp.Build = image, nonNull(buildNode)
	switch {
	case image == "" && !sp.Build && !nonNull(extends):
		l.add(k, name, sevError, "service_no_image",
			fmt.Sprintf("Service %s has neither an image nor a build section.", name), map[string]string{"service": name})
	case image != "" && !sp.Build:
		l.checkImage(name, imgNode, image)
	}
	if pk, pv := l.mapGet(v, "privileged"); pk != nil && scalarBool(pv) {
		l.add(pk, name, sevHint, "privileged", "A privileged container has full access to the host.", nil)
	}
	if nk, nv := l.mapGet(v, "network_mode"); nk != nil {
		if mode, _ := scalarText(nv); mode == "host" {
			l.add(nk, name, sevHint, "network_mode_host", "network_mode host shares the host's network stack.", nil)
		}
	}
	if ck, cv := l.mapGet(v, "cap_add"); ck != nil {
		if caps := scalarList(cv); len(caps) > 0 {
			l.add(ck, name, sevHint, "cap_add", "Extra Linux capabilities: "+strings.Join(caps, ", "),
				map[string]string{"caps": strings.Join(caps, ", ")})
		}
	}
	if rk, _ := l.mapGet(v, "restart"); rk == nil && !nonNull(extends) {
		_, deploy := l.mapGet(v, "deploy")
		if dk, _ := l.mapGet(deploy, "restart_policy"); dk == nil {
			l.add(k, name, sevHint, "restart_missing",
				fmt.Sprintf("Service %s has no restart policy, so it stays down after a reboot.", name), nil)
		}
	}
	_, ports := l.mapGet(v, "ports")
	for _, item := range seqItems(ports) {
		if !l.spend() {
			break
		}
		l.checkPort(name, item, &sp)
	}
	_, vols := l.mapGet(v, "volumes")
	for _, item := range seqItems(vols) {
		if !l.spend() {
			break
		}
		l.checkVolume(name, item, &sp)
	}
	_, tmpfs := l.mapGet(v, "tmpfs")
	for _, target := range scalarList(tmpfs) {
		if expanded, unresolved := interpolate(target, l.env, nil); !unresolved {
			path, _, _ := strings.Cut(expanded, ":")
			sp.Volumes = append(sp.Volumes, VolumePreview{Kind: "tmpfs", Target: path})
		}
	}
}

// scalarList returns the non-empty scalar texts of a scalar or a sequence of scalars.
func scalarList(n *yaml.Node) []string {
	var out []string
	if s, ok := scalarText(n); ok {
		return nonEmpty(out, s)
	}
	for _, item := range seqItems(n) {
		if s, ok := scalarText(item); ok {
			out = nonEmpty(out, s)
		}
	}
	return out
}

func nonEmpty(list []string, s string) []string {
	if s = strings.TrimSpace(s); s != "" {
		list = append(list, s)
	}
	return list
}

func (l *linter) checkImage(svc string, n *yaml.Node, image string) {
	if strings.Contains(image, "$") || strings.Contains(image, "@") {
		return // a variable decides the tag, or a digest pins it
	}
	last := image[strings.LastIndexByte(image, '/')+1:]
	tag := ""
	if i := strings.LastIndexByte(last, ':'); i >= 0 {
		tag = last[i+1:]
	}
	params := map[string]string{"image": image}
	switch tag {
	case "latest":
		l.add(n, svc, sevHint, "image_latest", fmt.Sprintf("%s follows the latest tag, so it changes without notice.", image), params)
	case "":
		l.add(n, svc, sevHint, "image_no_tag", fmt.Sprintf("%s has no tag, so Docker uses latest.", image), params)
	}
}

// findConflict returns the container that already publishes one of the host ports lo..hi on an overlapping
// address: the one with the lowest such port (the first listed on a tie), and that port. nil when there is none.
func (l *linter) findConflict(lo, hi int, proto, ip string) (*PublishedPort, int) {
	var found *PublishedPort
	for i := range l.published {
		p := &l.published[i]
		pp := strings.ToLower(p.Protocol)
		if pp == "" {
			pp = "tcp"
		}
		if (l.stack != "" && p.Project == l.stack) || p.Port < lo || p.Port > hi || pp != proto || !ipsOverlap(p.HostIP, ip) {
			continue
		}
		if found == nil || p.Port < found.Port {
			found = p
		}
	}
	if found == nil {
		return nil, 0
	}
	return found, found.Port
}

// portSpecOf reads one entry of a service's ports list (short or long syntax). ok is false when the entry cannot
// be judged because a variable in it has no value; err is the reason an entry is invalid.
func (l *linter) portSpecOf(item *yaml.Node) (spec portSpec, raw string, ok bool, err error) {
	expand := func(s string) (string, bool) { return interpolate(s, l.env, nil) }
	switch item.Kind {
	case yaml.ScalarNode:
		raw = item.Value
		text, unresolved := expand(raw)
		if unresolved {
			return portSpec{}, raw, false, nil
		}
		spec, err = parsePortSpec(text)
		return spec, raw, true, err
	case yaml.MappingNode:
		field := func(key string) (string, bool) {
			text, _ := scalarText(l.mapValue(item, key))
			return expand(text)
		}
		target, u1 := field("target")
		published, u2 := field("published")
		hostIP, u3 := field("host_ip")
		proto, u4 := field("protocol")
		raw = "target: " + target + ", published: " + published
		if u1 || u2 || u3 || u4 {
			return portSpec{}, raw, false, nil
		}
		if target == "" {
			return portSpec{}, raw, true, fmt.Errorf("the target port is missing")
		}
		spec, err = newPortSpec(hostIP, published, target, proto)
		return spec, raw, true, err
	}
	return portSpec{}, "", true, fmt.Errorf("a port must be a string, a number or a mapping")
}

func (l *linter) checkPort(svc string, item *yaml.Node, sp *ServicePreview) {
	spec, raw, ok, err := l.portSpecOf(item)
	if !ok {
		return
	}
	if err != nil {
		l.add(item, svc, sevError, "invalid_port", fmt.Sprintf("Invalid port %q: %v", raw, err),
			map[string]string{"value": raw, "reason": err.Error()})
		return
	}
	pv := PortPreview{HostIP: spec.HostIP, Host: spec.hostText(), Container: spec.containerText(), Protocol: spec.Protocol}
	if spec.fixedHostPorts() {
		l.checkHostPorts(svc, item, spec, &pv)
	}
	sp.Ports = append(sp.Ports, pv)
}

// checkHostPorts looks for the same host port twice in the file and for ports other containers already publish.
// A binding is remembered as one interval, not one entry per port, so a range costs the same as a single port.
func (l *linter) checkHostPorts(svc string, item *yaml.Node, spec portSpec, pv *PortPreview) {
	lo, hi := spec.HostLo, spec.HostHi
	// The lowest host port of this binding that an earlier binding uses as well (the earliest one on a tie).
	var dup *portReg
	dupPort := 0
	regs := l.ports[spec.Protocol]
	for i := range regs {
		prev := &regs[i]
		if prev.lo > hi || prev.hi < lo || !ipsOverlap(prev.ip, spec.HostIP) {
			continue
		}
		if port := max(lo, prev.lo); dup == nil || port < dupPort {
			dup, dupPort = prev, port
		}
	}
	if dup != nil && !l.full() {
		l.add(item, svc, sevError, "port_duplicate",
			fmt.Sprintf("Host port %d/%s is published twice (first at line %d).", dupPort, spec.Protocol, dup.line),
			map[string]string{"port": fmt.Sprint(dupPort), "protocol": spec.Protocol, "first_line": fmt.Sprint(dup.line)})
	}
	if len(regs) < maxPortRegistrations {
		l.ports[spec.Protocol] = append(regs, portReg{ip: spec.HostIP, lo: lo, hi: hi, line: item.Line})
	}
	if c, port := l.findConflict(lo, hi, spec.Protocol, spec.HostIP); c != nil {
		pv.Conflict = c.Container
		l.add(item, svc, sevWarning, "port_conflict",
			fmt.Sprintf("Host port %d/%s is already published by container %s.", port, spec.Protocol, c.Container),
			map[string]string{"port": fmt.Sprint(port), "protocol": spec.Protocol, "container": c.Container, "project": c.Project})
	}
}

func (l *linter) checkVolume(svc string, item *yaml.Node, sp *ServicePreview) {
	var spec volumeSpec
	var err error
	switch item.Kind {
	case yaml.ScalarNode:
		text, unresolved := interpolate(item.Value, l.env, nil)
		if unresolved {
			return
		}
		spec, err = parseVolumeShort(text)
	case yaml.MappingNode:
		field := func(key string) (string, bool) {
			text, _ := scalarText(l.mapValue(item, key))
			return interpolate(text, l.env, nil)
		}
		typ, u1 := field("type")
		source, u2 := field("source")
		target, u3 := field("target")
		if u1 || u2 || u3 {
			return
		}
		spec, err = volumeFromLong(typ, source, target, scalarBool(l.mapValue(item, "read_only")))
	default:
		return
	}
	if err != nil {
		return // Compose reports malformed storage entries when the stack is deployed
	}
	sp.Volumes = append(sp.Volumes, VolumePreview{Kind: spec.Kind, Source: spec.Source, Target: spec.Target, ReadOnly: spec.ReadOnly})
	switch {
	case spec.Kind == "named":
		if !l.usedSet[spec.Source] {
			l.usedSet[spec.Source] = true
			l.usedNamed = append(l.usedNamed, spec.Source)
		}
	case strings.Contains(spec.Source, "docker.sock") || strings.Contains(spec.Source, "docker_engine"):
		l.add(item, svc, sevHint, "docker_socket", "Mounting the Docker socket gives the container control of the host.",
			map[string]string{"source": spec.Source})
	}
}

func stringIn(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

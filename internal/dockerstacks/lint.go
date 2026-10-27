package dockerstacks

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	sevError   = "error"
	sevWarning = "warning"
	sevHint    = "hint"

	maxLintedPortRange = 4096 // host ports checked per binding; a range wider than this is only checked at its start
)

type portKey struct {
	port  int
	proto string
}

type portUse struct {
	ip   string
	line int
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
	env       map[string]string
	stack     string
	published []PublishedPort
	diags     []Diagnostic
	preview   Preview

	vars      map[string]*varUse
	varOrder  []string
	ports     map[portKey][]portUse
	usedNamed []string
	declared  map[string]bool // top-level volumes: name -> created by compose (not external)
}

// Lint checks a compose file and its .env without touching an engine. It never blocks anything: a YAML error stops
// the structural checks but still returns the syntax diagnostic with its line. published are the ports other
// containers publish on the host; the ports of project in.Stack are not conflicts.
func Lint(in LintInput, published []PublishedPort) LintResult {
	env, diags := parseEnvFile(in.Env)
	l := &linter{
		env: env, stack: in.Stack, published: published, diags: diags,
		preview:  Preview{Services: []ServicePreview{}, NamedVolumes: []string{}, Variables: []VariablePreview{}},
		vars:     map[string]*varUse{},
		ports:    map[portKey][]portUse{},
		declared: map[string]bool{},
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(in.Compose), &doc); err != nil {
		line, detail := yamlErrorDetails(err)
		if last := strings.Count(in.Compose, "\n") + 1; line > last {
			line = last // an error at the end of the stream may name the line after the last one
		}
		l.diags = append(l.diags, Diagnostic{
			Line: line, Severity: sevError, Code: "yaml_syntax",
			Message: "YAML syntax error: " + detail, Params: map[string]string{"detail": detail},
		})
	} else {
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
	l.diags = append(l.diags, d)
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
	sk, sv := mapGet(top, "services")
	switch {
	case sk == nil:
		l.add(nil, "", sevError, "no_services", "The compose file has no services.", nil)
	case !nonNull(sv) || (sv.Kind == yaml.MappingNode && len(sv.Content) == 0):
		l.add(sk, "", sevError, "no_services", "The services section is empty.", nil)
	case sv.Kind != yaml.MappingNode:
		l.add(sv, "", sevError, "not_a_mapping", "services must be a mapping of service names.",
			map[string]string{"what": "services"})
	default:
		for _, p := range mapPairs(sv) {
			l.checkService(p.Key, p.Value)
		}
	}
	l.finishPreview()
}

func (l *linter) collectDeclaredVolumes(top *yaml.Node) {
	_, vols := mapGet(top, "volumes")
	for _, p := range mapPairs(vols) {
		_, ext := mapGet(p.Value, "external")
		l.declared[p.Key.Value] = !nonNull(ext) || !(ext.Kind == yaml.MappingNode || scalarBool(ext))
	}
}

// collectVariables finds every ${VAR} use in the document's scalars (comments are not nodes, so they are skipped).
func (l *linter) collectVariables(top *yaml.Node) {
	walkScalars(top, func(n *yaml.Node) {
		if !strings.Contains(n.Value, "$") {
			return
		}
		if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			for i, text := range strings.Split(n.Value, "\n") { // a block scalar's text starts on the line after the header
				interpolate(text, l.env, l.recordVar(n.Line+1+i, 0))
			}
			return
		}
		interpolate(n.Value, l.env, l.recordVar(n.Line, n.Column))
	})
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
		l.diags = append(l.diags, d)
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
	_, imgNode := mapGet(v, "image")
	image, _ := scalarText(imgNode)
	image = strings.TrimSpace(image)
	_, buildNode := mapGet(v, "build")
	_, extends := mapGet(v, "extends")
	sp.Image, sp.Build = image, nonNull(buildNode)
	switch {
	case image == "" && !sp.Build && !nonNull(extends):
		l.add(k, name, sevError, "service_no_image",
			fmt.Sprintf("Service %s has neither an image nor a build section.", name), map[string]string{"service": name})
	case image != "" && !sp.Build:
		l.checkImage(name, imgNode, image)
	}
	if pk, pv := mapGet(v, "privileged"); pk != nil && scalarBool(pv) {
		l.add(pk, name, sevHint, "privileged", "A privileged container has full access to the host.", nil)
	}
	if nk, nv := mapGet(v, "network_mode"); nk != nil {
		if mode, _ := scalarText(nv); mode == "host" {
			l.add(nk, name, sevHint, "network_mode_host", "network_mode host shares the host's network stack.", nil)
		}
	}
	if ck, cv := mapGet(v, "cap_add"); ck != nil {
		if caps := scalarList(cv); len(caps) > 0 {
			l.add(ck, name, sevHint, "cap_add", "Extra Linux capabilities: "+strings.Join(caps, ", "),
				map[string]string{"caps": strings.Join(caps, ", ")})
		}
	}
	if rk, _ := mapGet(v, "restart"); rk == nil && !nonNull(extends) {
		_, deploy := mapGet(v, "deploy")
		if dk, _ := mapGet(deploy, "restart_policy"); dk == nil {
			l.add(k, name, sevHint, "restart_missing",
				fmt.Sprintf("Service %s has no restart policy, so it stays down after a reboot.", name), nil)
		}
	}
	_, ports := mapGet(v, "ports")
	for _, item := range seqItems(ports) {
		l.checkPort(name, item, &sp)
	}
	_, vols := mapGet(v, "volumes")
	for _, item := range seqItems(vols) {
		l.checkVolume(name, item, &sp)
	}
	_, tmpfs := mapGet(v, "tmpfs")
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

func (l *linter) findConflict(port int, proto, ip string) *PublishedPort {
	for i := range l.published {
		p := &l.published[i]
		pp := strings.ToLower(p.Protocol)
		if pp == "" {
			pp = "tcp"
		}
		if (l.stack != "" && p.Project == l.stack) || p.Port != port || pp != proto || !ipsOverlap(p.HostIP, ip) {
			continue
		}
		return p
	}
	return nil
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
			text, _ := scalarText(mapValue(item, key))
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
func (l *linter) checkHostPorts(svc string, item *yaml.Node, spec portSpec, pv *PortPreview) {
	dupReported := false
	width := spec.HostHi - spec.HostLo + 1
	if width > maxLintedPortRange {
		width = maxLintedPortRange
	}
	for port := spec.HostLo; port < spec.HostLo+width; port++ {
		key := portKey{port, spec.Protocol}
		if !dupReported {
			for _, prev := range l.ports[key] {
				if !ipsOverlap(prev.ip, spec.HostIP) {
					continue
				}
				dupReported = true
				l.add(item, svc, sevError, "port_duplicate",
					fmt.Sprintf("Host port %d/%s is published twice (first at line %d).", port, spec.Protocol, prev.line),
					map[string]string{"port": fmt.Sprint(port), "protocol": spec.Protocol, "first_line": fmt.Sprint(prev.line)})
				break
			}
		}
		l.ports[key] = append(l.ports[key], portUse{ip: spec.HostIP, line: item.Line})
		if pv.Conflict != "" {
			continue
		}
		if c := l.findConflict(port, spec.Protocol, spec.HostIP); c != nil {
			pv.Conflict = c.Container
			l.add(item, svc, sevWarning, "port_conflict",
				fmt.Sprintf("Host port %d/%s is already published by container %s.", port, spec.Protocol, c.Container),
				map[string]string{"port": fmt.Sprint(port), "protocol": spec.Protocol, "container": c.Container, "project": c.Project})
		}
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
			text, _ := scalarText(mapValue(item, key))
			return interpolate(text, l.env, nil)
		}
		typ, u1 := field("type")
		source, u2 := field("source")
		target, u3 := field("target")
		if u1 || u2 || u3 {
			return
		}
		spec, err = volumeFromLong(typ, source, target, scalarBool(mapValue(item, "read_only")))
	default:
		return
	}
	if err != nil {
		return // Compose reports malformed storage entries when the stack is deployed
	}
	sp.Volumes = append(sp.Volumes, VolumePreview{Kind: spec.Kind, Source: spec.Source, Target: spec.Target, ReadOnly: spec.ReadOnly})
	switch {
	case spec.Kind == "named":
		if !stringIn(l.usedNamed, spec.Source) {
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

package dockerstacks

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---- building the compose file ----

type converter struct {
	run      *runCommand
	svc      *mapping
	named    []string // named volumes the service uses
	networks []string // user-defined networks the service joins
}

func (c *converter) warn(code, flag, message string) { c.run.warn(code, flag, message) }

func (c *converter) addNamed(name string) {
	if !stringIn(c.named, name) {
		c.named = append(c.named, name)
	}
}

// build writes the service in a fixed key order: what runs, how it starts, how it is exposed, what it stores, then
// the tuning options.
func (r *runCommand) build() (ConvertResult, error) {
	r.normalizeValues()
	c := &converter{run: r, svc: newMapping()}
	name, hasName := r.last("name")
	service := serviceNameFromImage(r.image)
	if hasName {
		service = sanitizeServiceName(name)
	}
	c.svc.add("image", strNode(r.image))
	if hasName {
		c.svc.add("container_name", strNode(name))
	}
	if len(r.args) > 0 {
		c.svc.add("command", strSeq(r.args, false))
	}
	c.entrypoint()
	c.restart()
	c.ports()
	c.list("expose", "expose", true)
	c.volumes()
	c.list("tmpfs", "tmpfs", false)
	c.environment()
	c.list("env-file", "env_file", false)
	c.labels()
	c.network()
	c.text("hostname", "hostname")
	c.list("add-host", "extra_hosts", false)
	c.list("dns", "dns", false)
	c.text("workdir", "working_dir")
	c.user()
	c.list("group-add", "group_add", false)
	c.flag("privileged", "privileged")
	c.list("cap-add", "cap_add", false)
	c.list("cap-drop", "cap_drop", false)
	c.list("device", "devices", false)
	c.list("security-opt", "security_opt", false)
	c.keyValues("sysctl", "sysctls")
	c.ulimits()
	c.flag("read-only", "read_only")
	c.size("shm-size", "shm_size")
	c.size("memory", "mem_limit")
	c.cpus()
	c.flag("init", "init")
	c.text("stop-signal", "stop_signal")
	c.stopTimeout()
	c.healthcheck()
	c.logging()
	c.gpus()
	c.text("platform", "platform")
	c.pullPolicy()
	c.text("pid", "pid")
	c.text("ipc", "ipc")
	c.flag("tty", "tty")
	c.flag("interactive", "stdin_open")
	if r.bools["rm"] {
		c.warn(warnIgnored, "--rm", "Compose keeps stopped containers; --rm has no equivalent here.")
	}

	services := newMapping()
	services.add(service, c.svc.n)
	doc := newMapping()
	doc.add("services", services.n)
	if len(c.named) > 0 {
		volumes := newMapping()
		for _, v := range c.named {
			volumes.add(v, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle})
		}
		doc.add("volumes", volumes.n)
	}
	if len(c.networks) > 0 {
		networks := newMapping()
		for _, n := range c.networks {
			ext := newMapping()
			ext.add("external", boolNode(true))
			networks.add(n, ext.n)
		}
		doc.add("networks", networks.n)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc.n); err != nil {
		return ConvertResult{}, convertFailed("The compose file could not be written: " + err.Error())
	}
	if err := enc.Close(); err != nil {
		return ConvertResult{}, convertFailed("The compose file could not be written: " + err.Error())
	}
	warnings := r.warnings
	if warnings == nil {
		warnings = []ConvertWarning{}
	}
	return ConvertResult{Compose: buf.String(), Service: service, Warnings: warnings}, nil
}

// normalizeValues replaces $(pwd) in storage flags by "." (the stack folder) and warns about other command
// substitutions, which Compose cannot run.
func (r *runCommand) normalizeValues() {
	for _, flag := range []string{"volume", "mount"} {
		for i, v := range r.values[flag] {
			r.values[flag][i] = pwdPattern.ReplaceAllString(v, ".")
		}
	}
	flags := make([]string, 0, len(r.values))
	for f := range r.values {
		flags = append(flags, f)
	}
	sort.Strings(flags)
	for _, f := range flags {
		for _, v := range r.values[f] {
			if strings.Contains(v, "$(") {
				r.warn(warnValueInvalid, "--"+f, "--"+f+" contains a shell command substitution; Compose cannot run it, replace it by its value.")
				break
			}
		}
	}
	for _, a := range r.args {
		if strings.Contains(a, "$(") {
			r.warn(warnValueInvalid, "command", "The container command contains a shell command substitution; Compose cannot run it, replace it by its value.")
			break
		}
	}
}

// ---- generic mappings from one flag to one key ----

func (c *converter) text(flag, key string) {
	if v, ok := c.run.last(flag); ok && v != "" {
		c.svc.add(key, strNode(v))
	}
}

func (c *converter) list(flag, key string, quote bool) {
	if v := c.run.values[flag]; len(v) > 0 {
		c.svc.add(key, strSeq(v, quote))
	}
}

func (c *converter) flag(flag, key string) {
	if c.run.bools[flag] {
		c.svc.add(key, boolNode(true))
	}
}

func (c *converter) invalid(flag, value, why string) {
	c.warn(warnValueInvalid, "--"+flag, fmt.Sprintf("--%s %q was left out: %s.", flag, value, why))
}

// size handles --memory and --shm-size.
func (c *converter) size(flag, key string) {
	v, ok := c.run.last(flag)
	switch {
	case !ok || v == "":
	case !bytesPattern.MatchString(v):
		c.invalid(flag, v, "expected a number with an optional b, k, m or g")
	default:
		c.svc.add(key, numberNode(v))
	}
}

func (c *converter) cpus() {
	v, ok := c.run.last("cpus")
	if !ok || v == "" {
		return
	}
	if f, err := strconv.ParseFloat(v, 64); err != nil || f <= 0 || strings.ContainsAny(v, "eEnNiI+") {
		c.invalid("cpus", v, "expected a number above zero")
		return
	}
	c.svc.add("cpus", numberNode(v))
}

func (c *converter) stopTimeout() {
	v, ok := c.run.last("stop-timeout")
	if !ok || v == "" {
		return
	}
	if n, err := strconv.Atoi(v); err != nil || n < 0 {
		c.invalid("stop-timeout", v, "expected a number of seconds")
		return
	}
	c.svc.add("stop_grace_period", strNode(v+"s"))
}

func (c *converter) pullPolicy() {
	v, ok := c.run.last("pull")
	switch {
	case !ok || v == "":
	case v == "always" || v == "missing" || v == "never":
		c.svc.add("pull_policy", strNode(v))
	default:
		c.invalid("pull", v, "expected always, missing or never")
	}
}

func (c *converter) user() {
	v, ok := c.run.last("user")
	switch {
	case !ok || v == "":
	case userIDPattern.MatchString(v):
		c.svc.add("user", quotedNode(v)) // 1000:1000 would be read as a base-60 number by YAML 1.1 tools
	default:
		c.svc.add("user", strNode(v))
	}
}

func (c *converter) entrypoint() {
	v, ok := c.run.last("entrypoint")
	switch {
	case !ok:
	case v == "":
		c.svc.add("entrypoint", flowSeqNode())
	case strings.ContainsAny(v, " \t"):
		c.svc.add("entrypoint", flowSeqNode(quotedNode(v))) // docker run takes the whole value as the program
	default:
		c.svc.add("entrypoint", strNode(v))
	}
}

func (c *converter) restart() {
	v, ok := c.run.last("restart")
	switch {
	case !ok || v == "":
	case !restartPattern.MatchString(v):
		c.invalid("restart", v, "expected no, always, unless-stopped or on-failure[:max]")
	default:
		c.svc.add("restart", strNode(v))
	}
}

func (c *converter) ports() {
	items := c.run.values["publish"]
	if len(items) == 0 {
		return
	}
	for _, p := range items {
		if text, unresolved := interpolate(p, nil, nil); !unresolved {
			if _, err := parsePortSpec(text); err != nil {
				c.warn(warnValueInvalid, "--publish", fmt.Sprintf("Port %q looks wrong: %v. It was kept as typed.", p, err))
			}
		}
	}
	c.svc.add("ports", strSeq(items, true))
}

// ---- storage ----

func (c *converter) volumes() {
	seq := seqNode()
	for _, v := range c.run.values["volume"] {
		if text, unresolved := interpolate(v, nil, nil); !unresolved {
			spec, err := parseVolumeShort(text)
			switch {
			case err != nil:
				c.warn(warnValueInvalid, "--volume", fmt.Sprintf("Volume %q looks wrong: %v. It was kept as typed.", v, err))
			case spec.Kind == "named":
				c.addNamed(spec.Source)
			}
		}
		seq.Content = append(seq.Content, strNode(v))
	}
	for _, m := range c.run.values["mount"] {
		if node := c.mount(m); node != nil {
			seq.Content = append(seq.Content, node)
		}
	}
	if len(seq.Content) > 0 {
		c.svc.add("volumes", seq)
	}
}

// csvFields splits "a=b,c=d" the way docker does for --mount and --gpus: quotes protect commas.
func csvFields(s string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(s))
	r.LazyQuotes = true
	return r.Read()
}

// mount converts one --mount value to a long-syntax volume entry (nil when it cannot be converted).
func (c *converter) mount(spec string) *yaml.Node {
	fields, err := csvFields(spec)
	if err != nil {
		c.invalid("mount", spec, "the value cannot be read")
		return nil
	}
	typ, source, target, readOnly := "volume", "", "", false
	nocopy, subpath, propagation, tmpfsSize := false, "", "", ""
	for _, f := range fields {
		key, val, hasVal := strings.Cut(f, "=")
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "type":
			typ = val
		case "source", "src":
			source = val
		case "target", "destination", "dst":
			target = val
		case "readonly", "ro", "read_only":
			readOnly = !hasVal || (val != "false" && val != "0")
		case "bind-propagation":
			propagation = val
		case "volume-nocopy":
			nocopy = !hasVal || (val != "false" && val != "0")
		case "volume-subpath":
			subpath = val
		case "tmpfs-size":
			tmpfsSize = val
		case "consistency":
		default:
			c.warn(warnUnsupported, "--mount", fmt.Sprintf("--mount option %q is not converted; add it by hand.", key))
		}
	}
	if typ != "bind" && typ != "volume" && typ != "tmpfs" {
		c.invalid("mount", spec, "the type must be bind, volume or tmpfs")
		return nil
	}
	if target == "" {
		c.invalid("mount", spec, "the target is missing")
		return nil
	}
	m := newMapping()
	m.add("type", strNode(typ))
	if source != "" && typ != "tmpfs" {
		m.add("source", strNode(source))
		if typ == "volume" {
			c.addNamed(source)
		}
	}
	m.add("target", strNode(target))
	if readOnly {
		m.add("read_only", boolNode(true))
	}
	sub := newMapping()
	switch typ {
	case "bind":
		if propagation != "" {
			sub.add("propagation", strNode(propagation))
		}
	case "volume":
		if nocopy {
			sub.add("nocopy", boolNode(true))
		}
		if subpath != "" {
			sub.add("subpath", strNode(subpath))
		}
	case "tmpfs":
		if tmpfsSize != "" {
			sub.add("size", numberNode(tmpfsSize))
		}
	}
	if !sub.empty() {
		m.add(typ, sub.n)
	}
	return m.n
}

// ---- environment, labels, key=value flags ----

func (c *converter) environment() {
	env := newMapping()
	for _, e := range c.run.values["env"] {
		key, val, hasVal := strings.Cut(e, "=")
		key = strings.TrimSpace(key)
		switch {
		case key == "":
		case hasVal:
			env.set(key, strNode(val))
		default:
			env.set(key, nullNode()) // passed through from the environment Compose runs in
		}
	}
	if !env.empty() {
		c.svc.add("environment", env.n)
	}
}

func (c *converter) labels() {
	labels := newMapping()
	for _, l := range c.run.values["label"] {
		key, val, _ := strings.Cut(l, "=")
		if key = strings.TrimSpace(key); key != "" {
			labels.set(key, strNode(val))
		}
	}
	if !labels.empty() {
		c.svc.add("labels", labels.n)
	}
}

// keyValues handles --sysctl: KEY=VALUE pairs become a mapping.
func (c *converter) keyValues(flag, key string) {
	m := newMapping()
	for _, kv := range c.run.values[flag] {
		k, v, ok := strings.Cut(kv, "=")
		if k = strings.TrimSpace(k); !ok || k == "" {
			c.invalid(flag, kv, "expected KEY=VALUE")
			continue
		}
		m.set(k, strNode(v))
	}
	if !m.empty() {
		c.svc.add(key, m.n)
	}
}

func (c *converter) ulimits() {
	m := newMapping()
	for _, u := range c.run.values["ulimit"] {
		name, limits, ok := strings.Cut(u, "=")
		soft, hard, split := strings.Cut(limits, ":")
		_, errSoft := strconv.ParseInt(soft, 10, 64)
		_, errHard := strconv.ParseInt(hard, 10, 64)
		if !ok || name == "" || errSoft != nil || (split && errHard != nil) {
			c.invalid("ulimit", u, "expected NAME=SOFT[:HARD] with numbers")
			continue
		}
		if !split {
			m.set(name, numberNode(soft))
			continue
		}
		both := newMapping()
		both.add("soft", numberNode(soft))
		both.add("hard", numberNode(hard))
		m.set(name, both.n)
	}
	if !m.empty() {
		c.svc.add("ulimits", m.n)
	}
}

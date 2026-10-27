package dockerstacks

import (
	"strings"
)

// ConvertResult is the compose file for one `docker run` command.
type ConvertResult struct {
	Compose  string           `json:"compose"` // "services:\n  <name>:\n    image: ..." (2-space indent)
	Service  string           `json:"service"`
	Warnings []ConvertWarning `json:"warnings"`
}

// ConvertWarning names a flag the converter could not carry over completely.
type ConvertWarning struct {
	Code    string `json:"code"` // "flag_unsupported" | "flag_ignored" | "flag_value_invalid"
	Flag    string `json:"flag"`
	Message string `json:"message"`
}

const (
	warnUnsupported  = "flag_unsupported"
	warnIgnored      = "flag_ignored"
	warnValueInvalid = "flag_value_invalid"
)

// flag kinds of `docker run`
const (
	kindUnknown = iota
	kindBool
	kindValue
)

// shortFlags maps the one-letter flags of `docker run` to their long names.
var shortFlags = map[byte]string{
	'a': "attach", 'c': "cpu-shares", 'd': "detach", 'e': "env", 'h': "hostname", 'i': "interactive", 'l': "label",
	'm': "memory", 'p': "publish", 'P': "publish-all", 'q': "quiet", 't': "tty", 'u': "user", 'v': "volume", 'w': "workdir",
}

var flagAliases = map[string]string{"net": "network"}

// Flags the converter understands.
var (
	supportedBoolFlags  = flagSet("detach", "rm", "privileged", "init", "read-only", "interactive", "tty", "no-healthcheck")
	supportedValueFlags = flagSet("name", "publish", "expose", "volume", "mount", "env", "env-file", "restart", "network",
		"network-alias", "hostname", "workdir", "user", "label", "cap-add", "cap-drop", "device", "entrypoint", "memory",
		"cpus", "shm-size", "ulimit", "security-opt", "tmpfs", "sysctl", "add-host", "dns", "log-driver", "log-opt",
		"platform", "pull", "stop-signal", "stop-timeout", "health-cmd", "health-interval", "health-timeout",
		"health-retries", "health-start-period", "gpus", "pid", "ipc", "group-add")
	// Flags of `docker run` that are recognised, so that their value is skipped, but have no conversion.
	otherBoolFlags  = flagSet("publish-all", "oom-kill-disable", "quiet", "sig-proxy", "disable-content-trust", "use-api-socket")
	otherValueFlags = flagSet("annotation", "attach", "blkio-weight", "blkio-weight-device", "cgroup-parent", "cgroupns", "cidfile",
		"cpu-count", "cpu-percent", "cpu-period", "cpu-quota", "cpu-rt-period", "cpu-rt-runtime", "cpu-shares", "cpuset-cpus",
		"cpuset-mems", "detach-keys", "device-cgroup-rule", "device-read-bps", "device-read-iops", "device-write-bps",
		"device-write-iops", "dns-option", "dns-search", "domainname", "health-start-interval", "io-maxbandwidth",
		"io-maxiops", "ip", "ip6", "isolation", "kernel-memory", "label-file", "link", "link-local-ip", "mac-address",
		"memory-reservation", "memory-swap", "memory-swappiness", "oom-score-adj", "pids-limit", "runtime", "storage-opt",
		"userns", "uts", "volume-driver", "volumes-from")
	// Flags that may be given more than once; every other flag keeps its last value.
	repeatableFlags = flagSet("publish", "expose", "volume", "mount", "env", "env-file", "network", "network-alias", "label",
		"cap-add", "cap-drop", "device", "ulimit", "security-opt", "tmpfs", "sysctl", "add-host", "dns", "log-opt", "group-add")
)

func flagSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

func flagKind(name string) int {
	switch {
	case supportedBoolFlags[name], otherBoolFlags[name]:
		return kindBool
	case supportedValueFlags[name], otherValueFlags[name]:
		return kindValue
	}
	return kindUnknown
}

// runCommand is the parsed `docker run` command line.
type runCommand struct {
	image    string
	args     []string            // everything after the image
	values   map[string][]string // flag -> values in command order (one entry for non-repeatable flags)
	bools    map[string]bool
	warnings []ConvertWarning
}

func (r *runCommand) warn(code, flag, message string) {
	r.warnings = append(r.warnings, ConvertWarning{Code: code, Flag: flag, Message: message})
}

func convertFailed(message string) error { return &Error{Code: CodeConvertFailed, Message: message} }

// ConvertDockerRun turns one `docker run ...` command (also `docker container run`, with line continuations and
// quotes) into compose YAML. Errors carry CodeConvertFailed: not a docker run command, no image, unbalanced quotes.
// Flags that have no Compose equivalent produce warnings instead of failing.
func ConvertDockerRun(command string) (ConvertResult, error) {
	words, err := splitCommand(command)
	if err != nil {
		return ConvertResult{}, convertFailed("The command has an " + err.Error() + ".")
	}
	for len(words) > 0 && (words[0] == "$" || words[0] == "sudo") { // a copied prompt, or sudo
		words = words[1:]
	}
	if len(words) == 0 {
		return ConvertResult{}, convertFailed("The command is empty.")
	}
	i := 1
	if words[0] == "docker" && i < len(words) && words[i] == "container" {
		i++
	}
	if words[0] != "docker" || i >= len(words) || words[i] != "run" {
		return ConvertResult{}, convertFailed("This is not a docker run command.")
	}
	run := &runCommand{values: map[string][]string{}, bools: map[string]bool{}}
	run.parse(words[i+1:])
	if run.image == "" {
		return ConvertResult{}, convertFailed("The command names no image.")
	}
	return run.build()
}

// parse reads flags up to the image; everything after the image belongs to the container's command.
func (r *runCommand) parse(words []string) {
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "--":
			if i+1 < len(words) {
				r.image, r.args = words[i+1], words[i+2:]
			}
			return
		case strings.HasPrefix(w, "--"):
			name, value, hasValue := strings.Cut(w[2:], "=")
			if alias, ok := flagAliases[name]; ok {
				name = alias
			}
			switch flagKind(name) {
			case kindBool:
				r.setBool(name, !hasValue || value != "false")
			case kindValue:
				if !hasValue {
					if i+1 >= len(words) {
						r.warn(warnValueInvalid, "--"+name, "--"+name+" needs a value.")
						continue
					}
					i++
					value = words[i]
				}
				r.setValue(name, value)
			default:
				r.warn(warnUnsupported, "--"+name, "--"+name+" is not a docker run flag the converter knows; it was left out.")
			}
		case strings.HasPrefix(w, "-") && len(w) > 1:
			i = r.parseShort(words, i)
		default:
			r.image, r.args = w, words[i+1:]
			return
		}
	}
}

// parseShort handles a cluster of one-letter flags such as -it, -dp 80:80 or -p80:80 and returns the index of the
// last word it consumed.
func (r *runCommand) parseShort(words []string, i int) int {
	w := words[i]
	for j := 1; j < len(w); j++ {
		name, ok := shortFlags[w[j]]
		if !ok {
			r.warn(warnUnsupported, "-"+string(w[j]), "-"+string(w[j])+" is not a docker run flag the converter knows; it was left out.")
			return i
		}
		if flagKind(name) == kindBool {
			r.setBool(name, true)
			continue
		}
		value := strings.TrimPrefix(w[j+1:], "=")
		if value == "" {
			if i+1 >= len(words) {
				r.warn(warnValueInvalid, "--"+name, "--"+name+" needs a value.")
				return i
			}
			i++
			value = words[i]
		}
		r.setValue(name, value)
		return i
	}
	return i
}

func (r *runCommand) setBool(name string, on bool) {
	switch {
	case !on:
		delete(r.bools, name)
	case supportedBoolFlags[name]:
		r.bools[name] = true
	default:
		r.warn(warnUnsupported, "--"+name, "--"+name+" has no Compose equivalent; it was left out.")
	}
}

func (r *runCommand) setValue(name, value string) {
	switch {
	case supportedValueFlags[name] && repeatableFlags[name]:
		r.values[name] = append(r.values[name], value)
	case supportedValueFlags[name]:
		r.values[name] = []string{value}
	default:
		r.warn(warnUnsupported, "--"+name, "--"+name+" is not converted yet; add the matching Compose key by hand.")
	}
}

func (r *runCommand) last(name string) (string, bool) {
	v := r.values[name]
	if len(v) == 0 {
		return "", false
	}
	return v[len(v)-1], true
}

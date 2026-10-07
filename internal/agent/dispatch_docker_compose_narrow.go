package agent

import (
	"encoding/json"
	"strings"

	"aurago/internal/tools"
)

// dockerComposeNarrowFlags are, per subcommand, the flags whose meaning is
// known well enough to narrow the host-access check to the named services;
// true marks a flag whose value is the next argument. Any other flag keeps the
// whole-file check (fail-safe).
var dockerComposeNarrowFlags = map[string]map[string]bool{
	"up": {"-d": false, "--detach": false, "--build": false, "--no-build": false, "--force-recreate": false,
		"--no-recreate": false, "--always-recreate-deps": false, "--no-deps": false, "--remove-orphans": false,
		"-V": false, "--renew-anon-volumes": false, "--quiet-pull": false, "--wait": false, "--no-start": false,
		"--no-color": false, "--no-log-prefix": false, "--timestamps": false, "-y": false, "--yes": false,
		"--pull": true, "-t": true, "--timeout": true, "--wait-timeout": true},
	"create": {"--build": false, "--no-build": false, "--force-recreate": false, "--no-recreate": false,
		"--remove-orphans": false, "--quiet-pull": false, "-y": false, "--yes": false, "--pull": true},
	"build": {"--no-cache": false, "--pull": false, "-q": false, "--quiet": false, "--push": false,
		"--with-dependencies": false, "--build-arg": true, "--builder": true, "-m": true, "--memory": true,
		"--progress": true, "--provenance": true, "--sbom": true, "--ssh": true},
	"pull": {"--ignore-buildable": false, "--ignore-pull-failures": false, "--include-deps": false,
		"-q": false, "--quiet": false, "--policy": true},
	"down":    {"--remove-orphans": false, "-v": false, "--volumes": false, "--rmi": true, "-t": true, "--timeout": true},
	"stop":    {"-t": true, "--timeout": true},
	"start":   {},
	"restart": {"--no-deps": false, "-t": true, "--timeout": true},
	"rm":      {"-f": false, "--force": false, "-s": false, "--stop": false, "-v": false},
}

// dockerComposeNarrowableNames returns the services command names when every
// flag of it is in dockerComposeNarrowFlags and it names at least one.
func dockerComposeNarrowableNames(command string) ([]string, bool) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, false
	}
	flags, ok := dockerComposeNarrowFlags[parts[0]]
	if !ok {
		return nil, false
	}
	var names []string
	positional := false
	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		switch {
		case arg == "-":
			return nil, false
		case positional || !strings.HasPrefix(arg, "-"):
			names = append(names, arg)
			continue
		case arg == "--":
			positional = true
			continue
		}
		flag, _, inline := strings.Cut(arg, "=")
		if strings.HasPrefix(flag, "--") || len(flag) == 2 {
			takesValue, known := flags[flag]
			if !known {
				return nil, false
			}
			if takesValue && !inline {
				if i+1 >= len(parts) {
					return nil, false
				}
				i++
			}
			continue
		}
		if inline {
			return nil, false
		}
		for j := 1; j < len(flag); j++ { // a short cluster such as -dV or -dt 5
			takesValue, known := flags["-"+string(flag[j])]
			if !known {
				return nil, false
			}
			if takesValue {
				if j == len(flag)-1 {
					if i+1 >= len(parts) {
						return nil, false
					}
					i++
				}
				break // the rest of the cluster is the value
			}
		}
	}
	return names, len(names) > 0
}

// serviceRaw returns a service's raw resolved JSON: all-profiles first, else
// the default resolution.
func (p *dockerComposePreflight) serviceRaw(name string) json.RawMessage {
	if raw := p.allProfilesRaw.Services[name]; len(raw) > 0 {
		return raw
	}
	return p.defaultRaw.Services[name]
}

// namedServiceClosure returns the services of model that a command naming
// services acts on, or false when narrowing is not safe: an unknown flag, no
// or an unknown service name, or a service of the model whose raw resolved
// JSON is missing or unreadable. The closure holds the named services and
// everything they need (dockerComposeServiceReferences, transitively), plus,
// transitively, the dependents the subcommand acts on: every dependent for
// stop, down, rm and restart; for up the dependents whose depends_on entry sets
// restart: true (recreating a service restarts them); none for create, start,
// build and pull. It is a superset of what Compose acts on.
func (p *dockerComposePreflight) namedServiceClosure(model tools.DockerComposeModel, command string) (map[string]bool, bool) {
	names, ok := dockerComposeNarrowableNames(command)
	if !ok {
		return nil, false
	}
	for _, name := range names {
		if _, known := model.Services[name]; !known {
			return nil, false
		}
	}
	references := make(map[string][]string, len(model.Services))
	for name, service := range model.Services {
		raw := p.serviceRaw(name)
		if len(raw) == 0 {
			return nil, false
		}
		refs, ok := dockerComposeServiceReferences(raw, service)
		if !ok {
			return nil, false
		}
		references[name] = refs
	}
	closure := map[string]bool{}
	for queue := append([]string(nil), names...); len(queue) > 0; queue = queue[1:] {
		name := queue[0]
		if _, known := model.Services[name]; closure[name] || !known {
			continue // a dependency the model lacks is not evaluated anyway
		}
		closure[name] = true
		queue = append(queue, references[name]...)
	}
	dependents := map[string][]string{}
	switch strings.Fields(command)[0] {
	case "stop", "down", "rm", "restart":
		// They act on every service that depends on a named one.
		for name, refs := range references {
			for _, ref := range refs {
				dependents[ref] = append(dependents[ref], name)
			}
		}
	case "up":
		// Recreating a service restarts the dependents whose depends_on
		// entry sets restart: true.
		for name := range model.Services {
			for _, ref := range dockerComposeRestartDependencies(p.serviceRaw(name)) {
				dependents[ref] = append(dependents[ref], name)
			}
		}
	default: // create, start, build, pull: no dependents
		return closure, true
	}
	queue := make([]string, 0, len(closure))
	for name := range closure {
		queue = append(queue, name)
	}
	for ; len(queue) > 0; queue = queue[1:] {
		for _, dependent := range dependents[queue[0]] {
			if !closure[dependent] {
				closure[dependent] = true
				queue = append(queue, dependent)
			}
		}
	}
	return closure, true
}

// dockerComposeRestartDependencies returns the depends_on entries of a
// service's raw JSON that set restart: true: Compose restarts the service when
// one of them is recreated.
func dockerComposeRestartDependencies(raw json.RawMessage) []string {
	var refs struct {
		DependsOn map[string]struct {
			Restart bool `json:"restart"`
		} `json:"depends_on"`
	}
	if json.Unmarshal(raw, &refs) != nil {
		return nil
	}
	var names []string
	for _, name := range tools.SortedDockerComposeKeys(refs.DependsOn) {
		if refs.DependsOn[name].Restart {
			names = append(names, name)
		}
	}
	return names
}

// dockerComposeServiceReferences returns every service Compose starts with
// service: depends_on (required or not), service: build contexts, links,
// volumes_from (not container:) and network_mode/ipc/pid service: references.
func dockerComposeServiceReferences(raw json.RawMessage, service tools.DockerComposeService) ([]string, bool) {
	var refs struct {
		Links       []string `json:"links"`
		VolumesFrom []string `json:"volumes_from"`
		NetworkMode string   `json:"network_mode"`
		Ipc         string   `json:"ipc"`
		Pid         string   `json:"pid"`
	}
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil, false
	}
	names := dockerComposeServiceDependencies(raw, service)
	for _, link := range refs.Links {
		name, _, _ := strings.Cut(strings.TrimSpace(link), ":")
		names = append(names, name)
	}
	for _, from := range refs.VolumesFrom {
		from = strings.TrimSpace(from)
		if strings.HasPrefix(from, "container:") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(from, "service:"), ":")
		names = append(names, name)
	}
	for _, mode := range []string{refs.NetworkMode, refs.Ipc, refs.Pid} {
		if name, ok := strings.CutPrefix(strings.TrimSpace(mode), "service:"); ok {
			names = append(names, strings.TrimSpace(name))
		}
	}
	return names, true
}

// dockerComposeKeepClosureViolations drops host-tier violations of services
// outside the closure; the always tier and whole-file violations stay.
func dockerComposeKeepClosureViolations(violations []tools.DockerComposeViolation, closure map[string]bool) []tools.DockerComposeViolation {
	kept := make([]tools.DockerComposeViolation, 0, len(violations))
	for _, violation := range violations {
		if violation.Always || violation.Service == "" || closure[violation.Service] {
			kept = append(kept, violation)
		}
	}
	return kept
}

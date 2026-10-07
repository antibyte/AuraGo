package agent

import (
	"context"
	"strings"
)

// dockerComposeInputChangedCode marks a Compose call whose input changed
// between the policy check and the run.
const dockerComposeInputChangedCode = "docker_compose_input_changed"

// inputChanged narrows the check-then-run window (controller decision
// 2026-10-06, used only while docker.allow_host_access is off): right before
// the run it reads the Compose file again and repeats every resolution the
// checks read (default, all-profiles and named rounds, with the same Docker
// config and environment), and refuses the call when the file text or any
// output differs, or a resolution that succeeded now fails or the other way
// round. A resolution that failed at check time because the deadline ran out
// is not repeated: it proves nothing about the input. Included files and env
// files show up in the resolved output. It
// cannot close the window against a process that rewrites a file between
// this repeat and the CLI start.
func (p *dockerComposePreflight) inputChanged(ctx context.Context) string {
	if p == nil {
		return ""
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, dockerComposePreflightTimeout)
	defer cancel()
	raw, err := readDockerComposeFile(p.file)
	if err != nil || strings.ToLower(string(raw)) != p.raw {
		return dockerComposeInputChangedDenial()
	}
	for _, resolution := range p.resolutions {
		if resolution.deadline {
			// It ran out of the check's shared deadline; the checks used the
			// fallback that the other recorded resolutions cover.
			continue
		}
		output, err := resolveDockerComposeConfig(ctx, p.dockerCfg, p.file, resolution.opts)
		if (err != nil) != resolution.failed || (err == nil && output != resolution.output) {
			return dockerComposeInputChangedDenial()
		}
	}
	return ""
}

func dockerComposeInputChangedDenial() string {
	return dockerAgentError(dockerComposeInputChangedCode,
		"The Compose file, or a file it includes or reads (env files), changed between AuraGo's check and the run, so nothing was run. Run the command again once the stack's files are no longer being changed.")
}

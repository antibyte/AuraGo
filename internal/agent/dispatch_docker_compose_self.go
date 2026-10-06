package agent

import (
	"fmt"

	"aurago/internal/tools"
)

// dockerComposeSelfProjectCommands act on the containers of a project. Run on
// AuraGo's own Compose project they would stop, replace, remove or read
// AuraGo's containers (down --remove-orphans even when the file names none of
// them). config, convert, ps, images, port, ls, version, events, build and
// pull stay allowed.
var dockerComposeSelfProjectCommands = map[string]bool{
	"up": true, "create": true, "start": true, "stop": true, "restart": true, "down": true,
	"rm": true, "kill": true, "pause": true, "unpause": true, "logs": true, "top": true,
}

// dockerComposeSelfProjectDenial refuses a command that acts on the Compose
// project AuraGo's own container belongs to. Without a self identity (native
// installs) it never refuses.
func dockerComposeSelfProjectDenial(project, subcommand string, self tools.DockerSelfIdentity) string {
	if !dockerComposeSelfProjectCommands[subcommand] || !self.OwnsComposeProject(project) {
		return ""
	}
	return dockerAgentError("docker_managed_aurago_resource", fmt.Sprintf(
		"This Compose file resolves to the project name %q, which is the Compose project AuraGo itself runs in, so `%s` would act on AuraGo's own containers. Nothing was run. Give the stack its own top-level name: or move it into a folder with another name.", project, subcommand))
}

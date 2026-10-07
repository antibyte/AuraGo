package agent

import (
	"fmt"
	"strings"

	"aurago/internal/config"
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

// dockerComposeAuraGoStateVolume returns the first volume of the models that
// holds AuraGo's data directory, or "" (always "" on native installs).
func dockerComposeAuraGoStateVolume(cfg *config.Config, self tools.DockerSelfIdentity, models ...tools.DockerComposeModel) string {
	if cfg == nil || !cfg.Runtime.IsDocker {
		return ""
	}
	for _, model := range models {
		for _, name := range tools.DockerComposeModelVolumeNames(model) {
			if tools.IsAuraGoStateVolume(name, true, self) {
				return name
			}
		}
	}
	return ""
}

// dockerManagedSidecarNameMessage explains a refused managed sidecar name.
const dockerManagedSidecarNameMessage = "The container name %q belongs to an AuraGo-managed sidecar (for example Gotenberg, Ollama, Piper, Supertonic, Ansible, browser automation, go2rtc, Space Agent, Manifest, OmniRoute, Dograh or cloudflared). AuraGo finds that sidecar by its name and may reuse a container of that name, so the agent cannot create one while Docker host access is off. Choose another name."

// dockerReservedSidecarNames are the managed sidecar names the agent may not
// create: every one of them while the config flag docker.allow_host_access is
// false (controller decision 2026-10-06), none otherwise, so grandfathered
// installs are unchanged.
func dockerReservedSidecarNames(cfg *config.Config) []string {
	if cfg == nil || cfg.Docker.AllowHostAccess {
		return nil
	}
	return tools.ManagedSidecarContainerNames(cfg)
}

func dockerNameReserved(name string, reserved []string) bool {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	if name == "" {
		return false
	}
	for _, candidate := range reserved {
		if strings.EqualFold(name, strings.TrimPrefix(strings.TrimSpace(candidate), "/")) {
			return true
		}
	}
	return false
}

// dockerComposeReservedContainerName returns the first service container_name
// of model that matches reserved, or "". Only container_name counts: stacks
// that join a sidecar's network or other namespaces stay as they are.
func dockerComposeReservedContainerName(model tools.DockerComposeModel, reserved func(string) bool) string {
	for _, name := range tools.SortedDockerComposeKeys(model.Services) {
		if containerName := strings.TrimSpace(model.Services[name].ContainerName); containerName != "" && reserved(containerName) {
			return containerName
		}
	}
	return ""
}

package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"aurago/internal/acestep"
	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// dockerComposePreflight is what the agent Compose policy needs for one call,
// loaded once: the jailed file, its raw text, Compose's resolved model text
// (stdout only) and the parsed model. Texts are lower-cased.
type dockerComposePreflight struct {
	file     string
	raw      string
	resolved string
	model    tools.DockerComposeModel
}

// loadDockerComposePreflight resolves the Compose file exactly once. Every
// failure is returned so the caller blocks the call, as before this preflight.
func loadDockerComposePreflight(cfg tools.DockerConfig, file string) (*dockerComposePreflight, error) {
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		return nil, fmt.Errorf("the Compose policy needs a configured workspace directory")
	}
	composeFile, err := tools.ResolveDockerComposeFile(cfg, file)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	resolved, err := resolveDockerComposeConfig(cfg, composeFile)
	if err != nil {
		return nil, err
	}
	model, err := tools.ParseDockerComposeModel(resolved)
	if err != nil {
		return nil, err
	}
	return &dockerComposePreflight{
		file:     composeFile,
		raw:      strings.ToLower(string(raw)),
		resolved: strings.ToLower(resolved),
		model:    model,
	}, nil
}

// protectedOwner keeps every text match that blocked a call before the model
// existed (LocalLLM on raw and resolved text, Garage and Homepage on raw text)
// and adds the structured model checks. Garage and Homepage do not match the
// resolved text: resolved JSON carries project and path names that must not
// block unrelated stacks.
func (p *dockerComposePreflight) protectedOwner() string {
	if dockerComposePayloadReferencesProtectedLocalLLM(p.raw) || dockerComposePayloadReferencesProtectedLocalLLM(p.resolved) {
		return dockerutil.LocalLLMOwner
	}
	if dockerComposeTextReferencesGarage(p.raw) {
		return dockerutil.BoringGarageOwner
	}
	if dockerComposeTextReferencesHomepage(p.raw) {
		return dockerutil.HomepageOwner
	}
	return tools.DockerComposeModelOwner(p.model)
}

func dockerComposeTextReferencesGarage(lower string) bool {
	for _, token := range []string{
		dockerutil.BoringGarageContainerName,
		"boring-garage",
		"data/sidecars/garage",
		`aurago.managed: boring-garage`,
		`"aurago.managed":"boring-garage"`,
		`aurago.managed=boring-garage`,
	} {
		if strings.Contains(lower, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func dockerComposeTextReferencesHomepage(lower string) bool {
	return strings.Contains(lower, dockerutil.HomepageContainerName) ||
		strings.Contains(lower, dockerutil.HomepageWebContainerName) ||
		strings.Contains(lower, dockerutil.HomepageImageRepository)
}

// dockerComposeReferencesProtectedLocalLLMVolume is the fail-closed LocalLLM
// predicate pinned by TestManagedLocalLLMComposeProtectionIsFailClosed.
func dockerComposeReferencesProtectedLocalLLMVolume(cfg tools.DockerConfig, file string) bool {
	preflight, err := loadDockerComposePreflight(cfg, file)
	if err != nil {
		return true
	}
	owner := preflight.protectedOwner()
	return owner == dockerutil.LocalLLMOwner || owner == acestep.Owner
}

// dockerComposePolicy runs before every agent Compose call. It returns a Tool
// Output envelope that blocks the call, or "" to continue.
func dockerComposePolicy(ctx context.Context, cfg *config.Config, dockerCfg tools.DockerConfig, req dockerArgs) string {
	preflight, err := loadDockerComposePreflight(dockerCfg, req.File)
	if err != nil {
		return dockerAgentError("docker_compose_preflight_failed",
			"Docker Compose could not resolve this file, so nothing was run: "+Truncate(err.Error(), 600)+
				". Fix the Compose file (for example a missing env_file or invalid YAML) or install the Docker Compose plugin.")
	}
	return dockerComposeOwnerDenial(preflight.protectedOwner())
}

func dockerComposeOwnerDenial(owner string) string {
	switch owner {
	case dockerutil.LocalLLMOwner, acestep.Owner:
		return `Tool Output: {"status":"error","message":"Docker Compose access to AuraGo's managed local LLM volumes is blocked."}`
	case dockerutil.BoringGarageOwner:
		return `Tool Output: {"status":"error","message":"Docker Compose access to AuraGo's managed Boring Computers Garage is blocked."}`
	case dockerutil.HomepageOwner:
		return dockerAgentError("docker_managed_homepage_resource", "Docker Compose access to AuraGo-managed homepage resources is blocked. Use homepage_project, homepage_file, or homepage_deploy.")
	case dockerutil.AppOwner:
		return dockerAgentError("docker_managed_aurago_resource", "Docker Compose access to AuraGo's application container is blocked.")
	}
	return ""
}

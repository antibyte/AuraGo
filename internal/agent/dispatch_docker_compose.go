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
// loaded once: the jailed file, its raw text, Compose's default resolved model
// (stdout only; it is the validity gate) and the all-profiles model. Texts are
// lower-cased.
type dockerComposePreflight struct {
	file     string
	raw      string
	resolved string
	model    tools.DockerComposeModel
	// allProfilesModel is `--profile * config --no-env-resolution`: every
	// service, including inactive profiles that `up <service>` activates, with
	// env_file entries kept as paths. Ownership checks read it together with
	// model, and later policies evaluate it. It is nil when this Compose cannot
	// produce it (allProfilesErr); the call then falls back to model.
	allProfilesModel    *tools.DockerComposeModel
	allProfilesResolved string
	allProfilesErr      error
}

// loadDockerComposePreflight resolves the Compose file once per variant. Every
// failure of the default resolution is returned so the caller blocks the call,
// as before this preflight; a failed all-profiles resolution is recorded only.
// Without a configured workspace the process working directory is the jail,
// exactly as the Compose checks before this preflight confined the file.
func loadDockerComposePreflight(ctx context.Context, cfg tools.DockerConfig, file string) (*dockerComposePreflight, error) {
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		workdir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("determine working directory for the compose file: %w", err)
		}
		cfg.WorkspaceDir = workdir
	}
	composeFile, err := tools.ResolveDockerComposeFile(cfg, file)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	resolved, err := resolveDockerComposeConfig(ctx, cfg, composeFile, tools.DockerComposeConfigOptions{})
	if err != nil {
		return nil, err
	}
	model, err := tools.ParseDockerComposeModel(resolved)
	if err != nil {
		return nil, err
	}
	preflight := &dockerComposePreflight{
		file:     composeFile,
		raw:      strings.ToLower(string(raw)),
		resolved: strings.ToLower(resolved),
		model:    model,
	}
	allResolved, err := resolveDockerComposeConfig(ctx, cfg, composeFile, tools.DockerComposeConfigOptions{AllProfiles: true})
	if err == nil {
		var allModel tools.DockerComposeModel
		if allModel, err = tools.ParseDockerComposeModel(allResolved); err == nil {
			preflight.allProfilesModel = &allModel
			preflight.allProfilesResolved = strings.ToLower(allResolved)
		}
	}
	preflight.allProfilesErr = err
	if ctx != nil && ctx.Err() != nil {
		return nil, fmt.Errorf("resolve Compose config: %w", ctx.Err())
	}
	return preflight, nil
}

// protectedOwner keeps every text match that blocked a call before the model
// existed (LocalLLM on raw and resolved text, Garage and Homepage on raw text)
// and adds the structured model checks, over the default and the all-profiles
// model. Garage and Homepage do not match the resolved text: resolved JSON
// carries project and path names that must not block unrelated stacks.
func (p *dockerComposePreflight) protectedOwner() string {
	if dockerComposePayloadReferencesProtectedLocalLLM(p.raw) ||
		dockerComposePayloadReferencesProtectedLocalLLM(p.resolved) ||
		(p.allProfilesModel != nil && dockerComposePayloadReferencesProtectedLocalLLM(p.allProfilesResolved)) {
		return dockerutil.LocalLLMOwner
	}
	if dockerComposeTextReferencesGarage(p.raw) {
		return dockerutil.BoringGarageOwner
	}
	if dockerComposeTextReferencesHomepage(p.raw) {
		return dockerutil.HomepageOwner
	}
	if owner := tools.DockerComposeModelOwner(p.model); owner != "" {
		return owner
	}
	if p.allProfilesModel != nil {
		return tools.DockerComposeModelOwner(*p.allProfilesModel)
	}
	return ""
}

// unverifiedProfileService returns the first service that command starts or
// creates by name although the default model does not contain it, when no
// all-profiles model exists to check it. Naming a service activates its
// profiles, so its definition was never seen by the ownership checks.
func (p *dockerComposePreflight) unverifiedProfileService(command string) string {
	if p.allProfilesModel != nil {
		return ""
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "up", "create", "start", "restart":
	default:
		return ""
	}
	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		if strings.HasPrefix(arg, "-") {
			if !strings.Contains(arg, "=") && dockerComposeValueFlags[arg] {
				i++
			}
			continue
		}
		if _, ok := p.model.Services[arg]; !ok {
			return arg
		}
	}
	return ""
}

// dockerComposeValueFlags are the up/create/start/restart flags whose value is
// a separate argument, so it is not mistaken for a service name.
var dockerComposeValueFlags = map[string]bool{
	"--attach": true, "--exit-code-from": true, "--no-attach": true, "--pull": true,
	"--scale": true, "-t": true, "--timeout": true, "--wait-timeout": true,
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
	preflight, err := loadDockerComposePreflight(context.Background(), cfg, file)
	if err != nil {
		return true
	}
	owner := preflight.protectedOwner()
	return owner == dockerutil.LocalLLMOwner || owner == acestep.Owner
}

// dockerComposePolicy runs before every agent Compose call. It returns a Tool
// Output envelope that blocks the call, or "" to continue.
func dockerComposePolicy(ctx context.Context, cfg *config.Config, dockerCfg tools.DockerConfig, req dockerArgs) string {
	preflight, err := loadDockerComposePreflight(ctx, dockerCfg, req.File)
	if err != nil {
		return dockerAgentError("docker_compose_preflight_failed",
			"Docker Compose could not resolve this file, so nothing was run: "+Truncate(err.Error(), 600)+
				". Fix the Compose file (for example a missing env_file or invalid YAML) or install the Docker Compose plugin.")
	}
	if denied := dockerComposeOwnerDenial(preflight.protectedOwner()); denied != "" {
		return denied
	}
	if service := preflight.unverifiedProfileService(req.Command); service != "" {
		return dockerAgentError("docker_compose_profile_service_unverified", fmt.Sprintf(
			"Service %q is not part of the default Compose profiles, and this Docker Compose version cannot resolve services of inactive profiles for AuraGo's ownership check, so nothing was run. Update the Docker Compose plugin (it needs `config --profile '*' --no-env-resolution`), or start only services without a profile.", service))
	}
	return ""
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

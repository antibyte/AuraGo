package desktopstore

import (
	"context"
	"embed"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const commandCodeAppID = "commandcode"

// commandCodeCLIProbe is the command that proves the CLI in an image starts.
// command-code aborts on start when its dependency tree cannot be resolved, so
// a version check is enough to catch a broken image before it is installed.
var commandCodeCLIProbe = []string{"command-code", "--version"}

//go:embed commandcode_assets/Dockerfile commandcode_assets/commandcode-preview.js commandcode_assets/commandcode-entrypoint.sh commandcode_assets/preview-port.sh commandcode_assets/commandcode-preview.md
var commandCodeImageAssets embed.FS

func (s *Service) ensureCatalogImage(ctx context.Context, entry CatalogEntry) error {
	if entry.ID != commandCodeAppID {
		return s.requireDocker().PullImage(ctx, entry.Image)
	}
	if err := s.ensureCommandCodeBaseImage(ctx, entry); err != nil {
		return err
	}
	return s.refreshCommandCodeCLI(ctx, entry.Image)
}

// ensureCommandCodeBaseImage leaves a CommandCode image whose CLI starts under
// entry.Image: the published one when it works, otherwise a bundled build.
func (s *Service) ensureCommandCodeBaseImage(ctx context.Context, entry CatalogEntry) error {
	// A published CommandCode image can carry a dependency tree whose CLI
	// aborts on start. Installing it would hide the failure until the user
	// opens the terminal, so the image is probed before it is used and rebuilt
	// from the bundled Dockerfile when the probe fails. That keeps the Store
	// working against a stale or broken published image.
	pullErr := s.requireDocker().PullImage(ctx, entry.Image)
	if pullErr == nil {
		if err := s.probeCommandCodeImage(ctx, entry.Image); err == nil {
			return nil
		} else {
			s.logger().Warn("CommandCode image does not start the CLI, rebuilding it",
				"image", entry.Image, "error", err)
			pullErr = err
		}
	}
	if err := s.buildBundledCommandCodeImage(ctx, entry.Image, pullErr); err != nil {
		return err
	}
	if err := s.probeCommandCodeImage(ctx, entry.Image); err != nil {
		return fmt.Errorf("CommandCode image %s cannot start the CLI: %w", entry.Image, err)
	}
	return nil
}

// refreshCommandCodeCLI installs the newest command-code release on top of the
// image. The published image is only rebuilt on release tags and a bundled
// build reuses Docker's layer cache, so either can carry an old CLI; an install
// or update must still bring the newest one. The layer also hands the global
// npm tree to the image user, so the CLI's own background updater can replace
// itself between Store updates. A failed refresh (offline, npm outage, a new
// release that does not start) keeps the working base image: the classic
// builder only moves the tag when every step, including the start gate,
// succeeded.
func (s *Service) refreshCommandCodeCLI(ctx context.Context, image string) error {
	builder, ok := s.requireDocker().(DockerImageBuilder)
	if !ok {
		return nil
	}
	buildArgs := map[string]string{
		// A new value per run invalidates the cached npm layer, so every
		// refresh asks the registry for the newest release.
		commandCodeRefreshArg: strconv.FormatInt(time.Now().UnixNano(), 10),
	}
	if err := builder.BuildImage(ctx, image, "Dockerfile", commandCodeRefreshDockerfile(image), nil, buildArgs); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.logger().Warn("CommandCode CLI refresh failed, keeping the image's CLI",
			"image", image, "error", err)
		return nil
	}
	if err := s.probeCommandCodeImage(ctx, image); err != nil {
		return fmt.Errorf("CommandCode image %s cannot start the CLI after the update: %w", image, err)
	}
	return nil
}

const commandCodeRefreshArg = "COMMAND_CODE_REFRESH"

// commandCodeRefreshDockerfile builds the refresh layer on the local image.
// The Engine API build uses the classic builder without pull, so FROM resolves
// the image that was just pulled or built under the same tag.
func commandCodeRefreshDockerfile(image string) []byte {
	return []byte("FROM " + image + "\n" +
		"USER root\n" +
		"ARG " + commandCodeRefreshArg + "=\n" +
		"RUN npm install -g --no-audit --no-fund --no-update-notifier \"command-code@latest\" \\\n" +
		"    && chown developer:developer /usr/local/bin /usr/local/lib/node_modules \\\n" +
		"    && chown -R developer:developer /usr/local/lib/node_modules/command-code\n" +
		"USER developer\n" +
		"RUN command-code --version\n")
}

// probeCommandCodeImage runs the CLI in the image and reports whether it
// starts. Without a prober the image is trusted.
func (s *Service) probeCommandCodeImage(ctx context.Context, image string) error {
	prober, ok := s.requireDocker().(DockerImageProber)
	if !ok {
		return nil
	}
	exitCode, output, err := prober.RunImageCommand(ctx, image, commandCodeCLIProbe, imageProbeTimeout)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("%s exited with %d: %s", strings.Join(commandCodeCLIProbe, " "), exitCode, strings.TrimSpace(output))
	}
	return nil
}

func (s *Service) buildBundledCommandCodeImage(ctx context.Context, image string, cause error) error {
	builder, ok := s.requireDocker().(DockerImageBuilder)
	if !ok {
		return fmt.Errorf("image %s is unusable (%v) and bundled build is unavailable: %w", image, cause, cause)
	}
	dockerfile, files, err := commandCodeBuildContext()
	if err != nil {
		return fmt.Errorf("load bundled CommandCode build context: %w", err)
	}
	if err := builder.BuildImage(ctx, image, "Dockerfile", dockerfile, files, map[string]string{"TARGETARCH": commandCodeDockerTargetArch()}); err != nil {
		return fmt.Errorf("image %s is unusable (%v); build bundled CommandCode image: %w", image, cause, err)
	}
	return nil
}

func commandCodeBuildContext() ([]byte, map[string][]byte, error) {
	dockerfile, err := commandCodeImageAssets.ReadFile("commandcode_assets/Dockerfile")
	if err != nil {
		return nil, nil, err
	}
	files := map[string][]byte{}
	for _, name := range []string{"commandcode-preview.js", "commandcode-entrypoint.sh", "preview-port.sh", "commandcode-preview.md"} {
		data, err := commandCodeImageAssets.ReadFile("commandcode_assets/" + name)
		if err != nil {
			return nil, nil, err
		}
		files[name] = data
	}
	return dockerfile, files, nil
}

func commandCodeDockerTargetArch() string {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return runtime.GOARCH
	default:
		return runtime.GOARCH
	}
}

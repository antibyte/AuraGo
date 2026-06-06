package desktopstore

import (
	"context"
	"embed"
	"fmt"
	"runtime"
)

const commandCodeAppID = "commandcode"

//go:embed commandcode_assets/Dockerfile commandcode_assets/commandcode-preview.js commandcode_assets/commandcode-entrypoint.sh commandcode_assets/preview-port.sh
var commandCodeImageAssets embed.FS

func (s *Service) ensureCatalogImage(ctx context.Context, entry CatalogEntry) error {
	if err := s.requireDocker().PullImage(ctx, entry.Image); err != nil {
		if entry.ID == commandCodeAppID {
			return s.buildBundledCommandCodeImage(ctx, entry.Image, err)
		}
		return err
	}
	return nil
}

func (s *Service) buildBundledCommandCodeImage(ctx context.Context, image string, pullErr error) error {
	builder, ok := s.requireDocker().(DockerImageBuilder)
	if !ok {
		return fmt.Errorf("pull image %s failed and bundled build is unavailable: %w", image, pullErr)
	}
	dockerfile, files, err := commandCodeBuildContext()
	if err != nil {
		return fmt.Errorf("load bundled CommandCode build context: %w", err)
	}
	if err := builder.BuildImage(ctx, image, "Dockerfile", dockerfile, files, map[string]string{"TARGETARCH": commandCodeDockerTargetArch()}); err != nil {
		return fmt.Errorf("pull image %s failed (%v); build bundled CommandCode image: %w", image, pullErr, err)
	}
	return nil
}

func commandCodeBuildContext() ([]byte, map[string][]byte, error) {
	dockerfile, err := commandCodeImageAssets.ReadFile("commandcode_assets/Dockerfile")
	if err != nil {
		return nil, nil, err
	}
	files := map[string][]byte{}
	for _, name := range []string{"commandcode-preview.js", "commandcode-entrypoint.sh", "preview-port.sh"} {
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

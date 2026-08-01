package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootDockerfileEmbedsExplicitBuildID(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatalf("read root Dockerfile: %v", err)
	}
	dockerfile := string(content)
	for _, required := range []string{
		"ARG AURAGO_BUILD_ID=devel",
		"-X aurago/internal/buildinfo.BuildID=${AURAGO_BUILD_ID}",
	} {
		if !strings.Contains(dockerfile, required) {
			t.Fatalf("root Dockerfile must contain %q", required)
		}
	}
}

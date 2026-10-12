package decisions

import (
	"path/filepath"
	"testing"
)

func TestRuntimeBuildDirectoryNormalizesStandardBinLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "AuraGo")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "standard installer executable directory", input: filepath.Join(root, "bin"), want: root},
		{name: "standard installer with trailing separator", input: filepath.Join(root, "bin") + string(filepath.Separator), want: root},
		{name: "application root remains unchanged", input: root, want: root},
		{name: "directory containing bin is not normalized", input: filepath.Join(root, "binning"), want: filepath.Join(root, "binning")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runtimeBuildDirectory(managerOptions{runtimeDirectory: test.input})
			if got != test.want {
				t.Fatalf("runtimeBuildDirectory(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

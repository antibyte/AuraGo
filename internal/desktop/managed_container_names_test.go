package desktop

import (
	"testing"

	"aurago/internal/dockerutil"
)

// Code Studio and OpenSCAD create their containers through
// tools.DockerCreateContainerWithOptions, which rejects the Garage, homepage,
// app and local LLM reserved names. Their fixed names must never fall into
// those reserved sets, otherwise both desktop features stop starting.
func TestDesktopContainerNamesAreNotReservedManagedNames(t *testing.T) {
	names := map[string]string{
		"code studio": codeContainerName,
		"openscad":    openSCADContainerName,
	}
	checks := map[string]func(string) bool{
		"IsBoringGarageContainerName": dockerutil.IsBoringGarageContainerName,
		"IsHomepageContainerName":     dockerutil.IsHomepageContainerName,
		"IsAuraGoAppContainerName":    dockerutil.IsAuraGoAppContainerName,
		"IsLocalLLMContainerName":     dockerutil.IsLocalLLMContainerName,
	}
	for label, name := range names {
		if name == "" {
			t.Fatalf("%s container name is empty", label)
		}
		for checkName, check := range checks {
			if check(name) {
				t.Errorf("%s container name %q is matched by dockerutil.%s", label, name, checkName)
			}
		}
	}
}

package ui

import (
	"strings"
	"testing"
)

func TestSoftwareStoreDisablesMutatingActionsWhenBackendDisallowsThem(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"let mutationsAllowed = true;",
		"body.mutations_allowed !== false",
		"mutationDisabledText()",
		"if (isMutatingAction(action) && !mutationsAllowed)",
		"mutationDisabled ? `disabled title=\"${esc(mutationDisabled)}\"` : ''",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing mutation guard marker %q", want)
		}
	}
}

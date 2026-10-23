package audit

import (
	"fmt"
	"strings"
	"testing"
)

func TestUpdateAssetIDRequiresPinAndExplainsRecovery(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name     string
		metadata string
		exit     int
		valid    bool
	}{
		{"compact pin", `{"asset_set_id":"` + id + `"}`, 0, true},
		{"formatted pin", "{\n  \"asset_set_id\": \"" + id + "\"\n}", 0, true},
		{"recovery-only binary", `{"asset_set_id":"","archive_sha256":"","archive_bytes":0}`, 0, false},
		{"missing pin", `{}`, 0, false},
		{"invalid pin", `{"asset_set_id":"invalid"}`, 0, false},
		{"failed command", `{"asset_set_id":"` + id + `"}`, 7, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			helpers := updateScriptSpan(t, "update_asset_id() {", "\nupdate_space_check() {")
			output, err := runUpdateScript(t, helpers+fmt.Sprintf(`
mock_binary() {
    [ "$1" = --assets-info ] || return 90
    printf '%%s\n' '%s'
    return %d
}
update_asset_id mock_binary
`, tc.metadata, tc.exit))
			if tc.valid {
				if err != nil || output != id {
					t.Fatalf("valid pin: err=%v output=%q", err, output)
				}
				return
			}
			if err == nil {
				t.Fatalf("invalid metadata accepted: %q", output)
			}
			if tc.exit == 0 {
				for _, want := range []string{"mock_binary", "no valid web resource pin", "cmd/assetpack", "--check-assets", "Packaging assets alone cannot repair", "documentation/web-assets.md"} {
					if !strings.Contains(output, want) {
						t.Errorf("missing recovery hint %q: %s", want, output)
					}
				}
			}
		})
	}
}

package remote

import (
	"encoding/json"
	"testing"
)

func TestConfigUpdateDistinguishesOmittedPathsFromRevocation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		paths []string
		want  string
	}{
		{"partial", nil, "{}"},
		{"revoke", []string{}, `{"allowed_paths":[]}`},
		{"grant", []string{"/srv/shared"}, `{"allowed_paths":["/srv/shared"]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(ConfigUpdatePayload{AllowedPaths: tt.paths})
			if err != nil || string(data) != tt.want {
				t.Fatalf("wire=%s err=%v, want %s", data, err, tt.want)
			}
			var received ConfigUpdatePayload
			if err := json.Unmarshal(data, &received); err != nil {
				t.Fatal(err)
			}
			if (received.AllowedPaths == nil) != (tt.paths == nil) {
				t.Fatalf("revocation distinction lost: %s", data)
			}
		})
	}
}

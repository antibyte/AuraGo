//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLandlockUnixSocketGrantCanonicalizesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.sock")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "docker.sock")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(link, ":\n\x00") || strings.ContainsAny(want, ":\n\x00") {
		t.Fatalf("fixture paths must stay list-safe: link %q target %q", link, want)
	}

	env := AddUnixSocketGrant([]string{
		"AURAGO_SBX_RW=/work",
		"DOCKER_HOST=unix://" + link,
	})
	rw := envValue(env, "AURAGO_SBX_RW")
	required := envValue(env, "AURAGO_SBX_RW_REQUIRED")
	if !colonListContains(rw, want) || !colonListContains(required, want) {
		t.Fatalf("grant = %v, want canonical socket %q", env, want)
	}
	if colonListContains(rw, link) || colonListContains(required, link) {
		t.Fatalf("grant opened the symlink path: %v", env)
	}

	missing := filepath.Join(dir, "missing.sock")
	abs, err := filepath.Abs(missing)
	if err != nil {
		t.Fatal(err)
	}
	cleaned := filepath.Clean(abs)
	env = AddUnixSocketGrant([]string{
		"AURAGO_SBX_RW=/work",
		"DOCKER_HOST=unix://" + missing,
	})
	if !colonListContains(envValue(env, "AURAGO_SBX_RW"), cleaned) || !colonListContains(envValue(env, "AURAGO_SBX_RW_REQUIRED"), cleaned) {
		t.Fatalf("missing socket = %v, want cleaned absolute %q", env, cleaned)
	}

	colonTarget := filepath.Join(dir, "evil:name")
	if err := os.WriteFile(colonTarget, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	colonLink := filepath.Join(dir, "safe-link")
	if err := os.Symlink(colonTarget, colonLink); err != nil {
		t.Fatal(err)
	}
	env = AddUnixSocketGrant([]string{
		"AURAGO_SBX_RW=/work",
		"AURAGO_SBX_RW_REQUIRED=/work",
		"DOCKER_HOST=unix://" + colonLink,
	})
	if envValue(env, "AURAGO_SBX_RW") != "/work" || envValue(env, "AURAGO_SBX_RW_REQUIRED") != "/work" {
		t.Fatalf("resolved colon path was granted: %v", env)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

package dockerutil

import (
	"path/filepath"
	"testing"
)

func TestRustFSVolumePathsProtectDefaultAncestorsAndLiteralNames(t *testing.T) {
	for _, value := range []string{
		RustFSCredentialVolumeName, "project_" + RustFSDataVolumeName,
		"/var/lib/docker/volumes/aurago_rustfs_credentials/_data/key",
		"/var/lib/docker/volumes/ordinary/../aurago_rustfs_data/_data",
		"/var/lib/docker/volumes", "/var/lib/docker", "/var/lib", "/var", "/",
	} {
		if !IsRustFSVolumePath(value) {
			t.Errorf("private path admitted: %s", value)
		}
	}
	for _, value := range []string{"ordinary", "/var/lib/docker/volumes/ordinary/_data", "/srv/public", "/variable"} {
		if IsRustFSVolumePath(value) {
			t.Errorf("unrelated path blocked: %s", value)
		}
	}
	for _, name := range []string{"aurago-rustfs", "aurago-rustfs-key-seed", "stack-aurago-rustfs", "stack_aurago-rustfs", "stack-aurago-rustfs-1"} {
		if !IsRustFSContainerName(name) {
			t.Errorf("reserved container admitted: %s", name)
		}
	}
}

func TestRustFSVolumePathsRetainVerifiedCustomMountpoint(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private")
	RegisterRustFSVolumePath(private)
	for _, value := range []string{root, private, filepath.Join(private, "key")} {
		if !IsRustFSVolumePath(value) {
			t.Error("registered ownership boundary lost")
		}
	}
	if IsRustFSVolumePath(filepath.Join(root, "public")) {
		t.Fatal("sibling path blocked")
	}
	RegisterRustFSVolumePath(`D:\PrivateDocker\volumes\aurago_rustfs_credentials\_data`)
	for _, value := range []string{`d:\privatedocker`, `D:/PrivateDocker/volumes/aurago_rustfs_credentials/_data/key`} {
		if !IsRustFSVolumePath(value) {
			t.Error("Windows spelling escaped registered path")
		}
	}
}

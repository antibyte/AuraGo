// Package dockerstacks owns the Docker Manager's stacks: the compose projects
// that live as one folder per stack on a Docker host. It discovers them, reads
// and writes their compose file and .env with revisions, runs compose
// operations in short-lived runner containers, keeps the operation journal and
// reattaches to operations that were running when AuraGo restarted.
//
// Everything reaches the host through the Engine API of a dockerhost client:
// a lister container scans the stacks directory, a parked IO container serves
// the archive API for file access, and one runner container per operation runs
// `docker compose`. No Docker CLI is needed in AuraGo's own image.
package dockerstacks

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// The helper image, pinned by the digest of its multi-arch index (Docker CLI
// 29.9.2, buildx 0.38.1, Compose 5.6.1), read from the registry on 2026-10-27.
const (
	RunnerImageTag    = "docker:29.9.2-cli"
	RunnerImageDigest = "sha256:5e6baf67325a79f64ed3bed6863954d5e05d0f3d4e57cbdb14809346814bbfdb"
	RunnerImage       = "docker@" + RunnerImageDigest
	ComposeVersion    = "5.6.1"

	// runnerImageRepo and runnerImagePullTag are the two parts of the pull request
	// (POST /images/create?fromImage=docker&tag=sha256:...).
	runnerImageRepo    = "docker"
	runnerImagePullTag = RunnerImageDigest

	IOContainerName = "aurago-docker-manager-io"

	LabelRole      = "aurago.docker-manager.role"      // RoleRunner | RoleIO | RoleLister
	LabelOperation = "aurago.docker-manager.operation" // operation id (runner only)
	LabelStack     = "aurago.docker-manager.stack"     // stack name (runner only, "" for prepare)
	LabelStacksDir = "aurago.docker-manager.stacks-dir"
	RoleRunner     = "runner"
	RoleIO         = "io"
	RoleLister     = "lister"

	MinAPIMinor     = 41
	MaxFileBytes    = 1 << 20  // compose file or .env
	OutputTailBytes = 64 << 10 // kept per operation
	EnvFileName     = ".env"
)

// RunnerLocalTag names the image on a host that received the runner image by
// copy (GET /images/.../get, POST /images/load). A saved digest reference loses
// its digest name on load, so the copy is tagged and the helpers find it by this
// tag when the digest reference does not resolve.
var RunnerLocalTag = "aurago-docker-manager-runner:" + RunnerImageDigest[len("sha256:"):len("sha256:")+12]

// ComposeFileNames is Dockge's lookup order. New stacks are written as compose.yaml.
var ComposeFileNames = []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"}

var reStackName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// ValidateName enforces ^[a-z0-9][a-z0-9_-]{0,62}$ (the compose project name
// rules); the error carries CodeStackNameInvalid. A valid name has no path
// separator or dot, so it cannot leave the stacks directory.
func ValidateName(name string) error {
	if reStackName.MatchString(name) {
		return nil
	}
	return &Error{
		Code:    CodeStackNameInvalid,
		Message: "a stack name has 1 to 63 characters: lowercase letters, digits, - and _, starting with a letter or digit",
	}
}

// Revision is the lowercase hex SHA-256 of the bytes; a missing file has revision "".
func Revision(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// composeFileRank is the position of name in ComposeFileNames, or -1.
func composeFileRank(name string) int {
	for i, n := range ComposeFileNames {
		if n == name {
			return i
		}
	}
	return -1
}

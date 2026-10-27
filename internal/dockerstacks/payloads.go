package dockerstacks

import (
	"fmt"
	"path"

	"aurago/internal/dockerhost"
)

// containerSpec is the JSON body of POST /containers/create for the helper
// containers (runner, lister, IO). Field names are the Engine's.
type containerSpec struct {
	Image           string            `json:"Image"`
	Entrypoint      []string          `json:"Entrypoint"`
	Cmd             []string          `json:"Cmd,omitempty"`
	WorkingDir      string            `json:"WorkingDir,omitempty"`
	Env             []string          `json:"Env,omitempty"`
	Labels          map[string]string `json:"Labels"`
	Tty             bool              `json:"Tty"`
	NetworkDisabled bool              `json:"NetworkDisabled"`
	HostConfig      hostConfig        `json:"HostConfig"`
}

type hostConfig struct {
	Binds          []string          `json:"Binds"`
	NetworkMode    string            `json:"NetworkMode"`
	CapDrop        []string          `json:"CapDrop"`
	CapAdd         []string          `json:"CapAdd,omitempty"`
	ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
	Tmpfs          map[string]string `json:"Tmpfs"`
	SecurityOpt    []string          `json:"SecurityOpt"`
	LogConfig      logConfig         `json:"LogConfig"`
	RestartPolicy  restartPolicy     `json:"RestartPolicy"`
	AutoRemove     bool              `json:"AutoRemove"`
}

type logConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config"`
}

type restartPolicy struct {
	Name string `json:"Name"`
}

// Container names of the helpers.
const (
	runnerNamePrefix = "aurago-dm-op-"
	listerNamePrefix = "aurago-dm-ls-"
	socketInRunner   = "/var/run/docker.sock"
)

func runnerName(operationID string) string { return runnerNamePrefix + operationID }

// hardenedHostConfig is the host configuration all helpers share: no network,
// every capability dropped, a read-only root file system with a small tmpfs,
// no privilege escalation, a bounded json-file log and no restart.
func hardenedHostConfig(binds []string) hostConfig {
	return hostConfig{
		Binds:          binds,
		NetworkMode:    "none",
		CapDrop:        []string{"ALL"},
		ReadonlyRootfs: true,
		Tmpfs:          map[string]string{"/tmp": "rw,nosuid,nodev,size=64m"},
		SecurityOpt:    []string{"no-new-privileges:true"},
		LogConfig:      logConfig{Type: "json-file", Config: map[string]string{"max-size": "8m", "max-file": "1"}},
		RestartPolicy:  restartPolicy{Name: "no"},
	}
}

// stackDir is <stacks dir>/<name> as the host sees it (and the helpers, which
// bind the stacks directory at the same path).
func stackDir(h dockerhost.Host, name string) string { return path.Join(h.StacksDir, name) }

// runnerSpec is the runner container of one operation: it runs the kind's
// script with `sh -ec <script> aurago-docker-manager <args>`. composeFile is the
// file name inside the stack folder (unused by delete, which works by project
// name). Delete runs in the stacks directory, so a broken compose file in the
// folder it removes cannot stop the compose down, and it alone gets the
// capabilities to remove files that belong to other users.
func runnerSpec(image string, h dockerhost.Host, op Operation, composeFile string) (containerSpec, error) {
	script, ok := runnerScript(op.Kind, op.Options.Volumes)
	if !ok {
		return containerSpec{}, fmt.Errorf("operation kind %q has no runner script", op.Kind)
	}
	dir := stackDir(h, op.Stack)
	cmd := []string{script, "aurago-docker-manager"}
	workingDir := dir
	if op.Kind == KindDelete {
		workingDir = h.StacksDir
	} else {
		cmd = append(cmd, "-p", op.Stack, "-f", path.Join(dir, composeFile), "--progress", "json")
	}
	cfg := hardenedHostConfig([]string{h.StacksDir + ":" + h.StacksDir, h.HostSocketPath + ":" + socketInRunner})
	if op.Kind == KindDelete {
		cfg.CapAdd = []string{"DAC_OVERRIDE", "FOWNER"}
	}
	return containerSpec{
		Image:      image,
		Entrypoint: []string{"sh", "-ec"},
		Cmd:        cmd,
		WorkingDir: workingDir,
		Env:        []string{"HOME=/tmp", "DOCKER_CONFIG=/tmp/.docker", "STACK_DIR=" + dir, "PROJECT=" + op.Stack},
		Labels: map[string]string{
			LabelRole:      RoleRunner,
			LabelOperation: op.ID,
			LabelStack:     op.Stack,
			LabelStacksDir: h.StacksDir,
		},
		NetworkDisabled: true,
		HostConfig:      cfg,
	}, nil
}

// listerSpec is the short-lived container that lists the compose files and
// .env files one level below the stacks directory. The directory is bound
// read-only and the Docker socket is not bound at all.
func listerSpec(image string, h dockerhost.Host) containerSpec {
	cmd := []string{h.StacksDir, "-mindepth", "2", "-maxdepth", "2", "-type", "f", "("}
	for i, name := range ComposeFileNames {
		if i > 0 {
			cmd = append(cmd, "-o")
		}
		cmd = append(cmd, "-name", name)
	}
	cmd = append(cmd, "-o", "-name", EnvFileName, ")")
	return containerSpec{
		Image:           image,
		Entrypoint:      []string{"find"},
		Cmd:             cmd,
		Labels:          map[string]string{LabelRole: RoleLister, LabelStacksDir: h.StacksDir},
		NetworkDisabled: true,
		HostConfig:      hardenedHostConfig([]string{h.StacksDir + ":" + h.StacksDir + ":ro"}),
	}
}

// ioSpec is the parked IO container: created, never started. The Engine's
// archive API reads and writes files of a container's mounts without running
// it, which gives stack file access on every host type.
func ioSpec(image string, h dockerhost.Host) containerSpec {
	return containerSpec{
		Image:           image,
		Entrypoint:      []string{"true"},
		Labels:          map[string]string{LabelRole: RoleIO, LabelStacksDir: h.StacksDir},
		NetworkDisabled: true,
		HostConfig:      hardenedHostConfig([]string{h.StacksDir + ":" + h.StacksDir}),
	}
}

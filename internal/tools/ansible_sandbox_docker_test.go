package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/sandbox"
)

func TestAnsiblePreparedCommandReceivesDockerGrant(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "ansible.exe"), []byte("not-run"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	sb := preparedAnsibleLandlockSandbox{
		env: []string{
			"AURAGO_SBX_RW=/work",
			"AURAGO_ANSIBLE_GRANT_HELPER=1",
			"SystemRoot=" + os.Getenv("SystemRoot"),
		},
	}
	t.Cleanup(sandbox.SetForTest(sb))
	SetShellSandboxDockerHost("unix:///var/run/docker.sock")

	denied := ansibleGrantPermissions(false)
	ConfigureRuntimePermissions(denied)
	stdout, stderr, err := ansibleRunCmd(AnsibleLocalConfig{
		Context: WithRuntimePermissions(context.Background(), denied),
		WorkDir: t.TempDir(),
		Timeout: 20,
	}, "ansible", "--version")
	if err != nil {
		t.Fatalf("denied ansible run: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	sock := grantedDockerSocketSegment("/var/run/docker.sock")
	if strings.Contains(stdout, "DOCKER_HOST=unix:///var/run/docker.sock") || strings.Contains(stdout, sock) {
		t.Fatalf("denied command received docker grant:\n%s", stdout)
	}
	if !strings.Contains(stdout, "AURAGO_SBX_RW=/work") {
		t.Fatalf("denied command lost its prepared environment:\n%s\nstderr: %s", stdout, stderr)
	}

	allowed := ansibleGrantPermissions(true)
	ConfigureRuntimePermissions(allowed)
	stdout, stderr, err = ansibleRunCmd(AnsibleLocalConfig{
		Context: WithRuntimePermissions(context.Background(), allowed),
		WorkDir: t.TempDir(),
		Timeout: 20,
	}, "ansible", "--version")
	if err != nil {
		t.Fatalf("allowed ansible run: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "DOCKER_HOST=unix:///var/run/docker.sock") {
		t.Fatalf("prepared command missing docker host:\n%s", stdout)
	}
	if !strings.Contains(stdout, "AURAGO_SBX_RW=/work:"+sock) && !strings.Contains(stdout, "AURAGO_SBX_RW="+sock+":/work") {
		t.Fatalf("prepared command missing socket grant:\n%s", stdout)
	}
	if !strings.Contains(stdout, "AURAGO_SBX_RW_REQUIRED="+sock) &&
		!strings.Contains(stdout, "AURAGO_SBX_RW_REQUIRED=/work:"+sock) &&
		!strings.Contains(stdout, "AURAGO_SBX_RW_REQUIRED="+sock+":/work") {
		t.Fatalf("prepared command missing required socket:\n%s", stdout)
	}
}

func TestAnsiblePreparedDockerGrantHelper(t *testing.T) {
	if os.Getenv("AURAGO_ANSIBLE_GRANT_HELPER") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString("DOCKER_HOST=" + os.Getenv("DOCKER_HOST") + "\n")
	_, _ = os.Stdout.WriteString("AURAGO_SBX_RW=" + os.Getenv("AURAGO_SBX_RW") + "\n")
	_, _ = os.Stdout.WriteString("AURAGO_SBX_RW_REQUIRED=" + os.Getenv("AURAGO_SBX_RW_REQUIRED") + "\n")
	os.Exit(0)
}

func ansibleGrantPermissions(docker bool) RuntimePermissions {
	return RuntimePermissions{
		AllowShell:               true,
		AllowUnsafeHostExecution: true,
		DockerEnabled:            docker,
	}
}

type preparedAnsibleLandlockSandbox struct {
	env []string
}

func (s preparedAnsibleLandlockSandbox) Available() bool { return true }
func (s preparedAnsibleLandlockSandbox) Name() string    { return "landlock" }
func (s preparedAnsibleLandlockSandbox) PrepareCommand(command, workDir string) *exec.Cmd {
	return s.PrepareExecCommand(command, nil, workDir)
}
func (s preparedAnsibleLandlockSandbox) PrepareExecCommand(binary string, args []string, workDir string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestAnsiblePreparedDockerGrantHelper$")
	cmd.Dir = workDir
	cmd.Env = append([]string{}, s.env...)
	return cmd
}

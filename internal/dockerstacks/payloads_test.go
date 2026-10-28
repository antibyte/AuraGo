package dockerstacks

import (
	"strings"
	"testing"

	"aurago/internal/dockerhost"
)

func payloadHost() dockerhost.Host {
	return dockerhost.Host{ID: "h_test", StacksDir: "/opt/stacks", HostSocketPath: "/var/run/docker.sock"}
}

// The runner image digest is spelled out in the goldens on purpose: changing
// the pinned image must be a deliberate edit of these tests too.
const goldenRunnerImage = "docker@sha256:5e6baf67325a79f64ed3bed6863954d5e05d0f3d4e57cbdb14809346814bbfdb"

func TestRunnerImageConstants(t *testing.T) {
	if RunnerImage != goldenRunnerImage {
		t.Fatalf("RunnerImage = %q, want %q", RunnerImage, goldenRunnerImage)
	}
	if RunnerImageTag != "docker:29.9.2-cli" || ComposeVersion != "5.6.1" {
		t.Fatalf("tag %q compose %q", RunnerImageTag, ComposeVersion)
	}
	if want := "aurago-docker-manager-runner:5e6baf67325a"; RunnerLocalTag != want {
		t.Fatalf("RunnerLocalTag = %q, want %q", RunnerLocalTag, want)
	}
}

func TestRunnerSpecForUpMatchesTheGolden(t *testing.T) {
	op := Operation{ID: "op_0123456789abcdef", Stack: "web", Kind: KindUp}
	spec, err := runnerSpec(RunnerImage, payloadHost(), op, "compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	assertSameJSON(t, spec, `{
	  "Image": "`+goldenRunnerImage+`",
	  "Entrypoint": ["sh", "-ec"],
	  "Cmd": ["docker compose \"$@\" config --quiet && docker compose \"$@\" up -d --remove-orphans",
	          "aurago-docker-manager", "-p", "web", "-f", "/opt/stacks/web/compose.yaml", "--progress", "json"],
	  "WorkingDir": "/opt/stacks/web",
	  "Env": ["HOME=/tmp", "DOCKER_CONFIG=/tmp/.docker", "STACK_DIR=/opt/stacks/web", "PROJECT=web"],
	  "Labels": {
	    "aurago.docker-manager.role": "runner",
	    "aurago.docker-manager.operation": "op_0123456789abcdef",
	    "aurago.docker-manager.stack": "web",
	    "aurago.docker-manager.stacks-dir": "/opt/stacks"
	  },
	  "Tty": false,
	  "NetworkDisabled": true,
	  "HostConfig": {
	    "Binds": ["/opt/stacks:/opt/stacks", "/var/run/docker.sock:/var/run/docker.sock"],
	    "NetworkMode": "none",
	    "CapDrop": ["ALL"],
	    "ReadonlyRootfs": true,
	    "Tmpfs": {"/tmp": "rw,nosuid,nodev,size=64m"},
	    "SecurityOpt": ["no-new-privileges:true"],
	    "LogConfig": {"Type": "json-file", "Config": {"max-size": "8m", "max-file": "1"}},
	    "RestartPolicy": {"Name": "no"},
	    "AutoRemove": false
	  }
	}`)
}

func TestRunnerSpecForDeleteAddsCapabilitiesAndWorksByProjectName(t *testing.T) {
	op := Operation{ID: "op_fedcba9876543210", Stack: "old-app", Kind: KindDelete, Options: OpOptions{Volumes: true}}
	h := payloadHost()
	h.StacksDir, h.HostSocketPath = "/srv/stacks", "/run/user/1000/docker.sock"
	spec, err := runnerSpec(RunnerImage, h, op, "")
	if err != nil {
		t.Fatal(err)
	}
	assertSameJSON(t, spec, `{
	  "Image": "`+goldenRunnerImage+`",
	  "Entrypoint": ["sh", "-ec"],
	  "Cmd": ["docker compose -p \"$PROJECT\" down --remove-orphans --volumes && rm -rf -- \"$STACK_DIR\"", "aurago-docker-manager"],
	  "WorkingDir": "/srv/stacks",
	  "Env": ["HOME=/tmp", "DOCKER_CONFIG=/tmp/.docker", "STACK_DIR=/srv/stacks/old-app", "PROJECT=old-app"],
	  "Labels": {
	    "aurago.docker-manager.role": "runner",
	    "aurago.docker-manager.operation": "op_fedcba9876543210",
	    "aurago.docker-manager.stack": "old-app",
	    "aurago.docker-manager.stacks-dir": "/srv/stacks"
	  },
	  "Tty": false,
	  "NetworkDisabled": true,
	  "HostConfig": {
	    "Binds": ["/srv/stacks:/srv/stacks", "/run/user/1000/docker.sock:/var/run/docker.sock"],
	    "NetworkMode": "none",
	    "CapDrop": ["ALL"],
	    "CapAdd": ["DAC_OVERRIDE", "FOWNER"],
	    "ReadonlyRootfs": true,
	    "Tmpfs": {"/tmp": "rw,nosuid,nodev,size=64m"},
	    "SecurityOpt": ["no-new-privileges:true"],
	    "LogConfig": {"Type": "json-file", "Config": {"max-size": "8m", "max-file": "1"}},
	    "RestartPolicy": {"Name": "no"},
	    "AutoRemove": false
	  }
	}`)
}

func TestOnlyDeleteGetsCapabilities(t *testing.T) {
	for _, kind := range StackOperationKinds {
		spec, err := runnerSpec(RunnerImage, payloadHost(), Operation{ID: "op_1", Stack: "web", Kind: kind}, "compose.yaml")
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if got := len(spec.HostConfig.CapAdd) > 0; got != (kind == KindDelete) {
			t.Errorf("%s: CapAdd = %v, only delete may add capabilities", kind, spec.HostConfig.CapAdd)
		}
		if !spec.HostConfig.ReadonlyRootfs || spec.HostConfig.NetworkMode != "none" || spec.HostConfig.AutoRemove {
			t.Errorf("%s: runner is not hardened: %+v", kind, spec.HostConfig)
		}
	}
	if _, err := runnerSpec(RunnerImage, payloadHost(), Operation{ID: "op_1", Stack: "web", Kind: KindPrepare}, ""); err == nil {
		t.Fatal("prepare has no runner and must be refused")
	}
}

func TestRunnerScriptsNeverContainUserValues(t *testing.T) {
	// A hostile stack name only ever travels in the arguments and the environment.
	op := Operation{ID: "op_1", Stack: "web", Kind: KindUp}
	spec, _ := runnerSpec(RunnerImage, payloadHost(), op, "compose.yaml")
	for _, kind := range StackOperationKinds {
		script, ok := runnerScript(kind, true)
		if !ok {
			t.Fatalf("no script for %s", kind)
		}
		for _, forbidden := range []string{"web", "/opt/stacks", "compose.yaml"} {
			if strings.Contains(script, forbidden) {
				t.Errorf("script of %s contains %q: %s", kind, forbidden, script)
			}
		}
	}
	if spec.Cmd[0] != scriptUp {
		t.Fatalf("Cmd[0] = %q, want the up script", spec.Cmd[0])
	}
}

func TestListerSpecMatchesTheGolden(t *testing.T) {
	assertSameJSON(t, listerSpec(RunnerImage, payloadHost()), `{
	  "Image": "`+goldenRunnerImage+`",
	  "Entrypoint": ["find"],
	  "Cmd": ["/opt/stacks", "-mindepth", "2", "-maxdepth", "2", "-type", "f", "(",
	          "-name", "compose.yaml", "-o", "-name", "compose.yml", "-o", "-name", "docker-compose.yml",
	          "-o", "-name", "docker-compose.yaml", "-o", "-name", ".env", ")"],
	  "Labels": {"aurago.docker-manager.role": "lister", "aurago.docker-manager.stacks-dir": "/opt/stacks"},
	  "Tty": false,
	  "NetworkDisabled": true,
	  "HostConfig": {
	    "Binds": ["/opt/stacks:/opt/stacks:ro"],
	    "NetworkMode": "none",
	    "CapDrop": ["ALL"],
	    "ReadonlyRootfs": true,
	    "Tmpfs": {"/tmp": "rw,nosuid,nodev,size=64m"},
	    "SecurityOpt": ["no-new-privileges:true"],
	    "LogConfig": {"Type": "json-file", "Config": {"max-size": "8m", "max-file": "1"}},
	    "RestartPolicy": {"Name": "no"},
	    "AutoRemove": false
	  }
	}`)
}

func TestIOSpecMatchesTheGolden(t *testing.T) {
	assertSameJSON(t, ioSpec(RunnerImage, payloadHost()), `{
	  "Image": "`+goldenRunnerImage+`",
	  "Entrypoint": ["true"],
	  "Labels": {"aurago.docker-manager.role": "io", "aurago.docker-manager.stacks-dir": "/opt/stacks"},
	  "Tty": false,
	  "NetworkDisabled": true,
	  "HostConfig": {
	    "Binds": ["/opt/stacks:/opt/stacks"],
	    "NetworkMode": "none",
	    "CapDrop": ["ALL"],
	    "ReadonlyRootfs": true,
	    "Tmpfs": {"/tmp": "rw,nosuid,nodev,size=64m"},
	    "SecurityOpt": ["no-new-privileges:true"],
	    "LogConfig": {"Type": "json-file", "Config": {"max-size": "8m", "max-file": "1"}},
	    "RestartPolicy": {"Name": "no"},
	    "AutoRemove": false
	  }
	}`)
}

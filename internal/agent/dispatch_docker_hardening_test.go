package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchDockerCreateRunHardeningBlocksBeforeDocker(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = t.TempDir()

	tests := []struct {
		name string
		call ToolCall
		code string
	}{
		{
			name: "create requires name",
			call: ToolCall{Action: "docker", Operation: "create", Image: "alpine:latest"},
			code: "docker_name_required",
		},
		{
			name: "run requires name",
			call: ToolCall{Action: "docker", Operation: "run", Image: "alpine:latest"},
			code: "docker_name_required",
		},
		{
			name: "reserved homepage container",
			call: ToolCall{Action: "docker", Operation: "inspect", ContainerID: "aurago-homepage-web"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved homepage create name",
			call: ToolCall{Action: "docker", Operation: "create", Name: "aurago-homepage", Image: "alpine:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved homepage image",
			call: ToolCall{Action: "docker", Operation: "run", Name: "test-homepage", Image: "registry.test/team/aurago-homepage:v1"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved homepage image removal",
			call: ToolCall{Action: "docker", Operation: "remove_image", Image: "aurago-homepage:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "auto remove is run only",
			call: ToolCall{Action: "docker", Operation: "create", Name: "job", Image: "alpine:latest", AutoRemove: true},
			code: "docker_auto_remove_conflict",
		},
		{
			name: "auto remove conflicts with restart",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", AutoRemove: true, Restart: "always"},
			code: "docker_auto_remove_conflict",
		},
		{
			name: "command and args conflict",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "echo ok", CommandArgs: []string{"echo", "ok"}},
			code: "docker_command_conflict",
		},
		{
			name: "legacy shell syntax is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "ls /data | head"},
			code: "docker_command_args_required",
		},
		{
			name: "legacy shell quoting is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: `printf "%s" value`},
			code: "docker_command_args_required",
		},
		{
			name: "legacy redirection is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "echo ok > /tmp/result"},
			code: "docker_command_args_required",
		},
		{
			name: "legacy chaining is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "echo ok && true"},
			code: "docker_command_args_required",
		},
		{
			name: "legacy globbing is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "ls *.txt"},
			code: "docker_command_args_required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, ok := dispatchServices(context.Background(), tt.call, &DispatchContext{Cfg: cfg, Logger: testLogger})
			if !ok {
				t.Fatal("expected docker operation to be handled")
			}
			if !strings.Contains(output, `"code":"`+tt.code+`"`) {
				t.Fatalf("output = %s, want code %s", output, tt.code)
			}
			if strings.Contains(output, "connect") {
				t.Fatalf("validation reached Docker API: %s", output)
			}
		})
	}
}

func TestValidateAgentDockerCreateRunAllowsNamedGenericContainers(t *testing.T) {
	tests := []struct {
		name string
		req  dockerArgs
		want []string
	}{
		{
			name: "ordinary named container",
			req:  dockerArgs{Operation: "create", Name: "worker", Image: "alpine:latest", Command: "echo ok"},
			want: []string{"echo", "ok"},
		},
		{
			name: "named caddy container",
			req:  dockerArgs{Operation: "run", Name: "custom-caddy", Image: "caddy:2.11.2-alpine"},
		},
		{
			name: "exact command arguments",
			req:  dockerArgs{Operation: "run", Name: "shell-job", Image: "alpine:latest", CommandArgs: []string{"/bin/sh", "-lc", "printf '%s' \"$VALUE\""}, AutoRemove: true},
			want: []string{"/bin/sh", "-lc", "printf '%s' \"$VALUE\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, restart, options, validationError := validateAgentDockerCreateRun(tt.req)
			if validationError != "" {
				t.Fatalf("unexpected validation error: %s", validationError)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("command = %#v, want %#v", got, tt.want)
			}
			if restart != "no" {
				t.Fatalf("restart = %q, want no", restart)
			}
			if options.AutoRemove != tt.req.AutoRemove {
				t.Fatalf("AutoRemove = %v, want %v", options.AutoRemove, tt.req.AutoRemove)
			}
		})
	}
}

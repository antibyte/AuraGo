//go:build linux

package sandbox

import (
	"fmt"
	"os/exec"
	"strings"
)

func protectPlatformCommand(cmd *exec.Cmd, roots []string) (*exec.Cmd, error) {
	sb, ok := Get().(*LandlockSandbox)
	if !ok || !sb.Available() {
		return nil, fmt.Errorf("existing Desktop Notes require Linux Landlock isolation for local execution; enable the shell sandbox or use an isolated virtual workspace")
	}
	prepared := cmd
	isHelper := len(cmd.Args) > 1 && (cmd.Args[1] == "--sandbox-exec" || cmd.Args[1] == "--sandbox-exec-bin")
	if !isHelper {
		prepared = sb.PrepareExecCommand(cmd.Path, cmd.Args[1:], cmd.Dir)
		// The sandbox's policy variables are internal and cannot come from a tool.
		for _, env := range cmd.Env {
			if !strings.HasPrefix(env, "AURAGO_SBX_") {
				prepared.Env = append(prepared.Env, env)
			}
		}
		prepared.Stdin, prepared.Stdout, prepared.Stderr = cmd.Stdin, cmd.Stdout, cmd.Stderr
		prepared.SysProcAttr = cmd.SysProcAttr
	}
	rw := ""
	for _, env := range prepared.Env {
		if strings.HasPrefix(env, "AURAGO_SBX_RW=") {
			rw = strings.TrimPrefix(env, "AURAGO_SBX_RW=")
		}
	}
	if rw == "" {
		return nil, fmt.Errorf("notes protection could not validate the sandbox write policy")
	}
	for _, path := range strings.Split(rw, ":") {
		for _, root := range roots {
			if pathsOverlap(path, root) {
				return nil, fmt.Errorf("sandbox writable paths overlap protected Desktop Notes; use a separate execution workspace")
			}
		}
	}
	// Read-only ancestors never override an existing writable grant in Landlock.
	// Denial above is intentional; adding another read-only path would not protect notes.
	return prepared, nil
}

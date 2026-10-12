package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/sandbox"
	"aurago/internal/security"
)

func NewGogService(cfg *config.Config, vault *security.Vault, resolve func() *config.Config) *gog.Service {
	if resolve == nil {
		resolve = func() *config.Config {
			if cfg.AuthorizationSnapshots != nil {
				_, current := cfg.AuthorizationSnapshots()
				return current
			}
			return cfg
		}
	}
	root, _ := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "gog"))
	return &gog.Service{Root: root, Vault: vault, Config: resolve, RunProcess: runGogProcess, CheckRuntime: checkGogRuntime}
}

func checkGogRuntime(ctx context.Context) error {
	if err := requireHostShellExecutionContext(ctx); err != nil {
		return err
	}
	perms, configured := EffectiveRuntimePermissions(ctx)
	if !configured || !perms.AllowNetworkRequests || sandbox.Get().Name() == "blocked" {
		return fmt.Errorf("gog requires the configured sandbox or explicit host execution permission")
	}
	return nil
}

type gogBuffer struct{ data []byte }

func (b *gogBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > gog.MaxOutput {
		return 0, fmt.Errorf("gog output limit exceeded")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func runGogProcess(ctx context.Context, binary string, args []string, workdir string, extra []string) (string, int, bool, error) {
	if err := checkGogRuntime(ctx); err != nil {
		return "", -1, false, err
	}
	perms, _ := EffectiveRuntimePermissions(ctx)
	sb := sandbox.Get()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				now, ok := EffectiveRuntimePermissions(ctx)
				info, _ := os.Lstat(filepath.Join(workdir, "output"))
				oversized := info != nil && (!info.Mode().IsRegular() || info.Size() > gog.MaxFile)
				if !ok || !now.AllowShell || !now.AllowNetworkRequests || runtime.GOOS == "windows" && !now.AllowUnsafeHostExecution || hostShellFallbackError(runtime.GOOS, sb, now) != nil || !reflect.DeepEqual(perms.ProtectedNotesRoots, now.ProtectedNotesRoots) || oversized {
					cancel()
					return
				}
			}
		}
	}()
	cmd := sb.PrepareExecCommand(binary, args, workdir)
	SetupCmd(cmd)
	var env []string
	// Retain sandbox controls created by PrepareExecCommand, never inherited controls.
	for _, v := range cmd.Env {
		if strings.HasPrefix(v, "AURAGO_SBX_") {
			if strings.HasPrefix(v, "AURAGO_SBX_RO=") {
				v += ":" + binary
			}
			env = append(env, v)
		}
	}
	for _, k := range []string{"SYSTEMROOT", "WINDIR", "LANG"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, "HOME="+workdir, "TMPDIR="+workdir, "TEMP="+workdir, "TMP="+workdir, "PATH=", "NO_COLOR=1")
	cmd.Env = append(env, extra...)
	protected, err := sandbox.ProtectFilesCommand(cmd, perms.ProtectedNotesRoots)
	if err != nil {
		return "", -1, false, err
	}
	cmd = protected
	var stdout, stderr gogBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader("")
	cmd.WaitDelay = 2 * time.Second
	if ctx.Err() != nil {
		return "", -1, false, ctx.Err()
	}
	if err = cmd.Start(); err != nil {
		return "", -1, false, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		KillProcessTree(cmd.Process.Pid)
		err = <-done
		if err == nil {
			err = ctx.Err()
		}
	}
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		code = -1
	}
	return string(stdout.data), code, true, err
}

var _ io.Writer = (*gogBuffer)(nil)

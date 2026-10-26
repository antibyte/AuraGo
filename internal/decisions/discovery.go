package decisions

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"aurago/internal/onnxrt"
)

const runtimeDiscoveryTimeout = 60 * time.Second

func runDiscoveryCommand(command *exec.Cmd) error {
	cleanup, err := startRuntimeCommand(command)
	if err != nil {
		return err
	}
	defer cleanup()
	return command.Wait()
}

func (manager *Manager) discoverDevices(ctx context.Context, deployment string, access RuntimeAccess, paths modelPaths) ([]runtimeDevice, error) {
	probeCtx, cancel := context.WithTimeout(ctx, runtimeDiscoveryTimeout)
	defer cancel()
	devices, err := manager.factory.discover(probeCtx, deployment, access, paths, manager.dataDir)
	if ctx.Err() != nil {
		return nil, NewError(CodeCancelled, ctx.Err())
	}
	if probeCtx.Err() != nil {
		return nil, NewError(CodeRuntimeDiscoveryTimeout, probeCtx.Err())
	}
	return devices, err
}

var errDiscoveryOutputLimit = errors.New("discovery output exceeds limit")

type discoveryOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func (out *discoveryOutput) Write(data []byte) (int, error) {
	if len(data) > out.limit-out.buffer.Len() {
		out.exceeded = true
		out.cancel()
		return 0, errDiscoveryOutputLimit
	}
	return out.buffer.Write(data)
}

func runNativeDiscovery(ctx context.Context, binary, directory string, args []string) ([]byte, error) {
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	command := exec.CommandContext(probeCtx, binary, args...)
	onnxrt.ConfigureWorkerCommand(command, directory)
	configureHiddenProcess(command)
	stdout := &discoveryOutput{limit: 1 << 20, cancel: cancel}
	stderr := &discoveryOutput{limit: 64 << 10, cancel: cancel}
	command.Stdout, command.Stderr = stdout, stderr
	command.WaitDelay = 2 * time.Second
	err := runDiscoveryCommand(command)
	if ctx.Err() != nil {
		return nil, NewError(CodeCancelled, ctx.Err())
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, NewError(CodeRuntimeDiscoveryOutputLimit, errDiscoveryOutputLimit)
	}
	return stdout.buffer.Bytes(), err
}

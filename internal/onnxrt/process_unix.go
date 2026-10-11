//go:build !windows

package onnxrt

import "os/exec"

func hideWindow(_ *exec.Cmd) {}

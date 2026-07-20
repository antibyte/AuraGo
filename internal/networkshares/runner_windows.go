//go:build windows

package networkshares

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

func platformCommand(_ Options, privileged bool, name string, args []string, stdin []byte) (string, []string, []byte, error) {
	if privileged && !platformElevated() {
		return "", nil, nil, codedError(ErrorPermissionDenied, "Network share changes require AuraGo to run in an elevated Windows process.", nil)
	}
	return name, args, stdin, nil
}

func platformElevated() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		`$p=[Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent(); if($p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){'true'}else{'false'}`).Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func elevationReason() string {
	return "Network share changes require AuraGo to run in an elevated Windows process."
}

//go:build windows

package networkshares

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type windowsRunnerCall struct {
	name  string
	args  []string
	stdin []byte
}

type windowsFakeRunner struct {
	calls   []windowsRunnerCall
	handler func(name string, args []string, stdin []byte) ([]byte, error)
}

func (r *windowsFakeRunner) LookPath(file string) (string, error) {
	return file, nil
}

func (r *windowsFakeRunner) Run(_ context.Context, _ Options, _ bool, name string, args []string, stdin []byte) ([]byte, error) {
	r.calls = append(r.calls, windowsRunnerCall{
		name:  name,
		args:  append([]string(nil), args...),
		stdin: append([]byte(nil), stdin...),
	})
	if r.handler != nil {
		return r.handler(name, args, stdin)
	}
	return nil, nil
}

func TestWindowsProbeReportsInstalledReadableServiceWithoutWriteElevation(t *testing.T) {
	runner := &windowsFakeRunner{}
	runner.handler = func(_ string, args []string, _ []byte) ([]byte, error) {
		if len(args) > 0 && strings.Contains(args[len(args)-4], "Get-Module -ListAvailable") {
			return []byte(`{"installed":true,"version":"2.0","service_active":true}`), nil
		}
		return nil, nil
	}
	adapter := &windowsAdapter{runner: runner}
	status, err := adapter.Probe(context.Background(), Options{
		SMBEnabled: true,
		ReadOnly:   true,
	})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !status.SMB.Installed || !status.SMB.Configured || !status.SMB.ServiceActive || !status.SMB.Readable {
		t.Fatalf("SMB status = %+v", status.SMB)
	}
	if status.SMB.Writable {
		t.Fatal("read-only Windows probe unexpectedly reported write access")
	}
}

func TestWindowsProbeReportsMissingModuleSafely(t *testing.T) {
	runner := &windowsFakeRunner{handler: func(_ string, _ []string, _ []byte) ([]byte, error) {
		return []byte(`{"installed":false,"version":"","service_active":false}`), nil
	}}
	adapter := &windowsAdapter{runner: runner}
	status, err := adapter.Probe(context.Background(), Options{SMBEnabled: true})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if status.SMB.Installed || status.SMB.Readable || status.SMB.Reason == "" {
		t.Fatalf("missing-module SMB status = %+v", status.SMB)
	}
}

func TestWindowsProbeKeepsReadsButBlocksDockerWrites(t *testing.T) {
	runner := &windowsFakeRunner{}
	runner.handler = func(_ string, args []string, _ []byte) ([]byte, error) {
		if len(args) > 0 && strings.Contains(args[len(args)-4], "Get-Module -ListAvailable") {
			return []byte(`{"installed":true,"version":"2.0","service_active":true}`), nil
		}
		return nil, nil
	}
	adapter := &windowsAdapter{runner: runner}
	status, err := adapter.Probe(context.Background(), Options{
		SMBEnabled:  true,
		AllowCreate: true,
		IsDocker:    true,
	})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !status.SMB.Readable || status.SMB.Writable || !strings.Contains(status.SMB.Reason, "Docker") {
		t.Fatalf("Docker SMB status = %+v", status.SMB)
	}
}

func TestWindowsShareValuesTravelOnlyThroughJSONStdin(t *testing.T) {
	runner := &windowsFakeRunner{}
	adapter := &windowsAdapter{runner: runner}
	injectedName := `media'; Remove-SmbShare -Name C$; '`
	injectedPath := `C:\Shares\Media & whoami`
	err := adapter.Create(context.Background(), Options{}, ShareSpec{
		Protocol: ProtocolSMB,
		Name:     injectedName,
		Path:     injectedPath,
		ReadOnly: true,
		Access:   ShareAccess{ACL: []ACLEntry{{Principal: `DOMAIN\User`, Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("PowerShell calls = %d, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if call.name != "powershell.exe" {
		t.Fatalf("command = %q", call.name)
	}
	if strings.Contains(strings.Join(call.args, "\x00"), injectedName) ||
		strings.Contains(strings.Join(call.args, "\x00"), injectedPath) {
		t.Fatalf("untrusted share value leaked into PowerShell arguments: %#v", call.args)
	}
	var input windowsShareInput
	if err := json.Unmarshal(call.stdin, &input); err != nil {
		t.Fatalf("decode JSON stdin: %v", err)
	}
	if input.Name != injectedName || input.Path != injectedPath {
		t.Fatalf("JSON input = %+v", input)
	}
}

func TestWindowsSMBPrincipalValidationUsesJSONStdin(t *testing.T) {
	runner := &windowsFakeRunner{handler: func(_ string, _ []string, _ []byte) ([]byte, error) {
		return []byte(`{"missing":[]}`), nil
	}}
	adapter := &windowsAdapter{runner: runner}
	principal := `DOMAIN\User'; Get-Secret; '`
	err := adapter.Validate(context.Background(), Options{}, ShareSpec{
		Protocol: ProtocolSMB,
		Access:   ShareAccess{ACL: []ACLEntry{{Principal: principal, Level: "read"}}},
	})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("validation calls = %d, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if strings.Contains(strings.Join(call.args, "\x00"), principal) {
		t.Fatalf("principal leaked into PowerShell arguments: %#v", call.args)
	}
	var request struct {
		Principals []string `json:"principals"`
	}
	if err := json.Unmarshal(call.stdin, &request); err != nil {
		t.Fatalf("decode principal JSON: %v", err)
	}
	if len(request.Principals) != 1 || request.Principals[0] != principal {
		t.Fatalf("principal request = %+v", request)
	}
}

func TestWindowsCapabilityFailureDoesNotExposeCommandOutput(t *testing.T) {
	runner := &windowsFakeRunner{handler: func(_ string, _ []string, _ []byte) ([]byte, error) {
		return nil, errors.New("sensitive native details")
	}}
	adapter := &windowsAdapter{runner: runner}
	status, err := adapter.Probe(context.Background(), Options{SMBEnabled: true})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if strings.Contains(status.SMB.Reason, "sensitive") {
		t.Fatalf("probe reason exposes runner details: %q", status.SMB.Reason)
	}
}

func TestWindowsNFSScriptsUseDocumentedPermissionValues(t *testing.T) {
	for name, script := range map[string]string{
		"create": windowsCreateScript,
		"update": windowsUpdateScript,
	} {
		if !strings.Contains(script, "'readonly'") || !strings.Contains(script, "'readwrite'") {
			t.Fatalf("%s script is missing documented NFS permission values", name)
		}
		if strings.Contains(script, "'read-only'") || strings.Contains(script, "'read-write'") {
			t.Fatalf("%s script contains unsupported hyphenated NFS permission values", name)
		}
	}
}

func TestWindowsSMBCreateAppliesACLAtomically(t *testing.T) {
	for _, parameter := range []string{"ReadAccess", "ChangeAccess", "FullAccess", "NoAccess"} {
		if !strings.Contains(windowsCreateScript, "$parameters."+parameter) {
			t.Fatalf("Windows SMB create script does not set %s through New-SmbShare", parameter)
		}
	}
	if strings.Contains(windowsCreateScript, "Grant-SmbShareAccess") ||
		strings.Contains(windowsCreateScript, "Block-SmbShareAccess") {
		t.Fatal("Windows SMB create must not create the share before applying its ACL")
	}
}

func TestWindowsWritableSMBRequiresWriter(t *testing.T) {
	root := t.TempDir()
	_, err := validateShareSpec(context.Background(), ShareSpec{
		Protocol: ProtocolSMB,
		Name:     "documents",
		Path:     root,
		ReadOnly: false,
		Access: ShareAccess{ACL: []ACLEntry{
			{Principal: "Users", Level: "read"},
		}},
	}, "create", Options{
		SMBEnabled:           true,
		AllowedRoots:         []string{root},
		SMBAllowedPrincipals: []string{"Users"},
	}, Status{SMB: ProtocolStatus{Readable: true}})
	if ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("writable SMB without writer error = %v, want %s", err, ErrorInvalidArgument)
	}
}

func TestWindowsNFSRejectsCIDRClient(t *testing.T) {
	adapter := &windowsAdapter{runner: &windowsFakeRunner{}}
	err := adapter.Validate(context.Background(), Options{}, ShareSpec{
		Protocol: ProtocolNFS,
		Access:   ShareAccess{Clients: []string{"192.0.2.0/24"}},
	})
	if ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("CIDR validation error = %v, want %s", err, ErrorInvalidArgument)
	}
	if len(adapter.runner.(*windowsFakeRunner).calls) != 0 {
		t.Fatal("CIDR validation must fail before invoking PowerShell")
	}
}

func TestWindowsPathScopeIsCaseInsensitiveAndDriveSafe(t *testing.T) {
	if !pathWithinRoot(`C:\Shares\Media`, `c:\shares`) {
		t.Fatal("Windows path scope should be case-insensitive")
	}
	if pathWithinRoot(`D:\Shares\Media`, `C:\Shares`) {
		t.Fatal("Windows path scope must reject a different drive")
	}
}

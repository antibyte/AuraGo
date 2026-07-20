//go:build windows

package networkshares

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

type windowsAdapter struct {
	runner commandRunner
	logger *slog.Logger
}

type windowsShareInput struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Description string     `json:"description"`
	ReadOnly    bool       `json:"read_only"`
	ACL         []ACLEntry `json:"acl"`
	Clients     []string   `json:"clients"`
}

type windowsShareRow struct {
	Protocol    string     `json:"protocol"`
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Description string     `json:"description"`
	ReadOnly    bool       `json:"read_only"`
	ACL         []ACLEntry `json:"acl"`
	Clients     []string   `json:"clients"`
}

func platformSupported() bool {
	return true
}

func newPlatformAdapter(runner commandRunner, logger *slog.Logger) platformAdapter {
	return &windowsAdapter{runner: runner, logger: logger}
}

func (a *windowsAdapter) Probe(ctx context.Context, options Options) (Status, error) {
	status := Status{Supported: true}
	if options.SMBEnabled {
		status.SMB = a.probeModule(ctx, options, "SMBShare", "Get-SmbShare", "LanmanServer", "Windows SMBShare")
	} else {
		status.SMB.ReasonCode = "protocol_disabled"
		status.SMB.Reason = "SMB management is disabled in the AuraGo configuration."
	}
	if options.NFSEnabled {
		status.NFS = a.probeModule(ctx, options, "NFS", "Get-NfsShare", "NfsService", "Windows NFS")
	} else {
		status.NFS.ReasonCode = "protocol_disabled"
		status.NFS.Reason = "NFS management is disabled in the AuraGo configuration."
	}
	status.Usable = status.SMB.Readable || status.NFS.Readable
	status.ReasonCode = firstNonEmpty(status.SMB.ReasonCode, status.NFS.ReasonCode)
	status.Reason = firstNonEmpty(status.SMB.Reason, status.NFS.Reason)
	return status, nil
}

func (a *windowsAdapter) Validate(ctx context.Context, options Options, share ShareSpec) error {
	if share.Protocol == ProtocolNFS {
		for _, client := range share.Access.Clients {
			if strings.Contains(client, "/") {
				return codedError(ErrorInvalidArgument,
					"Windows NFS share permissions support individual client IP addresses, not CIDR networks.", nil)
			}
		}
		return nil
	}
	if share.Protocol != ProtocolSMB || len(share.Access.ACL) == 0 {
		return nil
	}
	principals := make([]string, 0, len(share.Access.ACL))
	for _, entry := range share.Access.ACL {
		principals = append(principals, entry.Principal)
	}
	payload, err := json.Marshal(map[string]interface{}{"principals": principals})
	if err != nil {
		return fmt.Errorf("encode Windows SMB principal validation: %w", err)
	}
	output, err := a.runPowerShell(ctx, options, false, windowsValidatePrincipalsScript, payload)
	if err != nil {
		return codedError(ErrorUnavailable, "Windows could not validate the configured SMB principals.", err)
	}
	var result struct {
		Missing []string `json:"missing"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return codedError(ErrorUnavailable, "Windows returned invalid SMB principal validation output.", err)
	}
	if len(result.Missing) > 0 {
		return codedError(ErrorInvalidArgument,
			fmt.Sprintf("SMB principal %q is not an existing operating-system identity.", result.Missing[0]), nil)
	}
	return nil
}

func (a *windowsAdapter) probeModule(ctx context.Context, options Options, module, command, service, backend string) ProtocolStatus {
	result := ProtocolStatus{Supported: true, Backend: backend}
	script := `
$ErrorActionPreference = 'Stop'
$module = Get-Module -ListAvailable -Name $args[0] | Select-Object -First 1
$command = Get-Command -Name $args[1] -ErrorAction SilentlyContinue
$service = Get-Service -Name $args[2] -ErrorAction SilentlyContinue
[pscustomobject]@{
  installed = [bool]$module -and [bool]$command
  version = if ($module) { [string]$module.Version } else { '' }
  service_active = [bool]$service -and $service.Status -eq 'Running'
} | ConvertTo-Json -Compress
`
	output, err := a.runPowerShell(ctx, options, false, script, nil, module, command, service)
	if err != nil {
		result.ReasonCode = "probe_failed"
		result.Reason = backend + " capability detection failed."
		return result
	}
	var probe struct {
		Installed     bool   `json:"installed"`
		Version       string `json:"version"`
		ServiceActive bool   `json:"service_active"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		result.ReasonCode = "probe_failed"
		result.Reason = backend + " capability output was invalid."
		return result
	}
	result.Installed = probe.Installed
	result.Version = probe.Version
	result.ServiceActive = probe.ServiceActive
	result.Configured = probe.Installed
	if !result.Installed {
		result.ReasonCode = "not_installed"
		result.Reason = backend + " PowerShell module is not installed."
		return result
	}
	if _, err := a.runPowerShell(ctx, options, false, `$ErrorActionPreference='Stop'; & $args[0] | Out-Null`, nil, command); err != nil {
		result.ReasonCode = "not_readable"
		result.Reason = backend + " shares are not readable."
		return result
	}
	result.Readable = true
	if !result.ServiceActive {
		result.ReasonCode = "service_inactive"
		result.Reason = backend + " service is not active."
	}
	result.Writable = result.Readable && result.ServiceActive && !options.ReadOnly &&
		(options.AllowCreate || options.AllowUpdate || options.AllowDelete) &&
		!options.IsDocker && platformElevated()
	if result.Writable {
		result.ReasonCode = ""
		result.Reason = ""
	} else if result.Readable && result.ServiceActive {
		switch {
		case options.ReadOnly:
			result.ReasonCode = "readonly"
			result.Reason = "Host mutations are disabled by network_shares.readonly."
		case !options.AllowCreate && !options.AllowUpdate && !options.AllowDelete:
			result.ReasonCode = "permission_disabled"
			result.Reason = "No network share mutation permission is enabled."
		case options.IsDocker:
			result.ReasonCode = "docker_restricted"
			result.Reason = "Network share mutations are unavailable in the standard Docker deployment."
		default:
			result.ReasonCode = "privilege_required"
			result.Reason = elevationReason()
		}
	}
	return result
}

func (a *windowsAdapter) List(ctx context.Context, options Options) ([]observedShare, error) {
	var shares []observedShare
	if options.SMBEnabled {
		rows, err := a.listWindowsShares(ctx, options, windowsListSMBScript)
		if err != nil {
			return nil, fmt.Errorf("list Windows SMB shares: %w", err)
		}
		shares = append(shares, rows...)
	}
	if options.NFSEnabled {
		rows, err := a.listWindowsShares(ctx, options, windowsListNFSScript)
		if err != nil {
			return nil, fmt.Errorf("list Windows NFS shares: %w", err)
		}
		shares = append(shares, rows...)
	}
	return shares, nil
}

func (a *windowsAdapter) listWindowsShares(ctx context.Context, options Options, script string) ([]observedShare, error) {
	output, err := a.runPowerShell(ctx, options, false, script, nil)
	if err != nil {
		return nil, err
	}
	var rows []windowsShareRow
	if len(strings.TrimSpace(string(output))) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("decode PowerShell share output: %w", err)
	}
	shares := make([]observedShare, 0, len(rows))
	for _, row := range rows {
		clients := normalizeClients(row.Clients)
		shares = append(shares, observedShare{
			ShareSpec: ShareSpec{
				Protocol: strings.ToLower(row.Protocol),
				Name:     row.Name,
				Path:     filepath.Clean(row.Path),
				Comment:  row.Description,
				ReadOnly: row.ReadOnly,
				Access: ShareAccess{
					ACL:     row.ACL,
					Clients: clients,
				},
			},
			MarkerID:        markerID(row.Description),
			MarkerSupported: strings.EqualFold(row.Protocol, ProtocolSMB),
			Active:          true,
			CommentObserved: strings.EqualFold(row.Protocol, ProtocolSMB),
		})
	}
	return shares, nil
}

func (a *windowsAdapter) Create(ctx context.Context, options Options, share ShareSpec) error {
	return a.apply(ctx, options, windowsCreateScript, share)
}

func (a *windowsAdapter) Update(ctx context.Context, options Options, previous, desired ShareSpec) error {
	_ = previous
	return a.apply(ctx, options, windowsUpdateScript, desired)
}

func (a *windowsAdapter) Delete(ctx context.Context, options Options, share ShareSpec) error {
	if err := a.deleteRaw(ctx, options, share); err != nil {
		return err
	}
	return nil
}

func (a *windowsAdapter) apply(ctx context.Context, options Options, script string, share ShareSpec) error {
	input := windowsShareInput{
		Name:        share.Name,
		Path:        share.Path,
		Description: share.Comment,
		ReadOnly:    share.ReadOnly,
		ACL:         share.Access.ACL,
		Clients:     share.Access.Clients,
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode Windows share request: %w", err)
	}
	if _, err := a.runPowerShell(ctx, options, true, script, payload, share.Protocol); err != nil {
		return fmt.Errorf("apply Windows %s share: %w", strings.ToUpper(share.Protocol), err)
	}
	return nil
}

func (a *windowsAdapter) deleteRaw(ctx context.Context, options Options, share ShareSpec) error {
	input, err := json.Marshal(map[string]string{"name": share.Name})
	if err != nil {
		return fmt.Errorf("encode Windows share delete request: %w", err)
	}
	script := windowsDeleteSMBScript
	if share.Protocol == ProtocolNFS {
		script = windowsDeleteNFSScript
	}
	if _, err := a.runPowerShell(ctx, options, true, script, input); err != nil {
		return fmt.Errorf("delete Windows %s share: %w", strings.ToUpper(share.Protocol), err)
	}
	return nil
}

func (a *windowsAdapter) runPowerShell(ctx context.Context, options Options, privileged bool, script string, stdin []byte, args ...string) ([]byte, error) {
	commandArgs := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
	commandArgs = append(commandArgs, args...)
	return a.runner.Run(ctx, options, privileged, "powershell.exe", commandArgs, stdin)
}

func normalizePlatformShare(share ShareSpec) ShareSpec {
	return share
}

const windowsListSMBScript = `
$ErrorActionPreference = 'Stop'
$rows = @()
foreach ($share in @(Get-SmbShare | Where-Object { -not $_.Special -and $_.Path -and $_.Name -notlike '*$' })) {
  $acl = @()
  foreach ($entry in @(Get-SmbShareAccess -Name $share.Name)) {
    $level = switch ([string]$entry.AccessControlType) {
      'Deny' { 'deny' }
      default {
        switch ([string]$entry.AccessRight) {
          'Full' { 'full' }
          'Change' { 'change' }
          default { 'read' }
        }
      }
    }
    $acl += [pscustomobject]@{ principal = [string]$entry.AccountName; level = $level }
  }
  $rows += [pscustomobject]@{
    protocol = 'smb'
    name = [string]$share.Name
    path = [string]$share.Path
    description = [string]$share.Description
    read_only = (@($acl | Where-Object { $_.level -in @('change','full') }).Count -eq 0)
    acl = $acl
    clients = @()
  }
}
ConvertTo-Json -InputObject @($rows) -Compress -Depth 8
`

const windowsValidatePrincipalsScript = `
$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$missing = @()
foreach ($principal in @($request.principals)) {
  try {
    $account = [System.Security.Principal.NTAccount]::new([string]$principal)
    $null = $account.Translate([System.Security.Principal.SecurityIdentifier])
  } catch {
    $missing += [string]$principal
  }
}
[pscustomobject]@{ missing = @($missing) } | ConvertTo-Json -Compress -Depth 4
`

const windowsListNFSScript = `
$ErrorActionPreference = 'Stop'
$rows = @()
foreach ($share in @(Get-NfsShare)) {
  $permissions = @(Get-NfsSharePermission -Name $share.Name)
  $clients = @($permissions | ForEach-Object { [string]$_.ClientName } | Where-Object { $_ })
  $readOnly = (@($permissions | Where-Object { [string]$_.Permission -match 'ReadWrite' }).Count -eq 0)
  $rows += [pscustomobject]@{
    protocol = 'nfs'
    name = [string]$share.Name
    path = [string]$share.Path
    description = ''
    read_only = $readOnly
    acl = @()
    clients = $clients
  }
}
ConvertTo-Json -InputObject @($rows) -Compress -Depth 8
`

const windowsCreateScript = `
$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
if ($args[0] -eq 'smb') {
  $parameters = @{
    Name = [string]$request.name
    Path = [string]$request.path
    Description = [string]$request.description
    FolderEnumerationMode = 'AccessBased'
  }
  $readAccess = @($request.acl | Where-Object { $_.level -eq 'read' } | ForEach-Object { [string]$_.principal })
  $changeAccess = @($request.acl | Where-Object { $_.level -eq 'change' } | ForEach-Object { [string]$_.principal })
  $fullAccess = @($request.acl | Where-Object { $_.level -eq 'full' } | ForEach-Object { [string]$_.principal })
  $noAccess = @($request.acl | Where-Object { $_.level -eq 'deny' } | ForEach-Object { [string]$_.principal })
  if ($readAccess.Count -gt 0) { $parameters.ReadAccess = $readAccess }
  if ($changeAccess.Count -gt 0) { $parameters.ChangeAccess = $changeAccess }
  if ($fullAccess.Count -gt 0) { $parameters.FullAccess = $fullAccess }
  if ($noAccess.Count -gt 0) { $parameters.NoAccess = $noAccess }
  New-SmbShare @parameters | Out-Null
} else {
  New-NfsShare -Name $request.name -Path $request.path -Permission no-access -EnableAnonymousAccess:$false -EnableUnmappedAccess:$false -AllowRootAccess:$false | Out-Null
  $permission = if ($request.read_only) { 'readonly' } else { 'readwrite' }
  foreach ($client in @($request.clients)) {
    Grant-NfsSharePermission -Name $request.name -ClientName $client -ClientType host -Permission $permission -AllowRootAccess:$false | Out-Null
  }
}
`

const windowsUpdateScript = `
$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
if ($args[0] -eq 'smb') {
  Set-SmbShare -Name $request.name -Description $request.description -Force | Out-Null
  foreach ($entry in @(Get-SmbShareAccess -Name $request.name)) {
    Revoke-SmbShareAccess -Name $request.name -AccountName $entry.AccountName -Force -ErrorAction SilentlyContinue | Out-Null
    Unblock-SmbShareAccess -Name $request.name -AccountName $entry.AccountName -Force -ErrorAction SilentlyContinue | Out-Null
  }
  foreach ($entry in @($request.acl)) {
    if ($entry.level -eq 'deny') {
      Block-SmbShareAccess -Name $request.name -AccountName $entry.principal -Force | Out-Null
    } else {
      $right = switch ($entry.level) { 'full' {'Full'} 'change' {'Change'} default {'Read'} }
      Grant-SmbShareAccess -Name $request.name -AccountName $entry.principal -AccessRight $right -Force | Out-Null
    }
  }
} else {
  foreach ($entry in @(Get-NfsSharePermission -Name $request.name)) {
    Revoke-NfsSharePermission -Name $request.name -ClientName $entry.ClientName -ClientType $entry.ClientType -ErrorAction SilentlyContinue | Out-Null
  }
  $permission = if ($request.read_only) { 'readonly' } else { 'readwrite' }
  foreach ($client in @($request.clients)) {
    Grant-NfsSharePermission -Name $request.name -ClientName $client -ClientType host -Permission $permission -AllowRootAccess:$false | Out-Null
  }
}
`

const windowsDeleteSMBScript = `
$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
Remove-SmbShare -Name $request.name -Force
`

const windowsDeleteNFSScript = `
$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
Remove-NfsShare -Name $request.name -Confirm:$false
`

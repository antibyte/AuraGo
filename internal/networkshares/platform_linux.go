//go:build linux

package networkshares

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const linuxNFSExportsDir = "/etc/exports.d"

type linuxAdapter struct {
	runner commandRunner
	logger *slog.Logger
}

func platformSupported() bool {
	return true
}

func newPlatformAdapter(runner commandRunner, logger *slog.Logger) platformAdapter {
	return &linuxAdapter{runner: runner, logger: logger}
}

func (a *linuxAdapter) Probe(ctx context.Context, options Options) (Status, error) {
	status := Status{Supported: true}
	if options.SMBEnabled {
		status.SMB = a.probeSMB(ctx, options)
	} else {
		status.SMB.ReasonCode = "protocol_disabled"
		status.SMB.Reason = "SMB management is disabled in the AuraGo configuration."
	}
	if options.NFSEnabled {
		status.NFS = a.probeNFS(ctx, options)
	} else {
		status.NFS.ReasonCode = "protocol_disabled"
		status.NFS.Reason = "NFS management is disabled in the AuraGo configuration."
	}
	status.Usable = status.SMB.Readable || status.NFS.Readable
	status.ReasonCode = firstNonEmpty(status.SMB.ReasonCode, status.NFS.ReasonCode)
	status.Reason = firstNonEmpty(status.SMB.Reason, status.NFS.Reason)
	return status, nil
}

func (a *linuxAdapter) Validate(ctx context.Context, options Options, share ShareSpec) error {
	if share.Protocol != ProtocolSMB || len(share.Access.ACL) == 0 {
		return nil
	}
	if _, err := a.runner.LookPath("getent"); err != nil {
		return codedError(ErrorUnavailable, "SMB principal validation requires the operating-system getent command.", err)
	}
	for _, entry := range share.Access.ACL {
		principal := strings.TrimPrefix(strings.TrimSpace(entry.Principal), "@")
		_, userErr := a.runner.Run(ctx, options, false, "getent", []string{"passwd", principal}, nil)
		if userErr == nil {
			continue
		}
		_, groupErr := a.runner.Run(ctx, options, false, "getent", []string{"group", principal}, nil)
		if groupErr != nil {
			return codedError(ErrorInvalidArgument,
				fmt.Sprintf("SMB principal %q is not an existing operating-system user or group.", entry.Principal), nil)
		}
	}
	return nil
}

func (a *linuxAdapter) probeSMB(ctx context.Context, options Options) ProtocolStatus {
	status := ProtocolStatus{Supported: true, Backend: "samba"}
	_, netErr := a.runner.LookPath("net")
	_, testparmErr := a.runner.LookPath("testparm")
	if netErr != nil && testparmErr != nil {
		status.ReasonCode = "not_installed"
		status.Reason = "Samba net and testparm are not installed."
		return status
	}
	status.Installed = true
	registryReadable := false
	if testparmErr == nil {
		if _, err := a.runRead(ctx, options, "testparm", "-s", "--suppress-prompt"); err == nil {
			status.Readable = true
		} else {
			status.ReasonCode = "not_readable"
			status.Reason = "The effective Samba file-share configuration is not readable."
		}
	}
	if netErr == nil {
		if _, err := a.runRead(ctx, options, "net", "conf", "listshares"); err == nil {
			registryReadable = true
		}
	}
	if !status.Readable && registryReadable {
		status.Readable = true
	}
	if netErr == nil {
		if output, err := a.runRead(ctx, options, "net", "--version"); err == nil {
			status.Version = strings.TrimSpace(string(output))
		}
	} else if output, err := a.runRead(ctx, options, "testparm", "--version"); err == nil {
		status.Version = strings.TrimSpace(string(output))
	}
	if testparmErr == nil {
		if output, runErr := a.runRead(ctx, options, "testparm", "-s", "--parameter-name=registry shares"); runErr == nil {
			status.Configured = strings.EqualFold(strings.TrimSpace(string(output)), "yes")
		}
	}
	if !status.Configured && status.Reason == "" {
		status.ReasonCode = "registry_shares_disabled"
		status.Reason = "Samba is installed, but registry shares = yes is not enabled in smb.conf."
	}
	if _, err := a.runner.LookPath("smbcontrol"); err == nil {
		_, pingErr := a.runRead(ctx, options, "smbcontrol", "all", "ping")
		status.ServiceActive = pingErr == nil
	}
	if !status.ServiceActive {
		status.ServiceActive = a.systemdServiceActive(ctx, options, "smbd", "smb")
	}
	if status.Configured && !status.ServiceActive && status.Reason == "" {
		status.ReasonCode = "service_inactive"
		status.Reason = "Samba is configured, but no active smbd service was detected."
	}
	status.Writable = status.Readable && registryReadable && status.Configured && status.ServiceActive &&
		a.hostWritePrivilegeAvailable(ctx, options)
	if status.Writable {
		status.ReasonCode = ""
		status.Reason = ""
	} else if status.Readable && status.Configured && status.ServiceActive {
		if !registryReadable {
			status.ReasonCode = "backend_unwritable"
			status.Reason = "Samba registry shares are not writable by the current process."
		} else {
			status.ReasonCode = linuxWriteRestrictionCode(options)
			status.Reason = linuxWriteRestrictionReason(options)
		}
	}
	return status
}

func (a *linuxAdapter) probeNFS(ctx context.Context, options Options) ProtocolStatus {
	status := ProtocolStatus{Supported: true, Backend: "nfs-utils"}
	if _, err := a.runner.LookPath("exportfs"); err != nil {
		status.ReasonCode = "not_installed"
		status.Reason = "nfs-utils exportfs is not installed."
		return status
	}
	status.Installed = true
	if output, err := a.runRead(ctx, options, "exportfs", "-s"); err == nil {
		_ = output
		status.Readable = true
	} else {
		status.ReasonCode = "not_readable"
		status.Reason = "The NFS export table is not readable."
	}
	if info, err := os.Stat(linuxNFSExportsDir); err == nil && info.IsDir() {
		status.Configured = true
	} else if status.Reason == "" {
		status.ReasonCode = "not_configured"
		status.Reason = "/etc/exports.d is not available."
	}
	status.ServiceActive = a.systemdServiceActive(ctx, options, "nfs-server", "nfs-kernel-server")
	if !status.ServiceActive && status.Configured && status.Reason == "" {
		status.ReasonCode = "service_inactive"
		status.Reason = "No active NFS server service was detected."
	}
	status.Writable = status.Readable && status.Configured && status.ServiceActive &&
		a.hostWritePrivilegeAvailable(ctx, options)
	if status.Writable {
		status.ReasonCode = ""
		status.Reason = ""
	} else if status.Readable && status.Configured && status.ServiceActive {
		status.ReasonCode = linuxWriteRestrictionCode(options)
		status.Reason = linuxWriteRestrictionReason(options)
	}
	return status
}

func hostWritesAllowed(options Options) bool {
	if options.IsDocker || options.NoNewPrivileges || options.ProtectSystemStrict {
		return false
	}
	if options.ReadOnly || (!options.AllowCreate && !options.AllowUpdate && !options.AllowDelete) {
		return false
	}
	if platformElevated() {
		return true
	}
	return options.SudoEnabled && options.SudoUnrestricted && !options.NoNewPrivileges && !options.ProtectSystemStrict
}

func (a *linuxAdapter) hostWritePrivilegeAvailable(ctx context.Context, options Options) bool {
	if !hostWritesAllowed(options) {
		return false
	}
	if platformElevated() {
		return true
	}
	_, err := a.runner.Run(ctx, options, true, "true", nil, nil)
	return err == nil
}

func linuxWriteRestrictionReason(options Options) string {
	switch {
	case options.ReadOnly:
		return "Host mutations are disabled by network_shares.readonly."
	case !options.AllowCreate && !options.AllowUpdate && !options.AllowDelete:
		return "No network share mutation permission is enabled."
	case options.IsDocker:
		return "Network share mutations are unavailable in the standard Docker deployment."
	case options.NoNewPrivileges:
		return "Network share mutations are blocked by NoNewPrivileges."
	case options.ProtectSystemStrict:
		return "Network share mutations are blocked by ProtectSystem=strict."
	case !platformElevated() && (!options.SudoEnabled || !options.SudoUnrestricted):
		return "Network share mutations require root or unrestricted AuraGo sudo."
	default:
		return "Network share mutations are unavailable with the current host privileges."
	}
}

func linuxWriteRestrictionCode(options Options) string {
	switch {
	case options.ReadOnly:
		return "readonly"
	case !options.AllowCreate && !options.AllowUpdate && !options.AllowDelete:
		return "permission_disabled"
	case options.IsDocker:
		return "docker_restricted"
	case options.NoNewPrivileges:
		return "no_new_privileges"
	case options.ProtectSystemStrict:
		return "protect_system_strict"
	default:
		return "privilege_required"
	}
}

func (a *linuxAdapter) systemdServiceActive(ctx context.Context, options Options, names ...string) bool {
	if _, err := a.runner.LookPath("systemctl"); err != nil {
		return false
	}
	for _, name := range names {
		if _, err := a.runRead(ctx, options, "systemctl", "is-active", "--quiet", name); err == nil {
			return true
		}
	}
	return false
}

func (a *linuxAdapter) runRead(ctx context.Context, options Options, name string, args ...string) ([]byte, error) {
	output, err := a.runner.Run(ctx, options, false, name, args, nil)
	if err == nil || !canUseSudo(options) {
		return output, err
	}
	return a.runner.Run(ctx, options, true, name, args, nil)
}

func canUseSudo(options Options) bool {
	return platformElevated() || (options.SudoEnabled && options.SudoUnrestricted && !options.NoNewPrivileges && !options.ProtectSystemStrict)
}

func (a *linuxAdapter) List(ctx context.Context, options Options) ([]observedShare, error) {
	var shares []observedShare
	if options.SMBEnabled {
		smb, err := a.listSMB(ctx, options)
		if err != nil {
			return nil, err
		}
		shares = append(shares, smb...)
	}
	if options.NFSEnabled {
		nfs, err := a.listNFS(ctx, options)
		if err != nil {
			return nil, err
		}
		shares = append(shares, nfs...)
	}
	return shares, nil
}

func (a *linuxAdapter) listSMB(ctx context.Context, options Options) ([]observedShare, error) {
	if _, pathErr := a.runner.LookPath("testparm"); pathErr == nil {
		effective, testErr := a.runRead(ctx, options, "testparm", "-s", "--suppress-prompt")
		if testErr == nil {
			var shares []observedShare
			for name, values := range parseSambaSections(string(effective)) {
				if share, ok := observedSMBShare(name, values); ok {
					shares = append(shares, share)
				}
			}
			sort.Slice(shares, func(i, j int) bool {
				return strings.ToLower(shares[i].Name) < strings.ToLower(shares[j].Name)
			})
			return shares, nil
		}
	}

	output, err := a.runRead(ctx, options, "net", "conf", "listshares")
	if err != nil {
		return nil, fmt.Errorf("list effective Samba shares: %w", err)
	}
	var registryShares []observedShare
	for _, name := range strings.Fields(string(output)) {
		name = strings.TrimSpace(name)
		if sambaSystemShare(name) {
			continue
		}
		detail, detailErr := a.runRead(ctx, options, "net", "conf", "showshare", name)
		if detailErr != nil {
			return nil, fmt.Errorf("show Samba registry share %q: %w", name, detailErr)
		}
		if share, ok := observedSMBShare(name, parseSambaShare(string(detail))); ok {
			registryShares = append(registryShares, share)
		}
	}

	return registryShares, nil
}

func observedSMBShare(name string, values map[string]string) (observedShare, bool) {
	if sambaSystemShare(name) {
		return observedShare{}, false
	}
	path := values["path"]
	if path == "" || parseYes(values["printable"]) || parseYes(values["print ok"]) {
		return observedShare{}, false
	}
	comment := values["comment"]
	readOnly := true
	if raw, exists := values["read only"]; exists {
		readOnly = parseYes(raw)
	}
	if writable, exists := values["writeable"]; exists {
		readOnly = !parseYes(writable)
	}
	return observedShare{
		ShareSpec: ShareSpec{
			Protocol: ProtocolSMB,
			Name:     name,
			Path:     filepath.Clean(path),
			Comment:  comment,
			ReadOnly: readOnly,
			Access: ShareAccess{
				Guest: parseYes(values["guest ok"]),
				ACL:   sambaACL(values),
			},
		},
		MarkerID:         markerID(comment),
		MarkerSupported:  true,
		UnsafeAdminUsers: len(splitSambaList(values["admin users"])) > 0,
		Active:           true,
		CommentObserved:  true,
	}, true
}

func sambaSystemShare(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || strings.HasSuffix(name, "$") {
		return true
	}
	switch name {
	case "global", "homes", "printers", "netlogon", "sysvol":
		return true
	default:
		return false
	}
}

func parseSambaShare(raw string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	return values
}

func parseSambaSections(raw string) map[string]map[string]string {
	sections := make(map[string]map[string]string)
	var current map[string]string
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if name == "" {
				current = nil
				continue
			}
			current = make(map[string]string)
			sections[name] = current
			continue
		}
		if current == nil {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			current[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	return sections
}

func sambaACL(values map[string]string) []ACLEntry {
	levels := make(map[string]string)
	apply := func(raw, level string) {
		for _, principal := range splitSambaList(raw) {
			key := strings.ToLower(principal)
			if _, exists := levels[key]; !exists || aclPriority(level) > aclPriority(levels[key]) {
				levels[key] = level + "\x00" + principal
			}
		}
	}
	apply(values["valid users"], "read")
	apply(values["read list"], "read")
	apply(values["write list"], "change")
	apply(values["admin users"], "full")
	apply(values["invalid users"], "deny")
	acl := make([]ACLEntry, 0, len(levels))
	for _, encoded := range levels {
		parts := strings.SplitN(encoded, "\x00", 2)
		acl = append(acl, ACLEntry{Principal: parts[1], Level: parts[0]})
	}
	sort.Slice(acl, func(i, j int) bool {
		return strings.ToLower(acl[i].Principal) < strings.ToLower(acl[j].Principal)
	})
	return acl
}

func aclPriority(level string) int {
	switch level {
	case "deny":
		return 4
	case "full":
		return 3
	case "change":
		return 2
	default:
		return 1
	}
}

func splitSambaList(raw string) []string {
	var result []string
	var current strings.Builder
	var quote rune
	flush := func() {
		value := strings.TrimSpace(current.String())
		if value != "" {
			result = append(result, value)
		}
		current.Reset()
	}
	for _, char := range raw {
		switch {
		case quote != 0 && char == quote:
			quote = 0
		case quote == 0 && (char == '"' || char == '\''):
			quote = char
		case quote == 0 && (char == ',' || char == ' ' || char == '\t'):
			flush()
		default:
			current.WriteRune(char)
		}
	}
	flush()
	return result
}

func joinSambaList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ReplaceAll(value, `"`, `\"`)
		if strings.ContainsAny(value, " \t,") {
			value = `"` + value + `"`
		}
		quoted = append(quoted, value)
	}
	return strings.Join(quoted, " ")
}

func parseYes(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes", "true", "1", "y":
		return true
	default:
		return false
	}
}

func (a *linuxAdapter) Create(ctx context.Context, options Options, share ShareSpec) error {
	switch share.Protocol {
	case ProtocolSMB:
		return a.smbAdd(ctx, options, share)
	case ProtocolNFS:
		return a.writeNFSShare(ctx, options, share)
	default:
		return codedError(ErrorInvalidArgument, "Unknown share protocol.", nil)
	}
}

func (a *linuxAdapter) Update(ctx context.Context, options Options, previous, desired ShareSpec) error {
	var err error
	switch desired.Protocol {
	case ProtocolSMB:
		err = a.applySMBParameters(ctx, options, desired)
		if err == nil {
			err = a.reloadSMB(ctx, options)
		}
	case ProtocolNFS:
		err = a.writeNFSShare(ctx, options, desired)
	default:
		err = codedError(ErrorInvalidArgument, "Unknown share protocol.", nil)
	}
	_ = previous
	return err
}

func (a *linuxAdapter) Delete(ctx context.Context, options Options, share ShareSpec) error {
	switch share.Protocol {
	case ProtocolSMB:
		if err := a.smbDeleteRaw(ctx, options, share.Name); err != nil {
			return err
		}
		return a.reloadSMB(ctx, options)
	case ProtocolNFS:
		if err := a.removeNFSShareFile(ctx, options, share.ID); err != nil {
			return err
		}
		return a.reloadNFS(ctx, options)
	default:
		return codedError(ErrorInvalidArgument, "Unknown share protocol.", nil)
	}
}

func (a *linuxAdapter) smbAdd(ctx context.Context, options Options, share ShareSpec) error {
	writeable := "writeable=y"
	if share.ReadOnly {
		writeable = "writeable=n"
	}
	guest := "guest_ok=n"
	if share.Access.Guest {
		guest = "guest_ok=y"
	}
	if _, err := a.runner.Run(ctx, options, true, "net",
		[]string{"conf", "addshare", share.Name, share.Path, writeable, guest, share.Comment}, nil); err != nil {
		return fmt.Errorf("add Samba registry share: %w", err)
	}
	if err := a.applySMBParameters(ctx, options, share); err != nil {
		return err
	}
	return a.reloadSMB(ctx, options)
}

func (a *linuxAdapter) applySMBParameters(ctx context.Context, options Options, share ShareSpec) error {
	params := map[string]string{
		"path":      share.Path,
		"comment":   share.Comment,
		"read only": yesNo(share.ReadOnly),
		"guest ok":  yesNo(share.Access.Guest),
	}
	aclValues := map[string][]string{
		"valid users":   {},
		"read list":     {},
		"write list":    {},
		"admin users":   {},
		"invalid users": {},
	}
	for _, entry := range share.Access.ACL {
		aclValues["valid users"] = append(aclValues["valid users"], entry.Principal)
		switch entry.Level {
		case "read":
			aclValues["read list"] = append(aclValues["read list"], entry.Principal)
		case "change":
			aclValues["write list"] = append(aclValues["write list"], entry.Principal)
		case "full":
			// Samba admin users bypasses filesystem permissions. Treat full as
			// the safe change/write-list compatibility alias on Linux.
			aclValues["write list"] = append(aclValues["write list"], entry.Principal)
		case "deny":
			aclValues["invalid users"] = append(aclValues["invalid users"], entry.Principal)
		}
	}
	for key, values := range aclValues {
		if len(values) > 0 {
			params[key] = joinSambaList(values)
			continue
		}
		_, _ = a.runner.Run(ctx, options, true, "net", []string{"conf", "delparm", share.Name, key}, nil)
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := a.runner.Run(ctx, options, true, "net",
			[]string{"conf", "setparm", share.Name, key, params[key]}, nil); err != nil {
			return fmt.Errorf("set Samba parameter %q: %w", key, err)
		}
	}
	return nil
}

func (a *linuxAdapter) smbDeleteRaw(ctx context.Context, options Options, name string) error {
	_, err := a.runner.Run(ctx, options, true, "net", []string{"conf", "delshare", name}, nil)
	if err != nil {
		return fmt.Errorf("delete Samba registry share: %w", err)
	}
	return nil
}

func (a *linuxAdapter) reloadSMB(ctx context.Context, options Options) error {
	if _, err := a.runner.LookPath("smbcontrol"); err != nil {
		return fmt.Errorf("smbcontrol is required to reload Samba configuration")
	}
	if _, err := a.runner.Run(ctx, options, true, "smbcontrol", []string{"all", "reload-config"}, nil); err != nil {
		return fmt.Errorf("reload Samba configuration: %w", err)
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func (a *linuxAdapter) listNFS(ctx context.Context, options Options) ([]observedShare, error) {
	output, err := a.runRead(ctx, options, "exportfs", "-s")
	if err != nil {
		return nil, fmt.Errorf("list NFS exports: %w", err)
	}
	active := parseExportFS(string(output))
	managed, err := readManagedNFSFiles()
	if err != nil {
		return nil, err
	}
	var shares []observedShare
	managedPaths := make(map[string]bool)
	for _, share := range managed {
		managedPaths[share.Path] = true
		native, ok := active[share.Path]
		if ok {
			share.Active = true
			share.Access.Clients = native.Access.Clients
			share.ReadOnly = native.ReadOnly
		}
		shares = append(shares, share)
	}
	for path, share := range active {
		if managedPaths[path] {
			continue
		}
		shares = append(shares, share)
	}
	return shares, nil
}

func parseExportFS(raw string) map[string]observedShare {
	shares := make(map[string]observedShare)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		path := unescapeExportsPath(fields[0])
		current, exists := shares[path]
		if !exists {
			current = observedShare{
				ShareSpec: ShareSpec{
					Protocol: ProtocolNFS,
					Name:     filepath.Base(path),
					Path:     path,
					ReadOnly: true,
				},
				MarkerSupported: true,
				Active:          true,
			}
		}
		for _, field := range fields[1:] {
			open := strings.LastIndex(field, "(")
			close := strings.LastIndex(field, ")")
			if open <= 0 || close <= open {
				continue
			}
			client := strings.TrimSpace(field[:open])
			options := strings.Split(field[open+1:close], ",")
			if client != "" {
				current.Access.Clients = append(current.Access.Clients, client)
			}
			for _, option := range options {
				if strings.TrimSpace(option) == "rw" {
					current.ReadOnly = false
				}
			}
		}
		current.Access.Clients = normalizeClients(current.Access.Clients)
		shares[path] = current
	}
	return shares
}

func readManagedNFSFiles() ([]observedShare, error) {
	entries, err := os.ReadDir(linuxNFSExportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read AuraGo NFS export directory: %w", err)
	}
	var shares []observedShare
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "aurago-") || !strings.HasSuffix(entry.Name(), ".exports") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(linuxNFSExportsDir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read managed NFS export %q: %w", entry.Name(), readErr)
		}
		lines := strings.SplitN(string(data), "\n", 2)
		const prefix = "# aurago-managed "
		if len(lines) == 0 || !strings.HasPrefix(lines[0], prefix) {
			continue
		}
		encoded := strings.TrimSpace(strings.TrimPrefix(lines[0], prefix))
		raw, decodeErr := base64.RawURLEncoding.DecodeString(encoded)
		if decodeErr != nil {
			continue
		}
		var spec ShareSpec
		if jsonErr := json.Unmarshal(raw, &spec); jsonErr != nil || spec.ID == "" {
			continue
		}
		shares = append(shares, observedShare{
			ShareSpec:       spec,
			MarkerID:        spec.ID,
			MarkerSupported: true,
			Active:          false,
			CommentObserved: true,
		})
	}
	return shares, nil
}

func (a *linuxAdapter) writeNFSShare(ctx context.Context, options Options, share ShareSpec) error {
	content, err := renderNFSExport(share)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp("", "aurago-nfs-*.exports")
	if err != nil {
		return fmt.Errorf("create temporary NFS export: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure temporary NFS export: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary NFS export: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary NFS export: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary NFS export: %w", err)
	}
	target := nfsShareFile(share.ID)
	staging := nfsStagingFile(share.ID)
	if _, err := a.runner.Run(ctx, options, true, "install", []string{"-m", "0644", "--", tempPath, staging}, nil); err != nil {
		return fmt.Errorf("stage managed NFS export: %w", err)
	}
	if _, err := a.runner.Run(ctx, options, true, "mv", []string{"-f", "--", staging, target}, nil); err != nil {
		_, _ = a.runner.Run(ctx, options, true, "rm", []string{"-f", "--", staging}, nil)
		return fmt.Errorf("publish managed NFS export atomically: %w", err)
	}
	return a.reloadNFS(ctx, options)
}

func renderNFSExport(share ShareSpec) ([]byte, error) {
	raw, err := json.Marshal(share)
	if err != nil {
		return nil, fmt.Errorf("encode managed NFS metadata: %w", err)
	}
	options := "sync,root_squash,no_subtree_check,ro"
	if !share.ReadOnly {
		options = "sync,root_squash,no_subtree_check,rw"
	}
	clients := make([]string, 0, len(share.Access.Clients))
	for _, client := range share.Access.Clients {
		clients = append(clients, fmt.Sprintf("%s(%s)", client, options))
	}
	content := "# aurago-managed " + base64.RawURLEncoding.EncodeToString(raw) + "\n" +
		escapeExportsPath(share.Path) + " " + strings.Join(clients, " ") + "\n"
	return []byte(content), nil
}

func (a *linuxAdapter) removeNFSShareFile(ctx context.Context, options Options, id string) error {
	if !validManagedID(id) {
		return codedError(ErrorInvalidArgument, "Invalid managed share ID.", nil)
	}
	if _, err := a.runner.Run(ctx, options, true, "rm", []string{"-f", "--", nfsShareFile(id)}, nil); err != nil {
		return fmt.Errorf("remove managed NFS export: %w", err)
	}
	return nil
}

func (a *linuxAdapter) reloadNFS(ctx context.Context, options Options) error {
	if _, err := a.runner.Run(ctx, options, true, "exportfs", []string{"-ra"}, nil); err != nil {
		return fmt.Errorf("reload NFS exports: %w", err)
	}
	return nil
}

func nfsShareFile(id string) string {
	return filepath.Join(linuxNFSExportsDir, "aurago-"+id+".exports")
}

func nfsStagingFile(id string) string {
	return filepath.Join(linuxNFSExportsDir, ".aurago-"+id+".exports.tmp")
}

func validManagedID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for index, char := range id {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}

func escapeExportsPath(path string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._+,:=@%/-"
	var escaped strings.Builder
	for index := 0; index < len(path); index++ {
		value := path[index]
		if strings.IndexByte(safe, value) >= 0 {
			escaped.WriteByte(value)
			continue
		}
		fmt.Fprintf(&escaped, `\%03o`, value)
	}
	return escaped.String()
}

func unescapeExportsPath(path string) string {
	var decoded strings.Builder
	for index := 0; index < len(path); {
		if path[index] == '\\' && index+3 < len(path) &&
			path[index+1] >= '0' && path[index+1] <= '7' &&
			path[index+2] >= '0' && path[index+2] <= '7' &&
			path[index+3] >= '0' && path[index+3] <= '7' {
			value := (path[index+1]-'0')*64 + (path[index+2]-'0')*8 + (path[index+3] - '0')
			decoded.WriteByte(value)
			index += 4
			continue
		}
		decoded.WriteByte(path[index])
		index++
	}
	return decoded.String()
}

func normalizePlatformShare(share ShareSpec) ShareSpec {
	if share.Protocol != ProtocolSMB {
		return share
	}
	for index := range share.Access.ACL {
		if share.Access.ACL[index].Level == "full" {
			share.Access.ACL[index].Level = "change"
		}
	}
	return share
}

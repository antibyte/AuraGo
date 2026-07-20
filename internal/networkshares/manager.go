package networkshares

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const markerPrefix = "[aurago:"

// Manager serializes host mutations and reconciles native state with AuraGo ownership.
type Manager struct {
	mu       sync.RWMutex
	mutateMu sync.Mutex
	adapter  platformAdapter
	ledger   *Ledger
	logger   *slog.Logger
	options  Options
	status   Status
}

var (
	defaultManagerMu sync.RWMutex
	defaultManager   *Manager
)

// OpenManager opens the durable ledger and constructs the platform adapter.
func OpenManager(path string, logger *slog.Logger) (*Manager, error) {
	ledger, err := OpenLedger(path)
	if err != nil {
		return nil, err
	}
	manager := newManager(ledger, execCommandRunner{}, logger)
	return manager, nil
}

func newManager(ledger *Ledger, runner commandRunner, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		adapter: newPlatformAdapter(runner, logger),
		ledger:  ledger,
		logger:  logger,
		status:  unavailableStatus("Network shares have not been probed yet."),
	}
}

// SetDefaultManager shares one manager between agent dispatch and the admin API.
func SetDefaultManager(manager *Manager) {
	defaultManagerMu.Lock()
	defaultManager = manager
	defaultManagerMu.Unlock()
}

// DefaultManager returns the process-wide manager.
func DefaultManager() *Manager {
	defaultManagerMu.RLock()
	defer defaultManagerMu.RUnlock()
	return defaultManager
}

// Configure atomically replaces the runtime policy.
func (m *Manager) Configure(options Options) {
	if m == nil {
		return
	}
	options = normalizeOptions(options)
	m.mu.Lock()
	m.options = options
	m.mu.Unlock()
}

// Status returns the latest passive capability snapshot.
func (m *Manager) Status() Status {
	if m == nil {
		return unavailableStatus("Network share manager is unavailable.")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStatus(m.status)
}

// Reprobe passively refreshes host capabilities and reconciles no host state.
func (m *Manager) Reprobe(ctx context.Context) Status {
	if m == nil {
		return unavailableStatus("Network share manager is unavailable.")
	}
	m.mu.RLock()
	options := m.options
	m.mu.RUnlock()

	status := unavailableStatus("")
	status.Supported = platformSupported()
	status.AllowedRoots = rootStatuses(options.AllowedRoots)
	if !options.Enabled {
		status.Reason = "Local network share management is disabled in the AuraGo configuration."
		m.storeStatus(status)
		return status
	}
	if !status.Supported {
		status.Reason = "Local network share management is supported only on Linux and Windows."
		m.storeStatus(status)
		return status
	}

	probed, err := m.adapter.Probe(ctx, options)
	if err != nil {
		status.Reason = "The installed SMB and NFS backends could not be probed."
		m.logger.Warn("[NetworkShares] Runtime probe failed", "error", err)
		m.storeStatus(status)
		return status
	}
	probedAt := time.Now().UTC()
	probed.AllowedRoots = status.AllowedRoots
	probed.LastProbedAt = probedAt
	probed.SMB.LastProbedAt = probedAt
	probed.NFS.LastProbedAt = probedAt
	probed.Supported = true
	probed.Usable = (options.SMBEnabled && probed.SMB.Readable) || (options.NFSEnabled && probed.NFS.Readable)
	if !probed.Usable && probed.Reason == "" {
		probed.Reason = firstNonEmpty(probed.SMB.Reason, probed.NFS.Reason, "No enabled SMB or NFS backend is readable.")
	}
	if !hasAvailableRoot(probed.AllowedRoots) {
		probed.Reason = "No configured allowed root is currently an accessible directory; share mutations are unavailable."
		probed.SMB.Writable = false
		probed.NFS.Writable = false
	}
	m.storeStatus(probed)
	m.logger.Info("[NetworkShares] Runtime probe completed",
		"usable", probed.Usable,
		"smb_readable", probed.SMB.Readable,
		"smb_writable", probed.SMB.Writable,
		"nfs_readable", probed.NFS.Readable,
		"nfs_writable", probed.NFS.Writable)
	return cloneStatus(probed)
}

func (m *Manager) storeStatus(status Status) {
	probedAt := time.Now().UTC()
	status.LastProbedAt = probedAt
	status.SMB.LastProbedAt = probedAt
	status.NFS.LastProbedAt = probedAt
	m.mu.Lock()
	m.status = status
	m.mu.Unlock()
}

// Validate checks a desired share without mutating the host.
func (m *Manager) Validate(ctx context.Context, desired ShareSpec, operation string) (ShareSpec, error) {
	if m == nil {
		return ShareSpec{}, codedError(ErrorUnavailable, "Network share manager is unavailable.", nil)
	}
	m.mu.RLock()
	options := m.options
	status := m.status
	m.mu.RUnlock()
	desired, err := validateShareSpec(ctx, desired, operation, options, status)
	if err != nil {
		return ShareSpec{}, err
	}
	if err := m.adapter.Validate(ctx, options, desired); err != nil {
		return ShareSpec{}, err
	}
	return desired, nil
}

// List returns only shares whose canonical paths are inside configured roots.
func (m *Manager) List(ctx context.Context) ([]Share, error) {
	options, status, err := m.requireReadable()
	if err != nil {
		return nil, err
	}
	observed, err := m.adapter.List(ctx, readableProtocolOptions(options, status))
	if err != nil {
		return nil, wrapApplyError("list", err)
	}
	records, err := m.ledger.list(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := make([]Share, 0, len(observed)+len(records))
	used := make(map[int]bool)

	for _, record := range records {
		if !isPathInScope(record.Spec.Path, options.AllowedRoots) {
			if setErr := m.ledger.setDrift(ctx, record.Spec.ID, "outside_allowed_roots"); setErr != nil {
				m.logger.Warn("[NetworkShares] Could not persist out-of-scope reconciliation state",
					"share_id", record.Spec.ID, "error", setErr)
			}
			continue
		}
		match := -1
		for index, native := range observed {
			if used[index] || !strings.EqualFold(native.Protocol, record.Spec.Protocol) {
				continue
			}
			if native.MarkerID == record.Spec.ID ||
				(native.MarkerID == "" && strings.EqualFold(native.Name, record.Spec.Name) && samePath(native.Path, record.Spec.Path)) {
				match = index
				break
			}
		}
		share := Share{
			ShareSpec:  record.Spec,
			Managed:    true,
			Active:     false,
			ObservedAt: now,
			Drift:      firstNonEmpty(record.Drift, "missing"),
		}
		if match >= 0 {
			used[match] = true
			native := observed[match]
			share.Active = native.Active
			if native.CommentObserved {
				share.Comment = stripNativeMarker(native.Comment)
			}
			share.ReadOnly = native.ReadOnly
			share.Access = native.Access
			share.Drift = compareDesired(record.Spec, native)
		}
		share.Mutable = share.Active && share.Drift == "" && protocolWritable(status, share.Protocol)
		if setErr := m.ledger.setDrift(ctx, record.Spec.ID, share.Drift); setErr != nil {
			m.logger.Warn("[NetworkShares] Could not persist reconciliation state", "share_id", record.Spec.ID, "error", setErr)
		}
		result = append(result, share)
	}

	for index, native := range observed {
		if used[index] || !native.Active || !isPathInScope(native.Path, options.AllowedRoots) {
			continue
		}
		if native.MarkerID != "" {
			// An AuraGo marker without a ledger entry is intentionally not adopted.
			native.Comment = stripNativeMarker(native.Comment)
		}
		native.ID = externalShareID(native)
		result = append(result, Share{
			ShareSpec:  native.ShareSpec,
			Managed:    false,
			Mutable:    false,
			Active:     true,
			Drift:      "",
			ObservedAt: now,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Protocol == result[j].Protocol {
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		}
		return result[i].Protocol < result[j].Protocol
	})
	return result, nil
}

// Get returns one scoped share by stable ID.
func (m *Manager) Get(ctx context.Context, id string) (Share, error) {
	shares, err := m.List(ctx)
	if err != nil {
		return Share{}, err
	}
	for _, share := range shares {
		if share.ID == id {
			return share, nil
		}
	}
	return Share{}, codedError(ErrorNotFound, "The network share was not found.", nil)
}

// Create applies and verifies one new host share.
func (m *Manager) Create(ctx context.Context, desired ShareSpec) (Share, error) {
	m.mutateMu.Lock()
	defer m.mutateMu.Unlock()

	options, status, err := m.requireOperation("create", desired.Protocol)
	if err != nil {
		return Share{}, err
	}
	desired, err = validateShareSpec(ctx, desired, "create", options, status)
	if err != nil {
		return Share{}, err
	}
	if err := m.adapter.Validate(ctx, options, desired); err != nil {
		return Share{}, err
	}
	records, err := m.ledger.list(ctx)
	if err != nil {
		return Share{}, err
	}
	for _, record := range records {
		if shareIdentityConflicts(record.Spec, desired) {
			return Share{}, codedError(ErrorConflict, "An AuraGo-managed share already uses this protocol name or NFS path.", nil)
		}
	}
	existing, err := m.adapter.List(ctx, singleProtocolOptions(options, desired.Protocol))
	if err != nil {
		return Share{}, wrapApplyError("inspect existing", err)
	}
	for _, share := range existing {
		if shareIdentityConflicts(share.ShareSpec, desired) {
			return Share{}, codedError(ErrorConflict, "A share with this protocol and name already exists.", nil)
		}
	}
	desired.ID, err = newShareID()
	if err != nil {
		return Share{}, fmt.Errorf("generate network share id: %w", err)
	}
	desired.Comment = nativeComment(desired.Comment, desired.ID)
	if err := m.adapter.Create(ctx, options, desired); err != nil {
		return Share{}, wrapApplyError("create", err)
	}
	if !m.verifyObserved(ctx, options, desired, true) {
		if rollbackErr := m.adapter.Delete(ctx, options, desired); rollbackErr != nil {
			m.recordCreateRollbackFailure(ctx, desired, rollbackErr)
		}
		return Share{}, codedError(ErrorApplyFailed, "The share backend did not report the newly created share.", nil)
	}
	ledgerSpec := desired
	ledgerSpec.Comment = stripNativeMarker(desired.Comment)
	if err := m.ledger.put(ctx, ledgerSpec, ""); err != nil {
		if rollbackErr := m.adapter.Delete(ctx, options, desired); rollbackErr != nil {
			m.recordCreateRollbackFailure(ctx, desired, rollbackErr)
		}
		return Share{}, codedError(ErrorApplyFailed, "The created share ownership record could not be committed.", err)
	}
	return m.Get(ctx, desired.ID)
}

func (m *Manager) recordCreateRollbackFailure(ctx context.Context, desired ShareSpec, rollbackErr error) {
	ledgerSpec := desired
	ledgerSpec.Comment = stripNativeMarker(desired.Comment)
	if ledgerErr := m.ledger.put(ctx, ledgerSpec, "rollback_failed"); ledgerErr != nil {
		m.logger.Error("[NetworkShares] Create rollback and drift recording failed",
			"share_id", desired.ID,
			"rollback_error", rollbackErr,
			"ledger_error", ledgerErr)
		return
	}
	m.logger.Error("[NetworkShares] Create rollback failed; share locked as drifted",
		"share_id", desired.ID,
		"error", rollbackErr)
}

// Update changes comment, read-only state, and access without changing identity.
func (m *Manager) Update(ctx context.Context, id string, patch SharePatch) (Share, error) {
	m.mutateMu.Lock()
	defer m.mutateMu.Unlock()

	current, record, options, status, err := m.requireManagedMutable(ctx, id, "update")
	if err != nil {
		return Share{}, err
	}
	desired := record.Spec
	if patch.Comment != nil {
		desired.Comment = *patch.Comment
	}
	if patch.ReadOnly != nil {
		desired.ReadOnly = *patch.ReadOnly
	}
	if patch.Access != nil {
		desired.Access = *patch.Access
	}
	desired, err = validateShareSpec(ctx, desired, "update", options, status)
	if err != nil {
		return Share{}, err
	}
	if err := m.adapter.Validate(ctx, options, desired); err != nil {
		return Share{}, err
	}
	nativePrevious := current.ShareSpec
	nativePrevious.Comment = nativeComment(record.Spec.Comment, record.Spec.ID)
	nativeDesired := desired
	nativeDesired.Comment = nativeComment(desired.Comment, desired.ID)
	if err := m.adapter.Update(ctx, options, nativePrevious, nativeDesired); err != nil {
		return Share{}, wrapApplyError("update", err)
	}
	if !m.verifyObserved(ctx, options, nativeDesired, true) {
		if rollbackErr := m.adapter.Update(ctx, options, nativeDesired, nativePrevious); rollbackErr != nil {
			_ = m.ledger.setDrift(ctx, id, "rollback_failed")
		}
		return Share{}, codedError(ErrorApplyFailed, "The share backend did not report the requested updated state.", nil)
	}
	if err := m.ledger.put(ctx, desired, ""); err != nil {
		if rollbackErr := m.adapter.Update(ctx, options, nativeDesired, nativePrevious); rollbackErr != nil {
			_ = m.ledger.setDrift(ctx, id, "rollback_failed")
		}
		return Share{}, codedError(ErrorApplyFailed, "The updated share ownership record could not be committed.", err)
	}
	return m.Get(ctx, id)
}

// Delete removes only the native share definition and ledger record.
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mutateMu.Lock()
	defer m.mutateMu.Unlock()

	current, record, options, _, err := m.requireManagedMutable(ctx, id, "delete")
	if err != nil {
		return err
	}
	if err := m.adapter.Delete(ctx, options, current.ShareSpec); err != nil {
		return wrapApplyError("delete", err)
	}
	if !m.verifyObserved(ctx, options, current.ShareSpec, false) {
		return codedError(ErrorApplyFailed, "The share backend still reports the deleted share.", nil)
	}
	if err := m.ledger.delete(ctx, id); err != nil {
		nativeRestore := record.Spec
		nativeRestore.Comment = nativeComment(record.Spec.Comment, record.Spec.ID)
		if rollbackErr := m.adapter.Create(ctx, options, nativeRestore); rollbackErr != nil {
			_ = m.ledger.setDrift(ctx, id, "rollback_failed")
		}
		return codedError(ErrorApplyFailed, "The deleted share ownership record could not be removed.", err)
	}
	return nil
}

func (m *Manager) requireReadable() (Options, Status, error) {
	if m == nil {
		return Options{}, Status{}, codedError(ErrorUnavailable, "Network share manager is unavailable.", nil)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.options.Enabled {
		return Options{}, m.status, codedError(ErrorDisabled, "Local network share management is disabled.", nil)
	}
	if !m.status.Usable {
		return Options{}, m.status, codedError(ErrorUnavailable, firstNonEmpty(m.status.Reason, "No network share backend is usable."), nil)
	}
	return m.options, m.status, nil
}

func (m *Manager) requireOperation(operation, protocol string) (Options, Status, error) {
	options, status, err := m.requireReadable()
	if err != nil {
		return Options{}, status, err
	}
	if options.ReadOnly {
		return Options{}, status, codedError(ErrorReadOnly, "Network share management is in read-only mode.", nil)
	}
	allowed := map[string]bool{"create": options.AllowCreate, "update": options.AllowUpdate, "delete": options.AllowDelete}[operation]
	if !allowed {
		return Options{}, status, codedError(ErrorPermissionDenied, fmt.Sprintf("Network share %s operations are disabled.", operation), nil)
	}
	if !protocolWritable(status, protocol) {
		return Options{}, status, codedError(ErrorUnavailable, fmt.Sprintf("The %s backend is not writable.", strings.ToUpper(protocol)), nil)
	}
	return options, status, nil
}

func (m *Manager) requireManagedMutable(ctx context.Context, id, operation string) (Share, ledgerRecord, Options, Status, error) {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "external:") {
		return Share{}, ledgerRecord{}, Options{}, Status{}, codedError(ErrorNotManaged, "Only AuraGo-managed shares can be changed.", nil)
	}
	record, err := m.ledger.get(ctx, id)
	if err != nil {
		return Share{}, ledgerRecord{}, Options{}, Status{}, err
	}
	options, status, err := m.requireOperation(operation, record.Spec.Protocol)
	if err != nil {
		return Share{}, ledgerRecord{}, Options{}, status, err
	}
	current, err := m.Get(ctx, id)
	if err != nil {
		return Share{}, ledgerRecord{}, Options{}, status, err
	}
	if !current.Managed {
		return Share{}, ledgerRecord{}, Options{}, status, codedError(ErrorNotManaged, "Only AuraGo-managed shares can be changed.", nil)
	}
	if current.Drift != "" || !current.Active {
		return Share{}, ledgerRecord{}, Options{}, status, codedError(ErrorDrift, "The managed share has drift and must be repaired outside AuraGo before it can be changed.", nil)
	}
	return current, record, options, status, nil
}

func validateShareSpec(_ context.Context, desired ShareSpec, operation string, options Options, status Status) (ShareSpec, error) {
	desired.Protocol = strings.ToLower(strings.TrimSpace(desired.Protocol))
	desired.Name = strings.TrimSpace(desired.Name)
	desired.Comment = strings.TrimSpace(stripNativeMarker(desired.Comment))
	if desired.Protocol != ProtocolSMB && desired.Protocol != ProtocolNFS {
		return ShareSpec{}, codedError(ErrorInvalidArgument, "protocol must be smb or nfs", nil)
	}
	if desired.Name == "" || len(desired.Name) > 80 || strings.HasSuffix(desired.Name, "$") {
		return ShareSpec{}, codedError(ErrorInvalidArgument, "Share name must contain 1 to 80 non-administrative characters.", nil)
	}
	for _, char := range desired.Name {
		if char < 32 || strings.ContainsRune(`\/[]:;|=,+*?<>`, char) {
			return ShareSpec{}, codedError(ErrorInvalidArgument, "Share name contains unsupported characters.", nil)
		}
	}
	if len(desired.Comment) > 200 {
		return ShareSpec{}, codedError(ErrorInvalidArgument, "Share comment must not exceed 200 characters.", nil)
	}
	path, err := canonicalAllowedPath(desired.Path, options.AllowedRoots)
	if err != nil {
		return ShareSpec{}, err
	}
	desired.Path = path
	if operation != "validate" && !protocolReadable(status, desired.Protocol) {
		return ShareSpec{}, codedError(ErrorUnavailable, fmt.Sprintf("The %s backend is not readable.", strings.ToUpper(desired.Protocol)), nil)
	}

	switch desired.Protocol {
	case ProtocolSMB:
		if !options.SMBEnabled {
			return ShareSpec{}, codedError(ErrorUnavailable, "SMB management is disabled.", nil)
		}
		if desired.Access.Guest && (!options.SMBAllowGuest || runtime.GOOS == "windows") {
			return ShareSpec{}, codedError(ErrorPermissionDenied, "SMB guest access is not allowed by this host configuration.", nil)
		}
		if !desired.Access.Guest && len(desired.Access.ACL) == 0 {
			return ShareSpec{}, codedError(ErrorInvalidArgument, "SMB shares require at least one allowed principal or explicit guest access.", nil)
		}
		allowed := makeStringSet(options.SMBAllowedPrincipals, true)
		seen := make(map[string]struct{})
		hasWriteAccess := false
		for index := range desired.Access.ACL {
			entry := &desired.Access.ACL[index]
			entry.Principal = strings.TrimSpace(entry.Principal)
			entry.Level = strings.ToLower(strings.TrimSpace(entry.Level))
			key := strings.ToLower(entry.Principal)
			if entry.Principal == "" || !allowed[key] {
				return ShareSpec{}, codedError(ErrorPermissionDenied, "SMB principal is not present in network_shares.smb.allowed_principals.", nil)
			}
			if _, duplicate := seen[key]; duplicate {
				return ShareSpec{}, codedError(ErrorInvalidArgument, "An SMB principal may appear only once.", nil)
			}
			seen[key] = struct{}{}
			switch entry.Level {
			case "read", "change", "full", "deny":
			default:
				return ShareSpec{}, codedError(ErrorInvalidArgument, "SMB ACL level must be read, change, full, or deny.", nil)
			}
			if entry.Level == "change" || entry.Level == "full" {
				hasWriteAccess = true
			}
			if desired.ReadOnly && (entry.Level == "change" || entry.Level == "full") {
				return ShareSpec{}, codedError(ErrorInvalidArgument, "Read-only SMB shares cannot grant change or full access.", nil)
			}
		}
		if runtime.GOOS == "windows" && !desired.ReadOnly && !hasWriteAccess {
			return ShareSpec{}, codedError(ErrorInvalidArgument, "Writable Windows SMB shares require change or full access for at least one principal.", nil)
		}
		desired.Access.Clients = nil
	case ProtocolNFS:
		if !options.NFSEnabled {
			return ShareSpec{}, codedError(ErrorUnavailable, "NFS management is disabled.", nil)
		}
		if len(desired.Access.Clients) == 0 {
			return ShareSpec{}, codedError(ErrorInvalidArgument, "NFS shares require at least one explicitly allowed client address or CIDR.", nil)
		}
		allowed := makeStringSet(options.NFSAllowedClients, false)
		clients := make([]string, 0, len(desired.Access.Clients))
		seen := make(map[string]struct{})
		for _, raw := range desired.Access.Clients {
			client, ok := canonicalClient(raw)
			if !ok || !allowed[client] {
				return ShareSpec{}, codedError(ErrorPermissionDenied, "NFS client is not present in network_shares.nfs.allowed_clients.", nil)
			}
			if _, duplicate := seen[client]; duplicate {
				continue
			}
			seen[client] = struct{}{}
			clients = append(clients, client)
		}
		sort.Strings(clients)
		desired.Access.Clients = clients
		desired.Access.Guest = false
		desired.Access.ACL = nil
	}
	return desired, nil
}

func (m *Manager) verifyObserved(ctx context.Context, options Options, expected ShareSpec, shouldExist bool) bool {
	observed, err := m.adapter.List(ctx, singleProtocolOptions(options, expected.Protocol))
	if err != nil {
		return false
	}
	for _, share := range observed {
		match := strings.EqualFold(share.Protocol, expected.Protocol) &&
			(strings.EqualFold(share.Name, expected.Name) || share.MarkerID == expected.ID)
		if !match {
			continue
		}
		if !shouldExist {
			return true
		}
		return samePath(share.Path, expected.Path) && compareDesired(expected, share) == ""
	}
	return !shouldExist
}

func readableProtocolOptions(options Options, status Status) Options {
	options.SMBEnabled = options.SMBEnabled && status.SMB.Readable
	options.NFSEnabled = options.NFSEnabled && status.NFS.Readable
	return options
}

func singleProtocolOptions(options Options, protocol string) Options {
	options.SMBEnabled = options.SMBEnabled && protocol == ProtocolSMB
	options.NFSEnabled = options.NFSEnabled && protocol == ProtocolNFS
	return options
}

func shareIdentityConflicts(existing, desired ShareSpec) bool {
	if !strings.EqualFold(existing.Protocol, desired.Protocol) {
		return false
	}
	if strings.EqualFold(existing.Name, desired.Name) {
		return true
	}
	return desired.Protocol == ProtocolNFS && samePath(existing.Path, desired.Path)
}

func compareDesired(desired ShareSpec, observed observedShare) string {
	if !samePath(desired.Path, observed.Path) {
		return "path_changed"
	}
	if desired.ReadOnly != observed.ReadOnly {
		return "read_only_changed"
	}
	if observed.CommentObserved &&
		strings.TrimSpace(stripNativeMarker(desired.Comment)) != strings.TrimSpace(stripNativeMarker(observed.Comment)) {
		return "comment_changed"
	}
	if desired.Protocol == ProtocolSMB {
		if desired.Access.Guest != observed.Access.Guest || !sameACL(desired.Access.ACL, observed.Access.ACL) {
			return "access_changed"
		}
	} else if !sameStrings(desired.Access.Clients, observed.Access.Clients) {
		return "access_changed"
	}
	return ""
}

func protocolReadable(status Status, protocol string) bool {
	if protocol == ProtocolSMB {
		return status.SMB.Readable
	}
	if protocol == ProtocolNFS {
		return status.NFS.Readable
	}
	return false
}

func protocolWritable(status Status, protocol string) bool {
	if protocol == ProtocolSMB {
		return status.SMB.Writable
	}
	if protocol == ProtocolNFS {
		return status.NFS.Writable
	}
	return false
}

func unavailableStatus(reason string) Status {
	probedAt := time.Now().UTC()
	return Status{
		Reason:       reason,
		SMB:          ProtocolStatus{Reason: reason, LastProbedAt: probedAt},
		NFS:          ProtocolStatus{Reason: reason, LastProbedAt: probedAt},
		LastProbedAt: probedAt,
	}
}

// UnavailableStatus constructs a public, non-usable runtime snapshot.
func UnavailableStatus(reason string) Status {
	return unavailableStatus(reason)
}

func cloneStatus(status Status) Status {
	status.AllowedRoots = append([]RootStatus(nil), status.AllowedRoots...)
	return status
}

func hasAvailableRoot(roots []RootStatus) bool {
	for _, root := range roots {
		if root.Available {
			return true
		}
	}
	return false
}

func newShareID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(raw[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:]), nil
}

func nativeComment(comment, id string) string {
	comment = strings.TrimSpace(stripNativeMarker(comment))
	marker := markerPrefix + id + "]"
	if comment == "" {
		return marker
	}
	return comment + " " + marker
}

func stripNativeMarker(comment string) string {
	index := strings.LastIndex(comment, markerPrefix)
	if index < 0 {
		return strings.TrimSpace(comment)
	}
	end := strings.Index(comment[index:], "]")
	if end < 0 {
		return strings.TrimSpace(comment)
	}
	return strings.TrimSpace(comment[:index] + comment[index+end+1:])
}

func markerID(comment string) string {
	index := strings.LastIndex(comment, markerPrefix)
	if index < 0 {
		return ""
	}
	rest := comment[index+len(markerPrefix):]
	end := strings.Index(rest, "]")
	if end <= 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func externalShareID(share observedShare) string {
	sum := sha256.Sum256([]byte(strings.ToLower(share.Protocol) + "\x00" + strings.ToLower(share.Name) + "\x00" + share.Path))
	return "external:" + share.Protocol + ":" + hex.EncodeToString(sum[:8])
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func sameStrings(left, right []string) bool {
	a := append([]string(nil), left...)
	b := append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func sameACL(left, right []ACLEntry) bool {
	toStrings := func(entries []ACLEntry) []string {
		values := make([]string, 0, len(entries))
		for _, entry := range entries {
			values = append(values, strings.ToLower(entry.Principal)+"\x00"+strings.ToLower(entry.Level))
		}
		return values
	}
	return sameStrings(toStrings(left), toStrings(right))
}

func makeStringSet(values []string, caseInsensitive bool) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if caseInsensitive {
			value = strings.ToLower(value)
		}
		result[value] = true
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// Close releases the ledger.
func (m *Manager) Close() error {
	if m == nil || m.ledger == nil {
		return nil
	}
	return m.ledger.Close()
}

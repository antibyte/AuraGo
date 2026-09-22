package upkeep

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/webassets"
	"github.com/gofrs/flock"
)

type Transaction struct {
	Version         int    `json:"version"`
	Root            string `json:"root"`
	ID              string `json:"id"`
	Created         int64  `json:"created"`
	Status          string `json:"status"`
	PreviousVersion string `json:"previous_version"`
	PreviousAsset   string `json:"previous_asset"`
	NewAsset        string `json:"new_asset"`
	NewVersion      string `json:"new_version,omitempty"`
	BackupComplete  bool   `json:"backup_complete"`
}

type Entry struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Action string `json:"action"`
	Reason string `json:"reason"`
	Freed  int64  `json:"freed_bytes"`
	info   os.FileInfo
	check  func() error
}

type Report struct {
	Entries  []*Entry `json:"entries"`
	Freed    int64    `json:"freed_bytes"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type Options struct {
	Root        string
	Apply       bool
	AdoptLegacy bool
	LegacyDir   string
	// Dependencies are injectable for fixture tests; the CLI always uses real probes.
	ReadPin func(string) (string, error)
	Verify  func(string, string) error
	Health  func(context.Context, string) error
}

type backup struct {
	Transaction
	path   string
	legacy bool
}

func installation(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	if filepath.Dir(root) == root {
		return "", fmt.Errorf("filesystem root is not an installation")
	}
	if err := noLinks(root); err != nil {
		return "", err
	}
	if err := regularPath(filepath.Join(root, "config.yaml")); err != nil {
		return "", fmt.Errorf("installation config: %w", err)
	}
	return root, nil
}

func lock(root string) (func(), error) {
	p := filepath.Join(root, StateDir)
	if err := noLinks(p); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(p, 0700); err != nil {
		return nil, err
	}
	name := filepath.Join(p, "update.lock")
	if err := noLinks(name); err != nil {
		return nil, err
	}
	if close, inherited, err := inheritedLock(name); inherited {
		return close, err
	}
	l := flock.New(name)
	ok, err := l.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("update or cleanup already running")
	}
	return func() { l.Unlock(); l.Close() }, nil
}

func installedBinary(root string) (string, error) {
	for _, rel := range []string{"bin/aurago_linux", "bin/aurago", "aurago.exe", "aurago"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			continue
		}
		if err := regularPath(p); err != nil {
			return "", err
		}
		return p, nil
	}
	return "", fmt.Errorf("installed binary not found")
}

func loadTransactions(root string) ([]backup, error) {
	base := filepath.Join(root, StateDir, "transactions")
	if err := noLinks(base); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []backup
	for _, e := range entries {
		if transactionPattern.MatchString(retiredOriginal(root, e.Name())) {
			continue
		}
		if !transactionPattern.MatchString(e.Name()) {
			return nil, fmt.Errorf("unknown transaction entry: %s", e.Name())
		}
		p := filepath.Join(base, e.Name())
		var t Transaction
		if err := readJSON(filepath.Join(p, "manifest.json"), &t); err != nil {
			return nil, fmt.Errorf("invalid transaction %s: %w", e.Name(), err)
		}
		if t.Version != 1 || t.Root != root || t.ID != e.Name() || t.Created <= 0 || !hashPattern.MatchString(t.PreviousVersion) || !hashPattern.MatchString(t.PreviousAsset) || (t.NewAsset != "" && !hashPattern.MatchString(t.NewAsset)) {
			return nil, fmt.Errorf("foreign or malformed transaction: %s", e.Name())
		}
		switch t.Status {
		case "pending", "confirmed", "rolled_back", "uncertain":
		default:
			return nil, fmt.Errorf("unknown transaction outcome")
		}
		result = append(result, backup{Transaction: t, path: p})
	}
	return result, nil
}

func (r *Report) keep(p, reason string) {
	size, _ := treeBytes(p)
	r.Entries = append(r.Entries, &Entry{Path: p, Bytes: size, Action: "keep", Reason: reason})
}
func (r *Report) candidate(p, reason string, check func() error) {
	size, err := treeBytes(p)
	if err != nil {
		r.keep(p, "unsafe or unreadable candidate: "+err.Error())
		r.Warnings = append(r.Warnings, "candidate could not be collected: "+p)
		return
	}
	i, err := snapshotInfo(p)
	if err != nil {
		r.keep(p, "candidate disappeared")
		return
	}
	r.Entries = append(r.Entries, &Entry{Path: p, Bytes: size, Action: "delete", Reason: reason, info: i, check: check})
}

// Cleanup computes protections before deleting anything. Incomplete transactions
// prevent collection, rather than guessing whether a write/restart succeeded.
func Cleanup(ctx context.Context, o Options) (r Report, err error) {
	root, err := installation(o.Root)
	if err != nil {
		return r, err
	}
	o.Root = root
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return r, err
	}
	defer rootHandle.Close()
	if o.ReadPin == nil {
		o.ReadPin = PinFromBinary
	}
	if o.Verify == nil {
		o.Verify = verifyAssets
	}
	if o.LegacyDir == "" {
		o.LegacyDir = "/tmp"
	}
	unlock, err := lock(root)
	if err != nil {
		return r, err
	}
	defer unlock()
	if o.Apply {
		defer func() {
			if e := saveResult(root, err == nil); e != nil {
				err = fmt.Errorf("persist cleanup result: %w", e)
			}
		}()
	}
	assetRoot := filepath.Join(root, "assets", "web")
	if err = noLinks(assetRoot); err != nil {
		return r, err
	}
	// Installation/import and collection share the same asset lock.
	al := flock.New(filepath.Join(assetRoot, ".install.lock"))
	if err = noLinks(al.Path()); err != nil {
		return r, err
	}
	locked, e := al.TryLockContext(ctx, 100*time.Millisecond)
	if e != nil {
		return r, e
	}
	if !locked {
		return r, fmt.Errorf("asset installation busy")
	}
	defer al.Unlock()
	defer al.Close()
	bin, err := installedBinary(root)
	if err != nil {
		return r, err
	}
	current, err := o.ReadPin(bin)
	if err != nil {
		return r, err
	}
	if err = o.Verify(assetRoot, current); err != nil {
		return r, fmt.Errorf("current assets: %w", err)
	}
	currentVersion, err := digestFile(bin)
	if err != nil {
		return r, err
	}
	protected := map[string]string{current: "installed/running version"}
	managedAssets := map[string]bool{current: true}
	backups, err := loadTransactions(root)
	if err != nil {
		return r, err
	}
	for _, b := range backups {
		managedAssets[b.PreviousAsset] = true
		managedAssets[b.NewAsset] = true
		if b.Status == "pending" || b.Status == "uncertain" || (!b.BackupComplete && b.Status != "rolled_back") {
			r.keep(b.path, "unresolved update; resolve only after verified recovery")
			return r, fmt.Errorf("unresolved update %s blocks cleanup", b.ID)
		}
	}
	if o.AdoptLegacy {
		legacy, e := legacyBackups(o, &r)
		if e != nil {
			return r, e
		}
		backups = append(backups, legacy...)
		for _, b := range legacy {
			managedAssets[b.PreviousAsset] = true
		}
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].Created == backups[j].Created {
			return backups[i].path > backups[j].path
		}
		return backups[i].Created > backups[j].Created
	})
	versions := map[string]bool{}
	var adopt []backup
	for _, b := range backups {
		b := b
		if b.Status == "confirmed" && b.PreviousVersion != currentVersion && !versions[b.PreviousVersion] && len(versions) < 2 {
			if e := verifyBackup(o, b); e != nil {
				r.keep(b.path, "rollback validation failed")
				return r, fmt.Errorf("cannot establish safe rollback retention: %w", e)
			}
			versions[b.PreviousVersion] = true
			protected[b.PreviousAsset] = "retained rollback version"
			r.keep(b.path, "one of two verified rollback versions")
			if b.legacy {
				adopt = append(adopt, b)
			}
		} else {
			r.candidate(b.path, "obsolete update backup", func() error {
				if b.legacy {
					return legacyBinding(root, b.path)
				}
				var t Transaction
				if e := readJSON(filepath.Join(b.path, "manifest.json"), &t); e != nil {
					return e
				}
				if t != b.Transaction {
					return fmt.Errorf("transaction changed")
				}
				return nil
			})
		}
	}
	// Explicitly archived binaries and release outputs retain their resource pins.
	for _, dir := range []string{"bin", "deploy"} {
		base := filepath.Join(root, dir)
		if e := noLinks(base); e != nil {
			return r, e
		}
		entries, e := os.ReadDir(base)
		if e != nil && !os.IsNotExist(e) {
			return r, e
		}
		for _, entry := range entries {
			n := entry.Name()
			if entry.IsDir() || !strings.HasPrefix(n, "aurago") || strings.HasPrefix(n, "aurago-remote") || strings.HasPrefix(n, "aurago-web-assets-") || strings.HasSuffix(n, ".lock") {
				continue
			}
			p := filepath.Join(base, n)
			if p == bin {
				continue
			}
			id, e := o.ReadPin(p)
			if e != nil {
				return r, fmt.Errorf("cannot identify retained binary %s: %w", p, e)
			}
			protected[id] = "explicit rollback/release binary"
			r.keep(p, "explicit rollback/release binary")
		}
	}
	markers, err := filepath.Glob(filepath.Join(root, "deploy", "aurago-web-assets-*.release.json"))
	if err != nil {
		return r, err
	}
	for _, p := range markers {
		var pin webassets.Pin
		if e := readJSON(p, &pin); e != nil {
			return r, fmt.Errorf("invalid release ownership marker: %w", e)
		}
		if !hashPattern.MatchString(pin.ID) || filepath.Base(p) != "aurago-web-assets-"+pin.ID+".release.json" {
			return r, fmt.Errorf("invalid release pin")
		}
		protected[pin.ID] = "explicit release archive"
	}
	// Older packers only wrote the current release pin to web-assets.json.
	metadata := filepath.Join(root, "deploy", "web-assets.json")
	if _, e := os.Lstat(metadata); e == nil {
		var pin webassets.Pin
		if e = readJSON(metadata, &pin); e != nil {
			return r, e
		}
		if pin.URL != "" {
			protected[pin.ID] = "explicit release metadata"
		}
	}
	entries, err := os.ReadDir(assetRoot)
	if err != nil {
		return r, err
	}
	knownAssets := map[string]bool{}
	for _, entry := range entries {
		n := entry.Name()
		p := filepath.Join(assetRoot, n)
		if n == ".install.lock" {
			continue
		}
		if hashPattern.MatchString(retiredOriginal(root, n)) {
			r.candidate(p, "retry interrupted resource deletion", nil)
			continue
		}
		if !hashPattern.MatchString(n) {
			r.keep(p, "unknown or quarantined asset entry")
			continue
		}
		knownAssets[n] = true
		if reason, ok := protected[n]; ok {
			r.keep(p, reason)
			continue
		}
		if !managedAssets[n] {
			info, e := entry.Info()
			if e != nil {
				return r, e
			}
			if !o.AdoptLegacy || time.Since(info.ModTime()) < 24*time.Hour {
				r.keep(p, "unregistered/recent build; legacy adoption has a 24-hour grace period")
				continue
			}
		}
		// Only verified resource sets can be classified as obsolete build output.
		if e := o.Verify(assetRoot, n); e != nil {
			r.keep(p, "unverified resource set")
			continue
		}
		id := n
		r.candidate(p, "unreferenced resource set", func() error { return o.Verify(assetRoot, id) })
	}
	// Legacy update archives are adopted only explicitly, and only when their
	// embedded manifest matches an installed set. Release pins always win.
	if o.AdoptLegacy {
		archives, _ := filepath.Glob(filepath.Join(root, "deploy", "aurago-web-assets-*.tar.gz"))
		for _, p := range archives {
			id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "aurago-web-assets-"), ".tar.gz")
			if _, ok := protected[id]; ok {
				r.keep(p, "retained version archive")
				continue
			}
			if !knownAssets[id] || archiveIdentity(p, id) != nil {
				r.keep(p, "unclassified archive")
				continue
			}
			p, id := p, id
			r.candidate(p, "obsolete adopted update archive", func() error { return archiveIdentity(p, id) })
		}
	}
	cache := filepath.Join(root, StateDir, "go-cache")
	if size, e := treeBytes(cache); e == nil {
		if size > CacheLimit {
			r.candidate(cache, "dedicated Go cache exceeds 4 GiB", nil)
		} else {
			r.keep(cache, "dedicated Go cache below limit")
		}
	} else if !os.IsNotExist(e) {
		return r, e
	}
	workDirs, err := filepath.Glob(filepath.Join(root, StateDir, "work.*"))
	if err != nil {
		return r, err
	}
	for _, p := range workDirs {
		if !workPattern.MatchString(filepath.Base(p)) {
			r.keep(p, "unknown private-state entry")
			continue
		}
		r.candidate(p, "finished or abandoned updater staging directory", nil)
	}
	transactionDirs, e := os.ReadDir(filepath.Join(root, StateDir, "transactions"))
	if e != nil && !os.IsNotExist(e) {
		return r, e
	}
	for _, entry := range transactionDirs {
		if transactionPattern.MatchString(retiredOriginal(root, entry.Name())) {
			r.candidate(filepath.Join(root, StateDir, "transactions", entry.Name()), "retry interrupted backup deletion", nil)
		}
	}
	if o.AdoptLegacy {
		retired, _ := filepath.Glob(filepath.Join(o.LegacyDir, ".aurago-backup-*.aurago-retired-"+webassets.Digest([]byte(root))[:16]))
		rootInfo, e := os.Stat(root)
		if e != nil {
			return r, e
		}
		for _, p := range retired {
			info, e := os.Lstat(p)
			if e != nil {
				return r, e
			}
			if sameOwner(rootInfo, info) {
				r.candidate(p, "retry interrupted legacy backup deletion", nil)
			} else {
				r.keep(p, "foreign retired backup")
			}
		}
	}
	if !o.Apply {
		return r, nil
	}
	if o.Health == nil {
		return r, fmt.Errorf("running version verification required")
	}
	if err = o.Health(ctx, bin); err != nil {
		return r, fmt.Errorf("running version not verified: %w", err)
	}
	for _, b := range adopt {
		dest, e := adoptBackup(o, b)
		if e != nil {
			return r, fmt.Errorf("retain legacy rollback in private storage: %w", e)
		}
		for _, entry := range r.Entries {
			if entry.Path == b.path {
				entry.Path = dest
				entry.Reason = "verified legacy rollback adopted into private storage"
			}
		}
	}
	// Remove adopted archives before their matching sets, and keep transaction
	// provenance until all dependent assets have been collected successfully.
	sort.SliceStable(r.Entries, func(i, j int) bool {
		priority := func(e *Entry) int {
			if strings.HasSuffix(e.Path, ".tar.gz") {
				return 0
			}
			if within(assetRoot, e.Path) {
				return 1
			}
			if strings.Contains(e.Reason, "backup") {
				return 3
			}
			return 2
		}
		return priority(r.Entries[i]) < priority(r.Entries[j])
	})
	for _, entry := range r.Entries {
		if entry.Action != "delete" {
			continue
		}
		if err = ctx.Err(); err != nil {
			return r, err
		}
		if entry.check != nil {
			if err = entry.check(); err != nil {
				return r, err
			}
		}
		if _, err = treeBytes(entry.Path); err != nil {
			return r, err
		}
		allocated, e := allocatedBytes(entry.Path)
		if e != nil {
			return r, e
		}
		// Retire directories atomically before recursive deletion. If permissions
		// or a crash interrupts RemoveAll, missing manifests cannot turn remnants
		// into unknown backups or prevent the next safe retry.
		base := filepath.Base(entry.Path)
		parent := filepath.Dir(entry.Path)
		retire := entry.info.IsDir() && ((parent == assetRoot && hashPattern.MatchString(base)) || (parent == filepath.Join(root, StateDir, "transactions") && transactionPattern.MatchString(base)) || (parent == o.LegacyDir && strings.HasPrefix(base, "aurago-backup-")))
		if retire {
			newPath := filepath.Join(parent, retiredName(root, base))
			if _, e := os.Lstat(newPath); !os.IsNotExist(e) {
				return r, fmt.Errorf("retired artifact already exists")
			}
			current, e := snapshotInfo(entry.Path)
			if e != nil || !os.SameFile(current, entry.info) {
				return r, fmt.Errorf("candidate changed before retirement")
			}
			var handle *os.Root
			var oldRel, newRel string
			if within(root, entry.Path) {
				handle = rootHandle
				oldRel, _ = filepath.Rel(root, entry.Path)
				newRel, _ = filepath.Rel(root, newPath)
			} else {
				handle, e = os.OpenRoot(o.LegacyDir)
				if e != nil {
					return r, e
				}
				oldRel = base
				newRel = filepath.Base(newPath)
			}
			e = handle.Rename(oldRel, newRel)
			if handle != rootHandle {
				handle.Close()
			}
			if e != nil {
				return r, e
			}
			entry.Path = newPath
		}
		if within(root, entry.Path) {
			rel, e := filepath.Rel(root, entry.Path)
			if e != nil {
				return r, e
			}
			err = removeTree(rootHandle, rel, entry.info)
		} else if o.AdoptLegacy && filepath.Dir(entry.Path) == o.LegacyDir {
			legacyRoot, e := os.OpenRoot(o.LegacyDir)
			if e != nil {
				return r, e
			}
			err = removeTree(legacyRoot, filepath.Base(entry.Path), entry.info)
			legacyRoot.Close()
		} else {
			err = fmt.Errorf("candidate outside managed roots")
		}
		if err != nil {
			return r, err
		}
		entry.Action = "deleted"
		entry.Freed = allocated
		r.Freed += entry.Freed
	}
	if len(r.Warnings) > 0 {
		return r, fmt.Errorf("cleanup incomplete: %d unsafe or unreadable candidates", len(r.Warnings))
	}
	return r, nil
}

func verifyBackup(o Options, b backup) error {
	p := filepath.Join(b.path, "bin", "aurago_linux")
	if _, e := os.Stat(p); os.IsNotExist(e) {
		p = filepath.Join(b.path, "bin", "aurago")
	}
	h, err := digestFile(p)
	if err != nil {
		return err
	}
	if h != b.PreviousVersion {
		return fmt.Errorf("backup binary checksum mismatch")
	}
	id, err := o.ReadPin(p)
	if err != nil {
		return err
	}
	if id != b.PreviousAsset {
		return fmt.Errorf("backup binary asset mismatch")
	}
	return o.Verify(filepath.Join(o.Root, "assets", "web"), id)
}

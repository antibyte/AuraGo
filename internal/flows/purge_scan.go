package flows

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	credentialScanTTL    = 10 * time.Minute
	credentialScanMinGap = 30 * time.Second
	// minPurgeLiteral is the shortest literal the purge replaces by exact text; shorter findings are
	// still redacted by the rule, but a short string would also hit harmless text.
	minPurgeLiteral = 8
)

// snapshotNamePattern matches the migration snapshot directories backupFlowsSnapshot creates.
var snapshotNamePattern = regexp.MustCompile(`^\.flows-v[0-9]+-backup-[A-Za-z0-9]+$`)

type CredentialFlowRef struct {
	FlowID   string `json:"flow_id"`
	Name     string `json:"name"`
	Findings int    `json:"findings"`
}

type SnapshotInfo struct {
	Name    string    `json:"name"`
	Bytes   int64     `json:"bytes"`
	ModTime time.Time `json:"mod_time"`
}

type BackupInfo struct {
	Name      string    `json:"name"`
	Bytes     int64     `json:"bytes"`
	ModTime   time.Time `json:"mod_time"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CredentialReport says where typed credentials still sit. It never holds a credential value.
type CredentialReport struct {
	GeneratedAt       time.Time           `json:"generated_at"`
	Drafts            []CredentialFlowRef `json:"drafts"`
	Live              []CredentialFlowRef `json:"live"`
	VersionsPurgeable int                 `json:"versions_purgeable"`
	VersionsPinned    int                 `json:"versions_pinned"`
	Runs              int                 `json:"runs"`
	Steps             int                 `json:"steps"`
	TestRows          int                 `json:"test_rows"`
	Snapshots         []SnapshotInfo      `json:"snapshots"`
	Leftovers         []SnapshotInfo      `json:"leftovers"`
	Backups           []BackupInfo        `json:"backups"`
}

// NeedsAttention is true while plaintext remains anywhere the purge can reach. The encrypted backups
// do not count: they are the purge's own safety net.
func (r CredentialReport) NeedsAttention() bool {
	return len(r.Drafts) > 0 || len(r.Live) > 0 || r.VersionsPurgeable > 0 || r.VersionsPinned > 0 ||
		r.Runs > 0 || r.Steps > 0 || r.TestRows > 0 || len(r.Snapshots) > 0 || len(r.Leftovers) > 0
}

func (r *CredentialReport) normalize() {
	if r.Drafts == nil {
		r.Drafts = []CredentialFlowRef{}
	}
	if r.Live == nil {
		r.Live = []CredentialFlowRef{}
	}
	if r.Snapshots == nil {
		r.Snapshots = []SnapshotInfo{}
	}
	if r.Leftovers == nil {
		r.Leftovers = []SnapshotInfo{}
	}
	if r.Backups == nil {
		r.Backups = []BackupInfo{}
	}
}

// credentialScan is a scan with the data the purge needs: the literal values found (kept in memory only).
type credentialScan struct {
	report    CredentialReport
	draftLive map[string]struct{} // literals in drafts and live documents, which the purge leaves alone
	versions  map[string]struct{} // literals in published versions
	rows      map[string]struct{} // literals found by the rule in runs, steps and test data
	purge     map[string][]int    // flow id -> revisions with findings that are not live
}

func (sc *credentialScan) addDoc(text string, into map[string]struct{}) int {
	doc, err := ParseFlow([]byte(text))
	if err != nil {
		return 0
	}
	n := 0
	for _, finding := range FindLiteralCredentials(doc) {
		n++
		if finding.Value != "" {
			into[finding.Value] = struct{}{}
		}
	}
	return n
}

// allLiterals is the set the purge replaces by exact text.
func (sc *credentialScan) allLiterals() []string {
	return expandLiterals(sc.draftLive, sc.versions, sc.rows)
}

// credentialLiteralsInText decodes JSON text and returns the literals the rule finds in it.
func credentialLiteralsInText(text string) []string {
	if text == "" {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	return literalCredentialsInValue(v)
}

// expandLiterals merges literal sets into the list the purge replaces: every value of at least
// minPurgeLiteral characters, plus the value without a leading "Bearer " or "Basic " (the vault
// stores the bare value), longest first so a prefixed value is replaced before its bare part.
func expandLiterals(sets ...map[string]struct{}) []string {
	seen := map[string]struct{}{}
	add := func(v string) {
		if len(v) >= minPurgeLiteral {
			seen[v] = struct{}{}
		}
	}
	for _, set := range sets {
		for v := range set {
			add(v)
			for _, prefix := range []string{"Bearer ", "Basic "} {
				if len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
					add(strings.TrimSpace(v[len(prefix):]))
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

func containsAnyLiteral(text string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

// credentialTextSources are the plaintext JSON columns the purge reads and rewrites (flows.db keeps
// scenarios and durable checkpoints encrypted, so they are not here). rowid is the row key of every table.
var credentialTextSources = []struct {
	kind, table string
	columns     []string
}{
	{"runs", "flow_runs", []string{"doc_json", "trigger_data_json", "checkpoint_json"}},
	{"steps", "flow_run_steps", []string{"params_json", "output_json"}},
	{"steps", "flow_child_traces", []string{"summary_json", "trace_json"}},
	{"test", "flow_test_data", []string{"json"}},
	{"test", "flow_test_docs", []string{"json"}},
}

func (s *Store) scanTexts(ctx context.Context, table string, columns []string, fn func(texts []string)) error {
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = "COALESCE(" + c + ", '')"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+strings.Join(parts, ", ")+` FROM `+table)
	if err != nil {
		return err
	}
	defer rows.Close()
	texts := make([]string, len(columns))
	ptrs := make([]any, len(columns))
	for i := range texts {
		ptrs[i] = &texts[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		fn(texts)
	}
	return rows.Err()
}

// scanRowCounts counts the rows that hold a finding of the rule or one of literals, per kind. The
// literals found by the rule are added to sc.rows.
func (s *Store) scanRowCounts(ctx context.Context, sc *credentialScan, literals []string) (map[string]int, error) {
	counts := map[string]int{}
	for _, src := range credentialTextSources {
		src := src
		err := s.scanTexts(ctx, src.table, src.columns, func(texts []string) {
			hit := false
			for _, text := range texts {
				if found := credentialLiteralsInText(text); len(found) > 0 {
					hit = true
					for _, v := range found {
						sc.rows[v] = struct{}{}
					}
				}
				if !hit && containsAnyLiteral(text, literals) {
					hit = true
				}
			}
			if hit {
				counts[src.kind]++
			}
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}

// scanCredentials reads every JSON column of flows.db: drafts, live documents, versions, runs, steps and test data.
func (s *Store) scanCredentials(ctx context.Context) (*credentialScan, error) {
	sc := &credentialScan{draftLive: map[string]struct{}{}, versions: map[string]struct{}{}, rows: map[string]struct{}{}, purge: map[string][]int{}}
	rep := &sc.report
	flowRows, err := s.db.QueryContext(ctx, `SELECT id, name, draft_json, live_json FROM flows ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	for flowRows.Next() {
		var id, name, draft, live string
		if err := flowRows.Scan(&id, &name, &draft, &live); err != nil {
			_ = flowRows.Close()
			return nil, err
		}
		if n := sc.addDoc(draft, sc.draftLive); n > 0 {
			rep.Drafts = append(rep.Drafts, CredentialFlowRef{FlowID: id, Name: name, Findings: n})
		}
		if live != "" {
			if n := sc.addDoc(live, sc.draftLive); n > 0 {
				rep.Live = append(rep.Live, CredentialFlowRef{FlowID: id, Name: name, Findings: n})
			}
		}
	}
	if err := flowRows.Err(); err != nil {
		_ = flowRows.Close()
		return nil, err
	}
	_ = flowRows.Close()

	versionRows, err := s.db.QueryContext(ctx, `SELECT v.flow_id, v.revision, v.json, f.live_revision, `+
		versionPinnedSQL("v.flow_id", "v.revision")+` FROM flow_versions v JOIN flows f ON f.id = v.flow_id ORDER BY v.flow_id, v.revision`)
	if err != nil {
		return nil, err
	}
	for versionRows.Next() {
		var flowID, text string
		var revision, live int
		var pinned bool
		if err := versionRows.Scan(&flowID, &revision, &text, &live, &pinned); err != nil {
			_ = versionRows.Close()
			return nil, err
		}
		if sc.addDoc(text, sc.versions) == 0 || revision == live { // the live version is the live document
			continue
		}
		if pinned {
			rep.VersionsPinned++
		} else {
			rep.VersionsPurgeable++
		}
		sc.purge[flowID] = append(sc.purge[flowID], revision)
	}
	if err := versionRows.Err(); err != nil {
		_ = versionRows.Close()
		return nil, err
	}
	_ = versionRows.Close()

	// Pass 1 finds the literals by the rule; pass 2 counts the rows again, now also the ones that only
	// repeat a literal (a moved token echoed in an output text).
	if _, err := s.scanRowCounts(ctx, sc, nil); err != nil {
		return nil, err
	}
	counts, err := s.scanRowCounts(ctx, sc, sc.allLiterals())
	if err != nil {
		return nil, err
	}
	rep.Runs, rep.Steps, rep.TestRows = counts["runs"], counts["steps"], counts["test"]
	return sc, nil
}

func dirBytes(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// isPlaintextLeftover names the temporary files a failed purge or restore may leave in clear.
func isPlaintextLeftover(name string) bool {
	if strings.HasSuffix(name, preRestoreTempExt) || strings.HasSuffix(name, ".restore.tmp") {
		return true
	}
	return (strings.HasPrefix(name, PurgeBackupPrefix) || strings.HasPrefix(name, PreRestoreBackupPrefix)) && !strings.HasSuffix(name, PurgeBackupSuffix)
}

// listPurgeFiles fills the snapshot, leftover and encrypted-backup lists from the directory of flows.db.
func listPurgeFiles(dir string, rep *CredentialReport) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}
		switch {
		case entry.IsDir() && snapshotNamePattern.MatchString(name):
			rep.Snapshots = append(rep.Snapshots, SnapshotInfo{Name: name, Bytes: dirBytes(filepath.Join(dir, name)), ModTime: info.ModTime()})
		case entry.Type().IsRegular() && isEncryptedBackupName(name):
			rep.Backups = append(rep.Backups, BackupInfo{Name: name, Bytes: info.Size(), ModTime: info.ModTime(), ExpiresAt: info.ModTime().Add(PurgeBackupMaxAge)})
		case entry.Type().IsRegular() && isPlaintextLeftover(name):
			rep.Leftovers = append(rep.Leftovers, SnapshotInfo{Name: name, Bytes: info.Size(), ModTime: info.ModTime()})
		}
	}
	return nil
}

// scanCache holds the last report. CredentialScan serves it and refreshes it in the background (at most
// every 30 seconds after a change, every 10 minutes otherwise); the dialog asks for a fresh one.
type scanCache struct {
	mu      sync.Mutex
	report  *CredentialReport
	at      time.Time
	dirty   bool
	running bool
}

// stopContext is a context that ends when the service shuts down.
func (s *Service) stopContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// InvalidateCredentialScan marks the cached report as out of date (a save, a move, a publish, a purge).
func (s *Service) InvalidateCredentialScan() {
	s.scan.mu.Lock()
	s.scan.dirty = true
	s.scan.mu.Unlock()
}

// CredentialScan returns the cached report. The first call computes it; later calls return the cache at
// once and start a background refresh when it is out of date.
func (s *Service) CredentialScan(ctx context.Context) (CredentialReport, error) {
	c := &s.scan
	c.mu.Lock()
	report, age := c.report, s.now().Sub(c.at)
	stale := report != nil && !c.running && ((c.dirty && age >= credentialScanMinGap) || age >= credentialScanTTL)
	if stale {
		c.running = true
	}
	c.mu.Unlock()
	if report == nil {
		return s.CredentialScanFresh(ctx)
	}
	if stale {
		go func() {
			bg, cancel := s.stopContext()
			defer cancel()
			_, _ = s.computeCredentialScan(bg)
		}()
	}
	return *report, nil
}

// CredentialScanFresh reads flows.db again and refills the cache.
func (s *Service) CredentialScanFresh(ctx context.Context) (CredentialReport, error) {
	return s.computeCredentialScan(ctx)
}

func (s *Service) computeCredentialScan(ctx context.Context) (CredentialReport, error) {
	c := &s.scan
	c.mu.Lock()
	c.dirty = false
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()
	sc, err := s.store.scanCredentials(ctx)
	if err != nil {
		return CredentialReport{}, err
	}
	report := sc.report
	report.GeneratedAt = s.now()
	if dir, derr := s.store.dir(); derr == nil && dir != "" {
		if err := listPurgeFiles(dir, &report); err != nil {
			return CredentialReport{}, err
		}
	}
	report.normalize()
	c.mu.Lock()
	c.report, c.at = &report, s.now()
	c.mu.Unlock()
	return report, nil
}

// initialCredentialScan fills the cache at start; it ends early when the service shuts down.
func (s *Service) initialCredentialScan() {
	ctx, cancel := s.stopContext()
	defer cancel()
	if _, err := s.computeCredentialScan(ctx); err != nil && ctx.Err() == nil {
		s.logger.Warn("the credential scan failed", "error", err)
	}
}

package memory

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// A publication starts only after all embeddings have been prepared. Its marker
// survives an interrupted cross-database commit; an uncertain outcome is never
// certified as a complete snapshot. The marker contains no source text.
type memoryPublication struct {
	store           *SQLiteMemory
	validate        func() error
	release         func()
	key             string
	createdIDs      []string
	rollbackAllowed bool
	rolledBack      bool
	// nothingWritten is set only by a caller that knows the store reported no
	// created and no unknown-ownership IDs (failed before any write, or reused an
	// existing document). Such a marker guards no partial state and is removed.
	nothingWritten bool
}

const (
	memoryPublicationPublishingValue = `{"version":1,"status":"publishing"}`
	memoryPublicationUncertainValue  = `{"version":1,"status":"uncertain"}`
)

// ErrMemorySnapshotBlocked marks a snapshot refused because unfinished
// publications could make it capture half-published cross-database state.
var ErrMemorySnapshotBlocked = errors.New("memory snapshot blocked by unfinished publications")

// sweepAbandonedMemoryPublications runs once per process when the short-term
// store opens (NewSQLiteMemory, after the instance lock). After a restart no
// owner exists that could still finish or roll back a publication, and the
// persisted retry path tolerates a missing marker, so every leftover generic
// marker is cleared. Each one is logged first (key and value hold the created
// doc IDs) so orphans can be found manually. Correction and privacy markers
// have their own resume path and file_index.replacement.% is never touched.
// Within a running process markers keep blocking snapshots (fail closed).
func sweepAbandonedMemoryPublications(db *sql.DB, logger *slog.Logger) error {
	const where = `key LIKE 'snapshot.publication.%' AND key NOT LIKE 'snapshot.publication.correction.%' AND key NOT LIKE 'snapshot.publication.privacy.%'`
	rows, err := db.Query(`SELECT key,value FROM memory_maintenance_meta WHERE ` + where)
	if err != nil {
		return fmt.Errorf("sweep abandoned memory publications: %w", err)
	}
	type marker struct{ key, value string }
	var found []marker
	for rows.Next() {
		var m marker
		if err := rows.Scan(&m.key, &m.value); err != nil {
			rows.Close()
			return fmt.Errorf("sweep abandoned memory publications: %w", err)
		}
		found = append(found, m)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, m := range found {
		if logger != nil {
			logger.Warn("Clearing unfinished memory publication marker left by a previous run", "key", m.key, "value", m.value)
		}
		if _, err := db.Exec(`DELETE FROM memory_maintenance_meta WHERE key=? AND value=?`, m.key, m.value); err != nil {
			return fmt.Errorf("sweep abandoned memory publications: %w", err)
		}
	}
	return nil
}

type memoryPublicationKey struct{}

func prepareMemoryPublication(ctx context.Context, s *SQLiteMemory, validate func() error) (context.Context, *memoryPublication) {
	p := &memoryPublication{store: s, validate: validate}
	return context.WithValue(ctx, memoryPublicationKey{}, p), p
}

func startMemoryPublication(ctx context.Context) error {
	p, _ := ctx.Value(memoryPublicationKey{}).(*memoryPublication)
	if p == nil || p.release != nil || p.store == nil {
		return ctx.Err()
	}
	_, release, err := p.store.BeginMemoryPublication(ctx)
	if err != nil {
		return err
	}
	started := false
	defer func() {
		if !started {
			release()
		}
	}()
	if p.validate != nil {
		if err := p.validate(); err != nil {
			release()
			return err
		}
	}
	p.key = "snapshot.publication." + rand.Text()
	if err := p.store.SetMemoryMaintenanceState(p.key, memoryPublicationPublishingValue); err != nil {
		release()
		return fmt.Errorf("persist memory publication marker: %w", err)
	}
	p.release = release
	started = true
	return nil
}

func (p *memoryPublication) finish(complete bool) error {
	if p.release == nil {
		return nil
	}
	defer p.release()
	if !complete && !p.rolledBack && !p.nothingWritten {
		// Something may be half-published. Keep the marker (fail closed for this
		// process); the value is only a hint for the startup sweep's log line.
		value := memoryPublicationUncertainValue
		if len(p.createdIDs) > 0 {
			data, err := json.Marshal(p.createdIDs)
			if err != nil {
				return err
			}
			value = string(data)
		}
		_, err := p.store.db.Exec(`UPDATE memory_maintenance_meta SET value=? WHERE key=?`, value, p.key)
		return err
	}
	_, err := p.store.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key=?`, p.key)
	if err != nil {
		return fmt.Errorf("complete memory publication: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) completeRolledBackPublications(writes []AutomaticMemoryWrite) error {
	provided := map[string]map[string]bool{}
	for _, write := range writes {
		if !strings.HasPrefix(write.PublicationID, "snapshot.publication.") {
			continue
		}
		if provided[write.PublicationID] == nil {
			provided[write.PublicationID] = map[string]bool{}
		}
		provided[write.PublicationID][write.DocID] = true
	}
	for key, ids := range provided {
		var data string
		if err := s.db.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key=?`, key).Scan(&data); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		var expected []string
		if json.Unmarshal([]byte(data), &expected) != nil || len(expected) == 0 {
			continue
		}
		complete := true
		for _, id := range expected {
			if !ids[id] {
				complete = false
			}
		}
		if complete {
			if _, err := s.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key=? AND value=?`, key, data); err != nil {
				return err
			}
		}
	}
	return nil
}

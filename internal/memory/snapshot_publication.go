package memory

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	if err := p.store.SetMemoryMaintenanceState(p.key, `{"version":1,"status":"publishing"}`); err != nil {
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
	if !complete && !p.rolledBack {
		if !p.rollbackAllowed || len(p.createdIDs) == 0 {
			return nil
		}
		data, err := json.Marshal(p.createdIDs)
		if err != nil {
			return err
		}
		_, err = p.store.db.Exec(`UPDATE memory_maintenance_meta SET value=? WHERE key=?`, string(data), p.key)
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

package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var plantActionID = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,80}$`)

func loadPlant(row *sql.Row) (*PlantState, error) {
	var raw string
	if err := row.Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("load Leafy: %w", err)
	}
	if len(raw) > 256*1024 {
		return nil, fmt.Errorf("Leafy state exceeds size limit")
	}
	var p PlantState
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("decode Leafy: %w", err)
	}
	if err := validatePlant(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
func plantSnapshot(p *PlantState, now time.Time) PlantSnapshot {
	out := PlantSnapshot{ServerTime: now, NextUpdate: now.Add(time.Hour)}
	if p != nil {
		updated := advancePlant(*p, now)
		if !updated.Dead && updated.VacationAt == nil {
			out.NextUpdate = updated.LastSimulated.Add(time.Hour)
		}
		updated.Receipts = nil // Request-deduplication data is private to storage.
		if updated.Undo != nil {
			updated.Undo = &PlantUndo{Until: updated.Undo.Until}
		}
		out.Plant = &updated
	}
	return out
}

// Plant returns a calculated snapshot; GET never persists simulated advancement.
func (s *Service) Plant(ctx context.Context, now time.Time) (PlantSnapshot, error) {
	if err := s.ensureReady(ctx); err != nil {
		return PlantSnapshot{}, err
	}
	p, err := loadPlant(s.getDB().QueryRowContext(ctx, "SELECT value FROM desktop_meta WHERE key = ?", plantStateKey))
	if err != nil {
		return PlantSnapshot{}, err
	}
	return plantSnapshot(p, now.UTC()), nil
}

// ApplyPlantAction serializes the single shared plant and commits one validated action.
func (s *Service) ApplyPlantAction(ctx context.Context, a PlantAction, now time.Time) (PlantSnapshot, error) {
	if err := s.ensureReady(ctx); err != nil {
		return PlantSnapshot{}, err
	}
	if s.Config().ReadOnly {
		return PlantSnapshot{}, fmt.Errorf("virtual desktop is read-only")
	}
	if !plantActionID.MatchString(a.ID) || a.Revision < 0 {
		return PlantSnapshot{}, ErrPlantAction
	}
	switch a.Action {
	case "water", "fertilize", "prune", "trim", "undo_prune", "vacation", "replant":
	default:
		return PlantSnapshot{}, ErrPlantAction
	}
	if a.Action == "vacation" && a.Paused == nil {
		return PlantSnapshot{}, ErrPlantAction
	}
	now = now.UTC()
	s.plantMu.Lock()
	defer s.plantMu.Unlock()
	tx, err := s.getDB().BeginTx(ctx, nil)
	if err != nil {
		return PlantSnapshot{}, fmt.Errorf("begin Leafy update: %w", err)
	}
	defer tx.Rollback()
	p, err := loadPlant(tx.QueryRowContext(ctx, "SELECT value FROM desktop_meta WHERE key = ?", plantStateKey))
	if err != nil {
		return PlantSnapshot{}, err
	}
	body, _ := json.Marshal(a)
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	revision := int64(0)
	if p != nil {
		revision = p.Revision
		for _, receipt := range p.Receipts {
			if receipt.ID == a.ID {
				if receipt.Hash != hash {
					return plantSnapshot(p, now), ErrPlantConflict
				}
				return plantSnapshot(p, now), nil
			}
		}
	}
	if a.Revision != revision {
		return plantSnapshot(p, now), ErrPlantConflict
	}
	if p == nil && a.Action != "replant" {
		return PlantSnapshot{}, ErrPlantAction
	}
	if p != nil {
		updated := advancePlant(*p, now)
		p = &updated
	}
	if a.Action == "replant" {
		seed := make([]byte, 4)
		if _, err := rand.Read(seed); err != nil {
			return PlantSnapshot{}, fmt.Errorf("seed Leafy: %w", err)
		}
		next := newPlant(binary.LittleEndian.Uint32(seed), now)
		if p != nil {
			next.Receipts = p.Receipts
		}
		p = &next
	} else {
		switch a.Action {
		case "water", "fertilize":
			if p.Dead || p.VacationAt != nil {
				return PlantSnapshot{}, ErrPlantAction
			}
			if a.Action == "water" {
				p.Moisture = 100
			} else {
				p.Nutrients = 100
			}
		case "vacation":
			if p.Dead {
				return PlantSnapshot{}, ErrPlantAction
			}
			if *a.Paused && p.VacationAt == nil {
				start := now
				if start.Before(p.LastSimulated) {
					start = p.LastSimulated
				}
				p.VacationAt = &start
			}
			if !*a.Paused && p.VacationAt != nil {
				if now.After(*p.VacationAt) {
					p.LastSimulated = p.LastSimulated.Add(now.Sub(*p.VacationAt))
				}
				p.VacationAt = nil
			}
		case "prune", "trim":
			if err := p.prune(a.Branch, a.Node, now, a.Action == "trim"); err != nil {
				return PlantSnapshot{}, err
			}
		case "undo_prune":
			if p.Undo == nil || !now.Before(p.Undo.Until) {
				return PlantSnapshot{}, ErrPlantAction
			}
			p.Branches = copyPlantBranches(p.Undo.Branches)
			p.Undo = nil
		}
	}
	p.Revision = revision + 1
	p.Receipts = append(p.Receipts, PlantReceipt{ID: a.ID, Hash: hash})
	if len(p.Receipts) > 32 {
		p.Receipts = p.Receipts[len(p.Receipts)-32:]
	}
	if err := validatePlant(p); err != nil {
		return PlantSnapshot{}, err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return PlantSnapshot{}, fmt.Errorf("encode Leafy: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO desktop_meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", plantStateKey, string(raw)); err != nil {
		return PlantSnapshot{}, fmt.Errorf("save Leafy: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return PlantSnapshot{}, fmt.Errorf("commit Leafy: %w", err)
	}
	return plantSnapshot(p, now), nil
}

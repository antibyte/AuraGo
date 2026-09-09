package desktop

import (
	"errors"
	"fmt"
	"math"
	"time"
)

const plantStateKey = "leafy.state.v1"
const plantMaxBranches = 64
const plantMaxNodes = 36

var ErrPlantConflict = errors.New("plant_revision_conflict")
var ErrPlantAction = errors.New("invalid_plant_action")

type PlantBranch struct {
	ID       int64   `json:"id"`
	Parent   int64   `json:"parent"`
	Attach   int     `json:"attach"`
	Nodes    []int64 `json:"nodes"`
	Capped   bool    `json:"capped,omitempty"`
	RegrowAt int64   `json:"regrow_at,omitempty"`
}
type PlantUndo struct {
	Until    time.Time     `json:"until"`
	Branches []PlantBranch `json:"branches"`
}
type PlantReceipt struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}
type PlantState struct {
	Schema        int            `json:"schema_version"`
	Generation    int            `json:"generation_version"`
	Seed          uint32         `json:"seed"`
	Revision      int64          `json:"revision"`
	AgeHours      int64          `json:"age_hours"`
	GrowthHours   int64          `json:"growth_hours"`
	LastSimulated time.Time      `json:"last_simulated_at"`
	Moisture      float64        `json:"moisture"`
	Nutrients     float64        `json:"nutrients"`
	Vitality      float64        `json:"vitality"`
	Dead          bool           `json:"dead"`
	VacationAt    *time.Time     `json:"vacation_at,omitempty"`
	Branches      []PlantBranch  `json:"branches"`
	NextBranch    int64          `json:"next_branch"`
	Undo          *PlantUndo     `json:"undo,omitempty"`
	Receipts      []PlantReceipt `json:"receipts,omitempty"`
}
type PlantSnapshot struct {
	Plant      *PlantState `json:"plant"`
	ServerTime time.Time   `json:"server_time"`
	NextUpdate time.Time   `json:"next_update"`
}
type PlantAction struct {
	Action   string `json:"action"`
	ID       string `json:"action_id"`
	Revision int64  `json:"revision"`
	Branch   int64  `json:"branch,omitempty"`
	Node     int    `json:"node,omitempty"`
	Paused   *bool  `json:"paused,omitempty"`
}

func newPlant(seed uint32, now time.Time) PlantState {
	p := PlantState{Schema: 1, Generation: 1, Seed: seed, Moisture: 100, Nutrients: 100, Vitality: 100, LastSimulated: now, NextBranch: 3}
	for i := int64(0); i < 3; i++ {
		p.Branches = append(p.Branches, PlantBranch{ID: i, Parent: -1, Nodes: []int64{0, 0}})
	}
	return p
}
func copyPlantBranches(in []PlantBranch) []PlantBranch {
	out := append([]PlantBranch(nil), in...)
	for i := range out {
		out[i].Nodes = append([]int64(nil), in[i].Nodes...)
	}
	return out
}
func plantRandom(seed uint32, salt int64) uint32 {
	n := seed ^ (uint32(salt+1) * 0x9e3779b9)
	n = (n ^ (n >> 16)) * 0x21f0aaad
	n = (n ^ (n >> 15)) * 0x735a2d97
	return n ^ (n >> 15)
}

// advancePlant is independent of browsers, frame rate and local time zones.
// No-care advancement stops at death, so even years of absence take bounded work.
func advancePlant(input PlantState, now time.Time) PlantState {
	p := input
	p.Branches = copyPlantBranches(input.Branches)
	if p.Undo != nil && !now.Before(p.Undo.Until) {
		p.Undo = nil
	}
	if now.Before(p.LastSimulated) || p.VacationAt != nil {
		return p
	}
	for !p.Dead && !p.LastSimulated.Add(time.Hour).After(now) {
		p.LastSimulated = p.LastSimulated.Add(time.Hour)
		p.AgeHours++
		p.Moisture = math.Max(0, p.Moisture-2)
		p.Nutrients = math.Max(0, p.Nutrients-.5)
		damage := 0.0
		if p.Moisture == 0 {
			damage += 2
		} else if p.Moisture < 20 {
			damage++
		}
		if p.Nutrients == 0 {
			damage += .5
		} else if p.Nutrients < 10 {
			damage += .1
		}
		if damage > 0 {
			p.Vitality = math.Max(0, p.Vitality-damage)
		} else {
			p.Vitality = math.Min(100, p.Vitality+1)
		}
		if p.Vitality == 0 {
			p.Dead = true
			break
		}
		if p.Moisture < 20 || p.Nutrients < 10 || p.Vitality < 30 {
			continue
		}
		p.GrowthHours++
		initial := len(p.Branches)
		for i := 0; i < initial; i++ {
			b := &p.Branches[i]
			if b.RegrowAt > 0 && p.AgeHours >= b.RegrowAt {
				parent, attach := b.ID, len(b.Nodes)-1
				b.RegrowAt = 0
				if len(p.Branches) < plantMaxBranches {
					p.addBranch(parent, attach)
				} else {
					// At capacity, fresh node birth times distinguish regrowth without another branch.
					b.Capped = false
					if len(b.Nodes) < plantMaxNodes {
						b.Nodes = append(b.Nodes, p.AgeHours)
					}
				}
			} else if !b.Capped && len(b.Nodes) < plantMaxNodes {
				b.Nodes = append(b.Nodes, p.AgeHours)
			}
		}
		if p.GrowthHours%12 == 0 && len(p.Branches) < plantMaxBranches {
			candidates := []int{}
			for i, b := range p.Branches {
				if !b.Capped && len(b.Nodes) >= 5 {
					candidates = append(candidates, i)
				}
			}
			if len(candidates) > 0 {
				b := p.Branches[candidates[int(plantRandom(p.Seed, p.NextBranch))%len(candidates)]]
				attach := 2 + int(plantRandom(p.Seed, p.NextBranch+73))%(len(b.Nodes)-3)
				p.addBranch(b.ID, attach)
			}
		}
	}
	if p.Dead {
		p.LastSimulated = now
	}
	return p
}
func (p *PlantState) addBranch(parent int64, attach int) {
	p.Branches = append(p.Branches, PlantBranch{ID: p.NextBranch, Parent: parent, Attach: attach, Nodes: []int64{p.AgeHours, p.AgeHours}})
	p.NextBranch++
}
func (p *PlantState) prune(branch int64, node int, now time.Time, all bool) error {
	if p.Dead || p.VacationAt != nil {
		return ErrPlantAction
	}
	found := false
	for _, b := range p.Branches {
		if b.ID == branch && node >= 1 && node < len(b.Nodes) {
			found = true
		}
	}
	if !all && !found {
		return ErrPlantAction
	}
	p.Undo = &PlantUndo{Until: now.Add(30 * time.Second), Branches: copyPlantBranches(p.Branches)}
	removed := map[int64]bool{}
	kept := make([]PlantBranch, 0, len(p.Branches))
	for _, b := range p.Branches {
		if (all && b.Parent >= 0) || removed[b.Parent] || (b.Parent == branch && b.Attach >= node && !all) {
			removed[b.ID] = true
			continue
		}
		if all || b.ID == branch {
			n := node
			if all {
				n = 1
			}
			b.Nodes = b.Nodes[:n]
			b.Capped = true
			b.RegrowAt = p.AgeHours + 6
		}
		kept = append(kept, b)
	}
	p.Branches = kept
	return nil
}
func validatePlant(p *PlantState) error {
	if p.Schema != 1 || p.Generation != 1 || p.LastSimulated.IsZero() || p.AgeHours < 0 || p.GrowthHours < 0 || p.Revision < 1 || p.NextBranch < 3 || len(p.Receipts) > 32 {
		return fmt.Errorf("invalid Leafy state version or bounds")
	}
	for _, v := range []float64{p.Moisture, p.Nutrients, p.Vitality} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return fmt.Errorf("invalid Leafy care value")
		}
	}
	check := func(branches []PlantBranch) error {
		if len(branches) < 3 || len(branches) > plantMaxBranches {
			return fmt.Errorf("invalid Leafy branch count")
		}
		seen := map[int64]PlantBranch{}
		for _, b := range branches {
			if b.ID < 0 || b.ID >= p.NextBranch || len(b.Nodes) < 1 || len(b.Nodes) > plantMaxNodes || b.Attach < 0 {
				return fmt.Errorf("invalid Leafy branch")
			}
			if _, exists := seen[b.ID]; exists {
				return fmt.Errorf("duplicate Leafy branch")
			}
			if b.Parent >= 0 {
				parent, ok := seen[b.Parent]
				if !ok || b.Attach >= len(parent.Nodes) {
					return fmt.Errorf("invalid Leafy parent")
				}
			} else if b.ID > 2 {
				return fmt.Errorf("invalid Leafy root")
			}
			for _, age := range b.Nodes {
				if age < 0 || age > p.AgeHours {
					return fmt.Errorf("invalid Leafy node age")
				}
			}
			seen[b.ID] = b
		}
		return nil
	}
	if err := check(p.Branches); err != nil {
		return err
	}
	if p.Undo != nil {
		return check(p.Undo.Branches)
	}
	return nil
}

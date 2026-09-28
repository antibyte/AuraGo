package gamemaker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxPlayStateBytes = 4 << 20

var ErrPlayStateConflict = errors.New("a newer voxel save or published revision exists")

type VoxelChunkState struct {
	Position [3]int `json:"position"`
	Runs     []int  `json:"runs"`
}
type VoxelPlayerState struct {
	Position [3]float64 `json:"position"`
	Yaw      float64    `json:"yaw"`
	Pitch    float64    `json:"pitch"`
	Health   float64    `json:"health"`
}
type VoxelSlot struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
}
type VoxelEnemyState struct {
	ID       string     `json:"id"`
	Position [3]float64 `json:"position"`
	Health   float64    `json:"health"`
}
type VoxelState struct {
	Format    int               `json:"format"`
	Chunks    []VoxelChunkState `json:"chunks"`
	Player    VoxelPlayerState  `json:"player"`
	Inventory []*VoxelSlot      `json:"inventory"`
	Selected  int               `json:"selected"`
	Progress  map[string]int    `json:"progress"`
	Enemies   []VoxelEnemyState `json:"enemies"`
}
type PlayState struct {
	Compatibility string          `json:"compatibility"`
	Version       int64           `json:"version"`
	Revision      int64           `json:"revision"`
	State         json.RawMessage `json:"state"`
}

func validateVoxelState(v *VoxelDefinition, data []byte) error {
	if len(data) > MaxPlayStateBytes {
		return fmt.Errorf("voxel save exceeds 4 MiB")
	}
	var state VoxelState
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&state); err != nil {
		return fmt.Errorf("voxel save: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("voxel save must contain one object")
	}
	bad := func() error { return fmt.Errorf("invalid voxel save fields") }
	if state.Format != 1 || state.Chunks == nil || state.Progress == nil || state.Enemies == nil || len(state.Inventory) != 36 || state.Selected < 0 || state.Selected > 8 || len(state.Chunks) > v.Size[0]*v.Size[1]*v.Size[2]/4096 || len(state.Enemies) > 24 {
		return bad()
	}
	position := func(p [3]float64) bool {
		for i, n := range p {
			if !finite(n) || n < 0 || n > float64(v.Size[i]) {
				return false
			}
		}
		return true
	}
	p := state.Player
	if !position(p.Position) || !finite(p.Yaw) || math.Abs(p.Yaw) > 2*math.Pi || !finite(p.Pitch) || math.Abs(p.Pitch) > 1.55 || !finite(p.Health) || p.Health < 0 || p.Health > 100 {
		return bad()
	}
	blocks := map[int]bool{0: true}
	for _, b := range v.Blocks {
		blocks[b.ID] = true
	}
	chunks := map[[3]int]bool{}
	for _, c := range state.Chunks {
		if chunks[c.Position] || len(c.Runs) < 2 || len(c.Runs) > 8192 || len(c.Runs)%2 != 0 {
			return bad()
		}
		chunks[c.Position] = true
		for i, n := range c.Position {
			if n < 0 || n >= v.Size[i]/16 {
				return bad()
			}
		}
		count := 0
		for i := 0; i < len(c.Runs); i += 2 {
			if !blocks[c.Runs[i]] || c.Runs[i+1] < 1 || c.Runs[i+1] > 4096 {
				return bad()
			}
			count += c.Runs[i+1]
		}
		if count != 4096 {
			return bad()
		}
	}
	items := map[string]VoxelItem{}
	progress := map[string]bool{"defeat": true}
	for _, i := range v.Items {
		items[i.ID] = i
		for _, kind := range []string{"collect", "craft", "place"} {
			progress[kind+":"+i.ID] = true
		}
	}
	for _, s := range state.Inventory {
		if s == nil {
			continue
		}
		item, ok := items[s.Item]
		if !ok || s.Count < 1 || s.Count > item.Stack {
			return bad()
		}
	}
	for key, n := range state.Progress {
		if !progress[key] || n < 0 || n > 1000000 {
			return bad()
		}
	}
	enemies := map[string]bool{}
	for _, e := range v.Enemies {
		for i := 0; i < e.Count; i++ {
			enemies[fmt.Sprintf("%s:%d", e.ID, i)] = true
		}
	}
	seen := map[string]bool{}
	for _, e := range state.Enemies {
		if !enemies[e.ID] || seen[e.ID] || !position(e.Position) || !finite(e.Health) || e.Health < 0 || e.Health > 1000 {
			return bad()
		}
		seen[e.ID] = true
	}
	// Fixed-size Go arrays otherwise silently accept short or long JSON arrays.
	var raw struct {
		Chunks []struct {
			Position []int `json:"position"`
		} `json:"chunks"`
		Player struct {
			Position []float64 `json:"position"`
		} `json:"player"`
		Enemies []struct {
			Position []float64 `json:"position"`
		} `json:"enemies"`
	}
	if json.Unmarshal(data, &raw) != nil || len(raw.Player.Position) != 3 {
		return bad()
	}
	for _, c := range raw.Chunks {
		if len(c.Position) != 3 {
			return bad()
		}
	}
	for _, e := range raw.Enemies {
		if len(e.Position) != 3 {
			return bad()
		}
	}
	return nil
}

func (s *Service) publishedVoxelFile(ctx context.Context, projectID string, revision int64, path string) ([]byte, error) {
	var hash string
	var size int64
	err := s.db.QueryRowContext(ctx, `SELECT f.content_hash,f.size FROM gm_revision_files f JOIN gm_revisions r ON r.id=f.revision_id WHERE r.project_id=? AND r.number=? AND f.path=?`, projectID, revision, path).Scan(&hash, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	h, err := hex.DecodeString(hash)
	if err != nil || len(h) != sha256.Size || size < 0 || size > s.opts.MaxProjectBytes {
		return nil, fmt.Errorf("invalid published voxel file metadata")
	}
	data, err := os.ReadFile(filepath.Join(s.blobDir, hash[:2], hash))
	if err != nil {
		return nil, fmt.Errorf("published voxel file unavailable")
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != size || hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("published voxel file integrity failure")
	}
	return data, nil
}

func (s *Service) playStateBinding(ctx context.Context, projectID, token string) (Project, *VoxelDefinition, error) {
	s.mu.RLock()
	grant, ok := s.tokens[strings.TrimSpace(token)]
	s.mu.RUnlock()
	if !ok || grant.Purpose != "play-state" || grant.ProjectID != projectID || grant.JobID != "" || grant.ValidationID != "" || grant.Revision <= 0 || time.Now().After(grant.ExpiresAt) {
		return Project{}, nil, ErrInvalidToken
	}
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		return p, nil, err
	}
	if p.Variant != "voxel" {
		return p, nil, ErrInvalidToken
	}
	if p.CurrentRevision != grant.Revision {
		return p, nil, ErrPlayStateConflict
	}
	data, err := s.publishedVoxelFile(ctx, p.ID, grant.Revision, "src/voxel.json")
	if err != nil {
		return p, nil, err
	}
	v, err := ParseVoxelDefinition(data)
	return p, v, err
}

func (s *Service) GetPlayState(ctx context.Context, projectID, token string) (PlayState, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return PlayState{}, ErrDisabled
	}
	p, v, err := s.playStateBinding(ctx, projectID, token)
	if err != nil {
		return PlayState{}, err
	}
	result := PlayState{Compatibility: v.Compatibility(), Revision: p.CurrentRevision}
	var payload []byte
	err = s.db.QueryRowContext(ctx, `SELECT version,payload FROM gm_play_states WHERE project_id=? AND compatibility=?`, p.ID, result.Compatibility).Scan(&result.Version, &payload)
	result.State = payload
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return result, fmt.Errorf("read voxel save: %w", err)
	}
	if len(result.State) > 0 {
		if err := validateVoxelState(v, result.State); err != nil {
			return PlayState{}, err
		}
	}
	return result, nil
}

func (s *Service) WritePlayState(ctx context.Context, projectID, token string, expected int64, data []byte, reset bool) (PlayState, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	policy := s.policy
	if !policy.Enabled {
		return PlayState{}, ErrDisabled
	}
	if policy.ReadOnly || (!reset && !policy.AllowEdit) || (reset && !policy.AllowDelete) {
		return PlayState{}, ErrReadOnly
	}
	if expected < 0 || expected >= 9007199254740991 {
		return PlayState{}, fmt.Errorf("invalid save version")
	}
	p, v, err := s.playStateBinding(ctx, projectID, token)
	if err != nil {
		return PlayState{}, err
	}
	var payload any
	if !reset {
		if err := validateVoxelState(v, data); err != nil {
			return PlayState{}, err
		}
		payload = data
	}
	compatibility := v.Compatibility()
	result, err := s.db.ExecContext(ctx, `INSERT INTO gm_play_states(project_id,compatibility,version,revision,payload,updated_at)
	 SELECT ?,?,1,?,?,? WHERE EXISTS(SELECT 1 FROM gm_projects WHERE id=? AND current_revision=?)
	 AND (?=0 OR EXISTS(SELECT 1 FROM gm_play_states WHERE project_id=? AND compatibility=? AND version=?))
	 ON CONFLICT(project_id,compatibility) DO UPDATE SET version=gm_play_states.version+1,revision=excluded.revision,payload=excluded.payload,updated_at=excluded.updated_at WHERE gm_play_states.version=?`,
		p.ID, compatibility, p.CurrentRevision, payload, time.Now().UTC(), p.ID, p.CurrentRevision, expected, p.ID, compatibility, expected, expected)
	if err != nil {
		return PlayState{}, fmt.Errorf("write voxel save: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return PlayState{}, err
	}
	if n != 1 {
		return PlayState{}, ErrPlayStateConflict
	}
	return PlayState{Compatibility: compatibility, Version: expected + 1, Revision: p.CurrentRevision}, nil
}

package flows

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
)

// BlockDependency identifies a retained parent version or run, without payloads.
type BlockDependency struct {
	Kind          string `json:"kind"`
	FlowID        string `json:"flow_id"`
	Revision      int    `json:"revision"`
	RunID         string `json:"run_id,omitempty"`
	BlockRevision int    `json:"block_revision"`
}

// BlockDependentsPage contains one stable, bounded keyset page.
type BlockDependentsPage struct {
	Items      []BlockDependency `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

func blockDependencyCursor(value BlockDependency) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func parseBlockDependencyCursor(cursor string) (BlockDependency, error) {
	var value BlockDependency
	if cursor == "" {
		return value, nil
	}
	if len(cursor) > 1024 {
		return value, ErrBlockInvalid
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || json.Unmarshal(data, &value) != nil || (value.Kind != "published" && value.Kind != "run") ||
		!blockFlowID.MatchString(value.FlowID) || value.Revision < 0 || value.Revision > math.MaxInt32 ||
		value.BlockRevision <= 0 || value.BlockRevision > math.MaxInt32 || len(value.RunID) > 128 ||
		(value.Kind == "published" && value.RunID != "") || (value.Kind == "run" && value.RunID == "") {
		return BlockDependency{}, ErrBlockInvalid
	}
	return value, nil
}

// BlockDependents returns only parent identities; deletion independently rechecks pins.
func (s *Service) BlockDependents(ctx context.Context, id string, revision int, cursor string, limit int) (*BlockDependentsPage, error) {
	if limit < 1 || limit > 100 || revision < 0 || revision > math.MaxInt32 {
		return nil, ErrBlockInvalid
	}
	if _, err := s.GetBlock(ctx, id); err != nil {
		return nil, err
	}
	if revision > 0 {
		if _, err := s.GetBlockVersion(ctx, id, revision); err != nil {
			return nil, err
		}
	}
	last, err := parseBlockDependencyCursor(cursor)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.db.QueryContext(ctx, `WITH refs(kind,flow_id,revision,run_id,block_revision) AS (
		SELECT 'published',parent_flow_id,parent_revision,'',block_revision FROM flow_block_version_pins WHERE block_flow_id=? AND (?=0 OR block_revision=?)
		UNION ALL
		SELECT 'run',r.flow_id,r.revision,p.run_id,p.block_revision FROM flow_block_run_pins p JOIN flow_runs r ON r.id=p.run_id WHERE p.block_flow_id=? AND (?=0 OR p.block_revision=?)
	) SELECT kind,flow_id,revision,run_id,block_revision FROM refs
	WHERE (kind,flow_id,revision,run_id,block_revision)>(?,?,?,?,?)
	ORDER BY kind,flow_id,revision,run_id,block_revision LIMIT ?`, id, revision, revision, id, revision, revision,
		last.Kind, last.FlowID, last.Revision, last.RunID, last.BlockRevision, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &BlockDependentsPage{Items: []BlockDependency{}}
	for rows.Next() {
		var item BlockDependency
		if err := rows.Scan(&item.Kind, &item.FlowID, &item.Revision, &item.RunID, &item.BlockRevision); err != nil {
			return nil, err
		}
		if !blockFlowID.MatchString(item.FlowID) || len(item.RunID) > 128 {
			return nil, ErrBlockInvalid
		}
		if len(page.Items) == limit {
			page.NextCursor = blockDependencyCursor(page.Items[len(page.Items)-1])
			break
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return page, nil
}

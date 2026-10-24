package infra

import (
	"context"
	"fmt"
	"strings"
)

// SaveLayout atomically stores one view's positions. Items may also move a node
// into or out of a place (ParentID: nil keeps the parent, "" clears it) and size
// a place (W, H: nil keeps the stored size).
func (s *Store) SaveLayout(ctx context.Context, request LayoutRequest) ([]Node, error) {
	if request.View != ViewAtlas && request.View != ViewTopology {
		return nil, fmt.Errorf("%w: unsupported layout view", ErrValidation)
	}
	if len(request.Nodes) > 1000 {
		return nil, fmt.Errorf("%w: layout update exceeds 1000 nodes", ErrValidation)
	}
	seen := make(map[string]struct{}, len(request.Nodes))
	for _, item := range request.Nodes {
		if err := validateText("layout node id", item.ID, 128, true); err != nil {
			return nil, err
		}
		if item.Version <= 0 || !validCoordinate(item.X) || !validCoordinate(item.Y) {
			return nil, fmt.Errorf("%w: layout item needs a version and bounded coordinates", ErrValidation)
		}
		if item.ParentID != nil {
			if err := validateText("layout parent id", *item.ParentID, 128, false); err != nil {
				return nil, err
			}
		}
		if _, exists := seen[item.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate node in layout update", ErrValidation)
		}
		seen[item.ID] = struct{}{}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin infra layout save: %w", err)
	}
	defer tx.Rollback()
	for _, item := range request.Nodes {
		node, err := loadNode(ctx, tx, item.ID)
		if err != nil {
			return nil, err
		}
		if node.Version != item.Version {
			return nil, ErrConflict
		}
		placement := Placement{X: item.X, Y: item.Y, Pinned: item.Pinned}
		if current, ok := node.Placements[request.View]; ok {
			placement.W, placement.H = current.W, current.H
		}
		if item.W != nil {
			placement.W = *item.W
		}
		if item.H != nil {
			placement.H = *item.H
		}
		if err := validatePlacement(node.Kind, placement); err != nil {
			return nil, err
		}
		if item.ParentID != nil {
			parentID := strings.TrimSpace(*item.ParentID)
			// Earlier items are already written in this transaction, so a cycle
			// built across several items of one batch is detected as well.
			if err := validateParent(ctx, tx, node.ID, parentID); err != nil {
				return nil, err
			}
			node.ParentID = parentID
		}
		previous := node.Version
		node.Version++
		node.Placements[request.View] = placement
		if err := updateNodeTx(ctx, tx, node, previous); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit infra layout save: %w", err)
	}
	updated := make([]Node, 0, len(request.Nodes))
	for _, item := range request.Nodes {
		node, err := s.GetNode(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		updated = append(updated, node)
	}
	return updated, nil
}

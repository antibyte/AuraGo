package gamemaker

import (
	"context"
	"encoding/json"
)

// requestedSelection is the asset choice posted with a job and retained on resume.
type requestedSelection struct {
	AssetPackIDs    []string         `json:"asset_pack_ids,omitempty"`
	ModelAssetIDs   []string         `json:"model_asset_ids,omitempty"`
	AssetSelections []AssetSelection `json:"asset_selections,omitempty"`
	Presentation    *Presentation    `json:"presentation,omitempty"`
}

func (r requestedSelection) empty() bool {
	return len(r.AssetPackIDs) == 0 && len(r.ModelAssetIDs) == 0 && len(r.AssetSelections) == 0 && r.Presentation == nil
}

func encodeRequestedSelection(packIDs, modelIDs []string, assets []AssetSelection, presentation *Presentation) string {
	payload := requestedSelection{
		AssetPackIDs:    append([]string(nil), packIDs...),
		ModelAssetIDs:   append([]string(nil), modelIDs...),
		AssetSelections: append([]AssetSelection(nil), assets...),
		Presentation:    presentation,
	}
	if payload.empty() {
		return ""
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeRequestedSelection(raw string) (requestedSelection, bool) {
	if raw == "" {
		return requestedSelection{}, false
	}
	var payload requestedSelection
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload.empty() {
		return requestedSelection{}, false
	}
	return payload, true
}

func (s *Service) storedSelection(ctx context.Context, jobID string) string {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT requested_selection FROM gm_jobs WHERE id=?`, jobID).Scan(&raw); err != nil {
		return ""
	}
	return raw
}

// replayPendingSelection applies the stored selection when the client sent none.
func (s *Service) replayPendingSelection(ctx context.Context, job Job, packIDs, modelIDs []string, presentation *Presentation, assets []AssetSelection) ([]string, []string, *Presentation, []AssetSelection) {
	if job.ResumeFrom == "" || !(requestedSelection{AssetPackIDs: packIDs, ModelAssetIDs: modelIDs, AssetSelections: assets, Presentation: presentation}).empty() {
		return packIDs, modelIDs, presentation, assets
	}
	payload, ok := decodeRequestedSelection(s.storedSelection(ctx, job.ID))
	if !ok {
		return packIDs, modelIDs, presentation, assets
	}
	return payload.AssetPackIDs, payload.ModelAssetIDs, payload.Presentation, payload.AssetSelections
}

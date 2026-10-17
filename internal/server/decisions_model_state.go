package server

import (
	"crypto/sha256"
	"encoding/json"
	"time"

	"aurago/internal/decisions"
)

type decisionModelEpochKey struct{}

// The caller holds d.mu. Initial discovery must not revoke its own first call.
func (d *routedDecisions) publishModelCatalog(g *decisionProviderGeneration, models []decisions.DecisionModel) {
	checkedAt := time.Now()
	if catalog, ok := g.client.(interface{ CatalogCheckedAt() time.Time }); ok {
		checkedAt = catalog.CatalogCheckedAt()
	}
	next := decisions.CheckModelCatalog(models, g.settings.Model, checkedAt)
	var identity any = next.Code
	if model := next.Model; model != nil {
		identity = []any{model.ResolvedID(), model.ContextLength, model.SupportsDecisions, model.SupportsImages, model.MediaLimits, next.Code}
	}
	data, _ := json.Marshal(identity)
	hash := sha256.Sum256(data)
	if g.modelChecked && g.modelIdentity != hash {
		g.modelEpoch++
		g.status.Ready = false
		previous := g.status.ModelAvailability.Model
		if previous != nil && next.Model != nil && previous.ResolvedID() != next.Model.ResolvedID() {
			g.status.AliasChanged = true
		}
	}
	g.modelChecked, g.modelIdentity = true, hash
	g.status.ModelAvailability = next
	if next.State == "blocked" {
		g.status.Ready, g.status.State, g.status.Code = false, "needs_setup", next.Code
	} else if g.status.State == "needs_setup" && g.status.Configured {
		g.status.State, g.status.Code = "configured", ""
	}
}

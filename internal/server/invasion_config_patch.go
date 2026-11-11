package server

import (
	"fmt"

	"aurago/internal/config"
	"aurago/internal/invasion"
)

// Keep connection selections across partial patches; permission snapshots own grants.
func prepareEggConfigPatch(cfg *config.Config, egg invasion.EggRecord, previous, patch invasion.SafeConfigPatch) (invasion.EggRecord, invasion.SafeConfigPatch, error) {
	if err := invasion.ValidateSafeConfigPatch(patch); err != nil {
		return egg, patch, err
	}
	if patch.Provider == nil && patch.InheritLLM == nil {
		patch.Provider, patch.InheritLLM = previous.Provider, previous.InheritLLM
		if patch.Model == nil {
			patch.Model = previous.Model
		}
		if patch.BaseURL == nil {
			patch.BaseURL = previous.BaseURL
		}
	}
	if patch.Provider != nil {
		if cfg.FindProvider(*patch.Provider) == nil {
			return egg, patch, fmt.Errorf("Egg provider must reference a saved provider")
		}
		inherit := false
		patch.InheritLLM = &inherit
		egg.Provider, egg.BaseURL, egg.Model = *patch.Provider, "", ""
	}
	if err := invasion.ValidateSafeConfigPatch(patch); err != nil {
		return egg, patch, err
	}
	if patch.InheritLLM != nil {
		egg.InheritLLM = *patch.InheritLLM
	}
	if patch.Model != nil {
		egg.Model = *patch.Model
	}
	if patch.BaseURL != nil {
		egg.BaseURL = *patch.BaseURL
	}
	return egg, patch, nil
}

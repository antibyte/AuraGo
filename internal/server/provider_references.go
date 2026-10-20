package server

import (
	"aurago/internal/config"
)

type providerReferencePayload struct {
	Path string `json:"path"`
	Role string `json:"role,omitempty"`
}

func providerReferences(cfg *config.Config, providerID string) []providerReferencePayload {
	var refs []providerReferencePayload
	for _, ref := range cfg.ProviderReferences() {
		if ref.ProviderID == providerID {
			refs = append(refs, providerReferencePayload{Path: ref.Path, Role: ref.Role})
		}
	}
	return refs
}

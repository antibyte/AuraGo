package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/llm"
	llmcatalog "aurago/internal/llm/catalog"

	"github.com/gorilla/websocket"
)

func agodeskServerCapabilitiesForDevice(s *Server, readOnly bool) []string {
	capabilities := agodeskServerCapabilities(s)
	if !readOnly {
		return capabilities
	}
	filtered := capabilities[:0]
	for _, capability := range capabilities {
		switch capability {
		case agodesk.CapabilityConfigProvidersWrite,
			agodesk.CapabilityConfigProvidersOAuth,
			agodesk.CapabilityKnowledgeArchive:
			continue
		default:
			filtered = append(filtered, capability)
		}
	}
	return append([]string(nil), filtered...)
}

func agodeskProviderManagementReadable(s *Server) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	enabled := s.Cfg.WebConfig.Enabled
	s.CfgMu.RUnlock()
	return enabled
}

func agodeskProviderManagementWritable(s *Server) bool {
	return agodeskProviderManagementReadable(s) && s != nil && s.Vault != nil
}

func agodeskStateReadOnly(state *agodeskConnectionState) bool {
	if state == nil {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.readOnly
}

func validateAgodeskProviderCommand(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID, sessionID, capability, messageName string) (string, bool) {
	transportSessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, sessionID, messageName)
	if !ok {
		return "", false
	}
	if !agodeskProviderManagementReadable(s) {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorUnsupportedCapability, "Provider management requires web_config.enabled")
		return "", false
	}
	if !validateAgodeskCapability(conn, state, requestID, capability, messageName) {
		return "", false
	}
	if capability == agodesk.CapabilityConfigProvidersWrite || capability == agodesk.CapabilityConfigProvidersOAuth {
		if agodeskStateReadOnly(state) {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorRemoteReadOnly, messageName+" is not available for read-only devices")
			return "", false
		}
		if !agodeskProviderManagementWritable(s) {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorUnsupportedCapability, messageName+" requires an available vault")
			return "", false
		}
	}
	return transportSessionID, true
}

func handleAgodeskProviderCatalogList(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderCatalogListPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersRead, "config.provider.catalog.list")
	if !ok {
		return
	}
	response, err := agodeskProviderCatalogPayload(s, sessionID, agodesk.ConfigProviderCatalogDetailPayload{
		SessionID:     sessionID,
		IncludeModels: payload.IncludeModels,
	})
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderCatalog, response)
}

func handleAgodeskProviderCatalogDetail(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderCatalogDetailPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersRead, "config.provider.catalog.detail")
	if !ok {
		return
	}
	payload.SessionID = sessionID
	response, err := agodeskProviderCatalogPayload(s, sessionID, payload)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderCatalog, response)
}

func handleAgodeskProvidersList(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProvidersListPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersRead, "config.providers.list")
	if !ok {
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviders, agodeskProvidersPayload(s, sessionID))
}

func handleAgodeskProviderGet(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderGetPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersRead, "config.provider.get")
	if !ok {
		return
	}
	provider, ok := agodeskProviderPayloadByID(s, sessionID, payload.ProviderID)
	if !ok {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, "provider not found")
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProvider, agodesk.ConfigProviderPayload{
		SessionID: sessionID,
		Status:    "ok",
		Provider:  provider,
	})
}

func handleAgodeskProviderUpsert(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderUpsertPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersWrite, "config.provider.upsert")
	if !ok {
		return
	}
	payload.SessionID = sessionID
	provider, err := upsertAgodeskProvider(s, payload)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProvider, agodesk.ConfigProviderPayload{
		SessionID: sessionID,
		Status:    "ok",
		Provider:  provider,
	})
}

func handleAgodeskProviderDelete(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderDeletePayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersWrite, "config.provider.delete")
	if !ok {
		return
	}
	payload.SessionID = sessionID
	response, err := deleteAgodeskProvider(s, payload)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviders, response)
}

func handleAgodeskProviderTest(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderTestPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersWrite, "config.provider.test")
	if !ok {
		return
	}
	result := testAgodeskProvider(s, sessionID, payload.ProviderID)
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderTestResult, result)
}

func handleAgodeskProviderOAuthStart(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderOAuthStartPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersOAuth, "config.provider.oauth.start")
	if !ok {
		return
	}
	payload.SessionID = sessionID
	started, err := startAgodeskProviderOAuth(s, payload, time.Now().UTC())
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderOAuthStarted, started)
}

func handleAgodeskProviderOAuthComplete(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderOAuthCompletePayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersOAuth, "config.provider.oauth.complete")
	if !ok {
		return
	}
	payload.SessionID = sessionID
	status, err := completeAgodeskProviderOAuth(s, payload, time.Now().UTC())
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, err.Error())
		return
	}
	status.SessionID = sessionID
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderOAuthStatus, status)
}

func handleAgodeskProviderOAuthStatus(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderOAuthStatusRequestPayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersRead, "config.provider.oauth.status")
	if !ok {
		return
	}
	status := agodeskProviderOAuthStatus(s, payload.ProviderID, payload.RedirectURI)
	status.SessionID = sessionID
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderOAuthStatus, status)
}

func handleAgodeskProviderOAuthRevoke(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ConfigProviderOAuthRevokePayload) {
	sessionID, ok := validateAgodeskProviderCommand(s, conn, state, requestID, payload.SessionID, agodesk.CapabilityConfigProvidersOAuth, "config.provider.oauth.revoke")
	if !ok {
		return
	}
	if err := revokeAgodeskProviderOAuth(s, payload.ProviderID); err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, err.Error())
		return
	}
	status := agodeskProviderOAuthStatus(s, payload.ProviderID, "")
	status.SessionID = sessionID
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeConfigProviderOAuthStatus, status)
}

func agodeskProvidersPayload(s *Server, sessionID string) agodesk.ConfigProvidersPayload {
	s.CfgMu.RLock()
	cfg := *s.Cfg
	providers := append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	fallback := llm.CapabilityFallback{
		ToolCalling:       s.Cfg.LLM.UseNativeFunctions,
		StructuredOutputs: s.Cfg.LLM.StructuredOutputs,
		Multimodal:        s.Cfg.LLM.Multimodal,
	}
	s.CfgMu.RUnlock()

	out := make([]agodesk.ConfigProviderEntryPayload, 0, len(providers))
	for _, provider := range providers {
		out = append(out, agodeskProviderPayload(s, &cfg, provider, fallback))
	}
	return agodesk.ConfigProvidersPayload{
		SessionID: strings.TrimSpace(sessionID),
		Status:    "ok",
		Providers: out,
	}
}

func agodeskProviderPayloadByID(s *Server, sessionID, providerID string) (agodesk.ConfigProviderEntryPayload, bool) {
	payload := agodeskProvidersPayload(s, sessionID)
	for _, provider := range payload.Providers {
		if strings.EqualFold(provider.ID, strings.TrimSpace(providerID)) {
			return provider, true
		}
	}
	return agodesk.ConfigProviderEntryPayload{}, false
}

func agodeskProviderPayload(s *Server, cfg *config.Config, provider config.ProviderEntry, fallback llm.CapabilityFallback) agodesk.ConfigProviderEntryPayload {
	authType := normalizeProviderAuthType(provider.AuthType)
	apiKeyPresent := authType != "oauth2" && strings.TrimSpace(provider.APIKey) != ""
	clientSecretPresent := authType == "oauth2" && strings.TrimSpace(provider.OAuthClientSecret) != ""
	if s != nil && s.Vault != nil {
		if authType != "oauth2" && agodeskVaultSecretPresent(s, "provider_"+provider.ID+"_api_key") {
			apiKeyPresent = true
		}
		if authType == "oauth2" && agodeskVaultSecretPresent(s, "provider_"+provider.ID+"_oauth_client_secret") {
			clientSecretPresent = true
		}
	}
	caps := providerCapabilitiesToJSON(provider.Capabilities)
	effective := providerCapabilitiesResultToJSON(llm.ResolveProviderCapabilities(provider, fallback))
	return agodesk.ConfigProviderEntryPayload{
		ID:                    provider.ID,
		Name:                  provider.Name,
		Type:                  provider.Type,
		BaseURL:               provider.BaseURL,
		Model:                 provider.Model,
		AccountID:             provider.AccountID,
		AuthType:              authType,
		OAuthAuthURL:          provider.OAuthAuthURL,
		OAuthTokenURL:         provider.OAuthTokenURL,
		OAuthClientID:         provider.OAuthClientID,
		OAuthScopes:           provider.OAuthScopes,
		Models:                agodeskModelCostPayloads(provider.Models),
		Capabilities:          agodeskProviderCapabilitiesPayload(caps),
		EffectiveCapabilities: *agodeskProviderCapabilitiesPayload(effective),
		Secrets: agodesk.ConfigProviderSecretsPayload{
			APIKey:            agodesk.SecretPresencePayload{Present: apiKeyPresent},
			OAuthClientSecret: agodesk.SecretPresencePayload{Present: clientSecretPresent},
		},
		OAuth:      agodeskProviderOAuthStatus(s, provider.ID, ""),
		References: agodeskProviderReferences(cfg, provider.ID),
	}
}

func agodeskVaultSecretPresent(s *Server, key string) bool {
	if s == nil || s.Vault == nil || strings.TrimSpace(key) == "" {
		return false
	}
	value, err := s.Vault.ReadSecret(key)
	return err == nil && strings.TrimSpace(value) != ""
}

func agodeskModelCostPayloads(models []config.ModelCost) []agodesk.ProviderModelCostPayload {
	if len(models) == 0 {
		return nil
	}
	out := make([]agodesk.ProviderModelCostPayload, 0, len(models))
	for _, model := range models {
		out = append(out, agodesk.ProviderModelCostPayload{
			Name:             model.Name,
			InputPerMillion:  model.InputPerMillion,
			OutputPerMillion: model.OutputPerMillion,
		})
	}
	return out
}

func agodeskProviderModelCosts(models []agodesk.ProviderModelCostPayload) []config.ModelCost {
	if len(models) == 0 {
		return nil
	}
	out := make([]config.ModelCost, 0, len(models))
	for _, model := range models {
		out = append(out, config.ModelCost{
			Name:             strings.TrimSpace(model.Name),
			InputPerMillion:  model.InputPerMillion,
			OutputPerMillion: model.OutputPerMillion,
		})
	}
	return out
}

func agodeskProviderCapabilitiesPayload(c providerCapabilitiesJSON) *agodesk.ProviderCapabilitiesPayload {
	return &agodesk.ProviderCapabilitiesPayload{
		Auto:              c.Auto,
		ToolCalling:       c.ToolCalling,
		StructuredOutputs: c.StructuredOutputs,
		Multimodal:        c.Multimodal,
		DetectedModel:     c.DetectedModel,
		Source:            c.Source,
		Known:             c.Known,
	}
}

func agodeskProviderCapabilitiesFromPayload(c *agodesk.ProviderCapabilitiesPayload) config.ProviderCapabilities {
	if c == nil {
		return config.ProviderCapabilities{}
	}
	return config.ProviderCapabilities{
		Auto:              boolPtr(c.Auto),
		ToolCalling:       c.ToolCalling,
		StructuredOutputs: c.StructuredOutputs,
		Multimodal:        c.Multimodal,
		DetectedModel:     c.DetectedModel,
		Source:            c.Source,
	}
}

func agodeskProviderAuthTypeForUpsert(existing config.ProviderEntry, provider agodesk.ConfigProviderEntryPayload) string {
	if strings.TrimSpace(provider.AuthType) != "" {
		return normalizeProviderAuthType(provider.AuthType)
	}
	if agodeskProviderPayloadHasOAuthFields(provider) {
		return "oauth2"
	}
	if strings.TrimSpace(existing.AuthType) != "" {
		return normalizeProviderAuthType(existing.AuthType)
	}
	return "api_key"
}

func agodeskProviderPayloadHasOAuthFields(provider agodesk.ConfigProviderEntryPayload) bool {
	return strings.TrimSpace(provider.OAuthAuthURL) != "" ||
		strings.TrimSpace(provider.OAuthTokenURL) != "" ||
		strings.TrimSpace(provider.OAuthClientID) != "" ||
		strings.TrimSpace(provider.OAuthScopes) != ""
}

func agodeskProviderReferences(cfg *config.Config, providerID string) []agodesk.ProviderReferencePayload {
	refs := providerReferences(cfg, providerID)
	if len(refs) == 0 {
		return nil
	}
	out := make([]agodesk.ProviderReferencePayload, 0, len(refs))
	for _, ref := range refs {
		out = append(out, agodesk.ProviderReferencePayload{Path: ref.Path, Role: ref.Role})
	}
	return out
}

func upsertAgodeskProvider(s *Server, payload agodesk.ConfigProviderUpsertPayload) (agodesk.ConfigProviderEntryPayload, error) {
	if s == nil || s.Cfg == nil {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("server config is not available")
	}
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	if mode == "" {
		mode = "update"
	}
	if mode != "create" && mode != "update" {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("mode must be create or update")
	}
	providerID := strings.TrimSpace(payload.Provider.ID)

	s.CfgMu.RLock()
	current := append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	fallback := llm.CapabilityFallback{
		ToolCalling:       s.Cfg.LLM.UseNativeFunctions,
		StructuredOutputs: s.Cfg.LLM.StructuredOutputs,
		Multimodal:        s.Cfg.LLM.Multimodal,
	}
	oldProviderIDs := make([]string, 0, len(current))
	existingIDSet := make(map[string]bool, len(current))
	for _, provider := range current {
		oldProviderIDs = append(oldProviderIDs, provider.ID)
		existingIDSet[provider.ID] = true
	}
	s.CfgMu.RUnlock()

	// Create always requires a canonical ID. Updates may keep a legacy ID that
	// already exists in config so renames stay explicit and vault keys stay stable.
	if mode == "create" {
		if err := validateProviderID(providerID); err != nil {
			return agodesk.ConfigProviderEntryPayload{}, err
		}
	} else if err := validateProviderIDForSave(providerID, existingIDSet); err != nil {
		return agodesk.ConfigProviderEntryPayload{}, err
	}

	index := -1
	var existing config.ProviderEntry
	for i, provider := range current {
		if strings.EqualFold(provider.ID, providerID) {
			index = i
			existing = provider
			break
		}
	}
	if mode == "create" && index >= 0 {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("provider already exists: %s", providerID)
	}
	if mode == "update" && index < 0 {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("provider not found: %s", providerID)
	}

	entry := existing
	entry.ID = providerID
	entry.Name = payload.Provider.Name
	entry.Type = payload.Provider.Type
	entry.BaseURL = payload.Provider.BaseURL
	entry.Model = payload.Provider.Model
	entry.AccountID = payload.Provider.AccountID
	entry.AuthType = agodeskProviderAuthTypeForUpsert(existing, payload.Provider)
	entry.OAuthAuthURL = payload.Provider.OAuthAuthURL
	entry.OAuthTokenURL = payload.Provider.OAuthTokenURL
	entry.OAuthClientID = payload.Provider.OAuthClientID
	entry.OAuthScopes = payload.Provider.OAuthScopes
	entry.Models = agodeskProviderModelCosts(payload.Provider.Models)
	entry.Capabilities = agodeskProviderCapabilitiesFromPayload(payload.Provider.Capabilities)

	existingAPIKey := existing.APIKey
	existingClientSecret := existing.OAuthClientSecret
	if s.Vault != nil {
		if value, err := s.Vault.ReadSecret("provider_" + providerID + "_api_key"); err == nil {
			existingAPIKey = value
		}
		if value, err := s.Vault.ReadSecret("provider_" + providerID + "_oauth_client_secret"); err == nil {
			existingClientSecret = value
		}
	}

	apiKey, err := applyAgodeskSecretOperation(payload.Secrets.APIKey, existingAPIKey)
	if err != nil {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("api_key secret op: %w", err)
	}
	clientSecret, err := applyAgodeskSecretOperation(payload.Secrets.OAuthClientSecret, existingClientSecret)
	if err != nil {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("oauth_client_secret secret op: %w", err)
	}

	vaultMutations := []vaultMutation{}
	if entry.AuthType == "oauth2" {
		entry.APIKey = ""
		entry.OAuthClientSecret = clientSecret
		vaultMutations = append(vaultMutations, vaultMutation{key: "provider_" + providerID + "_api_key", delete: true})
		if strings.TrimSpace(clientSecret) == "" {
			vaultMutations = append(vaultMutations, vaultMutation{key: "provider_" + providerID + "_oauth_client_secret", delete: true})
		} else {
			vaultMutations = append(vaultMutations, vaultMutation{key: "provider_" + providerID + "_oauth_client_secret", value: clientSecret})
		}
	} else {
		entry.APIKey = apiKey
		entry.OAuthClientSecret = ""
		vaultMutations = append(vaultMutations,
			vaultMutation{key: "provider_" + providerID + "_oauth_client_secret", delete: true},
			vaultMutation{key: "oauth_" + providerID, delete: true},
		)
		if strings.TrimSpace(apiKey) == "" {
			vaultMutations = append(vaultMutations, vaultMutation{key: "provider_" + providerID + "_api_key", delete: true})
		} else {
			vaultMutations = append(vaultMutations, vaultMutation{key: "provider_" + providerID + "_api_key", value: apiKey})
		}
	}

	if index >= 0 {
		current[index] = entry
	} else {
		current = append(current, entry)
	}
	if _, err := persistProviderEntries(s, current, vaultMutations, oldProviderIDs); err != nil {
		return agodesk.ConfigProviderEntryPayload{}, err
	}

	s.CfgMu.RLock()
	cfg := *s.Cfg
	updated := s.Cfg.FindProvider(providerID)
	var updatedEntry config.ProviderEntry
	if updated != nil {
		updatedEntry = *updated
	}
	s.CfgMu.RUnlock()
	if updated == nil {
		return agodesk.ConfigProviderEntryPayload{}, fmt.Errorf("provider was saved but could not be reloaded")
	}
	return agodeskProviderPayload(s, &cfg, updatedEntry, fallback), nil
}

func applyAgodeskSecretOperation(op agodesk.SecretOperationPayload, existing string) (string, error) {
	operation := strings.ToLower(strings.TrimSpace(op.Op))
	if operation == "" {
		operation = "keep"
	}
	switch operation {
	case "keep":
		return existing, nil
	case "set":
		return op.Value, nil
	case "clear":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported operation %q", op.Op)
	}
}

func deleteAgodeskProvider(s *Server, payload agodesk.ConfigProviderDeletePayload) (agodesk.ConfigProvidersPayload, error) {
	providerID := strings.TrimSpace(payload.ProviderID)
	if providerID == "" {
		return agodesk.ConfigProvidersPayload{}, fmt.Errorf("provider_id is required")
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	current := append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	oldProviderIDs := make([]string, 0, len(current))
	for _, provider := range current {
		oldProviderIDs = append(oldProviderIDs, provider.ID)
	}
	s.CfgMu.RUnlock()

	references := agodeskProviderReferences(&cfg, providerID)
	if len(references) > 0 && !payload.Force {
		return agodesk.ConfigProvidersPayload{}, fmt.Errorf("provider %s is still referenced", providerID)
	}
	filtered := make([]config.ProviderEntry, 0, len(current))
	found := false
	for _, provider := range current {
		if strings.EqualFold(provider.ID, providerID) {
			found = true
			continue
		}
		filtered = append(filtered, provider)
	}
	if !found {
		return agodesk.ConfigProvidersPayload{}, fmt.Errorf("provider not found: %s", providerID)
	}
	if _, err := persistProviderEntries(s, filtered, nil, oldProviderIDs); err != nil {
		return agodesk.ConfigProvidersPayload{}, err
	}
	return agodeskProvidersPayload(s, payload.SessionID), nil
}

func testAgodeskProvider(s *Server, sessionID, providerID string) agodesk.ConfigProviderTestResultPayload {
	providerID = strings.TrimSpace(providerID)
	s.CfgMu.RLock()
	prov := s.Cfg.FindProvider(providerID)
	var entry config.ProviderEntry
	if prov != nil {
		entry = *prov
	}
	s.CfgMu.RUnlock()
	if prov == nil {
		return agodesk.ConfigProviderTestResultPayload{SessionID: sessionID, ProviderID: providerID, Status: "error", OK: false, Message: "provider not found"}
	}
	authType := normalizeProviderAuthType(entry.AuthType)
	warnings := []string{}
	if strings.TrimSpace(entry.Type) == "" {
		warnings = append(warnings, "provider type is missing")
	}
	if strings.TrimSpace(entry.Model) == "" {
		warnings = append(warnings, "model is missing")
	}
	if authType == "oauth2" {
		status := agodeskProviderOAuthStatus(s, providerID, "")
		if !status.Authorized {
			warnings = append(warnings, "OAuth is not authorized")
		}
	} else if !providerTypeWorksWithoutKey(entry.Type) && strings.TrimSpace(entry.APIKey) == "" && !agodeskVaultSecretPresent(s, "provider_"+providerID+"_api_key") {
		warnings = append(warnings, "API key is missing")
	}
	ok := len(warnings) == 0
	status := "ok"
	message := "Provider configuration looks usable."
	if !ok {
		status = "warning"
		message = "Provider configuration needs attention."
	}
	return agodesk.ConfigProviderTestResultPayload{
		SessionID:  strings.TrimSpace(sessionID),
		ProviderID: providerID,
		Status:     status,
		OK:         ok,
		Message:    message,
		Warnings:   warnings,
	}
}

func agodeskProviderCatalogPayload(s *Server, sessionID string, detail agodesk.ConfigProviderCatalogDetailPayload) (agodesk.ConfigProviderCatalogPayload, error) {
	snapshot, err := llmcatalog.Load()
	if err != nil {
		return agodesk.ConfigProviderCatalogPayload{}, fmt.Errorf("model catalog unavailable")
	}
	var cfg *config.Config
	if s != nil && s.Cfg != nil {
		s.CfgMu.RLock()
		cfgCopy := *s.Cfg
		cfg = &cfgCopy
		s.CfgMu.RUnlock()
	}
	enabled := cfg == nil || cfg.ModelCatalog.Enabled
	response := agodesk.ConfigProviderCatalogPayload{
		SessionID: strings.TrimSpace(sessionID),
		Status:    "ok",
		Enabled:   enabled,
		Metadata:  agodeskCatalogMetadata(snapshot.Metadata),
	}
	if !enabled {
		return response, nil
	}

	disabled := disabledCatalogProviders(cfg)
	modelCounts := map[string]int{}
	for _, model := range snapshot.Models {
		modelCounts[model.Provider]++
	}

	providers := snapshot.Providers
	if key := strings.TrimSpace(firstNonEmpty(detail.ProviderID, detail.ProviderType)); key != "" {
		provider, ok := snapshot.FindProvider(key)
		if !ok {
			return agodesk.ConfigProviderCatalogPayload{}, fmt.Errorf("catalog provider not found: %s", key)
		}
		providers = []llmcatalog.Provider{provider}
	}
	for _, provider := range providers {
		if provider.CatalogOnly && cfg != nil && !cfg.ModelCatalog.CatalogOnlyVisible {
			continue
		}
		available, availability := catalogProviderAvailability(cfg, provider, disabled)
		response.Providers = append(response.Providers, agodeskCatalogProvider(provider, modelCounts[provider.AuraProviderType], available, availability))
	}

	if detail.IncludeModels {
		for _, model := range snapshot.Models {
			provider, _ := snapshot.FindProvider(model.Provider)
			if provider.CatalogOnly && cfg != nil && !cfg.ModelCatalog.CatalogOnlyVisible {
				continue
			}
			if strings.TrimSpace(detail.ProviderID) != "" || strings.TrimSpace(detail.ProviderType) != "" {
				key := strings.TrimSpace(firstNonEmpty(detail.ProviderID, detail.ProviderType))
				selected, _ := snapshot.FindProvider(key)
				if selected.AuraProviderType != model.Provider && !strings.EqualFold(selected.ID, model.Provider) {
					continue
				}
			}
			response.Models = append(response.Models, agodeskCatalogModel(model, provider.CatalogOnly))
		}
	}
	return response, nil
}

func agodeskCatalogMetadata(metadata llmcatalog.Metadata) agodesk.ProviderCatalogMetadataPayload {
	return agodesk.ProviderCatalogMetadataPayload{
		PackageName:   metadata.PackageName,
		Version:       metadata.Version,
		TarballURL:    metadata.TarballURL,
		SyncedAt:      metadata.SyncedAt,
		RepositoryURL: metadata.RepositoryURL,
		License:       metadata.License,
		Copyright:     metadata.Copyright,
		SourceFiles:   append([]string(nil), metadata.SourceFiles...),
	}
}

func agodeskCatalogProvider(provider llmcatalog.Provider, modelsCount int, available bool, availability string) agodesk.ProviderCatalogProviderPayload {
	return agodesk.ProviderCatalogProviderPayload{
		ID:                         provider.ID,
		AuraProviderType:           provider.AuraProviderType,
		Name:                       provider.Name,
		DefaultModel:               provider.DefaultModel,
		EnvVars:                    append([]string(nil), provider.EnvVars...),
		OAuthProvider:              provider.OAuthProvider,
		OAuthSetup:                 agodeskCatalogOAuthSetup(provider.OAuthSetup),
		AllowUnauthenticated:       provider.AllowUnauthenticated,
		DynamicModelsAuthoritative: provider.DynamicModelsAuthoritative,
		CatalogOnly:                provider.CatalogOnly,
		Available:                  available,
		Availability:               availability,
		ModelsCount:                modelsCount,
	}
}

func agodeskCatalogOAuthSetup(setup *llmcatalog.OAuthSetup) *agodesk.ProviderCatalogOAuthSetupPayload {
	if setup == nil {
		return nil
	}
	return &agodesk.ProviderCatalogOAuthSetupPayload{
		Source:            setup.Source,
		SourcePackage:     setup.SourcePackage,
		SourceProvider:    setup.SourceProvider,
		Flow:              setup.Flow,
		SetupURL:          setup.SetupURL,
		DocsURL:           setup.DocsURL,
		ConsoleLabel:      setup.ConsoleLabel,
		RedirectURIField:  setup.RedirectURIField,
		ClientIDField:     setup.ClientIDField,
		ClientSecretField: setup.ClientSecretField,
		ClientID:          setup.ClientID,
		AuthURL:           setup.AuthURL,
		TokenURL:          setup.TokenURL,
		Scopes:            append([]string(nil), setup.Scopes...),
		CallbackPort:      setup.CallbackPort,
		CallbackPath:      setup.CallbackPath,
	}
}

func agodeskCatalogModel(model llmcatalog.Model, catalogOnly bool) agodesk.ProviderCatalogModelPayload {
	return agodesk.ProviderCatalogModelPayload{
		ID:            model.ID,
		Provider:      model.Provider,
		Name:          model.Name,
		API:           model.API,
		BaseURL:       model.BaseURL,
		ContextWindow: model.ContextWindow,
		MaxTokens:     model.MaxTokens,
		Capabilities: agodesk.ProviderModelCapabilitiesPayload{
			ToolCalling:       model.SupportsTools,
			StructuredOutputs: model.StructuredOutputs,
			Multimodal:        model.Multimodal,
			Reasoning:         model.Reasoning,
		},
		Cost: agodesk.ProviderCatalogCostPayload{
			Input:      model.Cost.Input,
			Output:     model.Cost.Output,
			CacheRead:  model.Cost.CacheRead,
			CacheWrite: model.Cost.CacheWrite,
		},
		CatalogOnly: catalogOnly,
	}
}

func startAgodeskProviderOAuth(s *Server, payload agodesk.ConfigProviderOAuthStartPayload, now time.Time) (agodesk.ConfigProviderOAuthStartedPayload, error) {
	providerID := strings.TrimSpace(payload.ProviderID)
	if providerID == "" {
		return agodesk.ConfigProviderOAuthStartedPayload{}, fmt.Errorf("provider_id is required")
	}
	redirectURI := strings.TrimSpace(payload.RedirectURI)
	if err := validateAgodeskLoopbackRedirectURI(redirectURI); err != nil {
		return agodesk.ConfigProviderOAuthStartedPayload{}, err
	}
	if s == nil || s.Vault == nil {
		return agodesk.ConfigProviderOAuthStartedPayload{}, fmt.Errorf("vault is not available")
	}
	s.CfgMu.RLock()
	prov := s.Cfg.FindProvider(providerID)
	var entry config.ProviderEntry
	if prov != nil {
		entry = *prov
	}
	s.CfgMu.RUnlock()
	if prov == nil {
		return agodesk.ConfigProviderOAuthStartedPayload{}, fmt.Errorf("provider not found: %s", providerID)
	}
	if missing := oauthProviderMissingFields(&entry); len(missing) > 0 {
		return agodesk.ConfigProviderOAuthStartedPayload{}, fmt.Errorf("OAuth2 configuration incomplete: %s", strings.Join(missing, ", "))
	}

	session, err := newOAuthSession(providerID, oauthFlowModeAgodeskLoopback, redirectURI, now)
	if err != nil {
		return agodesk.ConfigProviderOAuthStartedPayload{}, err
	}
	if err := storeOAuthSession(s.Vault, session); err != nil {
		return agodesk.ConfigProviderOAuthStartedPayload{}, err
	}
	authURL, err := buildOAuthAuthorizationURL(entry, session)
	if err != nil {
		return agodesk.ConfigProviderOAuthStartedPayload{}, err
	}
	return agodesk.ConfigProviderOAuthStartedPayload{
		SessionID:     strings.TrimSpace(payload.SessionID),
		ProviderID:    providerID,
		AuthURL:       authURL,
		Mode:          session.Mode,
		OAuthState:    session.State,
		ExpiresAt:     session.ExpiresAt.Format(time.RFC3339),
		FallbackModes: session.FallbackModes,
		RedirectURI:   redirectURI,
	}, nil
}

func completeAgodeskProviderOAuth(s *Server, payload agodesk.ConfigProviderOAuthCompletePayload, now time.Time) (agodesk.ConfigProviderOAuthStatusPayload, error) {
	if s == nil || s.Vault == nil {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("vault is not available")
	}
	code := strings.TrimSpace(payload.Code)
	state := strings.TrimSpace(payload.State)
	redirectURI := strings.TrimSpace(payload.RedirectURI)
	if strings.TrimSpace(payload.RedirectURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(payload.RedirectURL))
		if err != nil {
			return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("invalid redirect_url")
		}
		q := parsed.Query()
		if errParam := q.Get("error"); errParam != "" {
			return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("authorization denied: %s", errParam)
		}
		if code == "" {
			code = q.Get("code")
		}
		if state == "" {
			state = q.Get("state")
		}
		if redirectURI == "" {
			parsed.RawQuery = ""
			parsed.Fragment = ""
			redirectURI = parsed.String()
		}
	}
	if code == "" || state == "" {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("code and state are required")
	}
	session, err := consumeOAuthSession(s.Vault, state, now)
	if err != nil {
		return agodesk.ConfigProviderOAuthStatusPayload{}, err
	}
	if session.Mode != oauthFlowModeAgodeskLoopback {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("OAuth session was not started for agodesk loopback")
	}
	providerID := strings.TrimSpace(payload.ProviderID)
	if providerID == "" {
		providerID = session.ProviderID
	}
	if !strings.EqualFold(providerID, session.ProviderID) {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("OAuth state does not belong to provider %s", providerID)
	}
	if redirectURI == "" {
		redirectURI = session.RedirectURI
	}
	if session.RedirectURI != "" && redirectURI != session.RedirectURI {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("redirect_uri does not match OAuth session")
	}

	s.CfgMu.RLock()
	prov := s.Cfg.FindProvider(providerID)
	var entry config.ProviderEntry
	if prov != nil {
		entry = *prov
	}
	s.CfgMu.RUnlock()
	if prov == nil {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("provider not found: %s", providerID)
	}
	tokenResp, err := exchangeCodeForToken(entry, code, session.RedirectURI, session.CodeVerifier)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("[OAuth] AgoDesk token exchange failed", "provider", providerID, "error", err)
		}
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("%s", oauthUserMessage(err))
	}
	if err := storeOAuthToken(s.Vault, providerID, tokenResp, now); err != nil {
		return agodesk.ConfigProviderOAuthStatusPayload{}, fmt.Errorf("failed to store token")
	}
	applyOAuthTokenToRuntime(s)
	status := agodeskProviderOAuthStatus(s, providerID, redirectURI)
	status.SessionID = strings.TrimSpace(payload.SessionID)
	status.Status = "ok"
	status.Mode = oauthFlowModeAgodeskLoopback
	status.Message = "Authorization successful."
	return status, nil
}

func validateAgodeskLoopbackRedirectURI(redirectURI string) error {
	if strings.TrimSpace(redirectURI) == "" {
		return fmt.Errorf("redirect_uri is required")
	}
	parsed, err := url.Parse(redirectURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("redirect_uri is invalid")
	}
	if parsed.Scheme != "http" {
		return fmt.Errorf("redirect_uri must use http loopback")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("redirect_uri must point to localhost or a loopback address")
	}
	return nil
}

func agodeskProviderOAuthStatus(s *Server, providerID, redirectURI string) agodesk.ConfigProviderOAuthStatusPayload {
	providerID = strings.TrimSpace(providerID)
	status := agodesk.ConfigProviderOAuthStatusPayload{
		ProviderID:    providerID,
		Status:        "ok",
		MissingFields: []string{},
		RedirectURI:   strings.TrimSpace(redirectURI),
	}
	if s == nil || s.Cfg == nil {
		status.MissingFields = []string{"provider"}
		return status
	}
	s.CfgMu.RLock()
	prov := s.Cfg.FindProvider(providerID)
	var entry config.ProviderEntry
	if prov != nil {
		entry = *prov
	}
	s.CfgMu.RUnlock()
	missing := oauthProviderMissingFields(prov)
	status.MissingFields = missing
	status.Configured = len(missing) == 0 && normalizeProviderAuthType(entry.AuthType) == "oauth2"
	if s.Vault == nil {
		return status
	}
	raw, err := s.Vault.ReadSecret("oauth_" + providerID)
	if err != nil || strings.TrimSpace(raw) == "" {
		return status
	}
	var token config.OAuthToken
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return status
	}
	status.Authorized = strings.TrimSpace(token.AccessToken) != ""
	status.HasRefreshToken = strings.TrimSpace(token.RefreshToken) != ""
	if strings.TrimSpace(token.Expiry) != "" {
		status.Expiry = token.Expiry
		if expiry, err := time.Parse(time.RFC3339, token.Expiry); err == nil {
			status.Expired = time.Now().UTC().After(expiry)
		}
	}
	return status
}

func revokeAgodeskProviderOAuth(s *Server, providerID string) error {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return fmt.Errorf("provider_id is required")
	}
	if s == nil {
		return fmt.Errorf("server is not available")
	}
	if s.Vault != nil {
		_ = s.Vault.DeleteSecret("oauth_" + providerID)
	}
	s.CfgMu.Lock()
	if p := s.Cfg.FindProvider(providerID); p != nil && normalizeProviderAuthType(p.AuthType) == "oauth2" {
		p.APIKey = ""
	}
	s.CfgMu.Unlock()
	agent.ResetGlobalHelperLLMManager()
	return nil
}

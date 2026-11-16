package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/httporigin"
	"aurago/internal/security"
)

type openWebUIProviders struct {
	Enabled bool                       `json:"ENABLE_OPENAI_API"`
	URLs    []string                   `json:"OPENAI_API_BASE_URLS"`
	Keys    []string                   `json:"OPENAI_API_KEYS"`
	Configs map[string]json.RawMessage `json:"OPENAI_API_CONFIGS"`
}

// Only the explicit connection save may append a provider. Tests are read-only.
func (s *Server) connectOpenWebUI(ctx context.Context, settings desktopstore.IntegrationSettings, save bool) error {
	if settings.Credential == "" {
		return errors.New("Open WebUI: enter an administrator access token")
	}
	info, err := (storeLLMRelayProvider{s}).RelayInfo(ctx, settings.DockerNetwork)
	if err != nil {
		return err
	}
	token, err := s.Vault.ReadSecret(desktopstore.RelayTokenKey(desktopstore.OpenWebUIAppID))
	if err != nil || token == "" {
		return errors.New("Store update required: Open WebUI relay token is missing")
	}
	security.RegisterSensitive(token)
	security.RegisterSensitive(settings.Credential)
	client := httporigin.NewClient(15 * time.Second)
	request := func(method, path string, body io.Reader) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, method, settings.URL+path, body)
		if err != nil {
			return nil, errors.New("Open WebUI address invalid")
		}
		req.Header.Set("Authorization", "Bearer "+settings.Credential)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.New("Open WebUI request interrupted; check the connection before saving again")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, errors.New("Open WebUI provider configuration unavailable; check administrator access and app version")
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		if err != nil || len(data) > 1<<20 {
			return nil, errors.New("Open WebUI response exceeds its limit or is incomplete")
		}
		return data, nil
	}
	data, err := request("GET", "/openai/config", nil)
	if err != nil {
		return err
	}
	var providers openWebUIProviders
	if json.Unmarshal(data, &providers) != nil || len(providers.URLs) > 200 || len(providers.Keys) != len(providers.URLs) {
		return errors.New("Store update required: unsupported Open WebUI provider configuration")
	}
	for _, key := range providers.Keys {
		security.RegisterSensitive(key)
	}
	wantURL := info.BaseURL + desktopstore.RelayPath(desktopstore.OpenWebUIAppID)
	index := -1
	for i, u := range providers.URLs {
		if strings.TrimRight(u, "/") == wantURL {
			index = i
			break
		}
	}
	if index >= 0 && providers.Enabled && subtle.ConstantTimeCompare([]byte(providers.Keys[index]), []byte(token)) == 1 {
		return nil
	}
	if !save {
		return errors.New("Open WebUI relay is not configured; save this connection to add it without removing existing providers")
	}
	if index < 0 {
		index = len(providers.URLs)
		providers.URLs = append(providers.URLs, wantURL)
		providers.Keys = append(providers.Keys, token)
		if providers.Configs == nil {
			providers.Configs = map[string]json.RawMessage{}
		}
		providers.Configs[strconv.Itoa(index)] = json.RawMessage(`{"enable":true,"model_ids":["aurago"]}`)
	} else {
		providers.Keys[index] = token
	}
	providers.Enabled = true
	data, err = json.Marshal(providers)
	if err != nil {
		return err
	}
	_, err = request("POST", "/openai/config/update", bytes.NewReader(data))
	return err
}

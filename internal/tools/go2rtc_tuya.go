package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

type Go2RTCTuyaCandidate struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
}
type Go2RTCTuyaResult struct {
	Status        string                `json:"status"`
	ErrorCode     string                `json:"error_code,omitempty"`
	DurationMS    int64                 `json:"duration_ms"`
	Width         int                   `json:"width"`
	Height        int                   `json:"height"`
	VideoCodecs   []string              `json:"video_codecs"`
	AudioCodecs   []string              `json:"audio_codecs"`
	TalkSupported bool                  `json:"talk_supported"`
	Suggestion    string                `json:"suggestion,omitempty"`
	Incomplete    bool                  `json:"incomplete"`
	Candidates    []Go2RTCTuyaCandidate `json:"-"`
}

func (m *Go2RTCManager) tuyaRequest(ctx context.Context, request map[string]string) (out Go2RTCTuyaResult) {
	start := time.Now()
	defer func() { out.DurationMS = time.Since(start).Milliseconds() }()
	out.Status = "error"
	status, err := m.Test(ctx)
	if err != nil {
		out.ErrorCode = "api_unavailable"
		return
	}
	if !status.TuyaSetup {
		out.ErrorCode = "tuya_sidecar_unsupported"
		return
	}
	data, _ := json.Marshal(request)
	var result struct {
		Go2RTCTuyaResult
		Candidates []Go2RTCTuyaCandidate `json:"candidates"`
	}
	if err := m.requestJSON(ctx, "POST", "/api/aurago/tuya", bytes.NewReader(data), &result, 1<<20); err != nil {
		out.ErrorCode = "tuya_connection_failed"
		if ctx.Err() != nil {
			out.ErrorCode = "tuya_timeout"
		}
		return
	}
	out = result.Go2RTCTuyaResult
	if out.Status != "ok" {
		out.Status = "error"
		switch out.ErrorCode {
		case "tuya_timeout", "tuya_region_error", "tuya_permission_missing", "tuya_auth_failed", "tuya_mqtt_failed", "tuya_connection_failed":
		default:
			out.ErrorCode = "tuya_connection_failed"
		}
		return
	}
	if len(result.Candidates) > 128 {
		out.Status = "error"
		out.ErrorCode = "tuya_response_invalid"
		return
	}
	for _, c := range result.Candidates {
		if len(c.DeviceID) > 128 || len(c.Name) > 192 {
			out.Status = "error"
			out.ErrorCode = "tuya_response_invalid"
			return
		}
		c.Name = security.Scrub(strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return -1
			}
			return r
		}, c.Name))
		out.Candidates = append(out.Candidates, c)
	}
	for _, codes := range []*[]string{&out.VideoCodecs, &out.AudioCodecs} {
		clean := []string{}
		for _, codec := range *codes {
			if s := sanitizeGo2RTCToken(codec, 32); s != "" {
				clean = append(clean, s)
			}
		}
		*codes = clean
	}
	return
}

func (m *Go2RTCManager) TuyaDiscover(ctx context.Context, a config.Go2RTCTuyaAccount, c config.Go2RTCTuyaCredentials) Go2RTCTuyaResult {
	release, ok := m.claimTuya(a.ID)
	if !ok {
		return Go2RTCTuyaResult{Status: "error", ErrorCode: "tuya_busy"}
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if a.Method != "smart" || config.ValidateGo2RTCTuyaCredentials(a.Method, c) != nil {
		return Go2RTCTuyaResult{Status: "error", ErrorCode: "tuya_invalid_configuration"}
	}
	security.RegisterSensitive(c.Email)
	security.RegisterSensitive(c.Password)
	return m.tuyaRequest(ctx, map[string]string{"action": "discover", "region": config.Go2RTCTuyaHost(a.Method, a.Region), "email": c.Email, "password": c.Password})
}

func (m *Go2RTCManager) TuyaTest(ctx context.Context, source string, accounts ...string) Go2RTCTuyaResult {
	account := ""
	if len(accounts) > 0 {
		account = accounts[0]
	} else {
		for _, camera := range m.Config().Streams {
			if camera.Source == source && camera.Tuya != nil {
				account = camera.Tuya.AccountID
				break
			}
		}
	}
	if account == "" {
		account = fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	}
	release, ok := m.claimTuya(account)
	if !ok {
		return Go2RTCTuyaResult{Status: "error", ErrorCode: "tuya_busy"}
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if config.ValidateGo2RTCTuyaSource(source) != nil {
		return Go2RTCTuyaResult{Status: "error", ErrorCode: "tuya_invalid_configuration"}
	}
	security.RegisterSensitive(source)
	result := m.tuyaRequest(ctx, map[string]string{"action": "test", "source": source})
	if result.Status != "ok" && strings.Contains(source, "resolution=hd") {
		result.Suggestion = "sd"
	}
	return result
}

func (m *Go2RTCManager) claimTuya(account string) (func(), bool) {
	m.tuyaMu.Lock()
	defer m.tuyaMu.Unlock()
	if m.tuyaBusy == nil {
		m.tuyaBusy = map[string]bool{}
	}
	if m.tuyaBusy[account] {
		return nil, false
	}
	m.tuyaBusy[account] = true
	return func() { m.tuyaMu.Lock(); delete(m.tuyaBusy, account); m.tuyaMu.Unlock() }, true
}

func go2RTCTuyaTalkSupported(raw interface{}) bool {
	object, _ := raw.(map[string]interface{})
	producers, _ := object["producers"].([]interface{})
	for _, producer := range producers {
		if p, ok := producer.(map[string]interface{}); ok {
			if supported, _ := p["tuya_talk_supported"].(bool); supported {
				return true
			}
		}
	}
	return false
}

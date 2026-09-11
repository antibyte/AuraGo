package acestep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/uid"
)

type runtimeStatus struct {
	ImagePin        string  `json:"image_pin"`
	Ready           bool    `json:"ready"`
	State           string  `json:"state"`
	ErrorCode       string  `json:"error_code"`
	Profile         Profile `json:"profile"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      int64   `json:"total_bytes"`
}

func (m *Manager) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.key)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("acestep_http_%d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(output)
}

func (m *Manager) generate(ctx context.Context, p Params, profile Profile) (Audio, error) {
	lyrics := p.Lyrics
	if p.Instrumental {
		lyrics = "[Instrumental]"
	}
	body := map[string]any{
		"prompt": p.Prompt, "lyrics": lyrics, "audio_duration": p.DurationSeconds, "audio_format": "mp3",
		"batch_size": 1, "inference_steps": 8, "task_type": "text2music", "model": profile.Model,
		"thinking": profile.LMModel != "", "lm_backend": profile.LMBackend,
		"use_cot_caption": false, "use_cot_language": profile.LMModel != "",
		"use_cot_lyrics":  !p.Instrumental && strings.TrimSpace(lyrics) == "" && profile.LMModel != "",
		"use_random_seed": p.Seed == nil, "allow_lm_batch": false,
	}
	if p.BPM != 0 {
		body["bpm"] = p.BPM
	}
	if p.Seed != nil {
		body["seed"] = *p.Seed
	}
	if p.VocalLanguage != "" {
		body["vocal_language"] = p.VocalLanguage
		body["use_cot_language"] = false
	}
	if !p.Instrumental && strings.TrimSpace(lyrics) == "" {
		body["sample_query"] = p.Prompt
	}
	var submission struct {
		Code int `json:"code"`
		Data struct {
			ID string `json:"task_id"`
		} `json:"data"`
	}
	if err := m.request(ctx, "POST", "/release_task", body, &submission); err != nil {
		return Audio{}, err
	}
	if submission.Code != 200 || submission.Data.ID == "" {
		return Audio{}, fmt.Errorf("acestep_invalid_submission")
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return Audio{}, ctx.Err()
		case <-ticker.C:
		}
		var poll struct {
			Code int `json:"code"`
			Data []struct {
				ID     string `json:"task_id"`
				Status int    `json:"status"`
				Result string `json:"result"`
			} `json:"data"`
		}
		if err := m.request(ctx, "POST", "/query_result", map[string]any{"task_id_list": []string{submission.Data.ID}}, &poll); err != nil {
			return Audio{}, err
		}
		if poll.Code != 200 || len(poll.Data) != 1 || poll.Data[0].ID != submission.Data.ID {
			return Audio{}, fmt.Errorf("acestep_invalid_result")
		}
		switch poll.Data[0].Status {
		case 0:
			continue
		case 2:
			return Audio{}, fmt.Errorf("acestep_generation_failed")
		case 1:
			var results []struct {
				File   string `json:"file"`
				Status int    `json:"status"`
				Metas  struct {
					Duration float64 `json:"duration"`
				} `json:"metas"`
			}
			if err := json.Unmarshal([]byte(poll.Data[0].Result), &results); err != nil || len(results) != 1 || results[0].Status != 1 {
				return Audio{}, fmt.Errorf("acestep_invalid_audio_result")
			}
			duration := results[0].Metas.Duration
			if duration < 1 || duration > 601 || duration > float64(profile.MaxDuration)+1 || math.Abs(duration-p.DurationSeconds) > 2 {
				return Audio{}, fmt.Errorf("acestep_invalid_audio_duration")
			}
			return m.downloadAudio(ctx, results[0].File, duration, profile.Model)
		default:
			return Audio{}, fmt.Errorf("acestep_invalid_job_status")
		}
	}
}

func audioDownloadPath(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Path != "/v1/audio" || u.Fragment != "" {
		return "", fmt.Errorf("acestep_invalid_audio_url")
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) != 1 || len(values["path"]) != 1 {
		return "", fmt.Errorf("acestep_invalid_audio_url")
	}
	p := values.Get("path")
	if !strings.HasPrefix(p, "/app/.cache/acestep/tmp/api_audio/") || !strings.HasSuffix(strings.ToLower(p), ".mp3") || strings.Contains(p, "..") || strings.ContainsAny(p, "\x00\r\n\\") {
		return "", fmt.Errorf("acestep_invalid_audio_path")
	}
	return "/v1/audio?" + url.Values{"path": []string{p}}.Encode(), nil
}

func (m *Manager) downloadAudio(ctx context.Context, raw string, duration float64, model string) (Audio, error) {
	path, err := audioDownloadPath(raw)
	if err != nil {
		return Audio{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", m.baseURL+path, nil)
	if err != nil {
		return Audio{}, err
	}
	req.Header.Set("Authorization", "Bearer "+m.key)
	resp, err := m.client.Do(req)
	if err != nil {
		return Audio{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.ContentLength > maxAudioBytes {
		return Audio{}, fmt.Errorf("acestep_audio_download_failed")
	}
	dir, err := m.outputDir()
	if err != nil {
		return Audio{}, err
	}
	f, err := os.CreateTemp(dir, ".acestep-*.part")
	if err != nil {
		return Audio{}, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxAudioBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return Audio{}, copyErr
	}
	if closeErr != nil {
		return Audio{}, closeErr
	}
	if n < 4 || n > maxAudioBytes {
		return Audio{}, fmt.Errorf("acestep_audio_size_invalid")
	}
	input, err := os.Open(tmp)
	if err != nil {
		return Audio{}, err
	}
	header := make([]byte, 3)
	_, err = io.ReadFull(input, header)
	input.Close()
	if err != nil || !(string(header) == "ID3" || header[0] == 0xff && header[1]&0xe0 == 0xe0) {
		return Audio{}, fmt.Errorf("acestep_invalid_mp3")
	}
	if err := ctx.Err(); err != nil {
		return Audio{}, err
	}
	name := "music_" + uid.New() + ".mp3"
	target := filepath.Join(dir, name)
	if err = os.Rename(tmp, target); err != nil {
		return Audio{}, err
	}
	return Audio{Filename: name, Path: target, DurationMs: int64(duration * 1000), Model: model, Size: n}, nil
}

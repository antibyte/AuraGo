package speechlab

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func testPCM16WAV() []byte {
	data := make([]byte, 44+320)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], 16000)
	binary.LittleEndian.PutUint32(data[28:32], 32000)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(data)-44))
	return data
}

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(config.SpeechLabConfig{
		Enabled: true, BaseURL: server.URL, Language: "de", Voice: "M1", TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestReadyParsesServiceUnavailableBody(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Ready{ASRID: "whisper", TTSID: "tts", ASROK: true, Message: "tts unreachable"})
	}))
	ready, err := client.Ready(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ready.Ready || !ready.ASROK || ready.TTSOK {
		t.Fatalf("unexpected readiness: %+v", ready)
	}
	if _, err := client.Require(context.Background(), true, false); err != nil {
		t.Fatalf("ASR-only readiness should pass: %v", err)
	}
	if _, err := client.Require(context.Background(), false, true); ErrorCode(err) != "speech_lab_not_ready" {
		t.Fatalf("unexpected TTS readiness error: %v", err)
	}
}

func TestTranscribeRequiresCanonicalWAVAndChecksBackendID(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != asrPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseMultipartForm(MaxASRBytes)
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if values := r.MultipartForm.File["file"]; len(values) != 1 || values[0].Header.Get("Content-Type") != "audio/wav" {
			t.Fatalf("unexpected multipart WAV content type: %+v", values)
		}
		body, _ := io.ReadAll(file)
		if err := ValidatePCM16WAV(body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(Transcript{Text: "Hallo", ASRID: "asr-a"})
	}))
	if _, err := client.Transcribe(context.Background(), []byte("not wav"), "de", ""); err == nil {
		t.Fatal("invalid WAV accepted")
	}
	result, err := client.Transcribe(context.Background(), testPCM16WAV(), "de", "asr-a")
	if err != nil || result.Text != "Hallo" {
		t.Fatalf("unexpected transcript: %+v err=%v", result, err)
	}
	if _, err := client.Transcribe(context.Background(), testPCM16WAV(), "de", "asr-b"); err == nil {
		t.Fatal("backend drift was not rejected")
	}
}

func TestSynthesizeRequiresWAVAndChecksBackendID(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("X-S2S-TTS-ID", "tts-a")
		_, _ = w.Write(testPCM16WAV())
	}))
	data, id, err := client.Synthesize(context.Background(), "Hallo", "de", "M1", "tts-a")
	if err != nil || id != "tts-a" || len(data) == 0 {
		t.Fatalf("unexpected synthesis id=%q bytes=%d err=%v", id, len(data), err)
	}
	if _, _, err := client.Synthesize(context.Background(), "Hallo", "de", "M1", "tts-b"); err == nil {
		t.Fatal("backend drift was not rejected")
	}
}

func TestValidateWAVRejectsHeaderOnlyAndTruncatedChunks(t *testing.T) {
	valid := testPCM16WAV()
	if err := ValidateWAV(valid); err != nil {
		t.Fatalf("valid WAV rejected: %v", err)
	}
	headerOnly := append([]byte(nil), valid[:44]...)
	binary.LittleEndian.PutUint32(headerOnly[4:8], uint32(len(headerOnly)-8))
	binary.LittleEndian.PutUint32(headerOnly[40:44], 0)
	if err := ValidateWAV(headerOnly); err == nil {
		t.Fatal("header-only WAV accepted")
	}
	truncated := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(truncated[40:44], uint32(len(truncated)))
	if err := ValidateWAV(truncated); err == nil {
		t.Fatal("truncated WAV data chunk accepted")
	}
}

func TestActivateStackNeverSendsLLMIDAndRechecksReadiness(t *testing.T) {
	var stackBody string
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case catalogPath:
			_, _ = io.WriteString(w, `{"schema_version":1,"backends":[{"id":"asr-a","stage":"asr","available":true},{"id":"tts-a","stage":"tts","available":true,"voices":["M1"]}]}`)
		case stackPath:
			body, _ := io.ReadAll(r.Body)
			stackBody = string(body)
			_, _ = io.WriteString(w, `{"ok":true,"message":"ok"}`)
		case readyPath:
			_ = json.NewEncoder(w).Encode(Ready{Ready: true, ASRID: "asr-a", TTSID: "tts-a", ASROK: true, TTSOK: true})
		default:
			http.NotFound(w, r)
		}
	}))
	_, ready, err := client.ActivateStack(context.Background(), StackRequest{ASRID: "asr-a", TTSID: "tts-a", Voice: "M1"})
	if err != nil || !ready.Ready {
		t.Fatalf("activation failed: ready=%+v err=%v", ready, err)
	}
	if strings.Contains(stackBody, "llm") {
		t.Fatalf("AuraGo forwarded an LLM field: %s", stackBody)
	}
}

func TestReadinessHonorsCancellation(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := client.Ready(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestRequireMapsUnreachableServiceToNotReadyWithoutLosingCancellation(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := client.Require(ctx, true, false)
	if ErrorCode(err) != "speech_lab_not_ready" {
		t.Fatalf("readiness error code = %q, want speech_lab_not_ready", ErrorCode(err))
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readiness error must preserve cancellation: %v", err)
	}
}

func TestSpeechLabLimitsAndPrivateDestinationPolicy(t *testing.T) {
	if _, err := readBounded(strings.NewReader("12345"), 4); err == nil {
		t.Fatal("bounded reader accepted an oversized response")
	}
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("oversized ASR input must fail before an HTTP request")
	}))
	if _, err := client.Transcribe(context.Background(), make([]byte, MaxASRBytes+1), "de", ""); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized ASR error = %v", err)
	}

	for _, raw := range []string{"127.0.0.1", "10.0.0.10", "172.16.0.1", "192.168.1.5", "::1", "fd00::1"} {
		if !allowedPrivateIP(net.ParseIP(raw)) {
			t.Fatalf("private address rejected: %s", raw)
		}
	}
	for _, raw := range []string{"0.0.0.0", "8.8.8.8", "1.1.1.1", "2001:4860:4860::8888"} {
		if allowedPrivateIP(net.ParseIP(raw)) {
			t.Fatalf("public or unspecified address accepted: %s", raw)
		}
	}
}

func TestSpeechLabResponseLimitsAreAppliedBySurface(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case capabilityPath:
			_, _ = w.Write(make([]byte, maxJSONBytes+1))
		case ttsPath:
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("X-S2S-TTS-ID", "tts-a")
			_, _ = w.Write(make([]byte, maxTTSBytes+1))
		default:
			http.NotFound(w, r)
		}
	}))
	if _, err := client.Capability(context.Background()); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized JSON response error = %v", err)
	}
	if _, _, err := client.Synthesize(context.Background(), "Hallo", "de", "M1", "tts-a"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized TTS response error = %v", err)
	}
}

func TestSuggestionsForwardsOnlySupportedBoundedQueryFields(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		for _, key := range []string{"language", "stable_only", "max_vram_gb", "limit"} {
			if query.Get(key) == "" {
				t.Errorf("missing supported query field %s: %s", key, r.URL.RawQuery)
			}
		}
		if query.Get("llm_id") != "" || query.Get("hf_token") != "" {
			t.Fatalf("unsafe query field forwarded: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	_, err := client.Suggestions(context.Background(), mapValues(
		"language", "de", "stable_only", "true", "max_vram_gb", "8", "limit", "4", "llm_id", "forbidden", "hf_token", "secret",
	))
	if err != nil {
		t.Fatal(err)
	}
}

func TestStackActivationIsRejectedWhileInferenceIsActive(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != asrPath {
			t.Fatalf("unexpected request while inference is active: %s", r.URL.Path)
		}
		once.Do(func() { close(started) })
		<-release
		_ = json.NewEncoder(w).Encode(Transcript{Text: "done", ASRID: "asr-a"})
	}))
	done := make(chan error, 1)
	go func() {
		_, err := client.Transcribe(context.Background(), testPCM16WAV(), "de", "asr-a")
		done <- err
	}()
	<-started
	if _, _, err := client.ActivateStack(context.Background(), StackRequest{ASRID: "asr-a", TTSID: "tts-a"}); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("stack activation error = %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func mapValues(pairs ...string) url.Values {
	values := make(url.Values, len(pairs)/2)
	for index := 0; index+1 < len(pairs); index += 2 {
		values[pairs[index]] = []string{pairs[index+1]}
	}
	return values
}

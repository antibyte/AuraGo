package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/sipphone"
	"aurago/internal/speechlab"
	"aurago/internal/speechlab/deployer"
)

func speechLabServerForTest(t *testing.T, handler http.Handler) (*Server, *speechlab.Client) {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	cfg := &config.Config{}
	cfg.SpeechLab = config.SpeechLabConfig{
		Enabled: true, BaseURL: upstream.URL, Language: "de", Voice: "M1", TimeoutSeconds: 2,
		SIPEnabled: true, ChatInputEnabled: true, ChatOutputEnabled: true,
	}
	client, err := speechlab.NewClient(cfg.SpeechLab)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{Cfg: cfg, SpeechLab: client, Logger: testServerLogger()}, client
}

func speechLabWAVForTest() []byte {
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

func speechLabWAVSecondsForTest(seconds int) []byte {
	data := make([]byte, 44+seconds*16000*2)
	copy(data, speechLabWAVForTest()[:44])
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(data)-44))
	return data
}

func TestSpeechLabRoutesProtectManagementButNotStatus(t *testing.T) {
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, ASRID: "asr-a", TTSID: "tts-a", ASROK: true, TTSOK: true, Voice: "Serena"})
			return
		}
		http.NotFound(w, r)
	}))
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "speech-lab-admin-test"
	s.Cfg.SpeechLab.Deployment.GPUBackend = config.SpeechLabGPUBackendVulkan
	mux := http.NewServeMux()
	registerSpeechLabRoutes(mux, s)

	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/speech-lab/status", nil))
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status route = %d, want 200", statusRec.Code)
	}
	if strings.Contains(statusRec.Body.String(), s.Cfg.SpeechLab.BaseURL) {
		t.Fatal("sanitized status exposed the Speech Lab base URL")
	}
	if !strings.Contains(statusRec.Body.String(), `"voice":"Serena"`) {
		t.Fatalf("status did not expose the active runtime voice: %s", statusRec.Body.String())
	}
	if !strings.Contains(statusRec.Body.String(), `"requested_gpu_backend":"vulkan"`) {
		t.Fatalf("status did not expose the requested hardware profile: %s", statusRec.Body.String())
	}
	if !strings.Contains(statusRec.Body.String(), `"warnings":["advanced_ui_url_missing"]`) || strings.Contains(statusRec.Body.String(), `"advanced_ui_url"`) {
		t.Fatalf("status did not report the missing explicit browser lab URL: %s", statusRec.Body.String())
	}

	adminRec := httptest.NewRecorder()
	mux.ServeHTTP(adminRec, httptest.NewRequest(http.MethodGet, "/api/speech-lab/catalog", nil))
	if adminRec.Code != http.StatusUnauthorized {
		t.Fatalf("catalog without admin session = %d, want 401", adminRec.Code)
	}
	for _, path := range []string{
		"/api/speech-lab/capability", "/api/speech-lab/catalog", "/api/speech-lab/suggestions", "/api/speech-lab/stack", "/api/speech-lab/deployment/install",
	} {
		if !isAdminProtectedPath(path) {
			t.Fatalf("%s should be classified as admin protected", path)
		}
	}
	if isAdminProtectedPath("/api/speech-lab/status") {
		t.Fatal("sanitized Speech Lab status must not require administrator scope")
	}
}

func TestSpeechLabDeploymentRequiresExplicitConfirmation(t *testing.T) {
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	s.SpeechLabDeployer = deployer.NewManager(s.Cfg.SpeechLab, false, true, false, "", s.Logger)
	rec := httptest.NewRecorder()
	handleSpeechLabDeployment(s, "install").ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/speech-lab/deployment/install", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("deployment without confirmation = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"confirmation_required"`) {
		t.Fatalf("confirmation error missing: %s", rec.Body.String())
	}
}

func TestSpeechLabStatusReportsRequestedHardwareForManagedAndExternalModes(t *testing.T) {
	for _, mode := range []string{"managed", "external"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			}))
			s.Cfg.SpeechLab.Deployment.Mode = mode
			s.Cfg.SpeechLab.Deployment.GPUBackend = config.SpeechLabGPUBackendCPU
			rec := httptest.NewRecorder()
			handleSpeechLabStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/speech-lab/status", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, `"requested_gpu_backend":"cpu"`) {
				t.Fatalf("requested profile missing in %s mode: %s", mode, body)
			}
			if !strings.Contains(body, `"active_gpu_backend":""`) {
				t.Fatalf("active profile field missing in %s mode: %s", mode, body)
			}
			wantManaged := mode == "managed"
			if strings.Contains(body, `"managed":true`) != wantManaged {
				t.Fatalf("managed flag mismatch for %s: %s", mode, body)
			}
		})
	}
}

func TestSpeechLabDeploymentErrorMessageForMissingVulkan(t *testing.T) {
	message := speechLabDeploymentErrorMessage("speech_lab_gpu_unavailable")
	if !strings.Contains(message, "/dev/dri") || !strings.Contains(message, "Auto") {
		t.Fatalf("missing actionable Vulkan diagnosis: %q", message)
	}
}

func TestSpeechLabBrowserURLRequiresExplicitExpertOverride(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://192.168.1.42:8088/api/speech-lab/status", nil)
	if got := speechLabBrowserURLForRequest("", req); got != "" {
		t.Fatalf("request host produced browser URL = %q", got)
	}
	if got := speechLabBrowserURLForRequest("https://speech.example.test:9443/", req); got != "https://speech.example.test:9443" {
		t.Fatalf("expert override = %q", got)
	}
	req.Host = "invalid/host"
	if got := speechLabBrowserURLForRequest("", req); got != "" {
		t.Fatalf("invalid request host produced browser URL %q", got)
	}
}

func TestSpeechLabStackUsesFixedContractAndNeverSendsLLM(t *testing.T) {
	var upstreamStackBody string
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/catalog":
			_, _ = io.WriteString(w, `{"schema_version":1,"backends":[{"id":"asr-a","stage":"asr","available":true},{"id":"tts-a","stage":"tts","available":true,"voices":["M1"]}]}`)
		case "/api/v1/stack":
			body, _ := io.ReadAll(r.Body)
			upstreamStackBody = string(body)
			_, _ = io.WriteString(w, `{"ok":true,"message":"activated"}`)
		case "/ready":
			_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, ASRID: "asr-a", TTSID: "tts-a", ASROK: true, TTSOK: true, Voice: "M1"})
		default:
			http.NotFound(w, r)
		}
	}))

	badRec := httptest.NewRecorder()
	handleSpeechLabStack(s).ServeHTTP(badRec, httptest.NewRequest(http.MethodPut, "/api/speech-lab/stack", strings.NewReader(
		`{"asr_id":"asr-a","tts_id":"tts-a","voice":"M1","llm_id":"forbidden"}`,
	)))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("request containing llm_id = %d, want 400", badRec.Code)
	}

	okRec := httptest.NewRecorder()
	handleSpeechLabStack(s).ServeHTTP(okRec, httptest.NewRequest(http.MethodPut, "/api/speech-lab/stack", strings.NewReader(
		`{"asr_id":"asr-a","tts_id":"tts-a","voice":"M1"}`,
	)))
	if okRec.Code != http.StatusOK {
		t.Fatalf("valid stack request = %d, want 200: %s", okRec.Code, okRec.Body.String())
	}
	if strings.Contains(upstreamStackBody, "llm") {
		t.Fatalf("AuraGo sent an LLM field upstream: %s", upstreamStackBody)
	}
}

func TestSpeechLabStackRejectsAtomicSIPReservation(t *testing.T) {
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("busy stack request reached Speech Lab: %s", r.URL.Path)
	}))
	manager, err := sipphone.NewManager(config.SIPConfig{}, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	s.SIPPhone = manager
	release, err := manager.ReserveSpeechLabStackChange()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	handleSpeechLabStack(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/speech-lab/stack", strings.NewReader(
		`{"asr_id":"asr-a","tts_id":"tts-a"}`,
	)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("stack during SIP reservation = %d, want 409", rec.Code)
	}
	release()
	if next, err := manager.ReserveSpeechLabStackChange(); err != nil {
		t.Fatalf("reservation remained held after release: %v", err)
	} else {
		next()
	}
}

func TestSpeechLabChatUploadTranscribesPCMWithoutLegacyConversion(t *testing.T) {
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, ASRID: "asr-a", TTSID: "tts-a", ASROK: true, TTSOK: true})
		case "/v1/audio/transcriptions":
			if err := r.ParseMultipartForm(speechlab.MaxASRBytes); err != nil {
				t.Fatal(err)
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			wav, _ := io.ReadAll(file)
			if err := speechlab.ValidatePCM16WAV(wav); err != nil {
				t.Fatalf("upstream received invalid WAV: %v", err)
			}
			_ = json.NewEncoder(w).Encode(speechlab.Transcript{Text: "lokaler Text", ASRID: "asr-a"})
		default:
			http.NotFound(w, r)
		}
	}))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="audio"; filename="speech.wav"`)
	header.Set("Content-Type", "audio/wav")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(speechLabWAVForTest())
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload-voice", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handleSpeechLabVoiceUpload(s, rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lokaler Text") {
		t.Fatalf("PCM upload response = %d %s", rec.Code, rec.Body.String())
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/upload-voice", strings.NewReader("webm"))
	badReq.Header.Set("Content-Type", "audio/webm")
	badRec := httptest.NewRecorder()
	handleSpeechLabVoiceUpload(s, badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("compressed upload = %d, want 400", badRec.Code)
	}
}

func TestSpeechLabChatUploadRejectsMoreThan120Seconds(t *testing.T) {
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("oversized-duration upload reached Speech Lab: %s", r.URL.Path)
	}))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="audio"; filename="speech.wav"`)
	header.Set("Content-Type", "audio/wav")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(speechLabWAVSecondsForTest(121))
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/upload-voice", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handleSpeechLabVoiceUpload(s, rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("121-second upload = %d, want 413: %s", rec.Code, rec.Body.String())
	}
}

func TestSpeechLabChatUploadMapsEmptyTranscriptToNoSpeech(t *testing.T) {
	s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, ASRID: "asr-a", ASROK: true})
		case "/v1/audio/transcriptions":
			_ = json.NewEncoder(w).Encode(speechlab.Transcript{Text: "", ASRID: "asr-a"})
		default:
			http.NotFound(w, r)
		}
	}))
	var logs bytes.Buffer
	s.Logger = slog.New(slog.NewTextHandler(&logs, nil))

	wav := speechLabWAVForTest()
	binary.LittleEndian.PutUint16(wav[44:46], uint16(16384))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="audio"; filename="speech.wav"`)
	header.Set("Content-Type", "audio/wav")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(wav)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload-voice", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handleSpeechLabVoiceUpload(s, rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty transcript response = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error":"speech_lab_no_speech"`) ||
		!strings.Contains(body, "No speech was detected") {
		t.Fatalf("empty transcript response is not actionable: %s", body)
	}
	for _, field := range []string{
		"Speech Lab chat transcription found no speech",
		"error_code=speech_lab_no_speech",
		"audio_duration_ms=10",
		"audio_sample_count=160",
		"audio_peak_level=0.5",
		"audio_rms_level=",
	} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("safe diagnostic log missing %q: %s", field, logs.String())
		}
	}
}

func TestSIPSpeechLabNotReadyMapsToServiceUnavailable(t *testing.T) {
	err := &speechlab.NotReadyError{NeedASR: true, Status: speechlab.Ready{Message: "ASR is not ready"}}
	rec := httptest.NewRecorder()
	writeSIPManagerError(rec, err)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("SIP Speech Lab readiness = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":"speech_lab_not_ready"`) {
		t.Fatalf("missing sanitized readiness code: %s", rec.Body.String())
	}
}

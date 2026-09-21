package server

import (
	"aurago/internal/config"
	"aurago/internal/rtlsdr"
	"aurago/ui"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

type rtlSDRFixture struct{}

func (rtlSDRFixture) Info(context.Context) (rtlsdr.Receiver, error) {
	bins := make([]float64, 1024)
	for i := range bins {
		bins[i] = -80 + float64((i*17)%21)
		if i > 495 && i < 530 {
			bins[i] += 35
		}
	}
	return rtlsdr.Receiver{Ready: true, Device: "RTL2838", Tuner: "R820T", Minimum: 24000000, Maximum: 1766000000, Gains: []float64{0, 10, 20, 30, 40, 49.6}, Power: -34, Label: "Radio Aurora", Text: "News and music from the airwaves", Spectrum: bins, Center: 100000000, Span: 2400000}, nil
}

func TestRTLSDRAllDesktopTranslations(t *testing.T) {
	read := func(lang string) map[string]string {
		b, err := ui.Content.ReadFile("lang/desktop/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(b, &values); err != nil {
			t.Fatal(err)
		}
		return values
	}
	english := read("en")
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := read(lang)
		count := 0
		for key := range english {
			if strings.HasPrefix(key, "rtlSdr.") {
				count++
				if strings.TrimSpace(values[key]) == "" {
					t.Errorf("%s missing %s", lang, key)
				}
			}
		}
		if count < 94 {
			t.Fatalf("incomplete RTL-SDR source catalog: %d", count)
		}
	}
}
func (rtlSDRFixture) Tune(context.Context, rtlsdr.Tuning) error { return nil }
func (rtlSDRFixture) Stop(context.Context) error                { return nil }
func (rtlSDRFixture) Stream(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("audio")), nil
}
func (rtlSDRFixture) Capture(ctx context.Context, n int, w io.Writer) (rtlsdr.CaptureInfo, error) {
	_, err := w.Write([]byte("fLaC fixture"))
	return rtlsdr.CaptureInfo{Seconds: float64(n)}, err
}
func (rtlSDRFixture) Scan(ctx context.Context, fn func([]rtlsdr.Station, string)) error {
	fn([]rtlsdr.Station{{ID: "5C:d210", Name: "DAB News", Tuning: rtlsdr.Tuning{Mode: "dab", Block: "5C", Frequency: 178352000, ServiceID: "d210", AGC: true}}}, "5C")
	return nil
}
func (rtlSDRFixture) WAV(context.Context, string, float64, int) ([]byte, error) {
	return []byte("wave"), nil
}
func rtlSDRServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.RTLSDR.Enabled = true
	cfg.RTLSDR.AllowAgent = true
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg}
	svc, err := rtlsdr.New(rtlsdr.Options{Directory: t.TempDir(), Backend: rtlSDRFixture{}, Policy: func() rtlsdr.Policy {
		return rtlsdr.Policy{Enabled: cfg.RTLSDR.Enabled, ReadOnly: cfg.RTLSDR.ReadOnly || cfg.VirtualDesktop.ReadOnly, AllowAgent: cfg.RTLSDR.AllowAgent}
	}})
	if err != nil {
		t.Fatal(err)
	}
	s.RTLSDR = svc
	t.Cleanup(func() { svc.Close() })
	return s
}
func TestRTLSDRAPIPermissionsAndAudioIDs(t *testing.T) {
	s := rtlSDRServer(t)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/api/desktop/rtl-sdr/"+path, strings.NewReader(body))
		s.handleRTLSDR(w, r)
		return w
	}
	if w := call("GET", "state", ""); w.Code != 200 {
		t.Fatalf("state %d %s", w.Code, w.Body.String())
	}
	payload, _ := json.Marshal(map[string]any{"client": "browser", "tuning": rtlsdr.DefaultTuning()})
	if w := call("POST", "tune", string(payload)); w.Code != 200 {
		t.Fatalf("tune %d %s", w.Code, w.Body.String())
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if w := call("POST", "tune", string(payload)); w.Code != 403 {
		t.Fatalf("readonly %d", w.Code)
	}
	if w := call("GET", "recordings/not-an-id/audio", ""); w.Code != 404 {
		t.Fatalf("audio ID %d", w.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	if w := call("POST", "tune", string(payload)+"{}"); w.Code != 400 {
		t.Fatalf("extra JSON %d", w.Code)
	}
	if w := call("GET", "stream?client=unknown", ""); w.Code != 404 {
		t.Fatalf("unowned stream %d", w.Code)
	}
}
func TestRTLSDRASRSilenceDoesNotCallProvider(t *testing.T) {
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	wav.Write([]byte{36, 125, 0, 0})
	wav.WriteString("WAVEfmt ")
	wav.Write([]byte{16, 0, 0, 0, 1, 0, 1, 0, 128, 62, 0, 0, 0, 125, 0, 0, 2, 0, 16, 0})
	wav.WriteString("data")
	wav.Write([]byte{0, 125, 0, 0})
	wav.Write(make([]byte, 32000))
	tr := rtlSDRASR{cfg: config.Config{}}
	text, err := tr.Transcribe(context.Background(), wav.Bytes())
	if err != nil || text != "" {
		t.Fatalf("silence: %q %v", text, err)
	}
}

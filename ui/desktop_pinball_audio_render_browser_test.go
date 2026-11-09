package ui

import (
	"encoding/base64"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// TestPinballAudioRender renders the AuraPinball synthesized audio (SFX, rolls, ambience, music, stingers, gameplay
// mixes) offline to WAV files. Opt-in:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_AUDIO_RENDER=1 [AURAGO_PINBALL_AUDIO_DIR=dir] [AURAGO_PINBALL_AUDIO_FILTER=regex] go test -run TestPinballAudioRender -count=1 -timeout 15m .
//
// Files go to AURAGO_PINBALL_AUDIO_DIR, else ../reports/pinball-audio (git-ignored). It fails on silence, NaN and clipping (peak > 1.0).
func TestPinballAudioRender(t *testing.T) {
	if os.Getenv("AURAGO_PINBALL_AUDIO_RENDER") != "1" {
		t.Skip("set AURAGO_PINBALL_AUDIO_RENDER=1 to render the AuraPinball audio")
	}
	requirePrecisionBrowserSmoke(t)
	dir := os.Getenv("AURAGO_PINBALL_AUDIO_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "pinball-audio")
	}
	dir, _ = filepath.Abs(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	filter := os.Getenv("AURAGO_PINBALL_AUDIO_FILTER")

	mux := http.NewServeMux()
	mux.Handle("/", jsTyped(http.FileServer(http.Dir("."))))
	mux.Handle("/node_modules/three/", http.StripPrefix("/node_modules/three/", jsTyped(http.FileServer(http.Dir(filepath.Join("..", "node_modules", "three"))))))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	browser := newSmokeBrowserWith(t, nil)
	page := browser.MustPage("about:blank").Timeout(14 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustNavigate(server.URL + "/testdata/pinball-audio-render.html")
	page.MustWait(`()=>window.pinballAudioRenderReady===true`)

	var names []struct {
		Name string `json:"name"`
	}
	if err := page.MustEval(`()=>window.pinballAudioNames()`).Unmarshal(&names); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(filter)
	rendered := 0
	for _, n := range names {
		if filter != "" && !re.MatchString(n.Name) {
			continue
		}
		res := page.MustEval(`async n=>{const r=await window.renderPinballAudio([n]);const x=r[0];return {name:x.name,seconds:x.seconds,peak:x.peak,rms:x.rms,nan:x.nan,wav:x.wavBase64}}`, n.Name)
		name := res.Get("name").Str()
		wav, err := base64.StdEncoding.DecodeString(res.Get("wav").Str())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".wav"), wav, 0o644); err != nil {
			t.Fatal(err)
		}
		rendered++
		peak, rms, nan, secs := res.Get("peak").Num(), res.Get("rms").Num(), res.Get("nan").Int(), res.Get("seconds").Num()
		flag := ""
		if peak > 0.98 {
			flag = "  HOT"
		}
		t.Logf("%-34s %6.2fs peak %.3f (%6.1f dBFS) rms %.4f (%6.1f dBFS)%s", name+".wav", secs, peak, db(peak), rms, db(rms), flag)
		if !(rms > 0) {
			t.Errorf("%s is silent", name)
		}
		if nan > 0 {
			t.Errorf("%s has %d non-finite samples", name, nan)
		}
		if peak > 1.0 {
			t.Errorf("%s clips: peak %.3f", name, peak)
		}
	}
	if rendered == 0 {
		t.Fatalf("no audio rendered (filter %q)", filter)
	}
	t.Logf("wrote %d files to %s", rendered, dir)
}

func db(v float64) float64 {
	if v <= 0 {
		return -999
	}
	return 20 * math.Log10(v)
}

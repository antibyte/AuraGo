package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChatAudioDownloadPreservesFormatBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	wav := make([]byte, 46)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 24000)
	binary.LittleEndian.PutUint32(wav[28:], 48000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], 2)
	mp3, err := Content.ReadFile("img/audio/galaga.mp3")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/js/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><script>window.t = key => key;</script><script src="/js/chat/audio-player.js"></script></body></html>`)
	})
	mux.HandleFunc("/audio/", func(w http.ResponseWriter, r *http.Request) {
		payload := wav
		w.Header().Set("Content-Type", "audio/wav")
		if r.URL.Path == "/audio/music.mp3" {
			w.Header().Set("Content-Type", "audio/mpeg")
			payload = mp3
		}
		if r.URL.Path == "/audio/export" {
			w.Header().Set("Content-Disposition", `inline; filename="spoken-reply.wav"`)
		}
		_, _ = w.Write(payload)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := browser.MustPage(server.URL + "/fixture").Timeout(30 * time.Second)
	defer page.MustClose()
	page.MustWaitLoad()
	for _, tc := range []struct {
		path, filename string
		payload        []byte
	}{
		{"/audio/speech.wav?signature=fixture#clip", "speech.wav", wav},
		{"/audio/Sprachtest%20Gr%C3%BC%C3%9Fe.wav", "Sprachtest Grüße.wav", wav},
		{"/audio/music.mp3", "music.mp3", mp3},
		{"/audio/export", "spoken-reply.wav", wav},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			page.MustEval(`url => {document.body.replaceChildren(new ChatAudioPlayer(url).element);}`, tc.path)
			dir := t.TempDir()
			download := browser.Timeout(15 * time.Second).WaitDownload(dir)
			page.MustElement(".audio-download-btn").MustClick()
			info := download()
			if info.SuggestedFilename != tc.filename {
				t.Fatalf("download name = %q, want %q", info.SuggestedFilename, tc.filename)
			}
			got, err := os.ReadFile(filepath.Join(dir, info.GUID))
			if err != nil || !bytes.Equal(got, tc.payload) {
				t.Fatalf("downloaded audio changed: %v", err)
			}
		})
	}
}

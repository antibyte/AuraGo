package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/agent"
	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
	"go.mau.fi/whatsmeow"
)

func uploadsOf(f *fakeClient) []uploadCall {
	var out []uploadCall
	f.with(func(f *fakeClient) { out = slices.Clone(f.uploads) })
	return out
}

func onePixelPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func entriesOf(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestPipelineSendsAnswerImagesAsImageMessages(t *testing.T) {
	data := onePixelPNG(t)
	newImageFixture := func(t *testing.T, answer string) (*pipelineFixture, string) {
		t.Helper()
		tmp := isolateTempDir(t)
		cfg := dedicatedPipelineConfig()
		cfg.Directories.WorkspaceDir = t.TempDir()
		if err := os.WriteFile(filepath.Join(cfg.Directories.WorkspaceDir, "chart.png"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		f := newPipelineFixture(t, cfg)
		f.answer = answer
		return f, tmp
	}

	t.Run("workspace image before the text", func(t *testing.T) {
		f, tmp := newImageFixture(t, "Here ![chart](/files/chart.png) done")
		f.p.handle(context.Background(), ownerText("I1", "draw"))

		uploads := uploadsOf(f.fake)
		if len(uploads) != 1 || uploads[0].mediaType != whatsmeow.MediaImage || !bytes.Equal(uploads[0].data, data) {
			t.Fatalf("uploads = %+v, want the PNG once as an image", uploads)
		}
		sent := sentTexts(f.fake)
		if len(sent) != 2 {
			t.Fatalf("sent = %+v, want the image and then the text", sent)
		}
		img := sent[0].msg.GetImageMessage()
		if img == nil || img.GetCaption() != "chart" || img.GetMimetype() != "image/png" || sent[0].to != ownerJID {
			t.Fatalf("first message = %+v, want an image with caption chart for the owner", sent[0].msg)
		}
		if text := sent[1].text; !strings.Contains(text, "Here") || !strings.Contains(text, "done") || strings.Contains(text, "[Image") {
			t.Fatalf("answer text = %q", text)
		}
		if left := entriesOf(t, tmp); len(left) != 0 {
			t.Fatalf("temporary files left behind: %v", left)
		}
	})

	t.Run("a long alt text becomes a bounded caption", func(t *testing.T) {
		f, _ := newImageFixture(t, "![ "+strings.Repeat("x", 1500)+"](/files/chart.png)")
		f.p.handle(context.Background(), ownerText("I2", "draw"))

		sent := sentTexts(f.fake)
		if len(sent) == 0 || sent[0].msg.GetImageMessage() == nil {
			t.Fatalf("sent = %+v, want an image message", sent)
		}
		caption := sent[0].msg.GetImageMessage().GetCaption()
		if utf8.RuneCountInString(caption) != maxCaptionRunes || !strings.HasSuffix(caption, "…") {
			t.Fatalf("caption has %d runes, suffix ok = %v; want exactly %d ending in an ellipsis", utf8.RuneCountInString(caption), strings.HasSuffix(caption, "…"), maxCaptionRunes)
		}
	})

	for _, c := range []struct {
		name, answer string
		uploadErr    error
		want         string
	}{
		{"missing workspace file", "Look ![missing](/files/missing.png) here", nil, "[Image: missing]"},
		{"unsupported URL scheme", "Look ![remote](ftp://example.org/a.png) here", nil, "[Image: remote]"},
		{"failed upload", "Look ![chart](/files/chart.png) here", errors.New("boom"), "[Image: chart]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, tmp := newImageFixture(t, c.answer)
			f.fake.with(func(fc *fakeClient) { fc.uploadErr = c.uploadErr })
			f.p.handle(context.Background(), ownerText("I3", "draw"))

			sent := sentTexts(f.fake)
			if len(sent) != 1 || sent[0].msg.GetImageMessage() != nil || !strings.Contains(sent[0].text, c.want) {
				t.Fatalf("sent = %+v, want one text with %q", sent, c.want)
			}
			if out := f.logs.String(); strings.Contains(out, "example.org") || strings.Contains(out, "missing.png") || strings.Contains(out, "chart.png") {
				t.Fatalf("log names an image URL or path: %q", out)
			}
			if left := entriesOf(t, tmp); len(left) != 0 {
				t.Fatalf("temporary files left behind: %v", left)
			}
		})
	}
}

// ttsFixture is a pipeline whose data directory holds <data>/tts/a.mp3; the agent stub sends
// an audio event for audioPath (the TTS file when empty).
func ttsFixture(t *testing.T, audioPath string) (*pipelineFixture, string) {
	t.Helper()
	cfg := dedicatedPipelineConfig()
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "data")
	ttsDir := filepath.Join(cfg.Directories.DataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	ttsFile := filepath.Join(ttsDir, "a.mp3")
	if err := os.WriteFile(ttsFile, []byte("ID3 fake mp3"), 0o600); err != nil {
		t.Fatal(err)
	}
	if audioPath == "" {
		audioPath = ttsFile
	}
	f := newPipelineFixture(t, cfg)
	event, err := json.Marshal(map[string]any{"file_path": audioPath, "title": "TTS Audio", "mime_type": "audio/mpeg", "filename": "a.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	f.script = func(b agent.FeedbackBroker) { b.Send("audio", string(event)) }
	return f, cfg.Directories.DataDir
}

func setVoiceMode(t *testing.T, on bool) {
	t.Helper()
	before := agent.GetVoiceMode()
	agent.SetVoiceMode(on)
	t.Cleanup(func() { agent.SetVoiceMode(before) })
}

func TestPipelineSendsTTSAsVoiceNoteInVoiceMode(t *testing.T) {
	t.Run("voice mode on", func(t *testing.T) {
		setVoiceMode(t, true)
		tmp := isolateTempDir(t)
		f, _ := ttsFixture(t, "")
		var converted []byte
		f.p.voiceNote = func(_ context.Context, in string) ([]byte, uint32, error) {
			var err error
			converted, err = os.ReadFile(in)
			return []byte("OggS fake opus"), 3, err
		}
		f.p.handle(context.Background(), ownerText("T1", "say hi"))

		calls := f.callList()
		if len(calls) != 1 || !calls[0].runCfg.VoiceOutputActive || calls[0].voiceSuppressed {
			t.Fatalf("agent run = %+v, want voice output active and not suppressed", calls)
		}
		if string(converted) != "ID3 fake mp3" {
			t.Fatalf("the converter read %q, want the TTS file", converted)
		}
		uploads := uploadsOf(f.fake)
		if len(uploads) != 1 || uploads[0].mediaType != whatsmeow.MediaAudio || string(uploads[0].data) != "OggS fake opus" {
			t.Fatalf("uploads = %+v, want the converted voice note once", uploads)
		}
		sent := sentTexts(f.fake)
		if len(sent) != 2 || sent[1].text == "" {
			t.Fatalf("sent = %+v, want the voice note and then the answer", sent)
		}
		audio := sent[0].msg.GetAudioMessage()
		if audio == nil || !audio.GetPTT() || audio.GetMimetype() != voiceNoteMIME || audio.GetSeconds() != 3 || sent[0].to != ownerJID {
			t.Fatalf("first message = %+v, want a PTT voice note of 3 seconds for the owner", sent[0].msg)
		}
		if left := entriesOf(t, tmp); len(left) != 0 {
			t.Fatalf("temporary files left behind: %v", left)
		}
	})

	t.Run("voice mode off", func(t *testing.T) {
		setVoiceMode(t, false)
		f, _ := ttsFixture(t, "")
		f.p.voiceNote = func(context.Context, string) ([]byte, uint32, error) {
			t.Error("a voice note was made outside voice mode")
			return nil, 0, errors.New("unexpected")
		}
		f.p.handle(context.Background(), ownerText("T2", "say hi"))

		if calls := f.callList(); len(calls) != 1 || calls[0].runCfg.VoiceOutputActive {
			t.Fatalf("agent run = %+v, want voice output inactive", calls)
		}
		uploads := uploadsOf(f.fake)
		if len(uploads) != 1 || uploads[0].mediaType != whatsmeow.MediaAudio || string(uploads[0].data) != "ID3 fake mp3" {
			t.Fatalf("uploads = %+v, want the MP3 itself", uploads)
		}
		audio := sentTexts(f.fake)[0].msg.GetAudioMessage()
		if audio == nil || audio.GetPTT() || audio.GetMimetype() != "audio/mpeg" {
			t.Fatalf("first message = %+v, want a plain audio/mpeg message", sentTexts(f.fake)[0].msg)
		}
	})

	t.Run("a failed conversion falls back to audio", func(t *testing.T) {
		setVoiceMode(t, true)
		f, _ := ttsFixture(t, "")
		f.p.voiceNote = func(context.Context, string) ([]byte, uint32, error) {
			return nil, 0, errors.New("ffmpeg voice conversion failed")
		}
		f.p.handle(context.Background(), ownerText("T3", "say hi"))

		sent := sentTexts(f.fake)
		audio := sent[0].msg.GetAudioMessage()
		if len(sent) != 2 || audio == nil || audio.GetPTT() || audio.GetMimetype() != "audio/mpeg" {
			t.Fatalf("sent = %+v, want a plain audio message and the answer", sent)
		}
		if out := f.logs.String(); !strings.Contains(out, "voice conversion failed") || strings.Contains(out, "a.mp3") {
			t.Fatalf("log = %q, want the fallback notice without the file name", out)
		}
	})
}

func TestPipelineDeliversOnlyTTSFromTheDataDir(t *testing.T) {
	setVoiceMode(t, false)
	// Each case gets its own data directory below its own root, so "<data>/.." is private.
	for _, c := range []struct {
		name   string
		path   func(t *testing.T, root, data string) string
		reason string
	}{
		{"parent of the data dir", func(t *testing.T, root, data string) string { return filepath.Join(data, "..", "secret.mp3") }, "outside_tts_dir"},
		{"parent of the tts dir", func(t *testing.T, root, data string) string { return filepath.Join(data, "tts", "..", "secret.mp3") }, "outside_tts_dir"},
		{"sibling with the same prefix", func(t *testing.T, root, data string) string { return filepath.Join(data, "tts2", "a.mp3") }, "outside_tts_dir"},
		{"the folder itself", func(t *testing.T, root, data string) string { return filepath.Join(data, "tts") }, "outside_tts_dir"},
		{"a link that leads out", func(t *testing.T, root, data string) string {
			link := filepath.Join(data, "tts", "link.mp3")
			if err := os.Symlink(filepath.Join(root, "secret.mp3"), link); err != nil {
				t.Skipf("symbolic links are not available: %v", err)
			}
			return link
		}, "unreadable"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, data := ttsFixture(t, "")
			root := filepath.Dir(data)
			if err := os.WriteFile(filepath.Join(root, "secret.mp3"), []byte("ID3 secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(data, "tts2"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(data, "tts2", "a.mp3"), []byte("ID3 other"), 0o600); err != nil {
				t.Fatal(err)
			}
			target := c.path(t, root, data)
			event, _ := json.Marshal(map[string]any{"file_path": target, "title": "TTS Audio"})
			f.script = func(b agent.FeedbackBroker) { b.Send("audio", string(event)) }

			f.p.handle(context.Background(), ownerText("T4", "say hi"))

			if uploads := uploadsOf(f.fake); len(uploads) != 0 {
				t.Fatalf("a file outside the TTS folder was uploaded: %d uploads", len(uploads))
			}
			sent := sentTexts(f.fake)
			if len(sent) != 1 || sent[0].msg.GetAudioMessage() != nil {
				t.Fatalf("sent = %+v, want only the answer text", sent)
			}
			out := f.logs.String()
			if !strings.Contains(out, "WhatsApp audio skipped") || !strings.Contains(out, "reason="+c.reason) || strings.Contains(out, "secret") {
				t.Fatalf("log = %q, want the skip with reason %s and no path", out, c.reason)
			}
		})
	}
}

func TestPipelineAppendsMissingYouTubeLinks(t *testing.T) {
	const event = `{"url":"https://youtu.be/x","title":"Title","video_id":"x"}`
	for _, c := range []struct {
		name, answer, wantSuffix string
		wantCount                int
	}{
		{"link is added", "", "Title: https://youtu.be/x", 1},
		{"link already in the answer", "Watch https://youtu.be/x now", "https://youtu.be/x now", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newPipelineFixture(t, dedicatedPipelineConfig())
			f.answer = c.answer
			f.script = func(b agent.FeedbackBroker) { b.Send("youtube_video", event) }
			f.p.handle(context.Background(), ownerText("Y1", "show a video"))

			sent := sentBodies(f.fake)
			if len(sent) != 1 || !strings.HasSuffix(sent[0], c.wantSuffix) || strings.Count(sent[0], "https://youtu.be/x") != c.wantCount {
				t.Fatalf("sent = %q, want one text ending in %q with the link %d time(s)", sent, c.wantSuffix, c.wantCount)
			}
		})
	}
}

// A /files/ image opens like every other file that leaves AuraGo: a protected name and a root that
// holds data_dir are refused, and the model only gets its "[Image: alt]" line.
func TestPipelineSendsNoProtectedOrDataDirAnswerImages(t *testing.T) {
	data := onePixelPNG(t)
	for _, c := range []struct {
		name, url string
		setup     func(t *testing.T, cfg *config.Config)
	}{
		{"protected name", "/files/vault.env", func(t *testing.T, cfg *config.Config) {
			writePNG(t, filepath.Join(cfg.Directories.WorkspaceDir, "vault.env"), data)
		}},
		{"data_dir below the workspace root", "/files/data/camera_snapshot.png", func(t *testing.T, cfg *config.Config) {
			cfg.Directories.DataDir = filepath.Join(cfg.Directories.WorkspaceDir, "data")
			writePNG(t, filepath.Join(cfg.Directories.DataDir, "camera_snapshot.png"), data)
		}},
		{"path out of the workspace", "/files/../secret.png", func(t *testing.T, cfg *config.Config) {
			writePNG(t, filepath.Join(filepath.Dir(cfg.Directories.WorkspaceDir), "secret.png"), data)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := dedicatedPipelineConfig()
			cfg.Directories.WorkspaceDir = filepath.Join(t.TempDir(), "workspace")
			if err := os.MkdirAll(cfg.Directories.WorkspaceDir, 0o700); err != nil {
				t.Fatal(err)
			}
			c.setup(t, cfg)
			f := newPipelineFixture(t, cfg)
			f.answer = "Here ![alt text](" + c.url + ") done"
			f.p.handle(context.Background(), ownerText("I9", "show it"))

			if up := uploadsOf(f.fake); len(up) != 0 {
				t.Fatalf("%d uploads: a refused file reached WhatsApp", len(up))
			}
			sent := sentTexts(f.fake)
			if len(sent) != 1 || sent[0].msg.GetImageMessage() != nil || !strings.Contains(sent[0].text, "[Image: alt text]") {
				t.Fatalf("sent = %+v, want one text with the [Image: alt text] line", sent)
			}
			if out := f.logs.String(); strings.Contains(out, "vault.env") || strings.Contains(out, "camera_snapshot") || strings.Contains(out, "secret.png") {
				t.Fatalf("log names the refused file: %q", out)
			}
		})
	}
}

func writePNG(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Only the first five images of a reply are fetched; the rest are lines. A fetched copy is
// removed from <workspace>/images once it has been sent, or has failed to send.
func TestPipelineFetchesAtMostFiveAnswerImages(t *testing.T) {
	data := onePixelPNG(t)
	for _, c := range []struct {
		name      string
		uploadErr error
		wantSent  int // image messages that went out
	}{
		{"sent", nil, 5},
		{"upload fails", errors.New("boom"), 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := dedicatedPipelineConfig()
			cfg.Directories.WorkspaceDir = t.TempDir()
			f := newPipelineFixture(t, cfg)
			f.fake.with(func(fc *fakeClient) { fc.uploadErr = c.uploadErr })
			var fetched []string
			f.p.fetchImage = func(_ context.Context, rawURL, destDir string) (string, error) {
				fetched = append(fetched, rawURL)
				path := filepath.Join(destDir, fmt.Sprintf("fetched%d.png", len(fetched)))
				writePNG(t, path, data)
				return path, nil
			}
			var answer strings.Builder
			for i := 0; i < 7; i++ {
				fmt.Fprintf(&answer, "![pic %d](https://img.example/%d.png)\n", i, i)
			}
			f.answer = answer.String()
			f.p.handle(context.Background(), ownerText("I8", "show them"))

			if len(fetched) != 5 {
				t.Fatalf("fetch attempts = %d, want 5", len(fetched))
			}
			images, text := 0, ""
			for _, m := range sentTexts(f.fake) {
				if m.msg.GetImageMessage() != nil {
					images++
				} else {
					text += m.text
				}
			}
			if images != c.wantSent {
				t.Fatalf("image messages = %d, want %d", images, c.wantSent)
			}
			for i := 0; i < 7; i++ {
				line := fmt.Sprintf("[Image: pic %d]", i)
				if wantLine := i >= 5 || c.wantSent == 0; strings.Contains(text, line) != wantLine {
					t.Fatalf("line %q present = %v, want %v in %q", line, !wantLine, wantLine, text)
				}
			}
			if left := entriesOf(t, filepath.Join(cfg.Directories.WorkspaceDir, "images")); len(left) != 0 {
				t.Fatalf("fetched copies left in the workspace: %v", left)
			}
		})
	}
}

// Audio and links a run captured are not delivered when the run fails, and the log says so.
func TestPipelineDropsCapturedMediaAfterAFailedRunWithALogLine(t *testing.T) {
	f, _ := ttsFixture(t, "")
	f.p.agentLoop = func(_ context.Context, _ openai.ChatCompletionRequest, _ agent.RunConfig, _ bool, b agent.FeedbackBroker) (openai.ChatCompletionResponse, error) {
		f.script(b)
		b.Send("youtube_video", `{"url":"https://youtu.be/x","title":"Title"}`)
		return openai.ChatCompletionResponse{}, errors.New("model unavailable")
	}
	f.p.handle(context.Background(), ownerText("E1", "say hi"))

	if up := uploadsOf(f.fake); len(up) != 0 {
		t.Fatalf("%d uploads after a failed run", len(up))
	}
	if got := sentBodies(f.fake); !slices.Equal(got, []string{agentErrorText}) {
		t.Fatalf("sent %q, want only the error notice", got)
	}
	out := f.logs.String()
	if !strings.Contains(out, "captured media dropped after a failed agent run") || !strings.Contains(out, "audio=1") || !strings.Contains(out, "videos=1") || strings.Contains(out, "a.mp3") {
		t.Fatalf("log = %q, want one line with the counts and no file name", out)
	}
}

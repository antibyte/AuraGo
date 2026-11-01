package whatsapp

import (
	"context"
	"slices"
	"testing"
	"time"

	"aurago/internal/config"
)

// newTestBroker returns a broker for the owner whose clock is *clock.
func newTestBroker(fake *fakeClient, clock *time.Time) *whatsAppBroker {
	s, _ := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, fake)
	return &whatsAppBroker{ctx: context.Background(), sender: s, to: ownerJID, now: func() time.Time { return *clock }}
}

func sentBodies(fake *fakeClient) []string {
	var out []string
	for _, m := range sentTexts(fake) {
		out = append(out, m.text)
	}
	return out
}

func TestBrokerDeliversQuestionsAndBudgetNoticesImmediately(t *testing.T) {
	fake := &fakeClient{own: testOwn, loggedIn: true}
	clock := fixedNow()
	b := newTestBroker(fake, &clock)

	b.Send("question_user", "Which folder should I clean up?")
	b.Send("budget_warning", "80% of the daily budget is used.")
	b.Send("budget_blocked", "The daily budget is used up.")

	want := []string{
		"Which folder should I clean up?",
		"⚠️ 80% of the daily budget is used.",
		"🚫 The daily budget is used up.",
	}
	if got := sentBodies(fake); !slices.Equal(got, want) {
		t.Fatalf("sent %q, want %q", got, want)
	}
	for _, m := range sentTexts(fake) {
		if m.to != ownerJID {
			t.Fatalf("a message went to %v", m.to)
		}
	}
}

func TestBrokerCoalescesStatusToOnePerTenSeconds(t *testing.T) {
	fake := &fakeClient{own: testOwn, loggedIn: true}
	t0 := fixedNow()
	clock := t0
	b := newTestBroker(fake, &clock)

	b.Send("tool_start", "shell") // t0: sent
	clock = t0.Add(3 * time.Second)
	b.Send("progress", "still working") // inside the window: dropped
	clock = t0.Add(11 * time.Second)
	b.Send("api_retry", "attempt 2") // sent
	b.Send("audio", `{"file_path":"/tmp/a.mp3"}`)
	b.Send("youtube_video", `{"url":"https://example.org/v"}`)
	b.Send("thinking", "ignored")
	clock = t0.Add(15 * time.Second)
	b.Send("error_recovery", "retrying") // 4 s after the last status: dropped
	clock = t0.Add(21 * time.Second)
	b.Send("progress", "almost done") // 10 s after the last status: sent

	want := []string{"⚙️ TOOL_START: shell", "⚙️ API_RETRY: attempt 2", "⏳ almost done"}
	if got := sentBodies(fake); !slices.Equal(got, want) {
		t.Fatalf("sent %q, want %q", got, want)
	}
}

func TestBrokerCapturesAudioAndYouTubeForAfterTheRun(t *testing.T) {
	fake := &fakeClient{own: testOwn, loggedIn: true}
	clock := fixedNow()
	b := newTestBroker(fake, &clock)

	b.Send("audio", `{"file_path":"/data/tts/a.mp3","title":"TTS Audio","mime_type":"audio/mpeg","filename":"a.mp3"}`)
	b.Send("audio", `{"path":"/audio/song.mp3","title":"Song"}`) // generate_music and send_audio carry web paths only
	b.Send("audio", "not json")
	b.Send("youtube_video", `{"url":"https://youtu.be/x","title":"Title","video_id":"x"}`)
	b.Send("youtube_video", `{"title":"no url"}`)

	audio, videos := b.captured()
	if len(audio) != 1 || audio[0].FilePath != "/data/tts/a.mp3" || audio[0].Title != "TTS Audio" || audio[0].MimeType != "audio/mpeg" || audio[0].Filename != "a.mp3" {
		t.Fatalf("captured audio = %+v, want only the event with a file_path", audio)
	}
	if len(videos) != 1 || videos[0].URL != "https://youtu.be/x" || videos[0].Title != "Title" || videos[0].VideoID != "x" {
		t.Fatalf("captured videos = %+v, want only the event with a url", videos)
	}
	if got := sentTexts(fake); len(got) != 0 {
		t.Fatalf("capturing sent %+v, nothing may go out during the run", got)
	}

	audio[0].FilePath = "changed"
	if again, _ := b.captured(); again[0].FilePath != "/data/tts/a.mp3" {
		t.Fatal("captured must return copies")
	}
}

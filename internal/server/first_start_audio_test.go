package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirstStartNamingPromptQueuesBrainInTheMachineAudio(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("handlers.go")
	if err != nil {
		t.Fatalf("read handlers.go: %v", err)
	}
	text := string(source)

	audioDispatch := `sendFirstStartIntroAudio(NewSSEBrokerAdapterWithSession(sse, sessionID))`
	namingPrompt := `[FIRST START INITIALIZATION`
	agentLoop := `agent.ExecuteAgentLoop`

	audioAt := strings.Index(text, audioDispatch)
	promptAt := strings.Index(text, namingPrompt)
	loopAt := strings.Index(text, agentLoop)
	if audioAt < 0 {
		t.Fatalf("first-start flow must queue Brain in the Machine audio before the naming prompt")
	}
	if promptAt < 0 {
		t.Fatalf("first-start naming prompt marker missing")
	}
	if loopAt < 0 {
		t.Fatalf("agent loop marker missing")
	}
	if !(audioAt < promptAt && promptAt < loopAt) {
		t.Fatalf("first-start intro order = audio:%d prompt:%d loop:%d, want audio before naming prompt before agent loop", audioAt, promptAt, loopAt)
	}
}

func TestFirstStartIntroAudioUsesBundledBrainInTheMachineSample(t *testing.T) {
	t.Parallel()

	manifest, err := os.ReadFile(filepath.Join("..", "..", "assets", "media_samples", "metadata.json"))
	if err != nil {
		t.Fatalf("read media sample manifest: %v", err)
	}
	if !strings.Contains(string(manifest), `"filename": "brain_in_the _machine.mp3"`) {
		t.Fatalf("media sample manifest must include the bundled Brain in the Machine audio file")
	}
}

func TestSendFirstStartIntroAudioPayload(t *testing.T) {
	t.Parallel()

	broker := &firstStartIntroCaptureBroker{}
	sendFirstStartIntroAudio(broker)

	if broker.event != "audio" {
		t.Fatalf("event = %q, want audio", broker.event)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(broker.message), &payload); err != nil {
		t.Fatalf("decode intro audio payload: %v", err)
	}
	if got := payload["path"]; got != "/files/audio/brain_in_the%20_machine.mp3" {
		t.Fatalf("path = %q, want encoded Brain in the Machine sample path", got)
	}
	if got := payload["title"]; got != "Brain in the Machine" {
		t.Fatalf("title = %q, want Brain in the Machine", got)
	}
	if got := payload["autoplay"]; got != true {
		t.Fatalf("autoplay = %v, want true", got)
	}
}

type firstStartIntroCaptureBroker struct {
	event   string
	message string
}

func (b *firstStartIntroCaptureBroker) Send(event, message string) {
	b.event = event
	b.message = message
}

func (b *firstStartIntroCaptureBroker) SendJSON(string) {}

func (b *firstStartIntroCaptureBroker) SendLLMStreamDelta(string, string, string, int, string) {}

func (b *firstStartIntroCaptureBroker) SendLLMStreamDone(string) {}

func (b *firstStartIntroCaptureBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}

func (b *firstStartIntroCaptureBroker) SendThinkingBlock(string, string, string) {}

func (b *firstStartIntroCaptureBroker) Scrub(s string) string { return s }

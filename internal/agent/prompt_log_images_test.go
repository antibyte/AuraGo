package agent

import (
	"encoding/json"
	openai "github.com/sashabaranov/go-openai"
	"strings"
	"testing"
)

func TestPromptLogOmitsImagesWithoutMutatingRequest(t *testing.T) {
	for _, url := range []string{"data:image/png;base64,private-image", "https://example.invalid/private-signed-image"} {
		input := []openai.ChatCompletionMessage{{Role: "user", MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeText, Text: "Inspect the game"}, {Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: url}}}}}
		output := promptLogMessages(input)
		b, err := json.Marshal(output)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), url) || !strings.Contains(string(b), "Inspect the game") {
			t.Fatal("image leaked or text lost")
		}
		if input[0].MultiContent[1].ImageURL.URL != url {
			t.Fatal("provider request mutated by logging")
		}
	}
}

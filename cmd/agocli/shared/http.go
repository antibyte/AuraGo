// shared HTTP client for agocli
package shared

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ChatClient is an HTTP client for the AuraGo chat API.
type ChatClient struct {
	BaseURL string
	Client  *http.Client
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for /v1/chat/completions.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatResponse is the response from /v1/chat/completions (non-streaming).
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// NewChatClient creates a new ChatClient.
func NewChatClient(baseURL string) *ChatClient {
	return &ChatClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// StreamChat sends a chat request and calls onChunk for each streaming chunk.
// Returns when the stream is complete.
func (c *ChatClient) StreamChat(ctx context.Context, messages []ChatMessage, onChunk func(text string, isDone bool)) error {
	reqBody := ChatRequest{
		Model:    "aurago",
		Messages: messages,
		Stream:   true,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	scanner := bufio.NewScanner(resp.Body)
	// Increase scanner buffer for large tokens
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !bytes.HasPrefix([]byte(line), []byte("data: ")) {
			continue
		}
		data := line[6:] // strip "data: "
		if data == "[DONE]" {
			onChunk("", true)
			return nil
		}

		// Parse SSE data chunk - OpenAI streaming format
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content, false)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	onChunk("", true)
	return nil
}

// SendMessage sends a non-streaming chat message and returns the response.
func (c *ChatClient) SendMessage(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	reqBody := ChatRequest{
		Model:    "aurago",
		Messages: messages,
		Stream:   false,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// Ping checks if the AuraGo server is reachable.
func (c *ChatClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}

// GetServerURL returns the AuraGo server URL by reading config.yaml.
func GetServerURL() string {
	// Search for config.yaml in standard locations
	dirs := []string{
		".", "..", "../..",
		"/etc/aurago",
		os.Getenv("HOME") + "/aurago",
	}
	for _, d := range dirs {
		p := filepath.Join(d, "config.yaml")
		if data, err := os.ReadFile(p); err == nil {
			// Try to extract server.host and server.port from YAML
			if host, port := extractServerHostPort(string(data)); host != "" || port > 0 {
				scheme := "http"
				if port == 443 || port == 8443 {
					scheme = "https"
				}
				if host == "" {
					host = "localhost"
				}
				if port <= 0 {
					port = 8088
				}
				return fmt.Sprintf("%s://%s:%d", scheme, host, port)
			}
			break
		}
	}
	// Default fallback
	return "http://localhost:8088"
}

// extractServerHostPort extracts server.host and server.port from YAML content.
func extractServerHostPort(yaml string) (host string, port int) {
	// Simple text search — we don't pull in the full yaml parser just for this.
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "host:") {
			host = strings.TrimSpace(strings.TrimPrefix(line, "host:"))
			host = strings.Trim(host, "\"")
		}
		if strings.HasPrefix(line, "port:") {
			portStr := strings.TrimSpace(strings.TrimPrefix(line, "port:"))
			fmt.Sscanf(portStr, "%d", &port)
		}
	}
	return
}

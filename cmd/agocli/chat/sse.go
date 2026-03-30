package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// SSEClient connects to the AuraGo /events endpoint and parses events.
type SSEClient struct {
	serverURL string
	events    chan Event
	errCh     chan error
	done      chan struct{}
}

// NewSSEClient creates a new SSE client.
func NewSSEClient(serverURL string) *SSEClient {
	return &SSEClient{
		serverURL: serverURL,
		events:    make(chan Event, 64),
		errCh:    make(chan error, 1),
		done:     make(chan struct{}),
	}
}

// Start initiates the SSE connection.
func (c *SSEClient) Start() error {
	req, err := http.NewRequest("GET", c.serverURL+"/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{
		Timeout: 0, // No timeout for SSE
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	go c.readLoop(resp)

	return nil
}

func (c *SSEClient) readLoop(resp *http.Response) {
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size for large messages
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	scanner.Split(bufio.ScanLines)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		// Set a read timeout
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				select {
				case c.errCh <- fmt.Errorf("SSE read error: %w", err):
				default:
				}
			}
			return
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		event := c.parseEvent(data)
		if event.Type == "" {
			continue
		}

		select {
		case c.events <- event:
		case <-c.done:
			return
		}
	}
}

func (c *SSEClient) parseEvent(data string) Event {
	// Try legacy format: {"event":"<type>","detail":"<content>"}
	var legacy struct {
		Event  string `json:"event"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(data), &legacy); err == nil && legacy.Event != "" {
		return Event{Type: legacy.Event, Content: legacy.Detail}
	}

	// Try typed format: {"type":"<type>","payload":{...}}
	var typed struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(data), &typed); err == nil && typed.Type != "" {
		// Try to extract content from payload
		var payload struct {
			Content string `json:"content"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(typed.Payload, &payload); err == nil {
			content := payload.Content
			if content == "" {
				content = payload.Message
			}
			if content == "" {
				content = payload.Error
			}
			return Event{Type: typed.Type, Content: content}
		}
		return Event{Type: typed.Type, Content: string(typed.Payload)}
	}

	// Try simple format: {"content": "..."} or plain text
	var simple struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(data), &simple); err == nil && simple.Content != "" {
		return Event{Type: "message", Content: simple.Content}
	}

	return Event{}
}

// Events returns the channel of parsed events.
func (c *SSEClient) Events() <-chan Event {
	return c.events
}

// Err returns the error channel.
func (c *SSEClient) Err() <-chan error {
	return c.errCh
}

// Stop closes the SSE connection.
func (c *SSEClient) Stop() {
	close(c.done)
}

// connectSSE is a tea.Cmd that starts the SSE connection.
func connectSSE(client *SSEClient) tea.Cmd {
	return func() tea.Msg {
		if err := client.Start(); err != nil {
			return SSEError{err}
		}

		// Wait for events or errors in a loop
		for {
			select {
			case event, ok := <-client.Events():
				if !ok {
					return SSEError{fmt.Errorf("SSE connection closed")}
				}
				return SSEEvent{Event: event}
			case err, ok := <-client.Err():
				if !ok {
					return SSEError{fmt.Errorf("SSE error channel closed")}
				}
				return SSEError{err}
			case <-time.After(5 * time.Second):
				// Periodic ping to keep connection alive
				continue
			}
		}
	}
}

// SSEEvent is a tea.Msg containing an SSE event.
type SSEEvent struct {
	Event Event
}

// SSEError is a tea.Msg containing an SSE error.
type SSEError struct {
	Err error
}

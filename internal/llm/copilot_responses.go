package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func chatCompletionToResponsesBody(body []byte) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	if _, exists := payload["input"]; exists {
		return body
	}
	if messages, ok := payload["messages"]; ok {
		payload["input"] = messages
		delete(payload, "messages")
	}
	if maxTokens, ok := payload["max_tokens"]; ok {
		payload["max_output_tokens"] = maxTokens
		delete(payload, "max_tokens")
	}
	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

func translateCopilotResponsesResponse(resp *http.Response, model string, stream bool) (*http.Response, error) {
	if resp == nil || resp.Body == nil {
		return resp, nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return resp, nil
	}
	if stream {
		return translateCopilotResponsesStream(resp, model)
	}
	return translateCopilotResponsesJSON(resp, model)
}

func translateCopilotResponsesJSON(resp *http.Response, model string) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("copilot responses: read body: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	text := extractResponsesOutputText(payload)
	respID, _ := payload["id"].(string)
	if respID == "" {
		respID = "resp-unknown"
	}
	if model == "" {
		if m, _ := payload["model"].(string); m != "" {
			model = m
		}
	}

	oai := map[string]interface{}{
		"id":      "chatcmpl-" + strings.TrimPrefix(respID, "resp_"),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": "stop",
			},
		},
	}
	if usage, ok := payload["usage"].(map[string]interface{}); ok {
		oai["usage"] = map[string]interface{}{
			"prompt_tokens":     usage["input_tokens"],
			"completion_tokens": usage["output_tokens"],
			"total_tokens":      sumTokenUsage(usage),
		}
	}

	oaiBody, err := json.Marshal(oai)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(oaiBody))
	resp.ContentLength = int64(len(oaiBody))
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func translateCopilotResponsesStream(resp *http.Response, model string) (*http.Response, error) {
	pr, pw := io.Pipe()
	originalBody := resp.Body

	go func() {
		defer originalBody.Close()
		err := streamCopilotResponsesToOpenAI(originalBody, pw, model)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	resp.Body = pr
	resp.Header.Set("Content-Type", "text/event-stream")
	return resp, nil
}

func streamCopilotResponsesToOpenAI(reader io.Reader, writer io.Writer, model string) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var respID string
	var currentEvent string
	roleSent := false

	writeChunk := func(delta map[string]interface{}, finishReason *string) error {
		chunk := map[string]interface{}{
			"id":      respID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": delta,
				},
			},
		}
		if finishReason != nil {
			choices := chunk["choices"].([]map[string]interface{})
			choices[0]["finish_reason"] = *finishReason
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(writer, "data: %s\n\n", data)
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}

		var evt map[string]interface{}
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}
		evtType, _ := evt["type"].(string)
		if evtType == "" {
			evtType = currentEvent
		}

		switch evtType {
		case "response.created", "response.in_progress":
			if response, ok := evt["response"].(map[string]interface{}); ok {
				if id, _ := response["id"].(string); id != "" {
					respID = "chatcmpl-" + strings.TrimPrefix(id, "resp_")
				}
				if m, _ := response["model"].(string); m != "" && model == "" {
					model = m
				}
			}
			if !roleSent && respID != "" {
				if err := writeChunk(map[string]interface{}{"role": "assistant"}, nil); err != nil {
					return err
				}
				roleSent = true
			}

		case "response.output_text.delta":
			delta, _ := evt["delta"].(string)
			if delta == "" {
				continue
			}
			if respID == "" {
				respID = "chatcmpl-copilot"
			}
			if err := writeChunk(map[string]interface{}{"content": delta}, nil); err != nil {
				return err
			}

		case "response.completed":
			if response, ok := evt["response"].(map[string]interface{}); ok {
				if id, _ := response["id"].(string); id != "" && respID == "" {
					respID = "chatcmpl-" + strings.TrimPrefix(id, "resp_")
				}
			}
			finish := "stop"
			if err := writeChunk(map[string]interface{}{}, &finish); err != nil {
				return err
			}
			fmt.Fprint(writer, "data: [DONE]\n\n")
			return scanner.Err()

		case "error":
			msg := "copilot responses stream error"
			if errObj, ok := evt["error"].(map[string]interface{}); ok {
				if m, _ := errObj["message"].(string); m != "" {
					msg = m
				}
			}
			if err := writeChunk(map[string]interface{}{"content": "[Copilot Error: " + msg + "]"}, nil); err != nil {
				return err
			}
			fmt.Fprint(writer, "data: [DONE]\n\n")
			return nil
		}
	}

	fmt.Fprint(writer, "data: [DONE]\n\n")
	return scanner.Err()
}

func extractResponsesOutputText(payload map[string]interface{}) string {
	output, ok := payload["output"].([]interface{})
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, itemRaw := range output {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if content, ok := item["content"].([]interface{}); ok {
			for _, partRaw := range content {
				part, ok := partRaw.(map[string]interface{})
				if !ok {
					continue
				}
				if text, _ := part["text"].(string); text != "" {
					if b.Len() > 0 {
						b.WriteString("\n")
					}
					b.WriteString(text)
				}
			}
			continue
		}
		if text, _ := item["text"].(string); text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(text)
		}
	}
	return b.String()
}

func sumTokenUsage(usage map[string]interface{}) int {
	inTok, _ := usage["input_tokens"].(float64)
	outTok, _ := usage["output_tokens"].(float64)
	return int(inTok + outTok)
}
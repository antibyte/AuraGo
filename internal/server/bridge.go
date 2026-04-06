package server

import (
	"aurago/internal/llm"
	"aurago/internal/tools"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

type BridgeCommand struct {
	Command   string `json:"command"`
	Challenge string `json:"challenge,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Status    bool   `json:"status,omitempty"` // For set_busy
	Token     string `json:"token,omitempty"`  // per-process auth token
}

type BridgeResponse struct {
	Status  string `json:"status"`
	Result  string `json:"result,omitempty"`
	Message string `json:"message,omitempty"`
}

func (s *Server) StartTCPBridge(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.Logger.Error("TCP Bridge: Failed to listen", "addr", addr, "error", err)
		return
	}
	s.Logger.Info("TCP Bridge: Listening", "addr", addr)
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.Logger.Error("TCP Bridge: Failed to accept connection", "error", err)
			continue
		}
		go s.handleBridgeConnection(conn)
	}
}

func (s *Server) handleBridgeConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		// Set a 10-minute idle deadline; resets on each command
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				// Suppress reporting of "connection reset" or "connection aborted" errors
				// which are common during shutdown/reload transitions.
				errStr := err.Error()
				if !strings.Contains(errStr, "reset") && !strings.Contains(errStr, "abort") && !strings.Contains(errStr, "closed") {
					s.Logger.Error("TCP Bridge: Read error", "error", err)
				} else {
					s.Logger.Debug("TCP Bridge: Connection closed/reset by peer", "error", err)
				}
			}
			return
		}

		var cmd BridgeCommand
		if err := json.Unmarshal(line, &cmd); err != nil {
			s.sendBridgeResponse(conn, BridgeResponse{Status: "error", Message: "Invalid JSON"})
			continue
		}

		// Authenticate: vitality_check has its own challenge/response; all other
		// commands require the per-process token to be present and correct.
		if cmd.Command != "vitality_check" {
			tok := s.internalToken
			if tok == "" || cmd.Token != tok {
				s.Logger.Warn("TCP Bridge: rejected unauthenticated command", "command", cmd.Command)
				s.sendBridgeResponse(conn, BridgeResponse{Status: "error", Message: "unauthorized"})
				continue
			}
		}

		switch cmd.Command {
		case "vitality_check":
			s.Logger.Info("TCP Bridge: Vitality check received", "challenge", cmd.Challenge, "summary", cmd.Summary)
			result, err := s.processVitalityCheck(cmd.Challenge, cmd.Summary)
			if err != nil {
				s.Logger.Error("TCP Bridge: Vitality check failed", "error", err)
				s.sendBridgeResponse(conn, BridgeResponse{Status: "error", Message: "Vitality check failed"})
			} else {
				s.sendBridgeResponse(conn, BridgeResponse{Status: "ok", Result: result})
			}

		case "set_busy":
			s.Logger.Info("TCP Bridge: Toggle maintenance status", "busy", cmd.Status)
			tools.SetBusy(cmd.Status)
			s.sendBridgeResponse(conn, BridgeResponse{Status: "ok", Message: fmt.Sprintf("Busy set to %v", cmd.Status)})

		case "shutdown_and_reload":
			s.Logger.Info("TCP Bridge: Shutdown and reload requested")
			s.sendBridgeResponse(conn, BridgeResponse{Status: "ok", Message: "Reloading..."})
			// Signal graceful shutdown instead of os.Exit
			go func() {
				time.Sleep(1 * time.Second)
				if s.ShutdownCh != nil {
					close(s.ShutdownCh)
				} else {
					os.Exit(0)
				}
			}()

		default:
			s.sendBridgeResponse(conn, BridgeResponse{Status: "error", Message: "Unknown command"})
		}
	}
}

func (s *Server) sendBridgeResponse(conn net.Conn, resp BridgeResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.Logger.Error("TCP Bridge: Failed to marshal response", "error", err)
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		s.Logger.Error("TCP Bridge: Failed to write response", "error", err)
	}
}

func (s *Server) processVitalityCheck(challenge, summary string) (string, error) {
	prompt := fmt.Sprintf("Du bist ein Vitality-Check-Service. Erhalte eine Challenge und gib NUR den darin enthaltenen Code oder das Ergebnis zwischen #-Zeichen zurück. Challenge: %s", challenge)

	systemPrompt := "Du antwortest strikt nur mit dem Code zwischen #-Zeichen."
	if summary != "" {
		systemPrompt += " Kontext der durchgeführten Änderungen: " + summary
	}

	req := openai.ChatCompletionRequest{
		Model: s.Cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := llm.ExecuteWithRetry(ctx, s.LLMClient, req, s.Logger, nil)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		content := resp.Choices[0].Message.Content
		// Extract content between # if present, otherwise return as is
		start := strings.Index(content, "#")
		end := strings.LastIndex(content, "#")
		if start != -1 && end != -1 && start < end {
			return content[start+1 : end], nil
		}
		return content, nil
	}

	return "", fmt.Errorf("no response from LLM")
}

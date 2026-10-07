package server

import (
	"context"
	"encoding/json"
	"fmt"
	pathpkg "path"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const codeStudioTerminalInputLimit = 64 * 1024

func (h codeStudioHandlers) runLineTerminal(ctx context.Context, conn *websocket.Conn, containerID string) {
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	conn.SetReadLimit(codeStudioTerminalInputLimit)
	session := newCodeStudioTerminalSession(codeStudioWorkspaceRoot)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(session.prompt())); err != nil {
		return
	}
	for ctx.Err() == nil {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(payload, &msg) == nil && msg.Type == "resize" {
			continue
		}
		input := string(payload)
		for ctx.Err() == nil {
			line, echo, complete, err := session.consume(input)
			input = ""
			if err != nil {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()+"\r\n"))
				return
			}
			if echo != "" {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(echo)); err != nil {
					return
				}
			}
			if !complete {
				break
			}
			output := h.executeTerminalLine(ctx, containerID, session, line)
			if ctx.Err() != nil {
				return
			}
			output = strings.ReplaceAll(strings.ReplaceAll(output, "\r\n", "\n"), "\n", "\r\n")
			if err := conn.WriteMessage(websocket.TextMessage, []byte(output+session.prompt())); err != nil {
				return
			}
		}
	}
}

func (h codeStudioHandlers) executeTerminalLine(ctx context.Context, containerID string, session *codeStudioTerminalSession, line string) string {
	if line == "" || ctx.Err() != nil {
		return ""
	}
	command := "cd " + shellQuote(session.cwd) + " && " + line
	cdTarget, isCD := codeStudioCDTarget(line)
	if isCD {
		if !pathpkg.IsAbs(cdTarget) {
			cdTarget = pathpkg.Join(session.cwd, cdTarget)
		}
		next, err := sanitizeCodeStudioPath(pathpkg.Clean(cdTarget))
		if err != nil {
			return err.Error() + "\n"
		}
		command = "cd " + shellQuote(next) + " && pwd -P"
	}
	result, err := h.docker.Exec(ctx, containerID, []string{"sh", "-c", command}, codeStudioMaxExecTime)
	if err != nil {
		return err.Error() + "\n"
	}
	output := result.Output
	if isCD && result.ExitCode == 0 {
		physical, err := sanitizeCodeStudioPath(strings.TrimSuffix(strings.TrimSuffix(output, "\n"), "\r"))
		if err != nil || physical == "" || strings.TrimSpace(output) == "" {
			return "directory is outside the workspace or unavailable\n"
		}
		session.cwd = physical
	}
	if output != "" && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	if result.ExitCode != 0 {
		output += fmt.Sprintf("exit %d\n", result.ExitCode)
	}
	return output
}

// Only a literal, single-argument cd changes the persistent session directory.
// Compound commands and shell expansions execute normally in their own shell.
func codeStudioCDTarget(line string) (string, bool) {
	if line == "cd" {
		return codeStudioWorkspaceRoot, true
	}
	if !strings.HasPrefix(line, "cd ") && !strings.HasPrefix(line, "cd\t") {
		return "", false
	}
	arg := strings.TrimSpace(line[2:])
	if arg == "" || arg == "--" {
		return codeStudioWorkspaceRoot, true
	}
	if strings.HasPrefix(arg, "-- ") || strings.HasPrefix(arg, "--\t") {
		arg = strings.TrimSpace(arg[2:])
	}
	if len(arg) >= 2 && arg[0] == '\'' && arg[len(arg)-1] == '\'' && !strings.Contains(arg[1:len(arg)-1], "'") {
		return arg[1 : len(arg)-1], true
	}
	if len(arg) >= 2 && arg[0] == '"' && arg[len(arg)-1] == '"' && !strings.ContainsAny(arg[1:len(arg)-1], "\"$`\\") {
		return arg[1 : len(arg)-1], true
	}
	if strings.ContainsAny(arg, " \t\r\n'\"\\;&|<>$`(){}[]*?!#") || strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "~") {
		return "", false
	}
	return arg, true
}

type codeStudioTerminalSession struct {
	cwd     string
	line    []byte
	pending string
	skipLF  bool
}

func newCodeStudioTerminalSession(cwd string) *codeStudioTerminalSession {
	path, err := sanitizeCodeStudioPath(cwd)
	if err != nil {
		path = codeStudioWorkspaceRoot
	}
	return &codeStudioTerminalSession{cwd: path}
}

func (s *codeStudioTerminalSession) prompt() string { return s.cwd + " $ " }

func (s *codeStudioTerminalSession) consume(input string) (string, string, bool, error) {
	if len(s.pending)+len(s.line)+len(input) > codeStudioTerminalInputLimit {
		return "", "", false, fmt.Errorf("terminal input exceeds %d bytes", codeStudioTerminalInputLimit)
	}
	s.pending += input
	var output strings.Builder
	for len(s.pending) > 0 {
		if !utf8.FullRuneInString(s.pending) {
			break
		}
		r, size := utf8.DecodeRuneInString(s.pending)
		s.pending = s.pending[size:]
		if s.skipLF {
			s.skipLF = false
			if r == '\n' {
				continue
			}
		}
		switch r {
		case '\r', '\n':
			s.skipLF = r == '\r'
			line := strings.TrimSpace(string(s.line))
			s.line = s.line[:0]
			output.WriteString("\r\n")
			return line, output.String(), true, nil
		case '\b', 0x7f:
			if len(s.line) > 0 {
				_, size := utf8.DecodeLastRune(s.line)
				s.line = s.line[:len(s.line)-size]
				output.WriteString("\b \b")
			}
		case 3:
			s.line = s.line[:0]
			output.WriteString("^C\r\n" + s.prompt())
		default:
			s.line = utf8.AppendRune(s.line, r)
			output.WriteRune(r)
		}
	}
	return "", output.String(), false, nil
}

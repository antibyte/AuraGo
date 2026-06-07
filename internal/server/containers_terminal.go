package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

type containerTerminalBackend interface {
	ContainerRunning(ctx context.Context, cfg tools.DockerConfig, containerID string) (bool, error)
	CreateSession(ctx context.Context, cfg tools.DockerConfig, containerID string, cols, rows int, execCmd []string) (containerTerminalSession, error)
}

var storeTerminalCommandPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func defaultContainerTerminalCommand() []string {
	return []string{"/bin/sh"}
}

func storeTerminalExecCommand(metadata map[string]string) []string {
	cmd := strings.TrimSpace(metadata["terminal_command"])
	if cmd == "" || !storeTerminalCommandPattern.MatchString(cmd) {
		return defaultContainerTerminalCommand()
	}
	return []string{"/bin/bash", "-lc", "exec " + cmd}
}

type containerTerminalSession interface {
	io.ReadWriteCloser
	Resize(ctx context.Context, cols, rows int) error
}

var activeContainerTerminalBackend containerTerminalBackend = dockerContainerTerminalBackend{}

var containerTerminalUpgrader = websocket.Upgrader{
	CheckOrigin: sameOriginOrNoOrigin,
}

func handleContainerTerminal(s *Server, cfg tools.DockerConfig, containerID string, w http.ResponseWriter, r *http.Request) {
	if !sameOriginOrNoOrigin(r) {
		containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "forbidden websocket origin"})
		return
	}

	running, err := activeContainerTerminalBackend.ContainerRunning(r.Context(), cfg, containerID)
	if err != nil {
		containerJSON(w, http.StatusBadGateway, map[string]string{"status": "error", "message": err.Error()})
		return
	}
	if !running {
		containerJSON(w, http.StatusConflict, map[string]string{"status": "error", "message": "Container is not running"})
		return
	}

	session, err := activeContainerTerminalBackend.CreateSession(r.Context(), cfg, containerID, 120, 30, nil)
	if err != nil {
		containerJSON(w, http.StatusBadGateway, map[string]string{"status": "error", "message": err.Error()})
		return
	}

	conn, err := containerTerminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = session.Close()
		return
	}
	defer conn.Close()
	serveContainerTerminalSession(r.Context(), conn, session)
}

func serveContainerTerminalSession(ctx context.Context, conn *websocket.Conn, session containerTerminalSession) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer session.Close()

	var writeMu sync.Mutex
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := session.Read(buf)
			if n > 0 {
				writeMu.Lock()
				writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				writeMu.Unlock()
				if writeErr != nil {
					_ = conn.Close()
					return
				}
			}
			if err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch messageType {
		case websocket.BinaryMessage:
			if len(payload) > 0 {
				_, _ = session.Write(payload)
			}
		case websocket.TextMessage:
			if resize, ok := parseContainerTerminalResize(payload); ok {
				_ = session.Resize(ctx, resize.Cols, resize.Rows)
			}
		}
	}
}

type containerTerminalResize struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

func parseContainerTerminalResize(payload []byte) (containerTerminalResize, bool) {
	var msg containerTerminalResize
	if err := json.Unmarshal(payload, &msg); err != nil {
		return containerTerminalResize{}, false
	}
	if msg.Type != "resize" || msg.Cols <= 0 || msg.Rows <= 0 {
		return containerTerminalResize{}, false
	}
	return msg, true
}

type dockerContainerTerminalBackend struct{}

func (dockerContainerTerminalBackend) ContainerRunning(ctx context.Context, cfg tools.DockerConfig, containerID string) (bool, error) {
	raw := tools.DockerInspectContainer(cfg, containerID)
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		State   struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		} `json:"state"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return false, fmt.Errorf("parse docker inspect response: %w", err)
	}
	if resp.Status != "ok" {
		return false, errors.New(strings.TrimSpace(resp.Message))
	}
	return resp.State.Running || strings.EqualFold(resp.State.Status, "running"), nil
}

func (dockerContainerTerminalBackend) CreateSession(ctx context.Context, cfg tools.DockerConfig, containerID string, cols, rows int, execCmd []string) (containerTerminalSession, error) {
	cmd := execCmd
	if len(cmd) == 0 {
		cmd = defaultContainerTerminalCommand()
	}
	payload := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
		"Env":          []string{"TERM=xterm-256color"},
		"Tty":          true,
	}
	body, _ := json.Marshal(payload)
	data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return nil, err
	}
	if code != http.StatusCreated {
		return nil, fmt.Errorf("docker terminal create failed: %s", strings.TrimSpace(string(data)))
	}

	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return nil, fmt.Errorf("parse docker terminal exec id: %w", err)
	}

	stream, err := openDockerExecStartStream(ctx, cfg, created.ID)
	if err != nil {
		return nil, err
	}

	session := &dockerContainerTerminalSession{cfg: cfg, execID: created.ID, stream: stream}
	_ = session.Resize(ctx, cols, rows)
	return session, nil
}

type dockerContainerTerminalSession struct {
	cfg    tools.DockerConfig
	execID string
	stream io.ReadWriteCloser
}

func (s *dockerContainerTerminalSession) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *dockerContainerTerminalSession) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *dockerContainerTerminalSession) Close() error {
	return s.stream.Close()
}

func (s *dockerContainerTerminalSession) Resize(ctx context.Context, cols, rows int) error {
	cols = clampTerminalDimension(cols, 120, 2, 500)
	rows = clampTerminalDimension(rows, 30, 1, 200)
	endpoint := fmt.Sprintf("/exec/%s/resize?h=%d&w=%d", url.PathEscape(s.execID), rows, cols)
	data, code, err := tools.DockerRequestContext(ctx, s.cfg, http.MethodPost, endpoint, "")
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusNoContent {
		return fmt.Errorf("docker terminal resize failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

func clampTerminalDimension(value, fallback, min, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func openDockerExecStartStream(ctx context.Context, cfg tools.DockerConfig, execID string) (io.ReadWriteCloser, error) {
	body := []byte(`{"Detach":false,"Tty":true}`)
	conn, err := dockerutil.DialContext(ctx, cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("connect to Docker engine: %w", err)
	}

	endpoint := "/exec/" + url.PathEscape(execID) + "/start"
	reqURL := "http://localhost/" + dockerutil.APIVersion + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("build docker exec start request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	req.ContentLength = int64(len(body))

	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send docker exec start request: %w", err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read docker exec start response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSwitchingProtocols {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("docker exec start failed: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return &bufferedDockerRawConn{Conn: conn, reader: reader}, nil
}

type bufferedDockerRawConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedDockerRawConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *bufferedDockerRawConn) Close() error {
	return c.Conn.Close()
}

func (c *bufferedDockerRawConn) Write(p []byte) (int, error) {
	return c.Conn.Write(p)
}

var _ io.ReadWriteCloser = (*bufferedDockerRawConn)(nil)

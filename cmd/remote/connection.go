package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/remote"
	"github.com/gorilla/websocket"
)

func (c *Client) configSnapshot() clientConfig {
	c.cfgMu.RLock()
	defer c.cfgMu.RUnlock()
	return c.cfg
}

func (c *Client) sendForConn(ctx context.Context, conn *websocket.Conn, msg *remote.RemoteMessage) error {
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	for !c.writeMu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	defer c.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.currentConn() != conn {
		return fmt.Errorf("connection was replaced")
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetWriteDeadline(deadline)
	if err := conn.WriteJSON(msg); err != nil {
		_ = conn.Close()
		return err
	}
	return nil
}

func (c *Client) commandContext() context.Context {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.policyCtx == nil {
		c.policyCtx, c.cancelPolicy = context.WithCancel(context.Background())
	}
	policyCtx := c.policyCtx
	return fileutil.WithPublicationGate(policyCtx, func(commit func() error) error {
		c.stateMu.RLock()
		defer c.stateMu.RUnlock()
		if err := policyCtx.Err(); err != nil {
			return err
		}
		return commit()
	})
}

func (c *Client) startCommand(msg remote.RemoteMessage) {
	var cmd remote.CommandPayload
	if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
		return
	}
	if cmd.CommandID == "" {
		cmd.CommandID = msg.MessageID
	}
	conn := c.currentConn()
	cfg := c.configSnapshot()
	seconds := cmd.TimeoutSec
	if seconds <= 0 {
		seconds = 60
	}
	ctx, cancel := context.WithTimeout(c.commandContext(), time.Duration(seconds)*time.Second)
	c.stateMu.RLock()
	ro := c.readOnly
	paths := append([]string{}, c.allowedPaths...)
	c.stateMu.RUnlock()
	c.executor.Start(ctx, cmd, ro, paths, func(result remote.ResultPayload) {
		defer cancel()
		frame, err := remote.NewMessage(remote.MsgResult, cfg.DeviceID, cfg.SharedKey, c.nextSeq(), result)
		if err == nil {
			_ = c.sendForConn(ctx, conn, frame)
		}
	})
}

func (c *Client) acknowledge(conn *websocket.Conn, ref string, ok bool) {
	cfg := c.configSnapshot()
	msg, err := remote.NewMessage(remote.MsgAck, cfg.DeviceID, cfg.SharedKey, c.nextSeq(), remote.AckPayload{RefID: ref, Success: ok})
	if err == nil {
		_ = c.sendForConn(context.Background(), conn, msg)
	}
}

package discord

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestDiscordGenerationStopsRunningAndQueuedWorkers(t *testing.T) {
	g := newDiscordGeneration(nil)
	slots := newDiscordWorkerSlots(1)
	slots <- struct{}{}
	workerDone := make(chan struct{})
	if !g.begin() {
		t.Fatal("new generation refused worker")
	}
	go func() {
		defer g.workers.Done()
		defer close(workerDone)
		runDiscordMessage(g.ctx, slots, 1, nil, "channel", nil, false, func() { t.Error("queued turn ran") })
	}()
	sessionMu.Lock()
	activeGeneration = g
	session = nil
	sessionMu.Unlock()
	StopBot(nil)
	select {
	case <-workerDone:
	default:
		t.Fatal("StopBot returned before worker drained")
	}
	if g.begin() {
		g.workers.Done()
		t.Fatal("stopped generation accepted another worker")
	}
	for _, control := range []bool{false, true} {
		if runDiscordMessage(g.ctx, nil, 0, nil, "channel", nil, control, func() { t.Error("cancelled turn ran") }) {
			t.Fatal("cancelled generation admitted turn")
		}
	}
	next := newDiscordGeneration(nil)
	defer next.stop()
	if !next.begin() {
		t.Fatal("replacement generation cannot start")
	}
	next.workers.Done()
	if g.ctx.Err() != context.Canceled {
		t.Fatal("replacement revived old generation")
	}
}

func TestDiscordReconnectBackoffStopsWithGeneration(t *testing.T) {
	g := newDiscordGeneration(nil)
	defer g.stop()
	called := make(chan struct{})
	var calls atomic.Int32
	g.startReconnect(func() error {
		if calls.Add(1) == 1 {
			close(called)
		}
		return errors.New("fixture disconnect")
	}, nil)
	g.requestReconnect()
	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not run")
	}
	g.stop()
	g.requestReconnect()
	if calls.Load() != 1 {
		t.Fatal("stopped generation kept reconnecting")
	}
}

func TestDiscordGenerationCancelsGatewayHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	s, err := discordgo.New("Bot fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	g := newDiscordGeneration(nil)
	defer g.stop()
	bindDiscordGateway(s, g)
	if !g.begin() {
		t.Fatal("handshake admission failed")
	}
	done := make(chan error, 1)
	go func() {
		defer g.workers.Done()
		conn, _, err := s.Dialer.Dial("ws://"+listener.Addr().String(), nil)
		if conn != nil {
			conn.Close()
		}
		done <- err
	}()
	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(time.Second):
		t.Fatal("handshake did not connect")
	}
	g.stop()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled handshake succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("handshake outlived generation")
	}
}

func TestDiscordStopInvalidatesPreparedStart(t *testing.T) {
	epoch := stopDiscordGeneration(nil)
	candidate := newDiscordGeneration(nil)
	defer candidate.stop()
	StopBot(nil)
	if candidate.install(epoch) {
		t.Fatal("start candidate survived a later stop")
	}
	epoch = stopDiscordGeneration(nil)
	if !candidate.install(epoch) {
		t.Fatal("current start candidate was rejected")
	}
	StopBot(nil)
	if candidate.ctx.Err() != context.Canceled {
		t.Fatal("installed generation was not cancelled")
	}
}

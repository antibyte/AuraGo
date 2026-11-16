package discord

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var startMu sync.Mutex
var stopMu sync.Mutex
var generationEpoch uint64              // guarded by sessionMu
var activeGeneration *discordGeneration // guarded by sessionMu

type discordGeneration struct {
	ctx       context.Context
	cancel    context.CancelFunc
	session   *discordgo.Session
	mu        sync.Mutex
	stopped   bool
	workers   sync.WaitGroup
	stopOnce  sync.Once
	reconnect chan struct{}
}

func newDiscordGeneration(s *discordgo.Session) *discordGeneration {
	ctx, cancel := context.WithCancel(context.Background())
	return &discordGeneration{ctx: ctx, cancel: cancel, session: s, reconnect: make(chan struct{}, 1)}
}

// A stop invalidates candidates still being prepared by StartBot.
func (g *discordGeneration) install(epoch uint64) bool {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if generationEpoch != epoch {
		return false
	}
	activeGeneration = g
	return true
}

func (g *discordGeneration) requestReconnect() {
	if g.ctx.Err() != nil {
		return
	}
	select {
	case g.reconnect <- struct{}{}:
	default:
	}
}

// The SDK reconnect loop sleeps without a context; keep reconnects in the generation.
func (g *discordGeneration) startReconnect(open func() error, logger *slog.Logger) {
	if !g.begin() {
		return
	}
	go func() {
		defer g.workers.Done()
		for {
			select {
			case <-g.ctx.Done():
				return
			case <-g.reconnect:
			}
			for delay := time.Second; ; delay = min(2*delay, 10*time.Minute) {
				timer := time.NewTimer(delay)
				select {
				case <-g.ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				if g.ctx.Err() != nil {
					return
				}
				err := open()
				if g.ctx.Err() != nil {
					return
				}
				if err == nil || err == discordgo.ErrWSAlreadyOpen {
					break
				}
				if logger != nil {
					logger.Warn("[Discord] Gateway reconnect failed; retry scheduled")
				}
			}
		}
	}()
}

func (g *discordGeneration) begin() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped {
		return false
	}
	g.workers.Add(1)
	return true
}

func (g *discordGeneration) stop() {
	g.stopOnce.Do(func() {
		g.mu.Lock()
		g.stopped = true
		g.cancel()
		g.mu.Unlock()
		if g.session != nil {
			_ = g.session.Close()
		}
		g.workers.Wait()
	})
}

// Track the entire SDK call, including callers queued behind its bucket mutex.
func discordRequestContext(s *discordgo.Session, parent context.Context) (context.Context, func(), error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, discordRequestBudget)
	if s != nil && s.Client != nil {
		if transport, ok := s.Client.Transport.(*discordTransport); ok {
			g := transport.generation
			if !g.begin() {
				cancel()
				return ctx, func() {}, context.Canceled
			}
			stop := context.AfterFunc(g.ctx, cancel)
			return ctx, func() { stop(); cancel(); g.workers.Done() }, nil
		}
	}
	return ctx, cancel, nil
}

type discordGatewayConn struct {
	net.Conn
	stop func() bool
}

func (c *discordGatewayConn) Close() error { c.stop(); return c.Conn.Close() }

func bindDiscordGateway(s *discordgo.Session, g *discordGeneration) {
	dialer := *s.Dialer
	dialer.HandshakeTimeout = 30 * time.Second
	dialer.NetDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		ctx, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(g.ctx, cancel)
		defer func() { stop(); cancel() }()
		conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &discordGatewayConn{Conn: conn, stop: context.AfterFunc(g.ctx, func() { _ = conn.Close() })}, nil
	}
	s.Dialer = &dialer
}

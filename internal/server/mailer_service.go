package server

import (
	"context"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/mailer"
)

func (s *Server) initMailer(ctx context.Context) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	service, err := mailer.NewService(mailer.ServiceOptions{
		DataDir:        cfg.Directories.DataDir,
		ResolveAdapter: s.resolveMailerAdapter,
		ListSources:    s.mailerSourceAccounts,
		SetAgentAccess: s.updateMailerAgentAccess,
		Policy: func() mailer.RuntimePolicy {
			current := s.ConfigSnapshot()
			if current == nil {
				return mailer.RuntimePolicy{}
			}
			return mailer.RuntimePolicy{Enabled: current.VirtualDesktop.Enabled, ReadOnly: current.VirtualDesktop.ReadOnly}
		},
		BeginOperation: func(parent context.Context) (context.Context, func(), error) {
			run, cancel, err := s.beginDesktopRun(parent)
			return run, cancel, err
		},
		OnChange: func(change mailer.Event) {
			broadcastDesktopEvent(s, s.DesktopHub, desktop.Event{Type: "mailer_changed", Payload: change, CreatedAt: time.Now().UTC()})
		},
		MaxAttachmentBytes:      25 << 20,
		MaxAttachmentTotalBytes: 512 << 20,
	})
	if err != nil {
		s.Logger.Error("Mailer initialization failed", "error", err)
		return
	}
	if err := service.Start(ctx); err != nil {
		_ = service.Close()
		s.Logger.Error("Mailer startup failed", "error", err)
		return
	}
	s.Mailer = service
}

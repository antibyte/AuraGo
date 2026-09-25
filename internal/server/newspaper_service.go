package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"aurago/internal/newspaper"
)

func (s *Server) initNewspaper() {
	if s.Cfg == nil || !s.Cfg.VirtualDesktop.Enabled {
		return
	}
	if s.AgentSkillManager != nil {
		if _, err := s.AgentSkillManager.RegisterBundledAgentSkillFor(context.Background(), "newspaper", "aurago-newspaper", []byte(newspaper.Skill)); err != nil {
			s.Logger.Error("Newspaper editorial skill verification failed", "error", err)
		} else {
			s.newspaperSkillReady = true
		}
	}
	path := "data/newspaper.db"
	if s.Cfg.SQLite.GameMakerPath != "" {
		path = filepath.Join(filepath.Dir(s.Cfg.SQLite.GameMakerPath), "newspaper.db")
	}
	service, err := newspaper.New(newspaper.Options{
		Path: path,
		Policy: func() newspaper.Policy {
			cfg := s.ConfigSnapshot()
			if cfg == nil {
				return newspaper.Policy{}
			}
			return newspaper.Policy{Enabled: cfg.VirtualDesktop.Enabled && cfg.Newspaper.Enabled, ReadOnly: cfg.VirtualDesktop.ReadOnly || cfg.Newspaper.ReadOnly, MaxMinutes: cfg.Newspaper.MaxMinutes, MaxEditions: cfg.Newspaper.MaxEditions, Email: cfg.Newspaper.AllowEmail, Telegram: cfg.Newspaper.AllowTelegram}
		},
		Research: func(ctx context.Context, p newspaper.Profile, cutoff time.Time, progress func(newspaper.Progress)) (newspaper.Draft, error) {
			return s.newspaperResearch(ctx, p, cutoff, progress)
		},
		Deliver: func(ctx context.Context, e newspaper.Edition, p newspaper.Profile, channel string) (string, error) {
			return s.newspaperDelivery(ctx, e, p, channel)
		},
		Destination: func(_ context.Context, p newspaper.Profile, channel string) (string, error) {
			cfg := s.ConfigSnapshot()
			if cfg == nil {
				return "", errors.New("configuration unavailable")
			}
			switch channel {
			case "email":
				return p.EmailAccountID + "\x00" + p.EmailTo, nil
			case "telegram":
				if cfg.Telegram.UserID == 0 {
					return "", errors.New("Telegram destination unavailable")
				}
				return fmt.Sprint(cfg.Telegram.UserID), nil
			default:
				return "", errors.New("unsupported delivery channel")
			}
		},
	})
	if err != nil {
		s.Logger.Error("Newspaper initialization failed", "error", err)
		return
	}
	s.Newspaper = service
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/discord"
	"aurago/internal/rocketchat"
	"aurago/internal/security"
	"aurago/internal/telegram"
	"aurago/internal/tools"
)

type integrationTestSpec struct {
	name     string
	enabled  func(*config.Config) bool
	validate func(*config.Config) error
	check    func(context.Context, *config.Config) error
}

func writeIntegrationTestResult(w http.ResponseWriter, status int, state, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  state,
		"message": message,
	})
}

func handleConfiguredIntegrationTest(s *Server, spec integrationTestSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cfg := s.ConfigSnapshot()
		if cfg == nil {
			writeIntegrationTestResult(w, http.StatusInternalServerError, "error", "Configuration is unavailable")
			return
		}
		if spec.enabled != nil && !spec.enabled(cfg) {
			writeIntegrationTestResult(w, http.StatusBadRequest, "error", spec.name+" integration is disabled")
			return
		}
		if spec.validate != nil {
			if err := spec.validate(cfg); err != nil {
				writeIntegrationTestResult(w, http.StatusBadRequest, "error", security.Scrub(err.Error()))
				return
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := spec.check(ctx, cfg); err != nil {
			safeError := security.Scrub(err.Error())
			if s.Logger != nil {
				s.Logger.Warn("Integration connection test failed", "integration", spec.name, "error", safeError)
			}
			writeIntegrationTestResult(w, http.StatusBadGateway, "error", fmt.Sprintf("%s connection test failed: %s", spec.name, safeError))
			return
		}

		writeIntegrationTestResult(w, http.StatusOK, "ok", spec.name+" connection successful")
	}
}

func requireIntegrationValue(value, message string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(message)
	}
	return nil
}

func handleTelegramConnectionTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name: "Telegram",
		validate: func(cfg *config.Config) error {
			return requireIntegrationValue(cfg.Telegram.BotToken, "Telegram bot token is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return telegram.CheckBotToken(ctx, cfg.Telegram.BotToken)
		},
	})
}

func handleDiscordTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "Discord",
		enabled: func(cfg *config.Config) bool { return cfg.Discord.Enabled },
		validate: func(cfg *config.Config) error {
			return requireIntegrationValue(cfg.Discord.BotToken, "Discord bot token is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return discord.CheckBotToken(ctx, cfg.Discord.BotToken)
		},
	})
}

func handleRocketChatTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "Rocket.Chat",
		enabled: func(cfg *config.Config) bool { return cfg.RocketChat.Enabled },
		validate: func(cfg *config.Config) error {
			if err := requireIntegrationValue(cfg.RocketChat.URL, "Rocket.Chat URL is not configured"); err != nil {
				return err
			}
			if err := requireIntegrationValue(cfg.RocketChat.UserID, "Rocket.Chat user ID is not configured"); err != nil {
				return err
			}
			return requireIntegrationValue(cfg.RocketChat.AuthToken, "Rocket.Chat auth token is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return rocketchat.CheckConnection(ctx, cfg)
		},
	})
}

func handleHomeAssistantTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "Home Assistant",
		enabled: func(cfg *config.Config) bool { return cfg.HomeAssistant.Enabled },
		validate: func(cfg *config.Config) error {
			if err := requireIntegrationValue(cfg.HomeAssistant.URL, "Home Assistant URL is not configured"); err != nil {
				return err
			}
			return requireIntegrationValue(cfg.HomeAssistant.AccessToken, "Home Assistant access token is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return tools.CheckHomeAssistantConnection(ctx, tools.HAConfig{
				URL:         cfg.HomeAssistant.URL,
				AccessToken: cfg.HomeAssistant.AccessToken,
				ReadOnly:    cfg.HomeAssistant.ReadOnly,
			})
		},
	})
}

func handleProxmoxTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "Proxmox",
		enabled: func(cfg *config.Config) bool { return cfg.Proxmox.Enabled },
		validate: func(cfg *config.Config) error {
			if err := requireIntegrationValue(cfg.Proxmox.URL, "Proxmox URL is not configured"); err != nil {
				return err
			}
			if err := requireIntegrationValue(cfg.Proxmox.TokenID, "Proxmox token ID is not configured"); err != nil {
				return err
			}
			return requireIntegrationValue(cfg.Proxmox.Secret, "Proxmox API token secret is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return tools.CheckProxmoxConnection(ctx, tools.ProxmoxConfig{
				URL:              cfg.Proxmox.URL,
				TokenID:          cfg.Proxmox.TokenID,
				Secret:           cfg.Proxmox.Secret,
				Insecure:         cfg.Proxmox.Insecure,
				ReadOnly:         cfg.Proxmox.ReadOnly,
				AllowDestructive: cfg.Proxmox.AllowDestructive,
			})
		},
	})
}

func handleS3Test(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "S3",
		enabled: func(cfg *config.Config) bool { return cfg.S3.Enabled },
		validate: func(cfg *config.Config) error {
			if err := requireIntegrationValue(cfg.S3.Bucket, "S3 bucket is not configured"); err != nil {
				return err
			}
			if err := requireIntegrationValue(cfg.S3.AccessKey, "S3 access key is not configured"); err != nil {
				return err
			}
			return requireIntegrationValue(cfg.S3.SecretKey, "S3 secret key is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return tools.CheckS3Connection(ctx, tools.S3Config{
				Endpoint:     cfg.S3.Endpoint,
				Region:       cfg.S3.Region,
				Bucket:       cfg.S3.Bucket,
				AccessKey:    cfg.S3.AccessKey,
				SecretKey:    cfg.S3.SecretKey,
				UsePathStyle: cfg.S3.UsePathStyle,
				Insecure:     cfg.S3.Insecure,
				ReadOnly:     cfg.S3.ReadOnly,
			})
		},
	})
}

func handleAnsibleTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "Ansible sidecar",
		enabled: func(cfg *config.Config) bool { return cfg.Ansible.Enabled },
		validate: func(cfg *config.Config) error {
			if strings.EqualFold(strings.TrimSpace(cfg.Ansible.Mode), "local") {
				return errors.New("Ansible sidecar test requires sidecar mode")
			}
			if err := requireIntegrationValue(cfg.Ansible.URL, "Ansible sidecar URL is not configured"); err != nil {
				return err
			}
			return requireIntegrationValue(cfg.Ansible.Token, "Ansible sidecar token is not configured")
		},
		check: func(ctx context.Context, cfg *config.Config) error {
			return tools.CheckAnsibleConnection(ctx, tools.AnsibleConfig{
				URL:     cfg.Ansible.URL,
				Token:   cfg.Ansible.Token,
				Timeout: cfg.Ansible.Timeout,
			})
		},
	})
}

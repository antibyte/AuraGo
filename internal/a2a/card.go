package a2a

import (
	"fmt"

	"aurago/internal/config"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// BuildAgentCard constructs an a2a.AgentCard from the application config.
func BuildAgentCard(cfg *config.Config) *a2a.AgentCard {
	serverCfg := &cfg.A2A.Server

	card := &a2a.AgentCard{
		Name:               serverCfg.AgentName,
		Description:        serverCfg.AgentDescription,
		Version:            serverCfg.AgentVersion,
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities: a2a.AgentCapabilities{
			Streaming:         serverCfg.Streaming,
			PushNotifications: serverCfg.PushNotifications,
		},
	}

	// Build skills from config
	for _, s := range serverCfg.Skills {
		card.Skills = append(card.Skills, a2a.AgentSkill{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
		})
	}

	// Default skill if none configured
	if len(card.Skills) == 0 {
		card.Skills = []a2a.AgentSkill{
			{
				ID:          "general",
				Name:        "General Assistant",
				Description: "AuraGo AI agent for home lab management, coding, automation, and general tasks",
				Tags:        []string{"general", "homelab", "automation"},
			},
		}
	}

	// Build supported interfaces
	baseURL := resolveBaseURL(cfg)
	basePath := serverCfg.BasePath

	if serverCfg.Bindings.REST {
		card.SupportedInterfaces = append(card.SupportedInterfaces,
			a2a.NewAgentInterface(
				fmt.Sprintf("%s%s", baseURL, basePath),
				a2a.TransportProtocolHTTPJSON,
			),
		)
	}
	if serverCfg.Bindings.JSONRPC {
		card.SupportedInterfaces = append(card.SupportedInterfaces,
			a2a.NewAgentInterface(
				fmt.Sprintf("%s%s/jsonrpc", baseURL, basePath),
				a2a.TransportProtocolJSONRPC,
			),
		)
	}
	if serverCfg.Bindings.GRPC {
		grpcPort := serverCfg.Bindings.GRPCPort
		card.SupportedInterfaces = append(card.SupportedInterfaces,
			a2a.NewAgentInterface(
				fmt.Sprintf("%s:%d", resolveHost(cfg), grpcPort),
				a2a.TransportProtocolGRPC,
			),
		)
	}

	// Security schemes
	authCfg := &cfg.A2A.Auth
	if authCfg.APIKeyEnabled {
		card.SecuritySchemes = a2a.NamedSecuritySchemes{
			"apiKey": a2a.APIKeySecurityScheme{
				Location: a2a.APIKeySecuritySchemeLocationHeader,
				Name:     "X-API-Key",
			},
		}
		card.SecurityRequirements = a2a.SecurityRequirementsOptions{
			{"apiKey": []string{}},
		}
	}
	if authCfg.BearerEnabled {
		if card.SecuritySchemes == nil {
			card.SecuritySchemes = a2a.NamedSecuritySchemes{}
		}
		card.SecuritySchemes["bearer"] = a2a.HTTPAuthSecurityScheme{
			Scheme: "Bearer",
		}
		card.SecurityRequirements = append(card.SecurityRequirements,
			a2a.SecurityRequirements{"bearer": []string{}},
		)
	}

	return card
}

// resolveBaseURL determines the public URL for the A2A server.
func resolveBaseURL(cfg *config.Config) string {
	if cfg.A2A.Server.AgentURL != "" {
		return cfg.A2A.Server.AgentURL
	}
	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	port := cfg.A2A.Server.Port
	if port == 0 {
		port = cfg.Server.Port
	}
	scheme := "http"
	if cfg.Server.HTTPS.Enabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

// resolveHost returns the host for non-HTTP transports (gRPC).
func resolveHost(cfg *config.Config) string {
	if cfg.A2A.Server.AgentURL != "" {
		return cfg.A2A.Server.AgentURL
	}
	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	return host
}

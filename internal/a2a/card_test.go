package a2a

import (
	"testing"

	"aurago/internal/config"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.A2A.Server.Enabled = true
	cfg.A2A.Server.AgentName = "TestAgent"
	cfg.A2A.Server.AgentDescription = "A test agent"
	cfg.A2A.Server.AgentVersion = "1.0.0"
	cfg.A2A.Server.BasePath = "/a2a"
	cfg.A2A.Server.Bindings.REST = true
	cfg.A2A.Server.Bindings.JSONRPC = true
	cfg.A2A.Server.Bindings.GRPC = false
	cfg.Server.Host = "localhost"
	cfg.Server.Port = 8080
	return cfg
}

func TestBuildAgentCard_Basic(t *testing.T) {
	cfg := testConfig()
	card := BuildAgentCard(cfg)

	if card.Name != "TestAgent" {
		t.Errorf("expected name TestAgent, got %s", card.Name)
	}
	if card.Description != "A test agent" {
		t.Errorf("expected description, got %s", card.Description)
	}
	if card.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", card.Version)
	}
}

func TestBuildAgentCard_DefaultSkill(t *testing.T) {
	cfg := testConfig()
	card := BuildAgentCard(cfg)

	if len(card.Skills) != 1 {
		t.Fatalf("expected 1 default skill, got %d", len(card.Skills))
	}
	if card.Skills[0].ID != "general" {
		t.Errorf("expected skill ID 'general', got %s", card.Skills[0].ID)
	}
}

func TestBuildAgentCard_CustomSkills(t *testing.T) {
	cfg := testConfig()
	cfg.A2A.Server.Skills = []config.A2ASkill{
		{ID: "coding", Name: "Coding Assistant", Description: "Helps with code", Tags: []string{"code"}},
		{ID: "infra", Name: "Infra Manager", Description: "Manages infra", Tags: []string{"infra"}},
	}

	card := BuildAgentCard(cfg)

	if len(card.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(card.Skills))
	}
	if card.Skills[0].ID != "coding" {
		t.Errorf("expected first skill 'coding', got %s", card.Skills[0].ID)
	}
}

func TestBuildAgentCard_Bindings(t *testing.T) {
	cfg := testConfig()
	card := BuildAgentCard(cfg)

	// REST + JSON-RPC enabled, gRPC disabled
	if len(card.SupportedInterfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(card.SupportedInterfaces))
	}
}

func TestBuildAgentCard_APIKeySecurity(t *testing.T) {
	cfg := testConfig()
	cfg.A2A.Auth.APIKeyEnabled = true

	card := BuildAgentCard(cfg)

	if card.SecuritySchemes == nil {
		t.Fatal("expected security schemes to be set")
	}
	scheme, ok := card.SecuritySchemes["apiKey"]
	if !ok {
		t.Fatal("expected 'apiKey' in security schemes")
	}
	if _, isAPIKey := scheme.(a2a.APIKeySecurityScheme); !isAPIKey {
		t.Errorf("expected APIKeySecurityScheme, got %T", scheme)
	}
}

func TestBuildAgentCard_BearerSecurity(t *testing.T) {
	cfg := testConfig()
	cfg.A2A.Auth.BearerEnabled = true

	card := BuildAgentCard(cfg)

	if card.SecuritySchemes == nil {
		t.Fatal("expected security schemes to be set")
	}
	scheme, ok := card.SecuritySchemes["bearer"]
	if !ok {
		t.Fatal("expected 'bearer' in security schemes")
	}
	if httpScheme, ok := scheme.(a2a.HTTPAuthSecurityScheme); ok {
		if httpScheme.Scheme != "Bearer" {
			t.Errorf("expected scheme 'Bearer', got %s", httpScheme.Scheme)
		}
	} else {
		t.Errorf("expected HTTPAuthSecurityScheme, got %T", scheme)
	}
}

func TestBuildAgentCard_BothSecuritySchemes(t *testing.T) {
	cfg := testConfig()
	cfg.A2A.Auth.APIKeyEnabled = true
	cfg.A2A.Auth.BearerEnabled = true

	card := BuildAgentCard(cfg)

	if len(card.SecuritySchemes) != 2 {
		t.Errorf("expected 2 security schemes, got %d", len(card.SecuritySchemes))
	}
	if len(card.SecurityRequirements) != 2 {
		t.Errorf("expected 2 security requirements, got %d", len(card.SecurityRequirements))
	}
}

func TestResolveBaseURL(t *testing.T) {
	cfg := testConfig()

	// No override — should build from server host/port
	url := resolveBaseURL(cfg)
	if url != "http://localhost:8080" {
		t.Errorf("expected http://localhost:8080, got %s", url)
	}

	// With explicit agent URL
	cfg.A2A.Server.AgentURL = "https://agent.example.com"
	url = resolveBaseURL(cfg)
	if url != "https://agent.example.com" {
		t.Errorf("expected https://agent.example.com, got %s", url)
	}
}

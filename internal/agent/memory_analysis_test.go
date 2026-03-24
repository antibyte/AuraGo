package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"aurago/internal/config"
)

func newTestConfig(enabled, queryExpansion, llmReranking bool) *config.Config {
	cfg := &config.Config{}
	cfg.MemoryAnalysis.Enabled = enabled
	cfg.MemoryAnalysis.QueryExpansion = queryExpansion
	cfg.MemoryAnalysis.LLMReranking = llmReranking
	return cfg
}

func TestExpandQueryForRAG_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	tests := []struct {
		name string
		cfg  *config.Config
		msg  string
		want string
	}{
		{
			name: "disabled when MemoryAnalysis not enabled",
			cfg:  newTestConfig(false, true, false),
			msg:  "this is a long enough user message for expansion",
			want: "this is a long enough user message for expansion",
		},
		{
			name: "disabled when QueryExpansion is false",
			cfg:  newTestConfig(true, false, false),
			msg:  "this is a long enough user message for expansion",
			want: "this is a long enough user message for expansion",
		},
		{
			name: "short message bypassed",
			cfg:  newTestConfig(true, true, false),
			msg:  "short msg",
			want: "short msg",
		},
		{
			name: "empty message bypassed",
			cfg:  newTestConfig(true, true, false),
			msg:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandQueryForRAG(ctx, tt.cfg, logger, tt.msg)
			if got != tt.want {
				t.Errorf("expandQueryForRAG() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRerankWithLLM_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	candidates := []rankedMemory{
		{text: "memory 1", docID: "1", score: 0.9},
		{text: "memory 2", docID: "2", score: 0.8},
	}

	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "disabled when MemoryAnalysis not enabled",
			cfg:  newTestConfig(false, false, true),
		},
		{
			name: "disabled when LLMReranking is false",
			cfg:  newTestConfig(true, false, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rerankWithLLM(ctx, tt.cfg, logger, candidates, "test query")
			if len(got) != len(candidates) {
				t.Errorf("rerankWithLLM() returned %d items, want %d", len(got), len(candidates))
			}
			for i, c := range got {
				if c.score != candidates[i].score {
					t.Errorf("candidate[%d].score = %v, want %v (unchanged)", i, c.score, candidates[i].score)
				}
			}
		})
	}
}

func TestRerankWithLLM_EmptyCandidates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()
	cfg := newTestConfig(true, false, true)

	got := rerankWithLLM(ctx, cfg, logger, nil, "test query")
	if len(got) != 0 {
		t.Errorf("rerankWithLLM() with nil candidates returned %d items, want 0", len(got))
	}

	got = rerankWithLLM(ctx, cfg, logger, []rankedMemory{}, "test query")
	if len(got) != 0 {
		t.Errorf("rerankWithLLM() with empty candidates returned %d items, want 0", len(got))
	}
}

func TestIsAmbiguousShortCommand(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		// Exact match ambiguous commands
		{"yes", "ja", true},
		{"ok", "ok", true},
		{"continue", "weiter", true},
		{"thanks", "danke", true},
		{"yes_en", "yes", true},
		{"go_ahead", "go ahead", true},

		// Pattern match retry commands
		{"retry_de", "versuche es erneut", true},
		{"retry_en", "try again", true},
		{"nochmal", "nochmal", true},
		{"wiederholen", "wiederholen", true},
		{"repeat", "repeat", true},
		{"do_it_again", "do it again", true},
		{"mach_das_nochmal", "mach das nochmal", true},
		{"nochmals", "nochmals", true},
		{"teste_erneut", "teste erneut", true},
		{"teste_nochmal", "teste nochmal", true},
		{"test_again", "test again", true},

		// Non-ambiguous — specific enough for RAG
		{"specific_retry", "versuche die PDF-Erstellung erneut", false},
		{"specific_topic", "Wie funktioniert die FritzBox Integration?", false},
		{"long_message", "Erstelle mir bitte ein Python Script das die CPU Auslastung überwacht und bei hoher Last eine Warnung gibt", false},
		{"normal_question", "Was ist der aktuelle Status der Docker Container?", false},
		{"code_request", "Schreib mir eine Funktion die Fibonacci berechnet", false},

		// Edge cases
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"long_retry", "versuche es erneut aber diesmal mit den richtigen parametern und der neuen konfiguration", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAmbiguousShortCommand(tt.msg)
			if got != tt.want {
				t.Errorf("isAmbiguousShortCommand(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestShouldStoreExtractedMemory(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		category string
		want     bool
	}{
		{
			name:     "keeps useful operational detail",
			content:  "Homepage web server listens on port 8080.",
			category: "recent_operational_details",
			want:     true,
		},
		{
			name:     "drops unavailable integration claim english",
			content:  "VirusTotal integration is not available or not configured.",
			category: "recent_operational_details",
			want:     false,
		},
		{
			name:     "drops tool list claim german",
			content:  "Die Integration steht nicht zur Verfügung, da das Tool nicht in der Werkzeugliste auftaucht.",
			category: "recent_operational_details",
			want:     false,
		},
		{
			name:     "drops disabled tool statement",
			content:  "The web scraper tool is disabled.",
			category: "workflow",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldStoreExtractedMemory(tt.content, tt.category)
			if got != tt.want {
				t.Errorf("shouldStoreExtractedMemory(%q, %q) = %v, want %v", tt.content, tt.category, got, tt.want)
			}
		})
	}
}

func TestShouldUseRAGForMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "very short disabled",
			msg:  "teste erneut",
			want: false,
		},
		{
			name: "short even if not ambiguous disabled",
			msg:  "docker status",
			want: false,
		},
		{
			name: "exactly twenty chars enabled",
			msg:  "status aller container",
			want: true,
		},
		{
			name: "long ambiguous still disabled",
			msg:  "versuche es erneut bitte",
			want: false,
		},
		{
			name: "substantial query enabled",
			msg:  "Wie ist der Status der Docker Container?",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseRAGForMessage(tt.msg)
			if got != tt.want {
				t.Errorf("shouldUseRAGForMessage(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

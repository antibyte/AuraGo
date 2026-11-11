package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusLimitDefaultValidationAndReopen(t *testing.T) {
	for _, value := range []string{"", "1", "1024", "1048576", "0", "-1", "1048577", "1.5", "null", "\"1024\""} {
		t.Run(value, func(t *testing.T) {
			data := "embeddings:\n  provider: disabled\n"
			if value != "" {
				data += fmt.Sprintf("  corpus_max_size_mb: %s\n", value)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			valid := value == "" || value == "1" || value == "1024" || value == "1048576"
			if !valid {
				if err == nil {
					t.Fatal("invalid limit accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := 1024
			if value == "1" {
				want = 1
			}
			if value == "1048576" {
				want = 1048576
			}
			if cfg.Embeddings.CorpusMaxSizeMB != want {
				t.Fatalf("got %d want %d", cfg.Embeddings.CorpusMaxSizeMB, want)
			}
			raw, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			reopened, err := Load(path)
			if err != nil || reopened.Embeddings.CorpusMaxSizeMB != want {
				t.Fatalf("reopen: %v", err)
			}
		})
	}
}

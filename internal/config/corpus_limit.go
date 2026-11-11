package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

const DefaultCorpusMaxSizeMB = 1024
const MaximumCorpusMaxSizeMB = 1048576

// ValidateCorpusLimitYAML also rejects fractional values before yaml.v3 can truncate them.
func ValidateCorpusLimitYAML(data []byte) error {
	var document struct {
		Embeddings map[string]any `yaml:"embeddings"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("validate corpus limit: %w", err)
	}
	value, present := document.Embeddings["corpus_max_size_mb"]
	if !present {
		return nil
	}
	number, ok := value.(int)
	if !ok || number < 1 || number > MaximumCorpusMaxSizeMB {
		return fmt.Errorf("embeddings.corpus_max_size_mb must be an integer between 1 and 1048576 (MiB)")
	}
	return nil
}

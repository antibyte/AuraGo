package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// PreparedPromptProfile is trusted internal configuration, never model data.
// Its complete system text and ordered schemas are immutable after construction.
// Ordinary callers keep the dynamic prompt builder by leaving it nil.
type PreparedPromptProfile struct {
	name      string
	revision  string
	system    string
	toolsJSON []byte
}

func promptFingerprint(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func NewPreparedPromptProfile(name, system string, tools []openai.Tool) (*PreparedPromptProfile, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(system) == "" {
		return nil, fmt.Errorf("prepared prompt requires a name and complete instructions")
	}
	data, err := json.Marshal(tools)
	if err != nil {
		return nil, fmt.Errorf("prepared prompt schemas: %w", err)
	}
	profile := &PreparedPromptProfile{name: name, system: system, toolsJSON: data}
	profile.revision = promptFingerprint([]string{name, system, string(data)})
	return profile, nil
}

func (p *PreparedPromptProfile) Revision() string {
	if p == nil {
		return ""
	}
	return p.revision
}
func (p *PreparedPromptProfile) SystemPrompt() string { return p.system }
func (p *PreparedPromptProfile) Tools() []openai.Tool {
	var result []openai.Tool
	_ = json.Unmarshal(p.toolsJSON, &result)
	if result == nil {
		result = []openai.Tool{}
	}
	return result
}

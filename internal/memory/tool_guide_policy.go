package memory

import (
	"aurago/internal/promptsource"
	promptsembed "aurago/prompts"
	chromem "github.com/philippgille/chromem-go"
	"gopkg.in/yaml.v3"
	"io/fs"
	"path/filepath"
	"strings"
)

// knownBundledToolGuideDocument verifies full content against compiled source,
// including chunk headers. A caller-controlled path/name alone is never proof.
func knownBundledToolGuideDocument(doc chromem.Document) bool {
	name := doc.Metadata["tool_name"]
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return false
	}
	raw, err := fs.ReadFile(promptsembed.FS, "tools_manuals/"+name+".md")
	if err != nil {
		return false
	}
	frontmatter, body, _, err := promptsource.Split(string(raw))
	if err != nil {
		return false
	}
	var meta ToolGuideMeta
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
			return false
		}
	}
	content := buildToolGuideEmbeddingContent(meta.Description, body)
	for _, expected := range buildToolGuideDocuments(name, doc.Metadata["path"], content) {
		if expected.ID == doc.ID && expected.Content == doc.Content {
			return true
		}
	}
	return false
}

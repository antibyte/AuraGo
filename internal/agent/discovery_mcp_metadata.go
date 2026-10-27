package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"aurago/internal/security"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// mcpToolDescriptionLimit bounds the description of an MCP tool and
	// mcpSchemaTextLimit every prose string of its input schema. The server
	// writes them, they are searched, listed and sent to the model on each
	// refresh, and no honest tool needs more.
	mcpToolDescriptionLimit = 2048
	mcpSchemaTextLimit      = 512
	mcpTruncationMarker     = "… [truncated]"

	// mcpSuspiciousReason is the fixed reason of an MCP entry whose metadata
	// carries an injection signature. It names no text of the metadata.
	mcpSuspiciousReason = "suspicious_description"

	// mcpScanBudget bounds the text scanned for one tool. A scan costs about
	// 6 us per byte, so a server could stall a catalog build with a huge schema;
	// text that is not scanned must not reach the model, so a tool above the
	// budget is hidden like a hostile one. The capped prose of an honest tool
	// stays far below it.
	mcpScanBudget = 64 << 10

	// mcpMetadataCacheLimit bounds the vetted metadata kept between refreshes.
	mcpMetadataCacheLimit = 2048
)

// mcpMetadata is the model-facing metadata of one MCP tool after vetting.
type mcpMetadata struct {
	description string
	schema      map[string]interface{}
	suspicious  bool
}

type mcpMetadataKey struct {
	guardian *security.Guardian
	sum      [sha256.Size]byte
}

// mcpMetadataCache keeps the vetting result of a tool by the hash of what the
// server sent. The catalog is rebuilt on every refresh and each build would scan
// every leaf of every tool again; a changed description has a new hash and is
// scanned at once, so a server cannot change its text unseen. The result is
// shared by the catalogs that hold it and is never modified.
var mcpMetadataCache struct {
	mu     sync.Mutex
	vetted map[mcpMetadataKey]mcpMetadata
}

// mcpFallbackGuardian scans MCP metadata when a run has no Guardian.
var mcpFallbackGuardian = sync.OnceValue(func() *security.Guardian { return security.NewGuardian(nil) })

// vetMCPMetadata scans the description and every string of the input schema of
// an MCP tool (prose, values and property names) with the local Guardian scan
// and caps the prose. At ThreatHigh the result is suspicious. The server's maps
// are not modified: the schema of the result is a copy.
func vetMCPMetadata(guardian *security.Guardian, description string, inputSchema map[string]interface{}, logger *slog.Logger, server, tool string) mcpMetadata {
	if guardian == nil {
		guardian = mcpFallbackGuardian()
	}
	raw, err := json.Marshal(inputSchema)
	if err != nil {
		// A schema that cannot be encoded cannot be sent to a model either.
		inputSchema, raw = nil, []byte("null")
	}
	hash := sha256.New()
	hash.Write([]byte(description))
	hash.Write([]byte{0})
	hash.Write(raw)
	key := mcpMetadataKey{guardian: guardian}
	hash.Sum(key.sum[:0])

	mcpMetadataCache.mu.Lock()
	cached, ok := mcpMetadataCache.vetted[key]
	mcpMetadataCache.mu.Unlock()
	if ok {
		return cached
	}

	meta := mcpMetadata{description: truncateUTF8ToLimit(description, mcpToolDescriptionLimit, mcpTruncationMarker)}
	level, scanned, overBudget := security.ThreatNone, 0, false
	scan := func(text string) {
		if level >= security.ThreatHigh || overBudget || shortWordToken(text) {
			return
		}
		if scanned += len(text); scanned > mcpScanBudget {
			overBudget = true
			return
		}
		if result := guardian.ScanForInjectionLocal(text); result.Level > level {
			level = result.Level
		}
	}
	scan(meta.description)
	walk := schemaWalk{
		name: scan,
		leaf: func(text string, kind schemaLeafKind) string {
			if kind == schemaLeafText {
				text = truncateUTF8ToLimit(text, mcpSchemaTextLimit, mcpTruncationMarker)
			}
			scan(text)
			return text
		},
	}
	if plain, ok := decodePlainSchema(raw, false); ok && plain != nil {
		meta.schema, _ = walk.schema(plain).(map[string]interface{})
	}
	if meta.suspicious = level >= security.ThreatHigh || overBudget; meta.suspicious {
		meta.description, meta.schema = "", nil
		if logger != nil {
			logger.Warn("[Discovery] MCP tool metadata hidden", "server", server, "tool", tool, "threat", level.String(), "over_scan_budget", overBudget)
		}
	}

	mcpMetadataCache.mu.Lock()
	if mcpMetadataCache.vetted == nil || len(mcpMetadataCache.vetted) >= mcpMetadataCacheLimit {
		mcpMetadataCache.vetted = make(map[mcpMetadataKey]mcpMetadata)
	}
	mcpMetadataCache.vetted[key] = meta
	mcpMetadataCache.mu.Unlock()
	return meta
}

// shortWordToken reports whether a string is empty or a single short word of
// letters, digits, underscore, dot and hyphen (a type name, a property name, an
// enum member). A scan costs about 0.15 ms and such a string holds no phrase, no
// marker and, below 16 bytes, no encoded phrase, so a schema of ninety strings
// is scanned for its dozen sentences only.
func shortWordToken(text string) bool {
	if len(text) >= 16 {
		return false
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

// mcpCatalogEntry builds the catalog entry of an MCP tool. Suspicious metadata
// hides the entry for good: it is disabled with the fixed reason, so it can be
// neither discovered as callable nor activated, and neither the description nor
// the schema is kept, so get_tool_info and search cannot repeat them. The scan
// uses the Guardian of the run (a default one without it).
func mcpCatalogEntry(dc *DispatchContext, parent *ToolCatalogEntry, item tools.MCPToolInfo) *ToolCatalogEntry {
	var guardian *security.Guardian
	var logger *slog.Logger
	if dc != nil {
		guardian, logger = dc.Guardian, dc.Logger
	}
	name := fmt.Sprintf("mcp__%x__%x", item.Server, item.Name)
	meta := vetMCPMetadata(guardian, item.Description, item.InputSchema, logger, item.Server, item.Name)
	entry := &ToolCatalogEntry{
		Name: name, Kind: ToolKindMCP, Aliases: []string{item.Server + "/" + item.Name}, Category: "mcp",
		Description: meta.description, Enabled: true, Status: ToolStatusHidden, HiddenReason: "external_tool",
		ManualPath: parent.ManualPath,
		Routing:    ToolRouting{NativeAction: "mcp_call", MCPServer: item.Server, MCPTool: item.Name},
		Schema:     openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: name, Description: meta.description, Parameters: meta.schema}},
	}
	if meta.suspicious {
		entry.Enabled, entry.Active, entry.Status, entry.HiddenReason = false, false, ToolStatusDisabled, mcpSuspiciousReason
		entry.Description, entry.Schema = "", openai.Tool{}
	}
	return entry
}

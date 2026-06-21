# Knowledge Graph Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce Knowledge Graph noise from low-confidence co-mentions, path-shaped IDs, generic tokens, duplicate identities, and incomplete edge quality metadata while preserving existing manual and synced graph behavior.

**Architecture:** Add a small dependency-free KG quality policy package, then wire it into extraction, activity capture, graph persistence, query filtering, cleanup, stats, and agent-facing tool APIs. Existing public methods stay as compatibility wrappers; new option-bearing methods provide explicit override paths such as `include_low_confidence=true`.

**Tech Stack:** Go 1.26, SQLite via `modernc.org/sqlite`, vanilla JS dashboard, existing AuraGo config/tool/manual patterns.

---

## Report Review

Implement:
- Hide and clean low-confidence `co_mentioned_with` edges by default.
- Add deterministic hash IDs for real file/path nodes and keep original paths in properties.
- Filter generic tokens before they become nodes or co-mention edges.
- Ensure all edge writes carry quality metadata such as `source`, `confidence`, and `extracted_at`.
- Expose graph health and quality metrics through the existing KG quality/stats paths and `knowledge_graph` tool.
- Keep duplicate detection as suggestions only; do not auto-merge.

Do not implement:
- Raw file paths as node IDs. Current IDs intentionally match `^[a-z][a-z0-9_]*$`; raw paths would ripple through FTS, URLs, JSON, and tool calls. Use `file_<sha1-12>` plus `properties.path`.
- Automatic duplicate merges. The existing `merge_nodes` path is user/tool controlled and protected-node aware; suggestions are safer.
- Broad ontology enforcement in one pass. Add `file` and `tool` types now, keep larger schema hardening for a later migration.
- Query Memory fusion changes in this phase. Retrieval fusion already exists as a config concept; changing it belongs in a separate retrieval project.

## Impact Notes

- `normalizeExtractedKG` is HIGH risk: direct caller `ExtractKGFromText`; transitive callers include file sync and nightly maintenance. Implement with focused tests before changing behavior.
- Codebase-memory inbound traces mark `ExtractKGFromText`, `IncrementCoOccurrence`, and `GetNeighbors` as CRITICAL at hop 1 because they feed maintenance, activity capture, and agent/server paths. Treat these as API-sensitive changes.
- `CleanupStaleGraph`, `AddEdge`, and `Search` had LOW GitNexus impact in summary mode, but `CleanupStaleGraph` is called by nightly maintenance; keep compatibility wrappers.

## Files

- Create: `internal/kgquality/policy.go` - dependency-free quality predicates, file ID hashing, generic token filtering, low-confidence predicates, scoring helpers.
- Create: `internal/kgquality/policy_test.go` - unit tests for generic tokens, path hashing, low-confidence detection, and score ordering.
- Modify: `internal/kgextraction/kg_extraction.go` - update prompt, node type vocabulary, ID remapping, path normalization, generic filtering, edge defaults.
- Modify: `internal/kgextraction/kg_extraction_test.go` - tests for generic filtering, path hash remapping, and new allowed types.
- Modify: `internal/services/file_kg_sync.go` - anchor synced files as canonical file nodes with `properties.path`; avoid path-like duplicate nodes.
- Modify: `internal/services/file_kg_sync_test.go` - tests for file node IDs and path property preservation.
- Modify: `internal/agent/activity_capture.go` - filter noisy activity digest entities before node and co-occurrence creation.
- Modify: `internal/agent/activity_capture_test.go` - tests for generic entity filtering and co-occurrence suppression.
- Modify: `internal/memory/graph_sqlite.go` - quality policy storage/defaults and extended stats/report structs.
- Modify: `internal/memory/graph_edge.go` - edge metadata defaults and semantic index updates with final properties.
- Modify: `internal/memory/graph_backup.go` - bulk merge quality metadata preservation/defaulting.
- Modify: `internal/memory/graph_query.go` - query options, low-confidence filtering, cleanup options, stats/report metrics.
- Modify: `internal/memory/graph_query_duplicates.go` and `internal/memory/kgquery/duplicates.go` only if existing ID duplicate detection does not already cover `caddy_server`/`caddyserver`.
- Modify: `internal/memory/*_test.go` - focused tests for edge defaults, filtered search/neighbors, cleanup TTL, stats/report metrics, and duplicate suggestions.
- Modify: `internal/config/config_types.go`, `internal/config/config.go`, `config_template.yaml`, `internal/server/config_handlers_main.go` - config defaults and serialized config display.
- Modify: `cmd/aurago/main.go`, `cmd/lifeboat/main.go` - apply KG quality policy to constructed graph instances.
- Modify: `internal/agent/tool_args_execution.go`, `internal/agent/agent_dispatch_exec.go`, `internal/agent/native_tools_memory.go` - add `include_low_confidence` and `graph_health`.
- Modify: `prompts/tools_manuals/knowledge_graph.md` - document default filtering and override parameter.
- Modify: `internal/server/knowledge_graph_handlers.go`, `internal/server/knowledge_graph_handlers_test.go` - parse `include_low_confidence` and expose new report fields.
- Modify if UI metrics are displayed: `ui/js/dashboard/dashboard-widgets.js`, `ui/lang/dashboard/*.json` for all supported dashboard languages.

---

### Task 1: Add KG Quality Policy Package

**Files:**
- Create: `internal/kgquality/policy.go`
- Create: `internal/kgquality/policy_test.go`

- [ ] **Step 1: Write failing tests**

```go
package kgquality

import "testing"

func TestIsGenericEntityToken(t *testing.T) {
	tests := map[string]bool{
		"png":                true,
		".jpeg":              true,
		"rgba8":              true,
		"x86_64":             true,
		"attachment_folders": true,
		"server":             false,
		"caddy_server":       false,
		"agodesk":            false,
	}
	for input, want := range tests {
		if got := IsGenericEntity(input); got != want {
			t.Fatalf("IsGenericEntity(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFileNodeIDIsStableAndPathBacked(t *testing.T) {
	left := FileNodeID(`/home/aurago/aurago/data/documents/test_pdf.pdf`)
	right := FileNodeID(`\home\aurago\aurago\data\documents\test_pdf.pdf`)
	if left == "" || left != right {
		t.Fatalf("FileNodeID should be stable across separators, got %q and %q", left, right)
	}
	if len(left) != len("file_")+12 {
		t.Fatalf("FileNodeID length = %d, want %d", len(left), len("file_")+12)
	}
}

func TestLowConfidenceCoMention(t *testing.T) {
	policy := DefaultPolicy()
	if !LowConfidenceCoMention("co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}, policy) {
		t.Fatal("pending weight=1 co-mention should be low confidence")
	}
	if !LowConfidenceCoMention("co_mentioned_with", map[string]string{"source": "activity_turn", "weight": "1"}, policy) {
		t.Fatal("activity_turn weight below threshold should be low confidence")
	}
	if LowConfidenceCoMention("co_mentioned_with", map[string]string{"source": "activity_turn", "weight": "15"}, policy) {
		t.Fatal("promoted high-weight co-mention should not be low confidence")
	}
	if LowConfidenceCoMention("uses", map[string]string{"source": "manual"}, policy) {
		t.Fatal("semantic manual edge should not be treated as low-confidence co-mention")
	}
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `rtk go test ./internal/kgquality`

Expected: FAIL because `internal/kgquality` does not exist.

- [ ] **Step 3: Add implementation**

```go
package kgquality

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const DefaultPendingCoMentionTTLDays = 7
const DefaultLowConfidenceCoMentionMinWeight = 2

type Policy struct {
	PendingCoMentionTTLDays        int
	LowConfidenceCoMentionMinWeight int
	HideLowConfidenceByDefault      bool
}

func DefaultPolicy() Policy {
	return Policy{
		PendingCoMentionTTLDays:         DefaultPendingCoMentionTTLDays,
		LowConfidenceCoMentionMinWeight: DefaultLowConfidenceCoMentionMinWeight,
		HideLowConfidenceByDefault:      true,
	}
}

func NormalizePolicy(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.PendingCoMentionTTLDays <= 0 {
		policy.PendingCoMentionTTLDays = defaults.PendingCoMentionTTLDays
	}
	if policy.LowConfidenceCoMentionMinWeight <= 0 {
		policy.LowConfidenceCoMentionMinWeight = defaults.LowConfidenceCoMentionMinWeight
	}
	return policy
}

var genericTokens = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "gif": {}, "webp": {}, "svg": {},
	"pdf": {}, "doc": {}, "docx": {}, "txt": {}, "md": {},
	"rgb": {}, "rgba": {}, "rgba8": {},
	"x86": {}, "x86_64": {}, "amd64": {}, "arm64": {},
	"attachment": {}, "attachments": {}, "attachment_folder": {}, "attachment_folders": {},
	"file": {}, "files": {}, "folder": {}, "folders": {},
	"image": {}, "images": {}, "photo": {}, "photos": {},
	"document": {}, "documents": {}, "unknown": {},
}

func IsGenericEntity(value string) bool {
	token := normalizeEntityToken(value)
	if token == "" || len(token) < 2 {
		return true
	}
	_, ok := genericTokens[token]
	return ok
}

func normalizeEntityToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, ".:/\\-_ ")
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func IsPathLike(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return strings.Contains(value, "/") || strings.Contains(value, `\`) || strings.Contains(value, ":\\")
}

func FileNodeID(path string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	sum := sha1.Sum([]byte(clean))
	return "file_" + hex.EncodeToString(sum[:])[:12]
}

func LowConfidenceCoMention(relation string, properties map[string]string, policy Policy) bool {
	if strings.TrimSpace(relation) != "co_mentioned_with" {
		return false
	}
	policy = NormalizePolicy(policy)
	source := strings.TrimSpace(properties["source"])
	if source == "pending" || source == "" {
		return true
	}
	weight, _ := strconv.Atoi(strings.TrimSpace(properties["weight"]))
	return weight < policy.LowConfidenceCoMentionMinWeight
}
```

- [ ] **Step 4: Run tests and confirm pass**

Run: `rtk go test ./internal/kgquality`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/kgquality/policy.go internal/kgquality/policy_test.go
rtk git commit -m "feat: add knowledge graph quality policy helpers"
```

---

### Task 2: Filter Extracted KG Nodes and Normalize Path-Like IDs

**Files:**
- Modify: `internal/kgextraction/kg_extraction.go`
- Modify: `internal/kgextraction/kg_extraction_test.go`

- [ ] **Step 1: Run impact before editing**

Run GitNexus impact for `normalizeExtractedKG` and report HIGH risk before editing:

```text
impact({repo: "C:\\Users\\Andi\\Documents\\repo\\AuraGo", target: "normalizeExtractedKG", file_path: "internal/kgextraction/kg_extraction.go", kind: "Function", direction: "upstream", summaryOnly: true})
```

Expected: HIGH risk, because session extraction and file sync both depend on it.

- [ ] **Step 2: Add failing tests**

Add tests that prove:
- `png`, `jpeg`, `rgba8`, `x86_64`, and `attachment_folders` nodes are dropped.
- A path-like node label becomes `file_<hash>` and stores `properties.path`.
- Edges referencing remapped IDs are remapped to the new file ID.
- `file` and `tool` are accepted node types.

Core assertion shape:

```go
func TestNormalizeExtractedKGFiltersGenericTokens(t *testing.T) {
	nodes, edges := normalizeExtractedKG(
		[]memory.Node{
			{ID: "png", Label: "png", Properties: map[string]string{"type": "concept"}},
			{ID: "agodesk", Label: "AgoDesk", Properties: map[string]string{"type": "service"}},
		},
		[]memory.Edge{{Source: "agodesk", Target: "png", Relation: "related_to"}},
	)
	if len(nodes) != 1 || nodes[0].ID != "agodesk" {
		t.Fatalf("nodes = %#v, want only agodesk", nodes)
	}
	if len(edges) != 0 {
		t.Fatalf("edges = %#v, want generic target edge filtered", edges)
	}
}

func TestNormalizeExtractedKGPathLikeNodeUsesFileHash(t *testing.T) {
	path := `/home/aurago/aurago/data/documents/test_pdf.pdf`
	nodes, edges := normalizeExtractedKG(
		[]memory.Node{
			{ID: "homeauragoauragodatadocumentstest_pdf_pdf", Label: path, Properties: map[string]string{"type": "concept"}},
			{ID: "agodesk", Label: "AgoDesk", Properties: map[string]string{"type": "service"}},
		},
		[]memory.Edge{{Source: "agodesk", Target: "homeauragoauragodatadocumentstest_pdf_pdf", Relation: "related_to"}},
	)
	fileID := kgquality.FileNodeID(path)
	if len(nodes) != 2 {
		t.Fatalf("nodes = %#v, want 2", nodes)
	}
	var foundFile bool
	for _, node := range nodes {
		if node.ID == fileID {
			foundFile = true
			if node.Properties["type"] != "file" || node.Properties["path"] != path {
				t.Fatalf("file node properties = %#v", node.Properties)
			}
		}
	}
	if !foundFile {
		t.Fatalf("missing canonical file node %q in %#v", fileID, nodes)
	}
	if len(edges) != 1 || edges[0].Target != fileID {
		t.Fatalf("edges = %#v, want remapped target %q", edges, fileID)
	}
}
```

- [ ] **Step 3: Run tests and confirm failure**

Run: `rtk go test ./internal/kgextraction`

Expected: FAIL because filtering/remapping is not implemented.

- [ ] **Step 4: Implement extraction filtering**

Implementation rules:
- Import `aurago/internal/kgquality`.
- Add `file` and `tool` to `allowedKGNodeTypes`.
- Update the LLM prompt to say file paths must not be used directly as IDs; use a file entity with `properties.path`.
- Keep `kgIDPattern` for final IDs.
- Maintain `idRemap := map[string]string{}` so path-normalized node IDs do not orphan edges.
- Drop nodes when both ID and label are generic tokens.
- Drop edges whose remapped endpoints are missing or whose relation is not allowed.
- Ensure normalized edge properties are non-nil.

- [ ] **Step 5: Run tests and confirm pass**

Run: `rtk go test ./internal/kgextraction`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/kgextraction/kg_extraction.go internal/kgextraction/kg_extraction_test.go
rtk git commit -m "fix: filter noisy knowledge graph extraction entities"
```

---

### Task 3: Anchor File Sync with Canonical File Nodes

**Files:**
- Modify: `internal/services/file_kg_sync.go`
- Modify: `internal/services/file_kg_sync_test.go`

- [ ] **Step 1: Write failing tests**

Add a test to file KG sync that verifies a synced file creates or preserves:

```json
{
  "id": "file_<sha1-12>",
  "properties": {
    "type": "file",
    "path": "/home/aurago/aurago/data/documents/test_pdf.pdf",
    "source": "file_sync",
    "source_file": "/home/aurago/aurago/data/documents/test_pdf.pdf"
  }
}
```

Also assert no node ID equals the legacy mangled form `homeauragoauragodatadocumentstest_pdf_pdf`.

- [ ] **Step 2: Run tests and confirm failure**

Run: `rtk go test ./internal/services -run FileKGSyncer`

Expected: FAIL because canonical file node anchoring is not implemented.

- [ ] **Step 3: Implement canonical file node injection**

In `SyncFile`, after extraction and before `BulkMergeExtractedEntities`:
- Compute `fileID := kgquality.FileNodeID(path)`.
- Append a `memory.Node` with label `filepath.Base(path)` and properties `type=file`, `path=path`, `source=file_sync`, `source_file=path`, `extracted_at=now`, `confidence=confidenceStr`.
- Drop any extracted node that is path-like and points to the same path unless it already has the canonical `fileID`.
- Optionally link extracted entities to the file node with `related_to` edges only when the resulting edge count is reasonable. Cap at 25 entity-file edges per file sync run.

- [ ] **Step 4: Run tests and confirm pass**

Run: `rtk go test ./internal/services -run FileKGSyncer`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/services/file_kg_sync.go internal/services/file_kg_sync_test.go
rtk git commit -m "fix: canonicalize knowledge graph file sync nodes"
```

---

### Task 4: Filter Noisy Activity Entities Before Co-Occurrence Edges

**Files:**
- Modify: `internal/agent/activity_capture.go`
- Modify: `internal/agent/activity_capture_test.go`

- [ ] **Step 1: Run impact before editing**

Run GitNexus impact for `normalizeActivityDigest` and `syncActivityTurnToKnowledgeGraph`.

Expected: report direct callers through activity capture; no implementation until risk is noted.

- [ ] **Step 2: Add failing tests**

Add:

```go
func TestNormalizeActivityDigestFiltersGenericEntities(t *testing.T) {
	digest := normalizeActivityDigest(memory.ActivityDigest{
		Intent:     "test",
		Importance: 2,
		Entities:   []string{"png", "AgoDesk", "x86_64", "Caddy Server"},
	})
	if got := strings.Join(digest.Entities, ","); got != "AgoDesk,Caddy Server" {
		t.Fatalf("entities = %q", got)
	}
}
```

Add a sync test that uses entities `["Andi", "png", "AgoDesk"]` and verifies no `co_mentioned_with` edge involving `png`.

- [ ] **Step 3: Run tests and confirm failure**

Run: `rtk go test ./internal/agent -run Activity`

Expected: FAIL because generic entities are still retained.

- [ ] **Step 4: Implement filtering**

Implementation:
- Import `aurago/internal/kgquality`.
- Filter `digest.Entities` in `normalizeActivityDigest`.
- In `syncActivityTurnToKnowledgeGraph`, re-check `kgquality.IsGenericEntity(label)` before `AddNode` as defense in depth.
- Keep `server` as allowed but leave it visible in quality reporting as a generic-label candidate, not a hard blacklist.

- [ ] **Step 5: Run tests and confirm pass**

Run: `rtk go test ./internal/agent -run Activity`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/agent/activity_capture.go internal/agent/activity_capture_test.go
rtk git commit -m "fix: suppress generic activity entities in knowledge graph"
```

---

### Task 5: Add Edge Quality Metadata Defaults

**Files:**
- Modify: `internal/memory/graph_sqlite.go`
- Modify: `internal/memory/graph_edge.go`
- Modify: `internal/memory/graph_backup.go`
- Modify: `internal/memory/graph_sqlite_test.go`

- [ ] **Step 1: Write failing tests**

Add tests:
- `AddEdge("andi", "agodesk", "uses", nil)` stores non-empty properties with `source=manual`, `confidence=1.00`, and `extracted_at=<today>`.
- `BulkMergeExtractedEntities` preserves higher-confidence existing props and fills missing quality props.
- Existing explicit `source`, `confidence`, and `extracted_at` are not overwritten by defaults.

- [ ] **Step 2: Run tests and confirm failure**

Run: `rtk go test ./internal/memory -run 'AddEdge|BulkMergeExtractedEntities'`

Expected: FAIL because manual edges can currently store `{}`.

- [ ] **Step 3: Implement defaulting helper**

Add a helper in `graph_sqlite.go`:

```go
func ensureKnowledgeGraphEdgeQualityProperties(properties map[string]string, defaultSource string, now time.Time) map[string]string {
	properties = normalizeKnowledgeGraphProperties(properties)
	if strings.TrimSpace(properties["source"]) == "" {
		properties["source"] = defaultSource
	}
	if strings.TrimSpace(properties["confidence"]) == "" {
		if defaultSource == "manual" {
			properties["confidence"] = "1.00"
		} else {
			properties["confidence"] = "0.50"
		}
	}
	if strings.TrimSpace(properties["extracted_at"]) == "" {
		properties["extracted_at"] = now.Format("2006-01-02")
	}
	return properties
}
```

Wire it into:
- `AddEdge` with `defaultSource="manual"`.
- `UpdateEdge` only when properties are provided and target edge would otherwise have no quality props.
- `BulkMergeExtractedEntities` with default source from incoming props, falling back to `auto_extraction`.

- [ ] **Step 4: Run tests and confirm pass**

Run: `rtk go test ./internal/memory -run 'AddEdge|BulkMergeExtractedEntities'`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/memory/graph_sqlite.go internal/memory/graph_edge.go internal/memory/graph_backup.go internal/memory/graph_sqlite_test.go
rtk git commit -m "fix: add knowledge graph edge quality metadata"
```

---

### Task 6: Hide Low-Confidence Edges by Default with Explicit Override

**Files:**
- Modify: `internal/memory/graph_sqlite.go`
- Modify: `internal/memory/graph_query.go`
- Modify: `internal/memory/graph_sqlite_test.go`
- Modify: `internal/agent/tool_args_execution.go`
- Modify: `internal/agent/agent_dispatch_exec.go`
- Modify: `internal/agent/native_tools_memory.go`
- Modify: `internal/agent/tool_args_execution_test.go`
- Modify: `prompts/tools_manuals/knowledge_graph.md`
- Modify: `internal/server/knowledge_graph_handlers.go`
- Modify: `internal/server/knowledge_graph_handlers_test.go`

- [ ] **Step 1: Write failing tests**

Tests:
- `Search("andi")` omits `co_mentioned_with` edges with `source=pending` and `weight=1`.
- `SearchWithOptions("andi", KnowledgeGraphQueryOptions{IncludeLowConfidence: true})` includes them.
- `GetNeighbors("andi", 20)` omits low-confidence co-mentions.
- `GetNeighborsWithOptions("andi", 20, KnowledgeGraphQueryOptions{IncludeLowConfidence: true})` includes them.
- HTTP search and node-detail handlers honor `include_low_confidence=true`.
- Agent tool args decode `include_low_confidence`.

- [ ] **Step 2: Run tests and confirm failure**

Run:

```bash
rtk go test ./internal/memory -run 'Search|GetNeighbors'
rtk go test ./internal/server -run KnowledgeGraph
rtk go test ./internal/agent -run KnowledgeGraph
```

Expected: FAIL because option-bearing query methods and arg parsing do not exist yet.

- [ ] **Step 3: Implement compatibility wrappers**

Add:

```go
type KnowledgeGraphQueryOptions struct {
	IncludeLowConfidence bool
}

func (kg *KnowledgeGraph) Search(query string) string {
	return kg.SearchWithOptions(query, KnowledgeGraphQueryOptions{})
}

func (kg *KnowledgeGraph) GetNeighbors(nodeID string, limit int) ([]Node, []Edge) {
	nodes, edges := kg.GetNeighborsWithOptions(nodeID, limit, KnowledgeGraphQueryOptions{})
	return nodes, edges
}
```

Filtering rule:
- Unless `IncludeLowConfidence` is true, skip `kgquality.LowConfidenceCoMention(edge.Relation, edge.Properties, kg.qualityPolicy())`.
- Do not filter semantic relations such as `uses`, `runs_on`, `depends_on`, `part_of`, or `located_in`.

- [ ] **Step 4: Wire explicit override through agent and server**

Agent:
- Add `IncludeLowConfidence bool` to `knowledgeGraphArgs`.
- Decode via `toolArgBool(tc.Params, "include_low_confidence")`.
- Add native schema property `include_low_confidence`.
- Use option-bearing search/neighbors for `search` and `get_neighbors`.

Server:
- Parse `include_low_confidence=true` from `/api/knowledge/search` and node detail endpoint.
- Keep default false.

Manual:
- Document that `search` and `get_neighbors` hide low-confidence co-mentions by default.
- Add example:

```json
{"action": "knowledge_graph", "operation": "search", "content": "andi", "include_low_confidence": true}
```

- [ ] **Step 5: Run tests and confirm pass**

Run:

```bash
rtk go test ./internal/memory -run 'Search|GetNeighbors'
rtk go test ./internal/server -run KnowledgeGraph
rtk go test ./internal/agent -run KnowledgeGraph
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/memory/graph_sqlite.go internal/memory/graph_query.go internal/memory/graph_sqlite_test.go internal/agent/tool_args_execution.go internal/agent/agent_dispatch_exec.go internal/agent/native_tools_memory.go internal/agent/tool_args_execution_test.go internal/server/knowledge_graph_handlers.go internal/server/knowledge_graph_handlers_test.go prompts/tools_manuals/knowledge_graph.md
rtk git commit -m "feat: hide low-confidence knowledge graph edges by default"
```

---

### Task 7: Split Cleanup TTLs and Add KG Quality Config

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config.go`
- Modify: `config_template.yaml`
- Modify: `internal/server/config_handlers_main.go`
- Modify: `internal/memory/graph_sqlite.go`
- Modify: `internal/memory/graph_query.go`
- Modify: `internal/memory/graph_phase9_test.go`
- Modify: `internal/agent/maintenance.go`
- Modify: `cmd/aurago/main.go`
- Modify: `cmd/lifeboat/main.go`

- [ ] **Step 1: Write failing tests**

Add tests:
- Default config sets `pending_co_mention_ttl_days=7`, `low_confidence_co_mention_min_weight=2`, `hide_low_confidence_by_default=true`.
- `CleanupStaleGraphWithOptions(KnowledgeGraphCleanupOptions{PendingCoMentionDays: 7, StaleNodeDays: 30})` deletes old pending co-mention edges after 7 days but does not delete stale unaccessed nodes until 30 days.
- Existing `CleanupStaleGraph(30)` remains compatible.

- [ ] **Step 2: Run tests and confirm failure**

Run:

```bash
rtk go test ./internal/config -run KnowledgeGraph
rtk go test ./internal/memory -run CleanupStaleGraph
```

Expected: FAIL because fields/options do not exist.

- [ ] **Step 3: Add config fields**

Add to `Tools.KnowledgeGraph`:

```go
PendingCoMentionTTLDays        int  `yaml:"pending_co_mention_ttl_days"`
LowConfidenceCoMentionMinWeight int  `yaml:"low_confidence_co_mention_min_weight"`
HideLowConfidenceByDefault      bool `yaml:"hide_low_confidence_by_default"`
```

Defaults:
- `pending_co_mention_ttl_days: 7`
- `low_confidence_co_mention_min_weight: 2`
- `hide_low_confidence_by_default: true`

- [ ] **Step 4: Add graph cleanup options**

Add:

```go
type KnowledgeGraphCleanupOptions struct {
	PendingCoMentionDays int
	StaleNodeDays        int
	PlaceholderDays      int
}

func (kg *KnowledgeGraph) CleanupStaleGraph(thresholdDays int) (int, int, error) {
	return kg.CleanupStaleGraphWithOptions(KnowledgeGraphCleanupOptions{
		PendingCoMentionDays: thresholdDays,
		StaleNodeDays:        thresholdDays,
		PlaceholderDays:      thresholdDays,
	})
}
```

Maintenance should call `CleanupStaleGraphWithOptions` with pending TTL from config and node cleanup still at 30 days.

- [ ] **Step 5: Wire policy into KG construction**

In `cmd/aurago/main.go` and `cmd/lifeboat/main.go`, after KG construction:

```go
kg.SetQualityPolicy(memory.KnowledgeGraphQualityPolicy{
	PendingCoMentionTTLDays:         cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays,
	LowConfidenceCoMentionMinWeight: cfg.Tools.KnowledgeGraph.LowConfidenceCoMentionMinWeight,
	HideLowConfidenceByDefault:      cfg.Tools.KnowledgeGraph.HideLowConfidenceByDefault,
})
```

- [ ] **Step 6: Run tests and confirm pass**

Run:

```bash
rtk go test ./internal/config -run KnowledgeGraph
rtk go test ./internal/memory -run CleanupStaleGraph
rtk go test ./internal/agent -run Maintenance
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
rtk git add internal/config/config_types.go internal/config/config.go config_template.yaml internal/server/config_handlers_main.go internal/memory/graph_sqlite.go internal/memory/graph_query.go internal/memory/graph_phase9_test.go internal/agent/maintenance.go cmd/aurago/main.go cmd/lifeboat/main.go
rtk git commit -m "feat: configure knowledge graph quality cleanup policy"
```

---

### Task 8: Expand Health, Quality, and Tool Reporting

**Files:**
- Modify: `internal/memory/graph_sqlite.go`
- Modify: `internal/memory/graph_query.go`
- Modify: `internal/memory/graph_sqlite_test.go`
- Modify: `internal/server/knowledge_graph_handlers.go`
- Modify: `internal/server/knowledge_graph_handlers_test.go`
- Modify: `internal/agent/agent_dispatch_exec.go`
- Modify: `internal/agent/native_tools_memory.go`
- Modify: `prompts/tools_manuals/knowledge_graph.md`

- [ ] **Step 1: Write failing tests**

Quality and stats should include:
- total nodes/edges
- co-mention edge count
- pending edge count
- low-confidence edge count
- pending co-mention count
- generic node count and sample
- duplicate candidates and ID duplicate candidates
- edge source breakdown

Tool operation:

```json
{"action":"knowledge_graph","operation":"graph_health"}
```

Expected output includes `stats` and `quality`.

- [ ] **Step 2: Run tests and confirm failure**

Run:

```bash
rtk go test ./internal/memory -run 'GetStats|QualityReport'
rtk go test ./internal/server -run KnowledgeGraph
rtk go test ./internal/agent -run KnowledgeGraph
```

Expected: FAIL because fields and `graph_health` do not exist.

- [ ] **Step 3: Extend stats/report structs**

Add JSON fields without removing existing fields:

```go
PendingEdges           int            `json:"pending_edges"`
LowConfidenceEdges     int            `json:"low_confidence_edges"`
PendingCoMentionEdges  int            `json:"pending_co_mention_edges"`
EdgeBySource           map[string]int `json:"edge_by_source"`
GenericNodes           int            `json:"generic_nodes"`
GenericSample          []Node         `json:"generic_sample"`
```

Use post-decode helpers for generic and low-confidence checks so SQLite JSON quirks do not duplicate policy logic.

- [ ] **Step 4: Add `graph_health` operation**

In agent dispatch:
- Add operation enum value `graph_health`.
- Return `{"status":"success","stats":..., "quality":...}`.
- Keep existing `search`, `get_neighbors`, and `optimize_graph` behavior.

Manual:
- Document `graph_health` as read-only.

- [ ] **Step 5: Run tests and confirm pass**

Run:

```bash
rtk go test ./internal/memory -run 'GetStats|QualityReport'
rtk go test ./internal/server -run KnowledgeGraph
rtk go test ./internal/agent -run KnowledgeGraph
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/memory/graph_sqlite.go internal/memory/graph_query.go internal/memory/graph_sqlite_test.go internal/server/knowledge_graph_handlers.go internal/server/knowledge_graph_handlers_test.go internal/agent/agent_dispatch_exec.go internal/agent/native_tools_memory.go prompts/tools_manuals/knowledge_graph.md
rtk git commit -m "feat: expose knowledge graph quality health metrics"
```

---

### Task 9: Surface Quality Signals in Dashboard

**Files:**
- Modify: `ui/js/dashboard/dashboard-widgets.js`
- Modify: every `ui/lang/dashboard/*.json`

- [ ] **Step 1: Inspect current dashboard dirty state**

Run: `rtk git status --short`

Expected: there may be pre-existing dashboard changes. Do not overwrite unrelated user work. Read the current file contents before editing.

- [ ] **Step 2: Add visual indicators**

Extend the existing Knowledge Graph health/visual widget with compact metrics:
- Pending edges
- Low-confidence edges
- Pending co-mentions
- Generic nodes
- Duplicate suggestions

UI rules:
- No `alert()`.
- Fit current dashboard style.
- Do not add a new landing page or decorative cards.
- Keep all text in translation keys.

- [ ] **Step 3: Add translations**

Add equivalent keys to all existing dashboard language files:

```json
"kgPendingEdges": "Pending edges",
"kgLowConfidenceEdges": "Low-confidence edges",
"kgPendingCoMentions": "Pending co-mentions",
"kgGenericNodes": "Generic nodes",
"kgDuplicateSuggestions": "Duplicate suggestions"
```

Translate these in all dashboard language JSON files present in `ui/lang/dashboard/`: `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`.

- [ ] **Step 4: Run UI-safe checks**

Run:

```bash
rtk go test ./internal/server -run Dashboard
rtk npm run lint
```

Expected: PASS. If no lint script exists, report that and run the closest existing frontend check.

- [ ] **Step 5: Commit**

```bash
rtk git add ui/js/dashboard/dashboard-widgets.js ui/lang/dashboard/*.json
rtk git commit -m "feat: show knowledge graph quality metrics on dashboard"
```

---

### Task 10: Verify Duplicates Are Suggested, Not Auto-Merged

**Files:**
- Modify only if needed: `internal/memory/kgquery/duplicates.go`
- Modify: `internal/memory/graph_sqlite_test.go`
- Modify: `internal/agent/memory_hygiene_test.go`

- [ ] **Step 1: Add regression test**

Add a test with nodes:
- `caddy_server`
- `caddyserver`

Expected:
- Quality report includes an ID duplicate candidate.
- No automatic merge occurs.
- Protected source nodes remain untouched.

- [ ] **Step 2: Run test**

Run: `rtk go test ./internal/memory -run Duplicate`

Expected: PASS if existing normalized-ID duplicate logic already covers it. If it fails, implement the smallest extension in `kgquery` and rerun.

- [ ] **Step 3: Commit if code changed**

If only tests were added:

```bash
rtk git add internal/memory/graph_sqlite_test.go internal/agent/memory_hygiene_test.go
rtk git commit -m "test: cover knowledge graph duplicate suggestions"
```

If implementation changed:

```bash
rtk git add internal/memory/kgquery/duplicates.go internal/memory/graph_sqlite_test.go internal/agent/memory_hygiene_test.go
rtk git commit -m "fix: improve knowledge graph duplicate suggestions"
```

---

### Task 11: Full Verification and Change Detection

**Files:**
- No new source files unless previous tasks require fixes.

- [ ] **Step 1: Run package tests**

```bash
rtk go test ./internal/kgquality ./internal/kgextraction ./internal/services ./internal/agent ./internal/memory ./internal/server ./internal/config
```

Expected: PASS.

- [ ] **Step 2: Run broader test if time allows**

```bash
rtk go test ./...
```

Expected: PASS, or document unrelated failures with exact packages.

- [ ] **Step 3: Run GitNexus detect_changes before final commit**

```text
detect_changes({repo: "C:\\Users\\Andi\\Documents\\repo\\AuraGo", scope: "all"})
```

Expected: affected symbols match KG quality, extraction, memory, config, agent tool, server handler, dashboard translation changes only.

- [ ] **Step 4: DOX closeout**

Re-check changed paths against AGENTS:
- Root `AGENTS.md` applies to all changed files.
- No child AGENTS applies unless dashboard files under `ui/js/desktop/apps` are touched, which this plan does not touch.

Docs update decision:
- Do not change `AGENTS.md`; this plan changes implementation behavior, not durable agent work rules.
- Update `prompts/tools_manuals/knowledge_graph.md` because the `knowledge_graph` tool behavior changes.

- [ ] **Step 5: Final commit if any verification fixes were needed**

```bash
rtk git add <only files changed by verification fixes>
rtk git commit -m "test: verify knowledge graph quality cleanup"
```

## Self-Review

Spec coverage:
- P1 co-mention noise: Tasks 4, 6, 7, 8.
- P1 path ID normalization: Tasks 2, 3.
- P2 duplicates: Tasks 8, 10.
- P2 generic nodes: Tasks 1, 2, 4, 8.
- P2 empty edge properties: Task 5.
- P3 pending lifecycle: Task 7.
- Health monitoring: Tasks 8, 9.
- User validation loop: kept to existing `merge_nodes`, `delete_edge`, and dashboard quality surfacing; a bulk validation UI is intentionally deferred.

No-placeholder scan:
- Every task names exact files, tests, commands, and expected outcomes.
- Risk handling is explicit for HIGH-risk normalization.

Type consistency:
- Query override is consistently named `IncludeLowConfidence`.
- Tool/API parameter is consistently named `include_low_confidence`.
- Cleanup override is consistently named `KnowledgeGraphCleanupOptions`.

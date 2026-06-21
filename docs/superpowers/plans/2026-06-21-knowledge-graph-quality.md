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

- Create: `internal/kgquality/policy.go` - dependency-free quality predicates, file ID hashing, generic token filtering, and low-confidence predicates.
- Create: `internal/kgquality/policy_test.go` - unit tests for generic tokens, path hashing, and low-confidence detection.
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
- Modify: `internal/memory/graph_query_duplicates.go` and `internal/memory/kgquery/duplicates.go` are expected to stay unchanged because existing normalized-ID duplicate logic strips `_` and `-`; Task 10 adds a regression test to lock that in.
- Modify: `internal/memory/*_test.go` - focused tests for edge defaults, filtered search/neighbors, cleanup TTL, stats/report metrics, and duplicate suggestions.
- Modify: `internal/config/config_types.go`, `internal/config/config.go`, `config_template.yaml`, `internal/server/config_handlers_main.go` - config defaults and serialized config display.
- Modify: `cmd/aurago/main.go`, `cmd/lifeboat/main.go` - apply KG quality policy to constructed graph instances.
- Modify: `internal/agent/tool_args_execution.go`, `internal/agent/agent_dispatch_exec.go`, `internal/agent/native_tools_memory.go` - add `include_low_confidence` and `graph_health`.
- Modify: `prompts/tools_manuals/knowledge_graph.md` - document default filtering and override parameter.
- Modify: `internal/server/knowledge_graph_handlers.go`, `internal/server/knowledge_graph_handlers_test.go` - parse `include_low_confidence` and expose new report fields.
- Modify for dashboard metrics only after target UI files are clean: `ui/js/dashboard/dashboard-widgets.js`, `ui/lang/dashboard/*.json` for all supported dashboard languages.

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
	"path"
	"strconv"
	"strings"
	"unicode"
)

const DefaultPendingCoMentionTTLDays = 7
const DefaultLowConfidenceCoMentionMinWeight = 2

type Policy struct {
	PendingCoMentionTTLDays         int
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

func CanonicalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, `\`, "/")
	return path.Clean(value)
}

func FileNodeID(pathValue string) string {
	clean := CanonicalPath(pathValue)
	sum := sha1.Sum([]byte(clean))
	return "file_" + hex.EncodeToString(sum[:])[:12]
}

func PathBase(pathValue string) string {
	clean := CanonicalPath(pathValue)
	if clean == "" {
		return ""
	}
	return path.Base(clean)
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
- Do not treat the `type=file` property itself as a generic token; a canonical file node with `properties.path` is valid even though the standalone word `file` is generic.
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
- Append a `memory.Node` with label `kgquality.PathBase(path)` and properties `type=file`, `path=kgquality.CanonicalPath(path)`, `source=file_sync`, `source_file=path`, `extracted_at=now`, `confidence=confidenceStr`.
- Drop any extracted node that is path-like and points to the same path unless it already has the canonical `fileID`.
- Add deterministic `related_to` edges from up to the first 25 non-file extracted nodes, sorted by node ID, to `fileID`; set `source=file_sync`, `source_file=path`, `extracted_at=now`, and `confidence=confidenceStr` on those edges.

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
- In neighbor queries, apply the low-confidence edge filter before building `neighborOrder`, so hidden edges cannot consume the result limit.

- [ ] **Step 4: Wire explicit override through agent and server**

Agent:
- Add `IncludeLowConfidence bool` to `knowledgeGraphArgs`.
- Decode with:

```go
includeLowConfidence, _ := toolArgBool(tc.Params, "include_low_confidence")
req.IncludeLowConfidence = includeLowConfidence
```

- Add native schema property `include_low_confidence`.
- Use option-bearing search/neighbors for `search` and `get_neighbors`.

Server:
- Parse `include_low_confidence=true` from `/api/knowledge-graph/search` and `/api/knowledge-graph/node`.
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
- `NewKnowledgeGraph` initializes `qualityPolicy` to `kgquality.DefaultPolicy()`.

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
PendingCoMentionTTLDays         int  `yaml:"pending_co_mention_ttl_days"`
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

In `CleanupStaleGraphWithOptions`, delete pending co-mentions with `updated_at <= datetime('now', '-' || ? || ' days')`, not `created_at`, so an edge that was recently observed but not yet promoted gets a full TTL window from the latest observation.

Maintenance should call `CleanupStaleGraphWithOptions` with pending TTL from config and node cleanup still at 30 days.

- [ ] **Step 5: Wire policy into KG construction**

In `internal/memory/graph_sqlite.go`, add policy storage to `KnowledgeGraph`:

```go
qualityPolicy   kgquality.Policy
qualityPolicyMu sync.RWMutex
```

Initialize `qualityPolicy: kgquality.DefaultPolicy()` in `NewKnowledgeGraph`.

Add setter/getter methods:

```go
func (kg *KnowledgeGraph) SetQualityPolicy(policy kgquality.Policy) {
	kg.qualityPolicyMu.Lock()
	defer kg.qualityPolicyMu.Unlock()
	kg.qualityPolicy = kgquality.NormalizePolicy(policy)
}

func (kg *KnowledgeGraph) qualityPolicy() kgquality.Policy {
	kg.qualityPolicyMu.RLock()
	defer kg.qualityPolicyMu.RUnlock()
	return kg.qualityPolicy
}
```

In `cmd/aurago/main.go` and `cmd/lifeboat/main.go`, import `aurago/internal/kgquality` and call after KG construction:

```go
kg.SetQualityPolicy(kgquality.Policy{
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

Run:

```bash
rtk git diff --quiet -- ui/js/dashboard/dashboard-widgets.js ui/lang/dashboard
rtk git status --short
```

Expected: the first command exits 0 before editing. If it exits 1, stop Task 9 and move it to a clean worktree or wait until the existing dashboard changes are committed; do not edit or stage partially dirty dashboard files.

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

Add these exact keys to all existing dashboard language files:

| File | kgPendingEdges | kgLowConfidenceEdges | kgPendingCoMentions | kgGenericNodes | kgDuplicateSuggestions |
|------|----------------|----------------------|---------------------|----------------|-------------------------|
| `cs.json` | `Nevyřízené hrany` | `Hrany s nízkou důvěrou` | `Nevyřízené společné zmínky` | `Obecné uzly` | `Návrhy duplicit` |
| `da.json` | `Afventende kanter` | `Kanter med lav tillid` | `Afventende fælles omtaler` | `Generiske noder` | `Duplikatforslag` |
| `de.json` | `Ausstehende Kanten` | `Kanten mit niedriger Zuversicht` | `Ausstehende gemeinsame Erwähnungen` | `Generische Knoten` | `Duplikatvorschläge` |
| `el.json` | `Εκκρεμείς ακμές` | `Ακμές χαμηλής εμπιστοσύνης` | `Εκκρεμείς συν-αναφορές` | `Γενικοί κόμβοι` | `Προτάσεις διπλοτύπων` |
| `en.json` | `Pending edges` | `Low-confidence edges` | `Pending co-mentions` | `Generic nodes` | `Duplicate suggestions` |
| `es.json` | `Aristas pendientes` | `Aristas de baja confianza` | `Co-menciones pendientes` | `Nodos genéricos` | `Sugerencias de duplicados` |
| `fr.json` | `Arêtes en attente` | `Arêtes à faible confiance` | `Co-mentions en attente` | `Nœuds génériques` | `Suggestions de doublons` |
| `hi.json` | `लंबित एज` | `कम भरोसे वाले एज` | `लंबित सह-उल्लेख` | `सामान्य नोड` | `डुप्लिकेट सुझाव` |
| `it.json` | `Archi in attesa` | `Archi a bassa affidabilità` | `Co-menzioni in attesa` | `Nodi generici` | `Suggerimenti duplicati` |
| `ja.json` | `保留中のエッジ` | `信頼度の低いエッジ` | `保留中の共起言及` | `汎用ノード` | `重複候補` |
| `nl.json` | `Openstaande randen` | `Randen met lage betrouwbaarheid` | `Openstaande co-vermeldingen` | `Generieke knopen` | `Duplicaatsuggesties` |
| `no.json` | `Ventende kanter` | `Kanter med lav tillit` | `Ventende samomtaler` | `Generiske noder` | `Duplikatforslag` |
| `pl.json` | `Oczekujące krawędzie` | `Krawędzie o niskiej pewności` | `Oczekujące współwzmianki` | `Węzły ogólne` | `Sugestie duplikatów` |
| `pt.json` | `Arestas pendentes` | `Arestas de baixa confiança` | `Co-menções pendentes` | `Nós genéricos` | `Sugestões de duplicados` |
| `sv.json` | `Väntande kanter` | `Kanter med låg tillit` | `Väntande samomnämnanden` | `Generiska noder` | `Dubblettförslag` |
| `zh.json` | `待处理边` | `低置信度边` | `待处理共同提及` | `通用节点` | `重复建议` |

- [ ] **Step 4: Run UI-safe checks**

Run:

```bash
rtk go test ./internal/server -run Dashboard
rtk npm run build:ui
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add ui/js/dashboard/dashboard-widgets.js ui/lang/dashboard/*.json
rtk git commit -m "feat: show knowledge graph quality metrics on dashboard"
```

---

### Task 10: Verify Duplicates Are Suggested, Not Auto-Merged

**Files:**
- Modify: `internal/memory/graph_sqlite_test.go`

- [ ] **Step 1: Add regression test**

Add:

```go
func TestKGQualityReportSuggestsNormalizedIDDuplicates(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddNode("caddy_server", "Caddy Server", map[string]string{
		"type":       "service",
		"source":     "manual",
		"confidence": "1.00",
	}); err != nil {
		t.Fatalf("add manual node: %v", err)
	}
	if err := kg.AddNode("caddyserver", "Caddy Server", map[string]string{
		"type":       "service",
		"source":     "auto_extraction",
		"confidence": "0.70",
	}); err != nil {
		t.Fatalf("add extracted node: %v", err)
	}

	report, err := kg.QualityReport(10)
	if err != nil {
		t.Fatalf("quality report: %v", err)
	}

	found := false
	for _, candidate := range report.IDDuplicateCandidates {
		haveA, haveB := false, false
		for _, id := range candidate.IDs {
			if id == "caddy_server" {
				haveA = true
			}
			if id == "caddyserver" {
				haveB = true
			}
		}
		if haveA && haveB {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected caddy_server and caddyserver duplicate suggestion, got %#v", report.IDDuplicateCandidates)
	}

	for _, id := range []string{"caddy_server", "caddyserver"} {
		node, err := kg.GetNode(id)
		if err != nil {
			t.Fatalf("get node %s: %v", id, err)
		}
		if node == nil {
			t.Fatalf("expected node %s to remain unmerged", id)
		}
	}
}
```

Expected:
- Quality report includes an ID duplicate candidate.
- No automatic merge occurs.
- Protected source nodes remain untouched.

- [ ] **Step 2: Run test**

Run: `rtk go test ./internal/memory -run TestKGQualityReportSuggestsNormalizedIDDuplicates`

Expected: PASS with the existing normalized-ID duplicate logic. If this does not pass, stop and revise this task plan before implementation; do not add a speculative duplicate-merging change.

- [ ] **Step 3: Commit**

```bash
rtk git add internal/memory/graph_sqlite_test.go
rtk git commit -m "test: cover knowledge graph duplicate suggestions"
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

- [ ] **Step 5: Handle verification fixes**

Do not create a generic verification-fix commit here. If verification reveals a failure, return to the failed task, patch the exact task files, rerun that task's tests, and commit using that task's explicit `git add` list.

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

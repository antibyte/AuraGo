package memory

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/security"

	chromem "github.com/philippgille/chromem-go"
)

// kgPolicyRegisteredSecret is registered with the process-wide scrubber (there
// is no unregister API), so it is unique to this test file.
const kgPolicyRegisteredSecret = "kg-policy-fixture-registered-5d1e90ab"

// kgSemanticPolicyAllowed runs content (and metadata) through the check that
// guards the KG semantic index.
func kgSemanticPolicyAllowed(content string, metadata map[string]string) bool {
	return semanticDocumentAllowed(chromem.Document{ID: "doc", Content: content, Metadata: metadata})
}

// kgBuiltContent mirrors what kgsemantic.BuildNodeContent writes for a node:
// the label, then one "key: value" line per property.
func kgBuiltContent(label string, lines ...string) string {
	return strings.Join(append([]string{label}, lines...), "\n")
}

// Ordinary graph content mentions credential words as property names or as prose
// ("pin: 17", "password: stored in vault"). The main-store credential policy
// rejects all of it, which silently removed such nodes and edges from semantic
// search. The KG check keeps every strong signal and lets only these through.
func TestKGSemanticPolicyAllowsOrdinaryGraphContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"gpio pin number", kgBuiltContent("ESP32 sensor", "pin: 17")},
		{"single digit pin", kgBuiltContent("ESP32 sensor", "pin: 5")},
		{"pin label", kgBuiltContent("ESP32 sensor", "pin: GPIO5")},
		{"auth scheme", kgBuiltContent("Home Assistant", "auth: OAuth2")},
		{"password prose", kgBuiltContent("Router", "password: stored in vault")},
		{"password prose with full stop", kgBuiltContent("Router", "password: stored.")},
		{"quoted prose value", kgBuiltContent("Router", `password: "stored" in the vault`)},
		{"token prose", kgBuiltContent("Grafana", "token: expires daily")},
		{"secret none", kgBuiltContent("Proxmox", "secret: none")},
		{"unicode word", kgBuiltContent("Cluster", "secret: läuft ab")},
		{"nested keyword in description", kgBuiltContent("Password policy", "description: password: at least 12 chars")},
		{"edge property", "uses\nrack-1 uses rack-3\ntoken: rotated"},
		{"uppercase keyword", kgBuiltContent("Gate", "PIN: 17")},
		{"two harmless matches", kgBuiltContent("Router", "password: stored in vault", "token: rotated weekly")},
		{"number inside prose", kgBuiltContent("Policy", "token: rotates every 90 days")},
		{"acronym and capitalized word", kgBuiltContent("Gateway", "token: JWT issued by Keycloak")},
		{"auth scheme with words", kgBuiltContent("Home Assistant", "auth: OAuth2 via Google")},
		{"scheme name as the last word", kgBuiltContent("API", "auth: bearer")},
		// Accepted trade-off: a passphrase made only of plain words cannot be told
		// apart from prose.
		{"passphrase of plain words", kgBuiltContent("Router", "password: correct horse battery")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !kgSemanticPolicyAllowed(tt.content, nil) {
				t.Fatalf("ordinary KG content was withheld from the semantic index:\n%s", tt.content)
			}
		})
	}

	t.Run("metadata goes through the same check", func(t *testing.T) {
		if !kgSemanticPolicyAllowed("ESP32 sensor", map[string]string{"node_id": "esp32", "label": "ESP32 sensor", "pin": "17"}) {
			t.Fatal("harmless metadata (pin=17) was withheld")
		}
	})
}

func TestKGSemanticPolicyStillBlocksCredentials(t *testing.T) {
	security.RegisterSensitive(kgPolicyRegisteredSecret)
	tests := []struct {
		name    string
		content string
	}{
		{"password with digits", kgBuiltContent("Router", "password: hunter2")},
		{"password followed by prose", kgBuiltContent("Router", "password: hunter2 for the router")},
		{"github token", kgBuiltContent("Bot", "token: ghp_"+strings.Repeat("aB3dE5gH7j", 3)+"aB3dE5")},
		{"openai style key", kgBuiltContent("Service", "api_key: sk-"+strings.Repeat("aB3dE5gH", 3))},
		{"door pin", kgBuiltContent("Front door", "pin: 4711")},
		{"long numeric pin", kgBuiltContent("Front door", "pin: 12345678")},
		{"digits make a pin even with a word", kgBuiltContent("Front door", "pin: ab1234")},
		{"fragmented secret", kgBuiltContent("Vault", "secret: s k - 1 2 3 4 5 6 7 8")},
		{"fragmented password", kgBuiltContent("Vault", "password: s k - 1 2 3 4 5 6 7 8")},
		{"fragmented auth", kgBuiltContent("Vault", "auth: s k - 1 2 3 4 5 6 7 8")},
		{"fragmented pin", kgBuiltContent("Vault", "pin: 1 2 3 4 5 6 7 8")},
		{"url credentials", kgBuiltContent("Mirror", "url: https://user:pass@host")},
		{"url credentials behind a keyword", kgBuiltContent("Mirror", "token: https://admin:s3cret@host/path")},
		{"registered secret", kgBuiltContent("Node", "notes: "+kgPolicyRegisteredSecret)},
		{"private key header", kgBuiltContent("Host", "-----BEGIN RSA PRIVATE KEY-----")},
		{"aws access key", kgBuiltContent("Cloud", "notes: AKIAIOSFODNN7EXAMPLE")},
		{"tailscale key", kgBuiltContent("Mesh", "notes: tskey-auth-"+strings.Repeat("a1B2c3D4", 3))},
		{"env style credential", kgBuiltContent("App", "MYAPP_PASSWORD=hunter2")},
		{"bearer token", kgBuiltContent("API", "header: Bearer "+strings.Repeat("aB3dE5gH", 4))},
		{"hex secret", kgBuiltContent("Vault", "secret: 00112233445566778899aabbccddeeff")},
		{"hex auth", kgBuiltContent("Vault", "auth: 00112233445566778899aabbccddeeff")},
		{"base64 token", kgBuiltContent("Vault", "token: dGhpcy1pcy1hLXNlY3JldC12YWx1ZQ==")},
		{"long letters-only password", kgBuiltContent("Vault", "password: "+strings.Repeat("qzxw", 8))},
		{"one harmless and one real", kgBuiltContent("Router", "token: rotated weekly", "password: hunter2")},
		{"equals form", kgBuiltContent("Router", "password=hunter2")},
		// The value is judged up to the end of the line, not by its first word.
		{"credential after a plain word", kgBuiltContent("Router", "password: is hunter2")},
		{"credential in a sentence", kgBuiltContent("Router", "password: changed to Tr0ub4dor&3")},
		{"credential behind a prefixed keyword", kgBuiltContent("Router", "wifi password: new Sommer2024!")},
		{"secret in a sentence", kgBuiltContent("Router", "secret: the value is abc123XYZ")},
		{"hex token in a sentence", kgBuiltContent("Router", "token: see 9f8e7d6c5b4a39281706f5e4d3c2b1a0")},
		{"basic auth header value", kgBuiltContent("Router", "auth: basic Zm9vOmJhcg==")},
		{"bearer scheme with a short value", kgBuiltContent("API", "auth: bearer abc123")},
		{"digest scheme with a short value", kgBuiltContent("API", "auth: DIGEST abc123")},
		{"random mixed-case string", kgBuiltContent("Router", "password: XkqTzRmPwLbQ")},
		{"capitalized word with digits", kgBuiltContent("Router", "password: same as wifi Hunter2024")},
		{"pin digits inside a sentence", kgBuiltContent("Front door", "pin: set to 4711")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if kgSemanticPolicyAllowed(tt.content, nil) {
				t.Fatalf("credential content reached the semantic index:\n%s", tt.content)
			}
		})
	}

	t.Run("metadata value", func(t *testing.T) {
		if kgSemanticPolicyAllowed("Router", map[string]string{"label": "Router", "password": "hunter2"}) {
			t.Fatal("credential in metadata reached the semantic index")
		}
	})
	t.Run("metadata pin with digits", func(t *testing.T) {
		if kgSemanticPolicyAllowed("Door", map[string]string{"pin": "4711"}) {
			t.Fatal("door PIN in metadata reached the semantic index")
		}
	})
}

// A node whose only credential-like word is a property name must be embedded and
// found again by semantic search, also after the index was rebuilt (restart).
func TestKGSemanticIndexKeepsNodesWithCredentialWordsSearchable(t *testing.T) {
	kg, rec := newSemanticRecorderKG(t)
	idx := kg.semanticIndex()
	ctx := t.Context()

	node := Node{ID: "esp32-sensor", Label: "ESP32 sensor", Properties: map[string]string{"type": "device", "pin": "17"}}
	if err := kg.AddNode(node.ID, node.Label, node.Properties); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := kg.upsertSemanticNodeIndexContext(ctx, node); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !semanticCollectionHas(t, idx.Collection, node.ID) || !semanticContentCached(kg, node.ID) {
		t.Fatal("node with 'pin: 17' was not embedded")
	}
	if !slices.ContainsFunc(rec.snapshot(), func(text string) bool { return strings.Contains(text, "pin: 17") }) {
		t.Fatalf("provider never received the node text: %q", rec.snapshot())
	}
	if ids := kg.semanticSearchNodeIDs("query: esp32 sensor pin", 5); !slices.Contains(ids, node.ID) {
		t.Fatalf("semantic search = %v, want it to find %s", ids, node.ID)
	}

	// Restart: the index is rebuilt from the graph rows.
	if err := idx.Collection.Delete(ctx, nil, nil, node.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = NULL`); err != nil {
		t.Fatal(err)
	}
	if err := kg.RunSemanticReindexContext(ctx); err != nil {
		t.Fatalf("RunSemanticReindexContext: %v", err)
	}
	if ids := kg.semanticSearchNodeIDs("query: esp32 sensor pin", 5); !slices.Contains(ids, node.ID) {
		t.Fatalf("after reindex semantic search = %v, want it to find %s", ids, node.ID)
	}

	// Edges: "token: rotated" is prose too.
	edge := Edge{Source: "esp32-sensor", Target: "gateway", Relation: "uses", Properties: map[string]string{"token": "rotated"}}
	if err := kg.upsertSemanticEdgeIndexContext(ctx, edge); err != nil {
		t.Fatalf("edge upsert: %v", err)
	}
	if ids := kg.semanticSearchEdgeIDs("query: esp32 sensor uses gateway", 5); len(ids) != 1 {
		t.Fatalf("edge search = %v, want the 'token: rotated' edge", ids)
	}
}

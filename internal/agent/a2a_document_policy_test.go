package agent

import (
	"context"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestA2ADocumentToolsAreReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled, cfg.VirtualDesktop.AllowAgentControl = true, true
	cfg.Tools.VirtualDesktop.Enabled = true
	cfg.Tools.OfficeDocument.Enabled, cfg.Tools.OfficeWorkbook.Enabled = true, true
	cfg.Tools.DocumentCreator.Enabled = true
	peer := &DispatchContext{Cfg: cfg, SessionID: "a2a-documents", IsCoAgent: true, MessageSource: "a2a"}
	var schemas []openai.Tool
	for tool, ops := range map[string][]string{
		"document_creator": {"create_pdf", "create_docx", "merge_pdfs", "unknown", ""},
		"office_document":  {"read", "read_document", "write", "patch", "export", "insert_image", "unknown", ""},
		"office_workbook":  {"read", "read_workbook", "evaluate_formula", "write", "set_cell", "export", "insert_image", "unknown", ""},
		"desktop_notes":    {"list", "search", "read", "create", "unknown", ""},
	} {
		for _, op := range ops {
			call := ToolCall{Action: tool, Operation: op, Params: map[string]interface{}{"operation": op}}
			read := tool == "office_document" && oneOf(op, "read", "read_document") || tool == "office_workbook" && oneOf(op, "read", "read_workbook", "evaluate_formula") || tool == "desktop_notes" && oneOf(op, "list", "search", "read")
			if got := roleToolRestriction(call, peer); (got == "") != read {
				t.Fatalf("%s/%s read=%v: %s", tool, op, read, got)
			}
			if !read {
				if result := checkDispatchActionPolicy(context.Background(), call, peer); result == nil || !strings.Contains(result.Output, "policy_denied") {
					t.Fatalf("%s/%s reached dispatch: %+v", tool, op, result)
				}
			}
			for _, local := range []*DispatchContext{{SessionID: "default"}, {SessionID: "coagent-local", IsCoAgent: true}} {
				if got := roleToolRestriction(call, local); got != "" {
					t.Fatalf("local %s/%s denied: %s", tool, op, got)
				}
			}
		}
		schemas = append(schemas, openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
			Name: tool, Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"operation": map[string]interface{}{"type": "string", "enum": ops},
			}},
		}})
	}
	catalog := BuildToolCatalog(schemas, nil, "")
	applyCatalogEntryPolicies(catalog, peer)
	for tool, want := range map[string][]string{
		"office_document": {"read", "read_document"},
		"office_workbook": {"read", "read_workbook", "evaluate_formula"},
		"desktop_notes":   {"list", "search", "read"},
	} {
		entry, ok := catalog.Get(tool)
		if !ok || !entry.Enabled || !slices.Equal(toolCatalogOperationField(entry, "operation"), want) {
			t.Fatalf("%s discovery mismatch: %+v", tool, entry)
		}
	}
	if entry, ok := catalog.Get("document_creator"); ok && entry.Enabled {
		t.Fatal("document creation exposed to peer")
	}
	for tool, op := range map[string]string{"office_document": "write", "office_workbook": "set_cell", "desktop_notes": "create"} {
		call := ToolCall{Action: tool, Operation: "read", Params: map[string]interface{}{"operation": op}}
		if got := roleToolRestriction(call, peer); !strings.Contains(got, "policy_denied") {
			t.Fatalf("parameter operation bypassed role policy: %s: %s", tool, got)
		}
	}
}

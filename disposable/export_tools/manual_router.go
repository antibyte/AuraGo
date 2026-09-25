package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"aurago/internal/agent"
	"aurago/internal/prompts"
	openai "github.com/sashabaranov/go-openai"
)

type routingManual struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Body        string   `json:"body"`
	Tools       []string `json:"tools"`
	Description string   `json:"description"`
}

type routingTool struct {
	ToolExport
	ManualID      string       `json:"manual_id,omitempty"`
	AbsenceReason string       `json:"absence_reason,omitempty"`
	Contract      ToolContract `json:"contract"`
	Aliases       []string     `json:"aliases,omitempty"`
	Category      string       `json:"category,omitempty"`
}

type routingCatalog struct {
	Version       int               `json:"version"`
	CatalogSHA256 string            `json:"catalog_sha256"`
	Sources       map[string]string `json:"sources"`
	Tools         []routingTool     `json:"tools"`
	Manuals       []routingManual   `json:"manuals"`
}

func routingDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func buildRoutingCatalog(root, source string) (routingCatalog, *agent.ToolCatalog, error) {
	result := routingCatalog{Version: 1, Sources: map[string]string{}}
	tools, err := loadStrictTools(root)
	if err != nil {
		return result, nil, err
	}
	var tiers TierManifest
	var contracts OperationContractManifest
	if err := readJSON(filepath.Join(source, "tool_tiers.json"), &tiers); err != nil {
		return result, nil, fmt.Errorf("read routing tiers: %w", err)
	}
	if err := applyTiers(tools, tiers); err != nil {
		return result, nil, err
	}
	if err := readJSON(filepath.Join(source, "operation_contracts.json"), &contracts); err != nil {
		return result, nil, fmt.Errorf("read routing operation contracts: %w", err)
	}
	if _, err := validateContracts(tools, contracts); err != nil {
		return result, nil, err
	}
	var schemas []openai.Tool
	for _, tool := range tools {
		schemas = append(schemas, openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters}})
	}
	search := agent.BuildToolCatalog(schemas, schemas, filepath.Join(root, "prompts"))
	manuals := map[string]*routingManual{}
	for _, tool := range tools {
		entry := routingTool{ToolExport: tool, Contract: contracts.Tools[tool.Name]}
		if metadata, ok := search.Get(tool.Name); ok {
			entry.Aliases, entry.Category = metadata.Aliases, metadata.Category
		}
		id := prompts.ToolManualID(tool.Name)
		rel := filepath.ToSlash(filepath.Join("prompts", "tools_manuals", id+".md"))
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			entry.AbsenceReason = prompts.ToolManualAbsenceReason(tool.Name)
			if !os.IsNotExist(err) || entry.AbsenceReason == "" {
				return result, nil, fmt.Errorf("read canonical manual for %s: %w", tool.Name, err)
			}
		} else {
			body := strings.ReplaceAll(string(data), "\r\n", "\n")
			if !utf8.ValidString(body) || strings.TrimSpace(body) == "" || strings.HasPrefix(body, "\ufeff") {
				return result, nil, fmt.Errorf("canonical manual %s is empty or invalid UTF-8", rel)
			}
			entry.ManualID, entry.ManualPath, entry.ManualSnippet = id, rel, ""
			manual := manuals[id]
			if manual == nil {
				manual = &routingManual{ID: id, Path: rel, Body: body, SHA256: routingDigest([]byte(body)), Description: tool.Description}
				manuals[id] = manual
			}
			manual.Tools = append(manual.Tools, tool.Name)
			result.Sources[rel] = manual.SHA256
		}
		result.Tools = append(result.Tools, entry)
	}
	for _, manual := range manuals {
		sort.Strings(manual.Tools)
		result.Manuals = append(result.Manuals, *manual)
	}
	sort.Slice(result.Manuals, func(i, j int) bool { return result.Manuals[i].ID < result.Manuals[j].ID })
	for _, path := range []string{"tool_tiers.json", "operation_contracts.json"} {
		data, err := os.ReadFile(filepath.Join(source, path))
		if err != nil {
			return result, nil, err
		}
		result.Sources["training/"+path] = routingDigest(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return result, nil, err
	}
	result.CatalogSHA256 = routingDigest(encoded)
	return result, search, nil
}

func runManualRouter(root string, opts options) error {
	catalog, search, err := buildRoutingCatalog(root, opts.sourceDir)
	if err != nil {
		return err
	}
	if opts.routerSearch {
		return serveRoutingSearch(os.Stdin, os.Stdout, search, catalog)
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("encode routing catalog: %w", err)
	}
	data = append(data, '\n')
	if opts.check {
		existing, err := os.ReadFile(opts.routerOut)
		if err != nil || !bytes.Equal(existing, data) {
			return fmt.Errorf("manual routing catalog is missing or stale: %s", opts.routerOut)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(opts.routerOut), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(opts.routerOut, data, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "Canonical manual catalog: %d tools, %d manuals, sha256=%s\n", len(catalog.Tools), len(catalog.Manuals), catalog.CatalogSHA256)
	return nil
}

func serveRoutingSearch(input io.Reader, output io.Writer, search *agent.ToolCatalog, catalog routingCatalog) error {
	known := map[string]bool{}
	for _, manual := range catalog.Manuals {
		known[manual.ID] = true
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	writer := json.NewEncoder(output)
	for scanner.Scan() {
		var request struct {
			Query   string   `json:"query"`
			Allowed []string `json:"allowed_manuals"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			return fmt.Errorf("decode routing search: %w", err)
		}
		allowed := known
		if request.Allowed != nil {
			allowed = map[string]bool{}
			for _, id := range request.Allowed {
				if !known[id] {
					return fmt.Errorf("unknown allowed manual %q", id)
				}
				allowed[id] = true
			}
		}
		ids, seen := []string{}, map[string]bool{}
		for _, entry := range search.Search(request.Query) {
			id := prompts.ToolManualID(entry.Name)
			if entry.Enabled && allowed[id] && !seen[id] {
				ids, seen[id] = append(ids, id), true
			}
		}
		if err := writer.Encode(map[string]interface{}{"manual_ids": ids, "catalog_sha256": catalog.CatalogSHA256}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

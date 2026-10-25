package agent

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/tools"
)

// yepAPIOperationSource binds one native YepAPI tool to the dispatcher that
// serves it and to the file that holds that dispatcher.
type yepAPIOperationSource struct {
	tool       string
	sourceFile string
	dispatch   func(context.Context, *tools.YepAPIClient, string, map[string]interface{}) (string, error)
}

// yepAPIOperationSources lists the seven native YepAPI tools. Every dispatcher
// names its operations as string literals in its own file, so the files hold a
// complete list of operation names the dispatchers can answer.
var yepAPIOperationSources = []yepAPIOperationSource{
	{"yepapi_seo", "yepapi_seo.go", tools.DispatchYepAPISEO},
	{"yepapi_serp", "yepapi_serp.go", tools.DispatchYepAPISERP},
	{"yepapi_scrape", "yepapi_scrape.go", tools.DispatchYepAPIScrape},
	{"yepapi_youtube", "yepapi_youtube.go", tools.DispatchYepAPIYouTube},
	{"yepapi_tiktok", "yepapi_tiktok.go", tools.DispatchYepAPITikTok},
	{"yepapi_instagram", "yepapi_instagram.go", tools.DispatchYepAPIInstagram},
	{"yepapi_amazon", "yepapi_amazon.go", tools.DispatchYepAPIAmazon},
}

// yepAPIManualPath is the manual that documents the operations of every tool.
const yepAPIManualPath = "../../prompts/tools_manuals/yepapi.md"

// yepAPIOperationNamePattern matches the shape of an operation name.
var yepAPIOperationNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// yepAPIBacktickPattern finds the names the manual writes in code quotes.
var yepAPIBacktickPattern = regexp.MustCompile("`([^`]+)`")

// TestYepAPIOperationsMatchSchemaDispatcherAndManual locks the three places
// that name a YepAPI operation together: the enum of the native tool schema
// the model sees, the dispatcher that answers the operation and the manual
// that documents it. A new operation that only lands in one of the three
// would send the model into an unknown operation or document an operation
// that no longer exists, so the three sets have to be equal per tool.
func TestYepAPIOperationsMatchSchemaDispatcherAndManual(t *testing.T) {
	schemaOps := yepAPISchemaOperationSets(t)
	manualOps := yepAPIManualOperationSets(t)
	candidates := yepAPIDispatcherOperationCandidates(t)

	known := map[string]bool{}
	for _, src := range yepAPIOperationSources {
		known[src.tool] = true
	}
	for tool := range schemaOps {
		if !known[tool] {
			t.Errorf("%s: the native tool schema has no dispatcher in this test; add it to yepAPIOperationSources", tool)
		}
	}

	for _, src := range yepAPIOperationSources {
		tool := src.tool

		schema, ok := schemaOps[tool]
		if !ok {
			t.Errorf("%s: the native tool schema has no operation enum", tool)
			continue
		}
		manual, ok := manualOps[tool]
		if !ok {
			t.Errorf("%s: %s has no operation list", tool, yepAPIManualPath)
			continue
		}
		dispatcher := yepAPIAcceptedOperations(t, src, candidates)

		if missing, extra := yepAPIOperationDifference(yepAPIOperationSet(schema), dispatcher); len(missing) > 0 || len(extra) > 0 {
			t.Errorf("%s: the schema and the dispatcher disagree; missing in the dispatcher: %v; unknown to the schema: %v", tool, missing, extra)
		}
		if missing, extra := yepAPIOperationDifference(yepAPIOperationSet(schema), yepAPIOperationSet(manual)); len(missing) > 0 || len(extra) > 0 {
			t.Errorf("%s: the schema and the manual disagree; missing in the manual: %v; unknown to the schema: %v", tool, missing, extra)
		}
	}
}

// yepAPISchemaOperationSets reads the operation enum of every native YepAPI
// tool schema.
func yepAPISchemaOperationSets(t *testing.T) map[string][]string {
	t.Helper()
	sets := map[string][]string{}
	for _, toolSchema := range BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil) {
		if toolSchema.Function == nil || !strings.HasPrefix(toolSchema.Function.Name, "yepapi_") {
			continue
		}
		name := toolSchema.Function.Name
		params, ok := toolSchema.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("%s: the tool parameters are not a schema map", name)
		}
		properties, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: the tool schema has no properties", name)
		}
		operation, ok := properties["operation"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: the tool schema has no operation property", name)
		}
		enum, ok := operation["enum"].([]string)
		if !ok {
			t.Fatalf("%s: the operation property has no enum", name)
		}
		sets[name] = enum
	}
	return sets
}

// yepAPIManualOperationSets reads the operation list of every tool section of
// the manual.
func yepAPIManualOperationSets(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(yepAPIManualPath)
	if err != nil {
		t.Fatalf("read %s: %v", yepAPIManualPath, err)
	}
	sets := map[string][]string{}
	section := ""
	inOperations := false
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			inOperations = false
			continue
		}
		if !strings.HasPrefix(section, "yepapi_") {
			continue
		}
		if trimmed == "**Operations:**" {
			inOperations = true
			continue
		}
		if strings.HasPrefix(trimmed, "**") {
			inOperations = false
			continue
		}
		if !inOperations || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		for _, name := range yepAPIBacktickPattern.FindAllStringSubmatch(yepAPIOperationBulletHead(trimmed), -1) {
			sets[section] = append(sets[section], name[1])
		}
	}
	return sets
}

// yepAPIOperationBulletHead returns the part of an operation bullet that
// names the operations. The description behind the dash is left out, because
// it may mention other operations as well.
func yepAPIOperationBulletHead(bullet string) string {
	head := bullet
	for _, sep := range []string{"—", " - "} {
		if idx := strings.Index(head, sep); idx >= 0 && idx < len(head) {
			head = head[:idx]
		}
	}
	return head
}

// yepAPIDispatcherOperationCandidates collects every operation name the
// dispatchers can name. The candidates come from the string literals of the
// dispatcher files, so the probe below sees every operation a dispatcher
// could answer.
func yepAPIDispatcherOperationCandidates(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, src := range yepAPIOperationSources {
		path := filepath.Join("..", "tools", src.sourceFile)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			if yepAPIOperationNamePattern.MatchString(value) {
				seen[value] = true
			}
			return true
		})
	}
	candidates := make([]string, 0, len(seen))
	for name := range seen {
		candidates = append(candidates, name)
	}
	sort.Strings(candidates)
	return candidates
}

// yepAPIAcceptedOperations asks the dispatcher which operations it accepts.
// The probe runs with a cancelled context, so an accepted operation stops at
// the request that is never sent and an unknown operation is reported as
// such. No request leaves the test.
func yepAPIAcceptedOperations(t *testing.T, src yepAPIOperationSource, candidates []string) map[string]bool {
	t.Helper()
	accepted := map[string]bool{}
	client := tools.NewYepAPIClientWithBaseURL("yepapi-operation-parity-"+src.tool, "https://operations.invalid")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	args := yepAPIOperationProbeArgs()
	for _, candidate := range candidates {
		_, err := src.dispatch(ctx, client, candidate, args)
		if err != nil && strings.Contains(err.Error(), "unknown "+src.tool+" operation") {
			continue
		}
		accepted[candidate] = true
	}
	return accepted
}

// yepAPIOperationProbeArgs holds a usable value for every parameter a YepAPI
// operation may read, so a probe never fails on a missing argument.
func yepAPIOperationProbeArgs() map[string]interface{} {
	return map[string]interface{}{
		"query":           "aurago",
		"search_query":    "aurago",
		"keywords":        []string{"aurago"},
		"seed":            "aurago",
		"domain":          "example.com",
		"target":          "example.com",
		"url":             "https://example.com/page",
		"tag":             "golang",
		"shortcode":       "abc",
		"username":        "user",
		"username_or_url": "user",
		"video_id":        "vid",
		"channel_id":      "chan",
		"playlist_id":     "pl",
		"post_id":         "post",
		"comment_id":      "123",
		"name":            "dance",
		"asin":            "B000",
		"category":        "electronics",
		"handle":          "creator",
		"seller_id":       "SELLER",
		"selector":        "h1",
		"xpath":           "//h1",
		"format":          "markdown",
		"prompt":          "extract",
		"limit":           float64(5),
		"depth":           float64(5),
		"page":            float64(1),
		"location":        "us",
		"language":        "en",
		"country":         "US",
		"sort_by":         "TOP_REVIEWS",
	}
}

// yepAPIOperationSet turns a list of operation names into a set.
func yepAPIOperationSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, name := range values {
		set[name] = true
	}
	return set
}

// yepAPIOperationDifference returns the operations of want that missing does
// not hold and the operations of missing that want does not hold.
func yepAPIOperationDifference(want, missing map[string]bool) ([]string, []string) {
	var absent, extra []string
	for name := range want {
		if !missing[name] {
			absent = append(absent, name)
		}
	}
	for name := range missing {
		if !want[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(absent)
	sort.Strings(extra)
	return absent, extra
}

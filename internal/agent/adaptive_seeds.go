package agent

// adaptiveFamilySeedLimit is how many seeds a family contributes. The seeds of
// a family are listed by importance (adaptiveFamilySeedTools), and only the
// first ones that the selection can offer count: a long list would let a
// generic family cue crowd out the tools the request is really about.
const adaptiveFamilySeedLimit = 4

// adaptiveIntentSeedRule adds tools when the message holds a specific cue:
// naming CPU or RAM, a container, a PDF, a camera, MCP, Composio or Manus.
type adaptiveIntentSeedRule struct {
	match func(toolIntentText) bool
	tools []string
}

var adaptiveIntentSeedRules = []adaptiveIntentSeedRule{
	{isResourceUsageIntent, []string{"system_metrics", "process_analyzer", "execute_shell"}},
	{isContainerIntent, []string{"docker", "execute_shell"}},
	{isPDFIntent, []string{"pdf_operations", "document_creator"}},
	{isNetworkCameraIntent, []string{"go2rtc"}},
	{isMCPIntent, []string{"mcp_call"}},
	{isComposioIntent, []string{"composio_call"}},
	{isManusIntent, []string{"manus"}},
}

// adaptiveSeedsForQuery returns the tools a request makes likely before any
// ranking: the tools of the specific cues it holds, then the first
// adaptiveFamilySeedLimit seeds of its family. offered says which tools the
// selection can use (nil: all); a tool it rejects neither appears nor counts
// toward the limit. The seeds are candidates for the fused ranking
// (buildAdaptiveToolPriority), not a reserved class: they compete for the
// adaptive slots with every other signal.
func adaptiveSeedsForQuery(userQuery string, offered func(string) bool) []string {
	text := newToolIntentText(userQuery)
	var out []string
	seen := make(map[string]bool)
	add := func(name string) {
		if name == "" || seen[name] || (offered != nil && !offered(name)) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, rule := range adaptiveIntentSeedRules {
		if rule.match(text) {
			for _, tool := range rule.tools {
				add(tool)
			}
		}
	}
	if family := toolFamilyOfText(text); family != "" {
		taken := 0
		for _, seed := range adaptiveFamilySeedTools[family] {
			if taken == adaptiveFamilySeedLimit {
				break
			}
			if offered != nil && !offered(seed) {
				continue
			}
			taken++
			add(seed)
		}
	}
	return out
}

// adaptiveFamilySeedsForQuery is adaptiveSeedsForQuery for a selection that
// offers every tool.
func adaptiveFamilySeedsForQuery(userQuery string) []string {
	return adaptiveSeedsForQuery(userQuery, nil)
}

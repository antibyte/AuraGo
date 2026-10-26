package agent

import "testing"

// toolSearchBenchQueries mix short keywords, task sentences and German text.
var toolSearchBenchQueries = []string{
	"monitor cpu temperature",
	"send an email to my boss",
	"Licht im Wohnzimmer einschalten",
	"docker container logs",
	"resize an image",
	"Bitte erstelle einen Kalendereintrag für morgen um zehn Uhr",
	"play an old arcade game",
	"search the web for news",
	"Ich möchte meine Notizen durchsuchen",
	"ssh to a remote server and restart a service",
	"scan open ports on a host",
	"Kannst du mir bitte die Festplattenbelegung auf dem Server anzeigen und sagen welche Dateien am meisten Platz belegen",
}

func BenchmarkToolCatalogSearchAllTools(b *testing.B) {
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	catalog := BuildToolCatalog(schemas, nil, "")
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		catalog.Search(toolSearchBenchQueries[i%len(toolSearchBenchQueries)])
	}
}

func BenchmarkBuildAdaptiveToolPriorityAllTools(b *testing.B) {
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		buildAdaptiveToolPriority(schemas, nil, toolSearchBenchQueries[i%len(toolSearchBenchQueries)], nil, nil)
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

// An upgraded config gets the whole local_wikipedia section from the template;
// explicit user values survive the merge.
func TestRepositoryTemplateMergeAddsLocalWikipediaDefaults(t *testing.T) {
	for _, tc := range []struct {
		name string
		user string
		want map[string]interface{}
		// loaded is what config.Load returns for the user's own file and, after
		// the upgrade, for the merged file.
		loaded config.LocalWikipediaConfig
	}{
		{
			name: "section missing",
			user: "server:\n    port: 8088\n",
			want: map[string]interface{}{
				"local_wikipedia.enabled": false, "local_wikipedia.agent_access": true, "local_wikipedia.language": "",
				"local_wikipedia.variant": "nopic", "local_wikipedia.data_dir": "", "local_wikipedia.update_check": true,
			},
			loaded: config.LocalWikipediaConfig{AgentAccess: true, Variant: "nopic", UpdateCheck: true},
		},
		{
			name: "explicit values",
			user: "local_wikipedia:\n    enabled: true\n    agent_access: false\n    variant: maxi\n    language: de\n",
			want: map[string]interface{}{
				"local_wikipedia.enabled": true, "local_wikipedia.agent_access": false, "local_wikipedia.language": "de",
				"local_wikipedia.variant": "maxi", "local_wikipedia.update_check": true,
			},
			loaded: config.LocalWikipediaConfig{Enabled: true, Language: "de", Variant: "maxi", UpdateCheck: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, tmplMap := repositoryTemplate(t)
			srcMap, err := parseYAMLMap(tc.user)
			if err != nil {
				t.Fatalf("parse user config: %v", err)
			}
			res := mergeUserConfig(tmplMap, srcMap)
			for dotted, value := range tc.want {
				if got := mergedValue(res.merged, dotted); got != value {
					t.Fatalf("%s = %#v, want %#v", dotted, got, value)
				}
			}
			assertMergeMatchesLoad(t, tc.user, res.merged)

			dir := t.TempDir()
			mergedPath := filepath.Join(dir, "merged.yaml")
			atomicWriteYAML(mergedPath, res.merged)
			merged, err := config.Load(mergedPath)
			if err != nil {
				t.Fatalf("Load(merged): %v", err)
			}
			sourcePath := filepath.Join(dir, "source.yaml")
			if err := os.WriteFile(sourcePath, []byte(tc.user), 0o600); err != nil {
				t.Fatal(err)
			}
			source, err := config.Load(sourcePath)
			if err != nil {
				t.Fatalf("Load(source): %v", err)
			}
			if source.LocalWikipedia != tc.loaded {
				t.Fatalf("Load(source).LocalWikipedia = %+v, want %+v", source.LocalWikipedia, tc.loaded)
			}
			if merged.LocalWikipedia != source.LocalWikipedia {
				t.Fatalf("Load(merged).LocalWikipedia = %+v, the user's own file loads as %+v", merged.LocalWikipedia, source.LocalWikipedia)
			}
		})
	}
}

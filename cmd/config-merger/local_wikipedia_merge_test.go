package main

import "testing"

// An upgraded config gets the whole local_wikipedia section from the template;
// explicit user values survive the merge.
func TestRepositoryTemplateMergeAddsLocalWikipediaDefaults(t *testing.T) {
	for _, tc := range []struct {
		name string
		user string
		want map[string]interface{}
	}{
		{
			name: "section missing",
			user: "server:\n    port: 8088\n",
			want: map[string]interface{}{
				"local_wikipedia.enabled": false, "local_wikipedia.agent_access": true, "local_wikipedia.language": "",
				"local_wikipedia.variant": "nopic", "local_wikipedia.data_dir": "", "local_wikipedia.update_check": true,
			},
		},
		{
			name: "explicit values",
			user: "local_wikipedia:\n    enabled: true\n    agent_access: false\n    variant: maxi\n    language: de\n",
			want: map[string]interface{}{
				"local_wikipedia.enabled": true, "local_wikipedia.agent_access": false, "local_wikipedia.language": "de",
				"local_wikipedia.variant": "maxi", "local_wikipedia.update_check": true,
			},
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
		})
	}
}

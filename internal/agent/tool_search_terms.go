package agent

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// The curated search terms of the tools live in tool_search_terms/*.yaml, one
// file per group of tool categories:
//
//	system.yaml               system
//	files.yaml                files
//	memory_data_apis.yaml     memory, data_apis
//	network_smart_home.yaml   network, smart_home
//	media_communication.yaml  media, communication
//	infrastructure_a.yaml     the first 27 infrastructure tools, sorted by name
//	infrastructure_b.yaml     the remaining infrastructure tools
//
// A file maps a tool name to the phrases a user would type for it, per
// language:
//
//	tool_name:
//	  en: ["phrase one", "phrase two"]
//	  de: ["Ausdruck eins", "Ausdruck zwei"]
//
// A tool name may appear in one file only.
//
//go:embed tool_search_terms/*.yaml
var toolSearchTermFiles embed.FS

const toolSearchTermsDir = "tool_search_terms"

// toolSearchTermLists are the phrases of one tool, per language.
type toolSearchTermLists struct {
	EN []string `yaml:"en"`
	DE []string `yaml:"de"`
}

// loadToolSearchTerms parses the embedded term files once. A broken file never
// stops the agent: the terms stay empty and TestToolSearchTermsLoad reports it.
var loadToolSearchTerms = sync.OnceValues(func() (map[string][]string, error) {
	terms, err := parseToolSearchTermFiles(toolSearchTermFiles, toolSearchTermsDir)
	if err != nil {
		slog.Error("tool search terms unavailable; discovery ranks without them", "error", err)
		return nil, err
	}
	return terms, nil
})

// toolSearchTermsFor returns the curated phrases of a tool, English first.
func toolSearchTermsFor(name string) []string {
	terms, _ := loadToolSearchTerms()
	return terms[name]
}

// parseToolSearchTermFiles reads every .yaml file in dir. Unknown fields and a
// tool name that appears twice, within a file or across files, are errors.
func parseToolSearchTermFiles(fsys fs.FS, dir string) (map[string][]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	terms := make(map[string][]string)
	owner := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".yaml" {
			continue
		}
		file, err := fsys.Open(path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		decoder := yaml.NewDecoder(file)
		decoder.KnownFields(true)
		var lists map[string]toolSearchTermLists
		err = decoder.Decode(&lists)
		file.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		names := make([]string, 0, len(lists))
		for name := range lists {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if previous, dup := owner[name]; dup {
				return nil, fmt.Errorf("%s: tool %q is already listed in %s", entry.Name(), name, previous)
			}
			owner[name] = entry.Name()
			phrases := make([]string, 0, len(lists[name].EN)+len(lists[name].DE))
			for _, phrase := range slices.Concat(lists[name].EN, lists[name].DE) {
				if phrase = strings.TrimSpace(phrase); phrase != "" {
					phrases = append(phrases, phrase)
				}
			}
			terms[name] = phrases
		}
	}
	return terms, nil
}

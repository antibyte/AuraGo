package config

import (
	"gopkg.in/yaml.v3"
	"strings"
)

func PersonalityMigrationNotices(data []byte) []string {
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	path := "personality.inner_voice.error_streak_min"
	if disclosureYAMLPath(doc.Content[0], strings.Split(path, "."), false) != nil {
		return []string{path + " (deprecated; ignored)"}
	}
	return nil
}

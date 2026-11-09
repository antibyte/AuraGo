package storymaker

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed data/env_direction.json
var envDirectionJSON []byte

// plainStageID is the neutral environment that the direction uses only when nothing else exists.
const plainStageID = "env.plain-stage"

// EnvDirection is the curated guide of one catalog environment. About is one line for the model, Tags are the words
// (English and German, matched as whole words or as prefixes of at least 4 letters) that mean the environment fits a
// topic, and Base marks an environment that fits many topics.
type EnvDirection struct {
	About string   `json:"about"`
	Tags  []string `json:"tags"`
	Base  bool     `json:"base,omitempty"`
}

var (
	envDirectionOnce sync.Once
	envDirectionData map[string]EnvDirection
)

// EnvironmentGuide returns the curated guide of the catalog environments, keyed by id. The map is shared: do not change it.
// The file is embedded and a test ties it to the catalog, so a decode error means a broken build and leaves the guide empty.
func EnvironmentGuide() map[string]EnvDirection {
	envDirectionOnce.Do(func() {
		envDirectionData = map[string]EnvDirection{}
		_ = json.Unmarshal(envDirectionJSON, &envDirectionData)
	})
	return envDirectionData
}

// environmentAbout is the one-line guide of an environment, or "".
func environmentAbout(id string) string {
	return EnvironmentGuide()[id].About
}

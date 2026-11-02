// Package promptsembed embeds the default prompt files into the binary.
// At startup, [EnsurePromptsDir] extracts them to the configured PromptsDir
// if the directory does not yet exist on the filesystem.
package promptsembed

import "embed"

// FS holds the embedded prompt files.
// Sub-directories (personalities, rules, templates, tools_manuals, persona_phases)
// are included automatically because embed.FS recurses into them. Directory
// patterns skip files starting with "_" or ".", so persona_phases/_generic.yaml
// is named explicitly.
//
//go:embed *.md personalities rules templates tools_manuals self_tests persona_phases persona_phases/_generic.yaml
var FS embed.FS

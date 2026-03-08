// Package promptsembed embeds the default prompt files into the binary.
// At startup, [EnsurePromptsDir] extracts them to the configured PromptsDir
// if the directory does not yet exist on the filesystem.
package promptsembed

import "embed"

// FS holds the embedded prompt files.
// Sub-directories (personalities, templates, tools_manuals) are included
// automatically because embed.FS recurses into them.
//
//go:embed *.md personalities templates tools_manuals
var FS embed.FS

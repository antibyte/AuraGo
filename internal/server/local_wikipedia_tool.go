package server

import (
	"aurago/internal/localwiki"
	"aurago/internal/tools"
)

// publishLocalWikipediaTool hands the server-owned edition manager to the
// agent's local_wikipedia tool; nil withdraws it before shutdown.
func publishLocalWikipediaTool(m *localwiki.Manager) {
	tools.SetLocalWikipediaSource(tools.LocalWikipediaManagerSource(m))
}

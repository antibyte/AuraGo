package server

import (
	"aurago/internal/localwiki"
	"aurago/internal/tools"
)

// publishLocalWikipediaTool hands the server-owned edition manager to the
// agent's local_wikipedia tool; nil clears the published source.
func publishLocalWikipediaTool(m *localwiki.Manager) {
	tools.SetLocalWikipediaSource(tools.LocalWikipediaManagerSource(m))
}

// withdrawLocalWikipediaTool withdraws the edition source before shutdown,
// but only while it is still the one this server published (go2rtc pattern).
func withdrawLocalWikipediaTool(m *localwiki.Manager) {
	tools.WithdrawLocalWikipediaManager(m)
}

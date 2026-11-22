package server

import "strings"

// configRestartReason names a settings group whose saved change only takes
// effect after AuraGo restarts. The config save answer lists the IDs in
// restart_reasons; the config UI translates each one through
// config.save_bar.restart_reason.<id> in ui/lang/config/common. Label is the
// English text for the legacy restart_reason list and the fallback message.
type configRestartReason struct {
	ID    string
	Label string
}

// configRestartReasons lists every reason a config save can report, so the
// locale test can require a translation for each one.
var configRestartReasons []configRestartReason

func newConfigRestartReason(id, label string) configRestartReason {
	reason := configRestartReason{ID: id, Label: label}
	configRestartReasons = append(configRestartReasons, reason)
	return reason
}

var (
	restartReasonReloadFailed     = newConfigRestartReason("reload_failed", "Configuration reload (parse error)")
	restartReasonAgentConcurrency = newConfigRestartReason("agent_concurrency", "Agent concurrency")
	restartReasonSkillManager     = newConfigRestartReason("skill_manager", "Skill Manager")
	restartReasonServer           = newConfigRestartReason("server", "Server (Host/Port)")
	restartReasonDatabases        = newConfigRestartReason("databases", "Databases")
	restartReasonDirectories      = newConfigRestartReason("directories", "Directories")
	restartReasonGameMakerRuntime = newConfigRestartReason("game_maker_runtime", "Game Maker runtime")
	restartReasonGameMakerService = newConfigRestartReason("game_maker_service", "Game Maker service")
	restartReasonCYDBuilder       = newConfigRestartReason("cyd_creator_builder", "CYD Creator builder")
	restartReasonCYDService       = newConfigRestartReason("cyd_creator_service", "CYD Creator service")
	restartReasonFlows            = newConfigRestartReason("flows", "EasyDrag flows")
	restartReasonEmbeddings       = newConfigRestartReason("embeddings", "Embeddings / long-term memory")
	restartReasonCorpusMaxSize    = newConfigRestartReason("embeddings_corpus_max_size_mb", "Embeddings corpus limit")
	restartReasonChromecast       = newConfigRestartReason("chromecast", "Chromecast/TTS Server")
	restartReasonWebhooks         = newConfigRestartReason("webhooks", "Webhooks (enabled/disabled)")
	restartReasonDaemonSkills     = newConfigRestartReason("daemon_skills", "Daemon Skills")
	restartReasonSudoUnrestricted = newConfigRestartReason("sudo_unrestricted", "Sudo system-wide write access (systemd unit update required)")
)

// configRestartAnswer builds the restart fields of the config save answer:
// stable IDs for the UI, the legacy English labels and the English message.
func configRestartAnswer(reasons []configRestartReason) (ids, labels []string, message string) {
	ids = make([]string, 0, len(reasons))
	labels = make([]string, 0, len(reasons))
	for _, reason := range reasons {
		ids = append(ids, reason.ID)
		labels = append(labels, reason.Label)
	}
	return ids, labels, "Saved. Restart required for: " + strings.Join(labels, ", ")
}

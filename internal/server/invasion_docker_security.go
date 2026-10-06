package server

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/invasion"
)

// invasionDockerRemotePlaintextAdvice ends the plaintext hint.
const invasionDockerRemotePlaintextAdvice = "Use these nests only on an isolated network, set Docker TLS (tls or mtls) on the nest, or switch to the docker_ssh deploy method (Docker via SSH)."

// invasionSecurityHints reports nest settings that expose deployment secrets.
// CheckSecurity only sees config.yaml; nests live in the invasion database,
// so the hints endpoint appends these. A failed nest query is logged and
// yields no hints, so the endpoint still answers.
func invasionSecurityHints(db *sql.DB, logger *slog.Logger) []SecurityHint {
	if db == nil {
		return nil
	}
	nests, err := invasion.ListActiveNests(db)
	if err != nil {
		if logger != nil {
			logger.Warn("[Security] Invasion nest check for the security hints failed", "error", err)
		}
		return nil
	}
	var names []string
	for _, nest := range nests {
		if invasion.DockerRemotePlaintext(nest) {
			names = append(names, fmt.Sprintf("%q", nest.Name))
		}
	}
	if len(names) == 0 {
		return nil
	}
	return []SecurityHint{{
		ID:       "invasion_docker_remote_plaintext",
		Severity: SevWarning,
		Title:    "Invasion: Docker (Remote) nests deploy over unencrypted HTTP",
		Description: "Active nest(s) " + strings.Join(names, ", ") + " use deploy method docker_remote over plain HTTP. " +
			"Every hatch and reconfigure sends the egg configuration (shared key, egg vault key and, with inherit_llm, the master's LLM API key) in clear text, " +
			"and a Docker Engine without TLS accepts commands from every host that reaches it. " + invasionDockerRemotePlaintextAdvice,
		AutoFixable: false,
	}}
}

// warnPlaintextDockerRemote logs that an operation is about to send the egg
// configuration over unencrypted HTTP. The operation itself is not changed.
func warnPlaintextDockerRemote(logger *slog.Logger, nest invasion.NestRecord, operation string) {
	if logger == nil || !invasion.DockerRemotePlaintext(nest) {
		return
	}
	logger.Warn("[Invasion] Docker (Remote) nest uses unencrypted HTTP; the egg configuration and its secrets cross the network in clear text",
		"nest_id", nest.ID, "nest", nest.Name, "host", nest.Host, "operation", operation)
}

package desktopstore

import (
	_ "embed"
	"strconv"
	"time"
)

// Velxio (Arduino/ESP32/Pico simulator) ships a FastMCP server that the image's
// own entrypoint never starts. The Store starts it through an entrypoint
// wrapper, like DeepTutor, and keeps it behind a generated bearer token.
const (
	// VelxioAppID is the catalog ID of the Velxio simulator.
	VelxioAppID = "velxio"
	// VelxioMCPPortID is the loopback-only extra port that carries the MCP server.
	VelxioMCPPortID = "agent-mcp"

	velxioMCPPort   = 8002
	velxioTokenKey  = "mcp_token"
	velxioTokenEnv  = "AURAGO_VELXIO_MCP_TOKEN"
	velxioPortEnv   = "AURAGO_VELXIO_MCP_PORT"
	velxioScriptEnv = "AURAGO_VELXIO_MCP_PY"

	// A fresh start needs about 75 s before the web port answers, longer on
	// slow links because the Arduino cores are downloaded then.
	velxioReadinessTimeout = 15 * time.Minute
)

// velxioWrapper starts the launcher in a restart loop in the background, then
// replaces the shell with the image's own entrypoint. The image has no
// Entrypoint and its Cmd is dropped once the Store sets one, so "$@" is empty
// and the explicit exec is correct. The script and token are unset before the
// image entrypoint starts; only the restart-loop subshell keeps them. Python
// -I keeps /app and PYTHON* out of the import path (the script inserts /app
// itself).
const velxioWrapper = `( while :; do python -I -c "$AURAGO_VELXIO_MCP_PY"; sleep 5; done ) & unset AURAGO_VELXIO_MCP_PY AURAGO_VELXIO_MCP_TOKEN; exec /app/entrypoint.sh "$@"`

//go:embed velxio_assets/aurago_velxio_mcp.py
var velxioMCPLauncher string

// VelxioMCPTokenKey is the Vault key of the generated MCP bearer token.
func VelxioMCPTokenKey() string { return storeSecretVaultKey(VelxioAppID, velxioTokenKey) }

// velxioContainerSpec adds the launcher script, its port and the wrapper
// entrypoint. The token is injected earlier by addGeneratedRuntimeEnv.
func velxioContainerSpec(spec ContainerSpec) ContainerSpec {
	env := make([]string, 0, len(spec.Env)+2)
	for _, item := range spec.Env {
		if hasEnvKey(item, []string{velxioScriptEnv, velxioPortEnv}) {
			continue
		}
		env = append(env, item)
	}
	env = append(env, velxioPortEnv+"="+strconv.Itoa(velxioMCPPort), velxioScriptEnv+"="+velxioMCPLauncher)
	spec.Env = env
	spec.Entrypoint = []string{"/bin/sh", "-c", velxioWrapper, "aurago-velxio"}
	return spec
}

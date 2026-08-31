package virtualcomputers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"aurago/internal/config"
)

// GarageManager ensures/stops the managed Garage sidecar on the control-plane host
// via CommandExecutor (local or SSH).
type GarageManager struct {
	Executor    CommandExecutor
	InstallDir  string
	AccessKeyID string
	SecretKey   string
	RPCSecret   string
	Fingerprint string
}

// GenerateGarageSecrets returns cryptographically random Garage credentials.
// Access key: 32 hex chars; secret key: 64 hex chars; RPC secret: 32 raw bytes as hex (64 chars).
func GenerateGarageSecrets() (accessKeyID, secretKey, rpcSecret string, err error) {
	ak := make([]byte, 16)
	sk := make([]byte, 32)
	rpc := make([]byte, 32)
	if _, err = rand.Read(ak); err != nil {
		return "", "", "", fmt.Errorf("generate garage access key: %w", err)
	}
	if _, err = rand.Read(sk); err != nil {
		return "", "", "", fmt.Errorf("generate garage secret key: %w", err)
	}
	if _, err = rand.Read(rpc); err != nil {
		return "", "", "", fmt.Errorf("generate garage rpc secret: %w", err)
	}
	return hex.EncodeToString(ak), hex.EncodeToString(sk), hex.EncodeToString(rpc), nil
}

// Ensure brings managed Garage to the desired steady state (two-phase bootstrap).
func (g GarageManager) Ensure(ctx context.Context) (string, error) {
	if g.Executor == nil {
		return "", fmt.Errorf("garage executor is not configured")
	}
	script := g.ensureScript()
	if runner, ok := g.Executor.(ScriptExecutor); ok {
		return runner.RunScript(ctx, script)
	}
	return g.Executor.Run(ctx, heredocCommand("/tmp/aurago-garage-ensure.sh", script))
}

// Stop stops the managed container without deleting data directories.
func (g GarageManager) Stop(ctx context.Context) (string, error) {
	if g.Executor == nil {
		return "", fmt.Errorf("garage executor is not configured")
	}
	script := g.stopScript()
	if runner, ok := g.Executor.(ScriptExecutor); ok {
		return runner.RunScript(ctx, script)
	}
	return g.Executor.Run(ctx, heredocCommand("/tmp/aurago-garage-stop.sh", script))
}

// Probe reports running/healthy lines for passive/active checks.
func (g GarageManager) Probe(ctx context.Context) (running, healthy bool, detail string, err error) {
	if g.Executor == nil {
		return false, false, "", fmt.Errorf("garage executor is not configured")
	}
	out, err := g.Executor.Run(ctx, g.probeCommand())
	detail = strings.TrimSpace(out)
	if err != nil {
		return false, false, detail, err
	}
	for _, line := range strings.Split(detail, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "RUNNING="):
			running = strings.TrimPrefix(line, "RUNNING=") == "1"
		case strings.HasPrefix(line, "HEALTHY="):
			healthy = strings.TrimPrefix(line, "HEALTHY=") == "1"
		}
	}
	return running, healthy, detail, nil
}

func (g GarageManager) dataRoot() string {
	return ManagedGarageDataDir(g.InstallDir)
}

func (g GarageManager) ensureScript() string {
	root := g.dataRoot()
	image := config.ManagedGarageImage
	name := config.ManagedGarageContainerName
	uid := config.ManagedGarageUID
	gid := config.ManagedGarageGID
	fp := strings.TrimSpace(g.Fingerprint)
	if fp == "" {
		fp = "default"
	}
	ak := envLine(g.AccessKeyID)
	sk := envLine(g.SecretKey)
	rpc := envLine(g.RPCSecret)
	bucket := config.ManagedGarageBucket

	return fmt.Sprintf(`set -euo pipefail
log() { printf '[aurago-garage] %%s\n' "$*"; }
GARAGE_ROOT=%s
GARAGE_IMAGE=%s
GARAGE_NAME=%s
GARAGE_UID=%d
GARAGE_GID=%d
GARAGE_FP=%s
GARAGE_AK=%s
GARAGE_SK=%s
GARAGE_RPC=%s
GARAGE_BUCKET=%s

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for managed Garage" >&2
	exit 10
fi
if ! docker info >/dev/null 2>&1; then
	echo "docker daemon is not reachable" >&2
	exit 10
fi
if [ -z "${GARAGE_AK}" ] || [ -z "${GARAGE_SK}" ] || [ -z "${GARAGE_RPC}" ]; then
	echo "garage credentials are incomplete" >&2
	exit 11
fi

install -d -m0755 "${GARAGE_ROOT}"
install -d -m0750 "${GARAGE_ROOT}/config" "${GARAGE_ROOT}/meta" "${GARAGE_ROOT}/data" "${GARAGE_ROOT}/snapshots" "${GARAGE_ROOT}/secrets"
# Project RPC secret as a file (never leave default keys in steady-state inspect).
umask 077
printf '%%s' "${GARAGE_RPC}" > "${GARAGE_ROOT}/secrets/rpc_secret"
chmod 0400 "${GARAGE_ROOT}/secrets/rpc_secret"
chown -R "${GARAGE_UID}:${GARAGE_GID}" "${GARAGE_ROOT}/config" "${GARAGE_ROOT}/meta" "${GARAGE_ROOT}/data" "${GARAGE_ROOT}/snapshots" "${GARAGE_ROOT}/secrets" 2>/dev/null || true

cat > "${GARAGE_ROOT}/config/garage.toml" <<'TOML'
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
metadata_snapshots_dir = "/var/lib/garage/snapshots"
db_engine = "sqlite"
metadata_fsync = true
metadata_auto_snapshot_interval = "6h"
block_ram_buffer_max = "64MiB"
allow_world_readable_secrets = false

replication_factor = 1
compression_level = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret_file = "/etc/garage/rpc_secret"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage.localhost"

[s3_web]
bind_addr = "127.0.0.1:3902"
root_domain = ".web.garage.localhost"

[admin]
api_bind_addr = "127.0.0.1:3903"
TOML
chown "${GARAGE_UID}:${GARAGE_GID}" "${GARAGE_ROOT}/config/garage.toml"
chmod 0640 "${GARAGE_ROOT}/config/garage.toml"

garage_key_present() {
	# Confirm the Vault access key exists inside Garage (never invent a new key).
	if docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml key info "${GARAGE_AK}" >/dev/null 2>&1; then
		return 0
	fi
	if docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml key list 2>/dev/null | grep -F "${GARAGE_AK}" >/dev/null 2>&1; then
		return 0
	fi
	return 1
}

# Foreign container with the reserved name must never be taken over.
if docker inspect "${GARAGE_NAME}" >/dev/null 2>&1; then
	OWNER="$(docker inspect -f '{{index .Config.Labels "aurago.managed"}}' "${GARAGE_NAME}" 2>/dev/null || true)"
	if [ "${OWNER}" != "boring-garage" ]; then
		echo "container ${GARAGE_NAME} exists but is not AuraGo-managed (owner=${OWNER:-unknown})" >&2
		exit 12
	fi
	CUR_FP="$(docker inspect -f '{{index .Config.Labels "aurago.fingerprint"}}' "${GARAGE_NAME}" 2>/dev/null || true)"
	CUR_IMG="$(docker inspect -f '{{.Config.Image}}' "${GARAGE_NAME}" 2>/dev/null || true)"
	RUNNING="$(docker inspect -f '{{.State.Running}}' "${GARAGE_NAME}" 2>/dev/null || true)"
	if [ "${CUR_FP}" = "${GARAGE_FP}" ] && [ "${CUR_IMG}" = "${GARAGE_IMAGE}" ] && [ "${RUNNING}" = "true" ]; then
		# Steady-state: no access keys in inspect env.
		if docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "${GARAGE_NAME}" 2>/dev/null | grep -qE '^GARAGE_ALLOW_WORLD_READABLE_SECRETS=|^GARAGE_S3_ACCESS_KEY_ID=|^GARAGE_S3_SECRET_ACCESS_KEY='; then
			log "recreating container to drop bootstrap secrets from inspect"
			docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
		elif garage_key_present; then
			log "managed Garage already running with matching fingerprint and Vault key"
			exit 0
		else
			log "managed Garage is running but Vault access key is missing; re-bootstrap keys"
			# Keep data dirs; only recreate runtime after key repair path below.
			docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
			# Force key/bucket repair even if a previous marker exists.
			rm -f "${GARAGE_ROOT}/meta/.aurago-bootstrapped" 2>/dev/null || true
			NEED_BOOTSTRAP=1
		fi
	else
		log "recreating managed Garage container (spec drift); data directories preserved"
		docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
	fi
fi

log "pulling Garage image (pinned digest)"
docker pull "${GARAGE_IMAGE}" >/dev/null

# Evaluate bootstrap marker after possible key-repair invalidation above.
BOOTSTRAPPED_MARKER="${GARAGE_ROOT}/meta/.aurago-bootstrapped"
if [ "${NEED_BOOTSTRAP:-}" != "1" ]; then
	NEED_BOOTSTRAP=1
	if [ -f "${BOOTSTRAPPED_MARKER}" ]; then
		NEED_BOOTSTRAP=0
	fi
fi

common_run() {
	# $1 = extra env for bootstrap phase (may be empty)
	# shellcheck disable=SC2086
	docker run -d \
		--name "${GARAGE_NAME}" \
		--restart unless-stopped \
		--user "${GARAGE_UID}:${GARAGE_GID}" \
		--read-only \
		--security-opt no-new-privileges:true \
		--cap-drop ALL \
		--pids-limit 256 \
		--cpus 1.0 \
		--memory 512m \
		--log-driver json-file \
		--log-opt max-size=2m \
		--log-opt max-file=3 \
		--label aurago.managed=boring-garage \
		--label aurago.component=virtual-computers \
		--label aurago.role=object-store \
		--label "aurago.fingerprint=${GARAGE_FP}" \
		-v "${GARAGE_ROOT}/config/garage.toml:/etc/garage/garage.toml:ro" \
		-v "${GARAGE_ROOT}/secrets/rpc_secret:/etc/garage/rpc_secret:ro" \
		-v "${GARAGE_ROOT}/meta:/var/lib/garage/meta" \
		-v "${GARAGE_ROOT}/data:/var/lib/garage/data" \
		-v "${GARAGE_ROOT}/snapshots:/var/lib/garage/snapshots" \
		--tmpfs /tmp:rw,noexec,nosuid,size=64m \
		-p 127.0.0.1:3900:3900 \
		$1 \
		"${GARAGE_IMAGE}" \
		/garage -c /etc/garage/garage.toml server
}

if [ "${NEED_BOOTSTRAP}" = "1" ]; then
	log "phase-1 bootstrap with single-node layout"
	# Configure only the layout at server start. Garage v2.3's --default-bucket
	# is a boolean flag backed by GARAGE_DEFAULT_* environment variables; passing
	# the bucket as a positional argument prevents startup, while supplying Vault
	# credentials as container environment would expose them through inspect.
	docker run -d \
		--name "${GARAGE_NAME}" \
		--restart unless-stopped \
		--user "${GARAGE_UID}:${GARAGE_GID}" \
		--read-only \
		--security-opt no-new-privileges:true \
		--cap-drop ALL \
		--pids-limit 256 \
		--cpus 1.0 \
		--memory 512m \
		--log-driver json-file \
		--log-opt max-size=2m \
		--log-opt max-file=3 \
		--label aurago.managed=boring-garage \
		--label aurago.component=virtual-computers \
		--label aurago.role=object-store-bootstrap \
		--label "aurago.fingerprint=${GARAGE_FP}" \
		-e GARAGE_ALLOW_WORLD_READABLE_SECRETS=true \
		-v "${GARAGE_ROOT}/config/garage.toml:/etc/garage/garage.toml:ro" \
		-v "${GARAGE_ROOT}/secrets/rpc_secret:/etc/garage/rpc_secret:ro" \
		-v "${GARAGE_ROOT}/meta:/var/lib/garage/meta" \
		-v "${GARAGE_ROOT}/data:/var/lib/garage/data" \
		-v "${GARAGE_ROOT}/snapshots:/var/lib/garage/snapshots" \
		--tmpfs /tmp:rw,noexec,nosuid,size=64m \
		-p 127.0.0.1:3900:3900 \
		"${GARAGE_IMAGE}" \
		/garage -c /etc/garage/garage.toml server --single-node
	# Wait until the node answers admin status (layout ready).
	READY=0
	for _ in $(seq 1 60); do
		if docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml status >/dev/null 2>&1; then
			READY=1
			break
		fi
		sleep 1
	done
	if [ "${READY}" != "1" ]; then
		echo "Garage bootstrap container did not become ready" >&2
		docker logs --tail 80 "${GARAGE_NAME}" 2>&1 || true
		docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
		exit 14
	fi
	# Import the Vault-backed key exactly. Never fall back to "key new" — that would
	# create credentials AuraGo does not store, and boringd would authenticate with
	# the wrong secret. Re-import after a repair may report "already exists"; accept
	# that only when the Vault access key is actually present.
	if ! docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml key import "${GARAGE_AK}" "${GARAGE_SK}" --name aurago --yes >/dev/null 2>&1; then
		if ! docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml key import -n aurago --yes "${GARAGE_AK}" "${GARAGE_SK}" >/dev/null 2>&1; then
			if ! garage_key_present; then
				echo "Garage key import failed; Vault credentials were not installed into Garage" >&2
				docker logs --tail 80 "${GARAGE_NAME}" 2>&1 || true
				docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
				exit 15
			fi
			log "Vault access key already present in Garage after import attempt"
		fi
	fi
	if ! docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml bucket info "${GARAGE_BUCKET}" >/dev/null 2>&1; then
		if ! docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml bucket create "${GARAGE_BUCKET}" >/dev/null 2>&1; then
			echo "Garage bucket ${GARAGE_BUCKET} could not be created" >&2
			docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
			exit 16
		fi
	fi
	if ! docker exec "${GARAGE_NAME}" /garage -c /etc/garage/garage.toml bucket allow --read --write --owner "${GARAGE_BUCKET}" --key aurago >/dev/null 2>&1; then
		echo "Garage bucket permissions could not be granted to the Vault key" >&2
		docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
		exit 16
	fi
	printf '1\n' > "${BOOTSTRAPPED_MARKER}"
	chown "${GARAGE_UID}:${GARAGE_GID}" "${BOOTSTRAPPED_MARKER}" 2>/dev/null || true
	log "phase-2 recreate without bootstrap secrets in inspect"
	docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
fi

	log "starting steady-state Garage"
common_run ""
for _ in $(seq 1 45); do
	if docker inspect -f '{{.State.Running}}' "${GARAGE_NAME}" 2>/dev/null | grep -qx true; then
		if garage_key_present; then
			log "Garage container is running with Vault key present"
			exit 0
		fi
		echo "Garage is running but Vault access key ${GARAGE_AK} was not found" >&2
		docker rm -f "${GARAGE_NAME}" >/dev/null 2>&1 || true
		exit 17
	fi
	sleep 1
done
echo "Garage container failed to stay running" >&2
docker logs --tail 80 "${GARAGE_NAME}" 2>&1 || true
exit 13
`, shellQuote(root), shellQuote(image), shellQuote(name), uid, gid, shellQuote(fp), shellQuote(ak), shellQuote(sk), shellQuote(rpc), shellQuote(bucket))
}

func (g GarageManager) stopScript() string {
	name := config.ManagedGarageContainerName
	return fmt.Sprintf(`set -euo pipefail
GARAGE_NAME=%s
if ! command -v docker >/dev/null 2>&1; then
	exit 0
fi
if ! docker inspect "${GARAGE_NAME}" >/dev/null 2>&1; then
	exit 0
fi
OWNER="$(docker inspect -f '{{index .Config.Labels "aurago.managed"}}' "${GARAGE_NAME}" 2>/dev/null || true)"
if [ "${OWNER}" != "boring-garage" ]; then
	echo "refusing to stop non-managed container ${GARAGE_NAME}" >&2
	exit 12
fi
docker stop "${GARAGE_NAME}" >/dev/null 2>&1 || true
# Keep container and data for fast reactivation; only stop runtime.
printf 'STOPPED=1\n'
`, shellQuote(name))
}

func (g GarageManager) probeCommand() string {
	name := config.ManagedGarageContainerName
	return fmt.Sprintf(`GARAGE_NAME=%s
RUNNING=0
HEALTHY=0
if command -v docker >/dev/null 2>&1 && docker inspect "${GARAGE_NAME}" >/dev/null 2>&1; then
  OWNER="$(docker inspect -f '{{index .Config.Labels "aurago.managed"}}' "${GARAGE_NAME}" 2>/dev/null || true)"
  if [ "${OWNER}" = "boring-garage" ]; then
    if docker inspect -f '{{.State.Running}}' "${GARAGE_NAME}" 2>/dev/null | grep -qx true; then
      RUNNING=1
      HEALTHY=1
    fi
  fi
fi
printf 'RUNNING=%%s\nHEALTHY=%%s\n' "${RUNNING}" "${HEALTHY}"
`, shellQuote(name))
}

// garageEnsureSnippet returns a bash fragment embedded into the main install script.
// On failure it sets GARAGE_OK=0 and does not abort the core install (set +e around it).
func garageEnsureSnippet(opts SetupInstallOptions) string {
	mode := NormalizeStorageMode(opts.StorageMode, opts.S3Endpoint)
	if mode != StorageModeManagedGarage || !opts.AllowVolumes || !opts.ProjectGarage {
		return `
GARAGE_OK=0
log "managed Garage not requested for this install"
`
	}
	gm := GarageManager{
		InstallDir:  opts.InstallDir,
		AccessKeyID: opts.S3AccessKeyID,
		SecretKey:   opts.S3SecretKey,
		RPCSecret:   opts.GarageRPCSecret,
		Fingerprint: StorageIdentity{
			Mode:             StorageModeManagedGarage,
			Endpoint:         config.ManagedGarageEndpoint,
			Bucket:           config.ManagedGarageBucket,
			Region:           config.ManagedGarageRegion,
			UseSSL:           false,
			ControlPlaneMode: opts.ControlPlaneMode,
			ControlPlaneHost: opts.ControlPlaneHost,
			InstallDir:       opts.InstallDir,
		}.Hash(),
	}
	// Strip set -euo from nested script; run under subshell with +e.
	body := gm.ensureScript()
	// Remove leading set -euo pipefail so failure can be captured.
	body = strings.TrimPrefix(body, "set -euo pipefail\n")
	return fmt.Sprintf(`
GARAGE_OK=0
log "ensuring managed Garage sidecar"
set +e
(
set -euo pipefail
%s
)
GARAGE_RC=$?
set -euo pipefail
if [ "${GARAGE_RC}" -eq 0 ]; then
	GARAGE_OK=1
	log "managed Garage is ready"
else
	log "managed Garage failed (rc=${GARAGE_RC}); continuing core install without volume S3 projection"
	GARAGE_OK=0
fi
`, body)
}

// applyS3EnvForInstall returns S3 key/secret/endpoint values for boringd.env.
// When managed garage failed, blanks are used so boringd starts without volume routes.
func applyS3EnvForInstall(opts SetupInstallOptions, garageOK bool) (key, secret, endpoint, bucket, region, ssl string) {
	mode := NormalizeStorageMode(opts.StorageMode, opts.S3Endpoint)
	ssl = "0"
	if opts.S3UseSSL {
		ssl = "1"
	}
	if mode == StorageModeManagedGarage {
		if !garageOK || !opts.AllowVolumes {
			return "", "", "", "", "", "0"
		}
		return opts.S3AccessKeyID, opts.S3SecretKey, config.ManagedGarageEndpoint, config.ManagedGarageBucket, config.ManagedGarageRegion, "0"
	}
	if !opts.AllowVolumes {
		return "", "", "", "", "", "0"
	}
	return opts.S3AccessKeyID, opts.S3SecretKey, opts.S3Endpoint, opts.S3Bucket, opts.S3Region, ssl
}

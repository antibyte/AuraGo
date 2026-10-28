package dockerstacks

// Runner scripts, one per operation kind. They run as
//
//	sh -ec <script> aurago-docker-manager "$@"
//
// where "$@" is `-p <name> -f <dir>/<name>/<compose file> --progress json`, and
// with STACK_DIR and PROJECT in the environment. No user-controlled value is
// ever part of a script's text: stack names, paths and compose file names reach
// it only through the positional arguments or the environment.
const (
	scriptConfig      = `docker compose "$@" config --quiet`
	scriptUp          = `docker compose "$@" config --quiet && docker compose "$@" up -d --remove-orphans`
	scriptDown        = `docker compose "$@" down --remove-orphans`
	scriptDownVolumes = `docker compose "$@" down --remove-orphans --volumes`
	scriptStop        = `docker compose "$@" stop`
	scriptRestart     = `docker compose "$@" restart`
	scriptPull        = `docker compose "$@" pull`
	scriptUpdate      = `docker compose "$@" pull && docker compose "$@" up -d --remove-orphans`
	scriptDelete      = `docker compose -p "$PROJECT" down --remove-orphans && rm -rf -- "$STACK_DIR"`
	scriptDeleteVols  = `docker compose -p "$PROJECT" down --remove-orphans --volumes && rm -rf -- "$STACK_DIR"`
)

// runnerScript returns the script of an operation kind; volumes selects the
// `--volumes` variant of down and delete. ok is false for kinds without a
// runner (prepare) and for unknown kinds.
func runnerScript(kind string, volumes bool) (script string, ok bool) {
	switch kind {
	case KindConfig:
		return scriptConfig, true
	case KindUp:
		return scriptUp, true
	case KindDown:
		if volumes {
			return scriptDownVolumes, true
		}
		return scriptDown, true
	case KindStop:
		return scriptStop, true
	case KindRestart:
		return scriptRestart, true
	case KindPull:
		return scriptPull, true
	case KindUpdate:
		return scriptUpdate, true
	case KindDelete:
		if volumes {
			return scriptDeleteVols, true
		}
		return scriptDelete, true
	}
	return "", false
}

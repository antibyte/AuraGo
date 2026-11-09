#!/usr/bin/env python3
"""AuraGo Software Store: merge the managed AuraGo profiles into DeepTutor.

The Store runs this script (python -I -c "$AURAGO_DT_MERGE_PY") as root in front of
the image entrypoint, on every container start. It adds one LLM profile named
"AuraGo" and, when the AuraGo relay reports embeddings, one embedding profile to
DeepTutor's data/user/settings/model_catalog.json. Both point at the token-gated
AuraGo relay, so the real provider key never enters the container.

Contract (kept in step with internal/desktopstore/AGENTS.md):
  * stdlib only, Python 3.8+, no output of the token;
  * a fingerprint of url|token|vision|embeddings|context in .aurago-managed.json
    makes a restart a no-op, so a user's deletion or choice in the DeepTutor
    settings is never undone;
  * a changed fingerprint (new relay URL or token) updates only base_url,
    api_key and the model alias of profiles that still exist;
  * the managed profiles become active only for a service without any usable
    profile; all other profiles and unknown keys are kept as they are;
  * every write is atomic (temp file + os.replace) and mode 0600;
  * root must never follow a link planted in the data volume, which the
    DeepTutor user can write: the script refuses to merge (one stderr line,
    exit 0) when the settings directory or one of its parents below the data
    root is not a plain directory, when it resolves outside the data root, or
    when the catalog or the marker is a symbolic link, a hard-linked file or
    not a regular file; files are opened with O_NOFOLLOW where available;
  * handled problems exit 0 (the Store wrapper starts DeepTutor anyway),
    unexpected exceptions exit 1.

Environment: AURAGO_DT_RELAY_URL, AURAGO_DT_RELAY_TOKEN, AURAGO_DT_VISION,
AURAGO_DT_EMBEDDINGS, AURAGO_DT_CONTEXT, and AURAGO_DT_SETTINGS_DIR (tests only).
"""

import hashlib
import json
import os
import stat
import sys
import tempfile
import time
import urllib.request

LLM_PROFILE_ID = "llm-profile-a0a0a0a0"
LLM_MODEL_ID = "llm-model-a0a0a0a0"
EMBEDDING_PROFILE_ID = "embedding-profile-a0a0a0a0"
EMBEDDING_MODEL_ID = "embedding-model-a0a0a0a0"
LLM_MODEL_ALIAS = "aurago"
EMBEDDING_MODEL_ALIAS = "aurago-embedding"
PROFILE_NAME = "AuraGo"

DEFAULT_SETTINGS_DIR = "/app/data/user/settings"
CATALOG_NAME = "model_catalog.json"
MARKER_NAME = ".aurago-managed.json"
SERVICE_NAMES = ("llm", "task", "embedding", "search", "tts", "stt", "imagegen", "videogen")
PROBE_TIMEOUT_SECONDS = 8

# Flags for opening files in a directory the DeepTutor user can write.
OPEN_READ = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0) | getattr(os, "O_BINARY", 0)
OPEN_CREATE_NEW = (os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
                   | getattr(os, "O_BINARY", 0))


class Refused(Exception):
    """The settings area is not what the Store created: leave it untouched."""


def log(message):
    print("aurago-deeptutor: " + message, flush=True)


def warn(message):
    print("aurago-deeptutor: " + message, file=sys.stderr, flush=True)


def env_flag(name):
    return os.environ.get(name, "").strip() == "1"


def env_positive_int(name):
    try:
        value = int(os.environ.get(name, "").strip() or 0)
    except ValueError:
        return 0
    return value if value > 0 else 0


def service_shell(name):
    if name == "search":
        return {"active_profile_id": None, "profiles": []}
    return {"active_profile_id": None, "active_model_id": None, "profiles": []}


def default_catalog():
    return {
        "version": 1,
        "connections": [],
        "services": {name: service_shell(name) for name in SERVICE_NAMES},
    }


def fingerprint_of(url, token, vision, embeddings, context):
    raw = "|".join((url, token, "1" if vision else "0", "1" if embeddings else "0", str(context)))
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()


def check_layout(settings_dir):
    """Refuse a settings directory that is, or sits below, a link or other non-directory.

    The data root is two levels above the settings directory (/app/data for
    /app/data/user/settings). Nothing below it may be a symbolic link, and the
    real path of the settings directory must be exactly where it is expected.
    """
    data_root = os.path.dirname(os.path.dirname(settings_dir))
    relative = os.path.relpath(settings_dir, data_root)
    current = data_root
    for part in relative.split(os.sep):
        current = os.path.join(current, part)
        try:
            info = os.lstat(current)
        except FileNotFoundError:
            break
        if stat.S_ISLNK(info.st_mode) or not stat.S_ISDIR(info.st_mode):
            raise Refused("%s is not a plain directory" % os.path.relpath(current, data_root))
    if os.path.realpath(settings_dir) != os.path.join(os.path.realpath(data_root), relative):
        raise Refused("the settings directory resolves outside the data volume")


def check_file(path):
    """Refuse a catalog or marker that is not an ordinary single-link regular file."""
    try:
        info = os.lstat(path)
    except FileNotFoundError:
        return
    if stat.S_ISLNK(info.st_mode):
        raise Refused("%s is a symbolic link" % os.path.basename(path))
    if not stat.S_ISREG(info.st_mode) or info.st_nlink > 1:
        raise Refused("%s is not an ordinary file" % os.path.basename(path))


def read_file(path):
    """Return the bytes of an ordinary file, or None when it does not exist."""
    check_file(path)
    try:
        fd = os.open(path, OPEN_READ)
    except FileNotFoundError:
        return None
    except OSError:
        # For example ELOOP when a link replaced the file after the check.
        raise Refused("%s cannot be opened safely" % os.path.basename(path))
    with os.fdopen(fd, "rb") as handle:
        info = os.fstat(handle.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_nlink > 1:
            raise Refused("%s is not an ordinary file" % os.path.basename(path))
        return handle.read()


def write_backup(path, raw):
    """Keep the unreadable catalog next to it; never write through an existing name."""
    stamp = int(time.time())
    for attempt in range(10):
        backup = "%s.corrupt-%d%s" % (path, stamp, "-%d" % attempt if attempt else "")
        try:
            fd = os.open(backup, OPEN_CREATE_NEW, 0o600)
        except FileExistsError:
            continue
        with os.fdopen(fd, "wb") as handle:
            handle.write(raw)
        return backup
    raise Refused("no free name for the backup of %s" % os.path.basename(path))


def read_marker(path):
    raw = read_file(path)
    if raw is None:
        return {}
    try:
        data = json.loads(raw.decode("utf-8"))
    except ValueError:
        return {}
    return data if isinstance(data, dict) else {}


def catalog_is_readable(path):
    raw = read_file(path)
    if raw is None:
        return False
    try:
        return isinstance(json.loads(raw.decode("utf-8")), dict)
    except ValueError:
        return False


def read_catalog(path):
    """Return (catalog, usable). usable is False for a missing or damaged file."""
    raw = read_file(path)
    if raw is None or not raw.strip():
        # DeepTutor treats an empty file like a missing one.
        return default_catalog(), False
    try:
        data = json.loads(raw.decode("utf-8"))
    except ValueError:
        data = None
    if isinstance(data, dict):
        return data, True
    backup = write_backup(path, raw)
    warn("%s is not a valid JSON object; kept a copy as %s and started from defaults"
         % (CATALOG_NAME, os.path.basename(backup)))
    return default_catalog(), False


def write_json_atomic(path, data):
    directory = os.path.dirname(path)
    fd, temp_path = tempfile.mkstemp(prefix=".aurago-", suffix=".tmp", dir=directory)
    try:
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as handle:
            json.dump(data, handle, indent=2, ensure_ascii=False)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temp_path, 0o600)
        os.replace(temp_path, path)
    except BaseException:
        try:
            os.unlink(temp_path)
        except OSError:
            pass
        raise


def ensure_structure(catalog):
    catalog.setdefault("version", 1)
    if not isinstance(catalog.get("connections"), list):
        catalog["connections"] = []
    if not isinstance(catalog.get("services"), dict):
        catalog["services"] = {}
    services = catalog["services"]
    for name in SERVICE_NAMES:
        service = services.get(name)
        if not isinstance(service, dict):
            service = service_shell(name)
            services[name] = service
        service.setdefault("active_profile_id", None)
        if name != "search":
            service.setdefault("active_model_id", None)
        if not isinstance(service.get("profiles"), list):
            service["profiles"] = []


def find_by_id(items, wanted):
    if not isinstance(items, list):
        return None
    for item in items:
        if isinstance(item, dict) and item.get("id") == wanted:
            return item
    return None


def has_usable_profile(service):
    """True when DeepTutor itself would already resolve a profile for the service."""
    profiles = [item for item in service["profiles"] if isinstance(item, dict)]
    active = service.get("active_profile_id")
    if active and any(item.get("id") == active for item in profiles):
        return True
    return any(not item.get("provider_only") and item.get("models") for item in profiles)


def probe_dimension(endpoint, token):
    """Ask the relay for one embedding to learn its dimension; 0 when unknown.

    DeepTutor's GraphRAG needs a known dimension and the settings page only
    fills it after a manual connection test. The request goes to the relay URL
    only and never follows a redirect, so the token cannot travel elsewhere.
    """

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            return None

    body = json.dumps({"model": EMBEDDING_MODEL_ALIAS, "input": ["dimension probe"]}).encode("utf-8")
    request = urllib.request.Request(
        endpoint,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json", "Authorization": "Bearer " + token},
    )
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        with opener.open(request, timeout=PROBE_TIMEOUT_SECONDS) as response:
            payload = json.loads(response.read(1 << 22).decode("utf-8"))
        vector = payload["data"][0]["embedding"]
    except Exception:
        return 0
    return len(vector) if isinstance(vector, list) else 0


def upsert_llm(catalog, relay_url, token, vision, context, created_before):
    service = catalog["services"]["llm"]
    profile = find_by_id(service["profiles"], LLM_PROFILE_ID)
    if profile is not None:
        profile["base_url"] = relay_url
        profile["api_key"] = token
        model = find_by_id(profile.get("models"), LLM_MODEL_ID)
        if model is not None:
            model["model"] = LLM_MODEL_ALIAS
        return "updated"
    if "llm" in created_before:
        return "left out (removed by the user)"
    had_usable = has_usable_profile(service)
    model = {
        "id": LLM_MODEL_ID,
        "name": PROFILE_NAME,
        "model": LLM_MODEL_ALIAS,
        "capabilities": {"tools": True, "vision": bool(vision), "json_output": True, "reasoning": False},
    }
    if context > 0:
        model["context_window"] = context
    service["profiles"].append({
        "id": LLM_PROFILE_ID,
        "name": PROFILE_NAME,
        "binding": "custom",
        "base_url": relay_url,
        "api_key": token,
        "api_version": "",
        "extra_headers": {},
        "api_format": "openai_chat",
        "wire_api": "chat_completions",
        "models": [model],
    })
    if not had_usable:
        service["active_profile_id"] = LLM_PROFILE_ID
        service["active_model_id"] = LLM_MODEL_ID
    return "created"


def upsert_embedding(catalog, endpoint, token, created_before):
    service = catalog["services"]["embedding"]
    profile = find_by_id(service["profiles"], EMBEDDING_PROFILE_ID)
    if profile is not None:
        profile["base_url"] = endpoint
        profile["api_key"] = token
        model = find_by_id(profile.get("models"), EMBEDDING_MODEL_ID)
        if model is not None:
            model["model"] = EMBEDDING_MODEL_ALIAS
            if not str(model.get("dimension") or "").strip():
                dimension = probe_dimension(endpoint, token)
                if dimension > 0:
                    model["dimension"] = str(dimension)
        return "updated"
    if "embedding" in created_before:
        return "left out (removed by the user)"
    had_usable = has_usable_profile(service)
    dimension = probe_dimension(endpoint, token)
    service["profiles"].append({
        "id": EMBEDDING_PROFILE_ID,
        "name": PROFILE_NAME,
        "binding": "custom",
        "base_url": endpoint,
        "api_key": token,
        "api_version": "",
        "extra_headers": {},
        "models": [{
            "id": EMBEDDING_MODEL_ID,
            "name": EMBEDDING_MODEL_ALIAS,
            "model": EMBEDDING_MODEL_ALIAS,
            "dimension": str(dimension) if dimension > 0 else "",
            "supported_dimensions": "",
        }],
    })
    if not had_usable:
        service["active_profile_id"] = EMBEDDING_PROFILE_ID
        service["active_model_id"] = EMBEDDING_MODEL_ID
    return "created"


def main():
    relay_url = os.environ.get("AURAGO_DT_RELAY_URL", "").strip().rstrip("/")
    token = os.environ.get("AURAGO_DT_RELAY_TOKEN", "").strip()
    if not relay_url or not token:
        warn("the relay URL or token is missing; DeepTutor starts without the AuraGo profile")
        return 0
    vision = env_flag("AURAGO_DT_VISION")
    embeddings = env_flag("AURAGO_DT_EMBEDDINGS")
    context = env_positive_int("AURAGO_DT_CONTEXT")
    settings_dir = os.path.abspath(os.environ.get("AURAGO_DT_SETTINGS_DIR", "").strip() or DEFAULT_SETTINGS_DIR)
    catalog_path = os.path.join(settings_dir, CATALOG_NAME)
    marker_path = os.path.join(settings_dir, MARKER_NAME)

    try:
        merge(settings_dir, catalog_path, marker_path, relay_url, token, vision, embeddings, context)
    except Refused as exc:
        warn("refusing to merge, %s; DeepTutor starts without the AuraGo profile" % exc)
    return 0


def merge(settings_dir, catalog_path, marker_path, relay_url, token, vision, embeddings, context):
    check_layout(settings_dir)
    check_file(catalog_path)
    check_file(marker_path)
    fingerprint = fingerprint_of(relay_url, token, vision, embeddings, context)
    marker = read_marker(marker_path)
    # Only a readable catalog counts as applied: DeepTutor replaces a damaged
    # one by defaults, which would otherwise lose the profile for good.
    if marker.get("fingerprint") == fingerprint and catalog_is_readable(catalog_path):
        log("the AuraGo model profile is already applied")
        return

    os.makedirs(settings_dir, exist_ok=True)
    catalog, usable = read_catalog(catalog_path)
    created_before = set()
    if usable and isinstance(marker.get("created"), list):
        created_before = set(str(item) for item in marker["created"])
    ensure_structure(catalog)

    llm_result = upsert_llm(catalog, relay_url, token, vision, context, created_before)
    embedding_result = "not offered"
    if embeddings:
        embedding_result = upsert_embedding(catalog, relay_url + "/embeddings", token, created_before)

    created_now = set(created_before)
    if find_by_id(catalog["services"]["llm"]["profiles"], LLM_PROFILE_ID) is not None:
        created_now.add("llm")
    if find_by_id(catalog["services"]["embedding"]["profiles"], EMBEDDING_PROFILE_ID) is not None:
        created_now.add("embedding")

    write_json_atomic(catalog_path, catalog)
    write_json_atomic(marker_path, {
        "version": 1,
        "fingerprint": fingerprint,
        "created": sorted(created_now),
        "applied_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    })
    log("llm profile %s, embedding profile %s (vision=%s, context=%s)"
        % (llm_result, embedding_result, "on" if vision else "off", context or "unknown"))


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:  # noqa: BLE001 - report the type and message only, never values
        warn("unexpected error: %s: %s" % (type(exc).__name__, exc))
        sys.exit(1)

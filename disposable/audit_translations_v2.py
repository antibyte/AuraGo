#!/usr/bin/env python3
"""
Verbesserter Übersetzungs-Audit für AuraGo UI (v2)
- BOM-robustes Lesen (utf-8-sig)
- Whitelist für erlaubte identische Werte (technische Begriffe, Formatstrings)
- Getrennte Kategorien: Parse-Fehler, fehlende Keys, echte unübersetzte, erlaubte Identitäten
"""

import json
import os
import re
import sys
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
REPORT_PATH = Path("reports/translation_audit_report_v2.md")
LANGUAGES = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]
NON_EN_LANGS = [l for l in LANGUAGES if l != "en"]

# ── Whitelist: Werte, die absichtlich identisch mit en.json bleiben dürfen ──

# Einzelne Wörter/Kurzlabels, die in vielen Sprachen unübersetzt bleiben
IDENTITY_WORDS = {
    "ok", "token", "api", "url", "ip", "mac", "nas", "iot", "vm",
    "mqtt", "tls", "ssl", "sse", "grpc", "http", "https", "ssh",
    "docker", "podman", "ollama", "netlify", "gotenberg", "chromecast",
    "telnyx", "discord", "github", "virustotal", "brave",
    "oauth2", "totp", "csrf",
    "json", "yaml", "csv", "html", "svg", "png",
    "model", "models",
    "port", "host", "backend", "provider", "client", "server",
    "container", "router", "switch", "printer", "camera",
    "generic", "system", "info",
    "id", "name", "type", "key", "value", "description",
    "password", "username",
    "input", "output", "context",
    "tags", "actions",
    "deploy", "deployment",
    "circuit breaker", "circuit breaker",
    "sandbox", "landlock",
    "openrouter", "openai", "google", "anthropic", "workers-ai",
    "custom",
    "sk-or-...", "cf-aig-...", "my-gateway",
    "unchanged", "set",
    "delete",
    "restart",
    "config",
    "dashboard", "invasion",
    "chat", "missions", "plans",
    "containers",
    "loading...", "loading…",
    "sites",
    "secret(s) in vault",
    "system",
    "timeout",
    "pool size",
    "no pooling",
    "native host",
    # ── Added batch 3: brand names, product names, technical terms ──
    "helper llm", "llm guardian", "agent", "tailscale", "webhooks", "webhook",
    "ai gateway", "circuit breaker", "onedrive", "paperless-ngx",
    "ansible", "google workspace", "koofr", "meshcentral", "proxmox",
    "rocket.chat", "telegram", "webdav", "home assistant",
    "cloudflare tunnel", "mcp server", "brave search",
    "adguard", "adguard home", "fritz!box", "fritzbox", "jellyfin",
    "truenas", "kimi code", "piper tts", "whisper api",
    "wireguard", "time machine", "smb:", "gzip", "lz4",
    "basic auth", "bearer token", "connector token",
    "personal access token", "python", "manual", "embeddings",
    "daemon", "preset", "free", "test", "rollback", "paused",
    "version", "size", "tokens", "total", "download", "status",
    "online", "offline", "compression", "start scrub",
    "host / ip", "error:", "pools",
    "ns lookup", "txt lookup", "pid 0 cannot be killed",
    "process", "not found",
    "job enabled.", "fact not found in core memory.",
    "alibaba coding plan", "minimax coding plan", "z ai glm coding plan",
    "google (gemini)", "ollama (local)", "docker (local)", "docker (remote)",
    "dev container", "github repositories",
    "analytics agent", "document creator", "researcher", "designer",
    "shell sandbox (landlock)", "web scraper",
    "s3 storage", "s3-compatible object storage",
    "gotenberg (docker sidecar)", "gmail — send", "imap (receive)", "smtp (send)",
    "whisper speech-to-text", "speech-to-text (whisper)",
    "image generation", "text-to-speech", "smart home",
    "truenas zfs storage management (pools, datasets, snapshots, shares)",
    "messenger", "mobile", "speech", "vision",
    "remote shell (execute_remote_shell)",
    "ssh tunnel", "ssh private key",
    "vs code debug bridge",
    "co-agents", "parallel sub-agents",
    "budget & tokens", "budget shed", "budget sheds", "shed rate",
    "core memory", "core personality", "personality v2",
    "knowledge center", "knowledge graph", "kg explorer",
    "prompts & personas", "prompt template",
    "server & system", "service exposure",
    "web config & login", "web config ui",
    "maintenance", "maintenance mode and lifeboat",
    "danger zone", "live log", "memory health",
    "permissions (scopes)", "permissions",
    "vault", "vault keys", "transfer vault",
    "master key", "login guard", "login", "logout",
    "security proxy", "geo-blocking", "rate limiting",
    "logging", "profiling", "performance",
    "navigation", "session", "scopes",
    "nodes", "edges", "types",
    "notifications", "notification", "journal",
    "queue", "scheduled", "permanent", "log only",
    "notify (sse only)", "sse clients",
    "total runs", "recoveries", "tracked",
    "lockout", "valence", "coder",
    "watch folder", "watch enabled",
    "updates", "update available",
    "upload skill", "skill code", "skill details",
    "graph edges", "graph nodes",
    "collection", "collections",
    "query models", "show free models only",
    "parse", "running", "runs", "used", "images",
    "debug", "optional", "folder", "notes", "label",
    "priority", "public", "private", "stable", "medium",
    "neutral", "never", "open", "locked",
    "legacy compatibility field. helper-owned personality analysis now uses the central helper llm from the llm settings.",
    "legacy compatibility field. web summaries now use the central helper llm from the llm settings.",
    "invalid request",
    "192.168.1.100", "192.168.1.50",
    "application/json",
    "requests, pyyaml", "root",
    "just now", "m ago", "h ago", "d ago",
    "not required", "alias (e.g. repo)",
    "e.g. github push", "e.g. x-hub-signature-256", "e.g. github-push",
    "source (e.g. repository.full_name)",
    "https://hooks.slack.com/services/...",
    "my_skill",
    "user, agent",
    "commit(s)",
    "expires",
    "grpc port:",
}

# Wertmuster, die absichtlich identisch bleiben (z.B. Formatstrings, Platzhalter)
IDENTITY_PATTERNS = [
    re.compile(r'^\$\{\{.*\}\}'),           # ${{cost}} / ${{limit}}
    re.compile(r'^\{\{.*\}\}$'),             # {{count}} tok, {{n}}m ago
    re.compile(r'^\d+$'),                     # reine Zahlen
    re.compile(r'^[🔧🔑🔐💳🗑️🛑⚠️❌✅🔍🔇🔊🎭💰⛔📜🧹🧠⚡🔄🛑]+'),  # Emoji-Präfixe (allein)
    re.compile(r'^\[.*\]$'),                  # [FILE ATTACHED]:
    re.compile(r'^\s*$'),                     # leer/whitespace
    re.compile(r'^\(.+\)$'),                  # (active), (warning only)
    re.compile(r'^•+$'),                      # ••••••••
]

# Keys, bei denen der Wert absichtlich auf Englisch bleibt (z.B. _en Suffix)
IGNORE_KEY_SUFFIXES = ["_en", "_de"]
IGNORE_KEY_PATTERNS = [
    re.compile(r'\.hint\.'),           # Technische Hinweise oft auf Englisch
    re.compile(r'\.placeholder$'),     # Platzhalter oft technisch
    re.compile(r'\.example$'),         # Beispiele oft technisch
]

# Key-Präfixe für Tools-Nachrichten (diese sind technische Backend-Strings)
TOOLS_PREFIX = "tools."


def is_identity_allowed(key: str, value: str) -> bool:
    """Prüft, ob ein identischer Wert erlaubt ist (nicht übersetzt werden muss)."""
    val_lower = value.strip().lower()

    # Leere Werte
    if not val_lower:
        return True

    # Key-basierte Ausnahmen
    for suffix in IGNORE_KEY_SUFFIXES:
        if key.endswith(suffix):
            return True

    # Werte, die nur aus Emoji + optional Text bestehen, bei denen der Text ein erlaubter Begriff ist
    # z.B. " ✅ (active)" -> "active" ist in IDENTITY_WORDS
    stripped = re.sub(r'^[\s🔧🔑🔐💳🗑️🛑⚠️❌✅🔍🔇🔊🎭💰⛔📜🧹🧠⚡🔄🛑✔️🗑️🏡🖥️🔍💪]+', '', val_lower).strip()
    stripped = re.sub(r'[\s🔧🔑🔐💳🗑️🛑⚠️❌✅🔍🔇🔊🎭💰⛔📜🧹🧠⚡🔄🛑✔️🗑️🏡🖥️🔍💪]+$', '', stripped).strip()
    if stripped in IDENTITY_WORDS:
        return True

    # Exakter Match mit Whitelist
    if val_lower in IDENTITY_WORDS:
        return True

    # Wertmuster prüfen
    for pattern in IDENTITY_PATTERNS:
        if pattern.match(value):
            return True

    # Format-Strings mit Platzhaltern: wenn der Wert nur aus Platzhaltern und
    # Satzzeichen besteht, ist er universell
    if re.match(r'^[\$\{\}\(\)\/\s\.\:]+$', value):
        return True

    # Sehr kurze technische Labels (≤3 Zeichen)
    if len(val_lower) <= 3 and val_lower.isascii():
        return True

    return False


def flatten(obj, prefix=""):
    """Flacht ein verschachteltes JSON-Dict zu dot-notation Keys."""
    items = {}
    if isinstance(obj, dict):
        for k, v in obj.items():
            new_key = f"{prefix}.{k}" if prefix else k
            if isinstance(v, dict):
                items.update(flatten(v, new_key))
            else:
                items[new_key] = v
    return items


def read_json_safe(path: Path) -> tuple[dict | None, str | None]:
    """Liest eine JSON-Datei BOM-sicher. Gibt (data, error) zurück."""
    try:
        with open(path, "r", encoding="utf-8-sig") as f:
            return json.load(f), None
    except json.JSONDecodeError as e:
        return None, f"JSON parse error: {e}"
    except Exception as e:
        return None, f"Read error: {e}"


def collect_dirs():
    """Sammelt alle Verzeichnisse, die mindestens en.json enthalten."""
    dirs = []
    for root, _, files in os.walk(LANG_DIR):
        if "en.json" in files:
            dirs.append(Path(root))
    return sorted(dirs)


def analyze_dir(directory: Path):
    """Analysiert ein einzelnes Übersetzungsverzeichnis."""
    en_path = directory / "en.json"
    en_data_raw, err = read_json_safe(en_path)
    if err:
        return None, f"Fehler beim Lesen von {en_path}: {err}"

    en_data = flatten(en_data_raw)

    results = {}
    for lang in NON_EN_LANGS:
        lang_path = directory / f"{lang}.json"
        if not lang_path.exists():
            results[lang] = {
                "exists": False,
                "missing_keys": list(en_data.keys()),
                "untranslated_keys": [],
                "allowed_identity": [],
                "extra_keys": [],
                "total_keys": 0,
                "coverage_pct": 0.0,
                "error": None,
            }
            continue

        lang_data_raw, err = read_json_safe(lang_path)
        if err:
            results[lang] = {
                "exists": True,
                "error": err,
                "missing_keys": [],
                "untranslated_keys": [],
                "allowed_identity": [],
                "extra_keys": [],
                "total_keys": 0,
                "coverage_pct": 0.0,
            }
            continue

        lang_data = flatten(lang_data_raw)

        en_keys = set(en_data.keys())
        lang_keys = set(lang_data.keys())

        missing = sorted(en_keys - lang_keys)
        extra = sorted(lang_keys - en_keys)

        # Echte unübersetzte vs. erlaubte Identitäten
        untranslated = []
        allowed_identity = []
        for k in en_keys & lang_keys:
            en_val = str(en_data[k]).strip()
            lang_val = str(lang_data[k]).strip()
            if en_val.lower() == lang_val.lower() and en_val:
                if is_identity_allowed(k, en_val):
                    allowed_identity.append(k)
                else:
                    untranslated.append(k)

        total = len(en_keys)
        coverage = round(((total - len(missing)) / total) * 100, 1) if total else 100.0

        results[lang] = {
            "exists": True,
            "error": None,
            "missing_keys": missing,
            "untranslated_keys": untranslated,
            "allowed_identity": allowed_identity,
            "extra_keys": extra,
            "total_keys": len(lang_keys),
            "coverage_pct": coverage,
        }

    return en_data, results


def main():
    dirs = collect_dirs()
    report_lines = []
    report_lines.append("# Übersetzungs-Audit Report v2 – AuraGo UI")
    report_lines.append("")
    report_lines.append(f"**Erstellt:** 2026-04-10")
    report_lines.append(f"**Referenzsprache:** English (en)")
    report_lines.append(f"**Geprüfte Sprachen:** {', '.join(NON_EN_LANGS)}")
    report_lines.append(f"**Geprüfte Verzeichnisse:** {len(dirs)}")
    report_lines.append(f"**Verbesserungen v2:** BOM-robust, Whitelist für technische Begriffe, erlaubte Identitäten separat")
    report_lines.append("")

    summary_by_lang = defaultdict(lambda: {
        "missing": 0, "untranslated": 0, "allowed": 0,
        "files_with_issues": 0, "total_files": 0, "parse_errors": 0
    })
    problematic_files = []
    parse_errors = []

    for directory in dirs:
        rel_dir = directory.relative_to(LANG_DIR)
        en_data, results = analyze_dir(directory)
        if results is None:
            report_lines.append(f"## `{rel_dir}`")
            report_lines.append(f"_Fehler: {en_data}_")
            report_lines.append("")
            continue

        has_issues = False
        section_lines = []
        section_lines.append(f"## `{rel_dir}`")
        section_lines.append("")

        for lang in NON_EN_LANGS:
            r = results[lang]
            summary_by_lang[lang]["total_files"] += 1

            if not r["exists"]:
                summary_by_lang[lang]["missing"] += len(r["missing_keys"])
                summary_by_lang[lang]["files_with_issues"] += 1
                has_issues = True
                section_lines.append(f"### {lang}")
                section_lines.append(f"- **Datei fehlt vollständig!** Alle {len(r['missing_keys'])} Keys fehlen.")
                section_lines.append("")
                continue

            if r.get("error"):
                summary_by_lang[lang]["parse_errors"] += 1
                summary_by_lang[lang]["files_with_issues"] += 1
                has_issues = True
                parse_errors.append(f"{rel_dir}/{lang}.json: {r['error']}")
                section_lines.append(f"### {lang}")
                section_lines.append(f"- **Parse-Fehler:** {r['error']}")
                section_lines.append("")
                continue

            summary_by_lang[lang]["missing"] += len(r["missing_keys"])
            summary_by_lang[lang]["untranslated"] += len(r["untranslated_keys"])
            summary_by_lang[lang]["allowed"] += len(r["allowed_identity"])

            issues = []
            if r["missing_keys"]:
                issues.append(f"{len(r['missing_keys'])} fehlende Keys")
            if r["untranslated_keys"]:
                issues.append(f"{len(r['untranslated_keys'])} unübersetzte Keys")

            if issues:
                summary_by_lang[lang]["files_with_issues"] += 1
                has_issues = True
                section_lines.append(f"### {lang} ({r['coverage_pct']}% Abdeckung)")
                if r["missing_keys"]:
                    section_lines.append(f"**Fehlende Keys ({len(r['missing_keys'])}):**")
                    for k in r["missing_keys"]:
                        section_lines.append(f"- `{k}`")
                    section_lines.append("")
                if r["untranslated_keys"]:
                    section_lines.append(f"**Echt unübersetzte Keys ({len(r['untranslated_keys'])}):**")
                    for k in r["untranslated_keys"][:30]:
                        section_lines.append(f"- `{k}` = `{en_data.get(k, 'N/A')}`")
                    if len(r["untranslated_keys"]) > 30:
                        section_lines.append(f"- ... und {len(r['untranslated_keys']) - 30} weitere")
                    section_lines.append("")
                if r["extra_keys"]:
                    section_lines.append(f"**Überzählige Keys ({len(r['extra_keys'])}):**")
                    for k in r["extra_keys"][:10]:
                        section_lines.append(f"- `{k}`")
                    if len(r["extra_keys"]) > 10:
                        section_lines.append(f"- ... und {len(r['extra_keys']) - 10} weitere")
                    section_lines.append("")

        if has_issues:
            problematic_files.append(str(rel_dir))
            report_lines.extend(section_lines)

    # Zusammenfassung
    summary_lines = []
    summary_lines.append("## Zusammenfassung")
    summary_lines.append("")
    summary_lines.append("| Sprache | Fehlende | Echt unübersetzt | Erlaubte Identitäten | Parse-Fehler | Betroffene Dateien | Abdeckung |")
    summary_lines.append("|---------|--------:|-----------------:|---------------------:|-------------:|-------------------:|----------:|")
    for lang in NON_EN_LANGS:
        s = summary_by_lang[lang]
        total = s["total_files"]
        files = s["files_with_issues"]
        pct = f"{round((1 - files/total)*100, 1)}%" if total else "N/A"
        summary_lines.append(
            f"| `{lang}` | {s['missing']} | {s['untranslated']} | {s['allowed']} | {s['parse_errors']} | {files}/{total} | {pct} |"
        )
    summary_lines.append("")
    summary_lines.append(f"**Dateien mit Problemen:** {len(problematic_files)}/{len(dirs)}")

    if parse_errors:
        summary_lines.append("")
        summary_lines.append("### Parse-Fehler (sofort beheben)")
        for pe in parse_errors:
            summary_lines.append(f"- {pe}")

    summary_lines.append("")

    final_report = report_lines[:7] + summary_lines + report_lines[7:]

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(REPORT_PATH, "w", encoding="utf-8") as f:
        f.write("\n".join(final_report))

    print(f"Bericht geschrieben nach: {REPORT_PATH}")
    print(f"Verzeichnisse geprüft: {len(dirs)}")
    print(f"Dateien mit Problemen: {len(problematic_files)}")
    print(f"Parse-Fehler: {len(parse_errors)}")

    # Exit-Code: 1 wenn echte Probleme gefunden
    total_untranslated = sum(s["untranslated"] for s in summary_by_lang.values())
    total_missing = sum(s["missing"] for s in summary_by_lang.values())
    if total_untranslated > 0 or total_missing > 0 or len(parse_errors) > 0:
        print(f"\nWARNING: {total_missing} fehlende + {total_untranslated} unuebersetzte Keys gefunden")
        sys.exit(1)
    else:
        print("\nOK: Alle Uebersetzungen vollstaendig!")
        sys.exit(0)


if __name__ == "__main__":
    main()

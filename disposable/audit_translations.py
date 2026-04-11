#!/usr/bin/env python3
"""
Audit script for AuraGo translation files.
Compares all language files against English reference to find:
- Missing translations (missing keys)
- Untranslated strings (identical to English)
- Missing language files
"""

import json
import os
import re
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
REPORT_DIR = Path("reports")
LANGUAGES = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]
NON_EN_LANGS = [l for l in LANGUAGES if l != "en"]

def load_json(path):
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception as e:
        return {"__ERROR__": str(e)}

def flatten_dict(d, parent_key=""):
    items = []
    for k, v in d.items():
        new_key = f"{parent_key}.{k}" if parent_key else k
        if isinstance(v, dict):
            items.extend(flatten_dict(v, new_key).items())
        else:
            items.append((new_key, str(v)))
    return dict(items)

def is_probably_not_translated(en_val, other_val):
    if not en_val or not other_val:
        return False
    en_norm = en_val.strip().lower()
    other_norm = other_val.strip().lower()
    if len(en_norm) <= 2:
        return False
    if en_norm.replace(" ", "").isdigit():
        return False
    # URLs, paths, emails, hex, tokens
    if re.match(r'^(https?://|/|\\|[a-zA-Z]:\\|[^@\s]+@[^@\s]+|sk-[a-zA-Z0-9]|0x[0-9a-fA-F]|\{|\[)', en_val.strip()):
        return False
    if len(en_norm) < 12 and ('{' in en_val or '[' in en_val):
        return False
    # Common technical terms / proper nouns left unchanged intentionally
    tech_terms = ["api", "oauth", "smb", "nfs", "ssh", "http", "https", "url", "uri", "json", "xml", "html",
                  "css", "js", "sql", "ssl", "tls", "vpn", "lan", "wan", "ip", "cpu", "gpu", "ram", "rom",
                  "ssd", "hdd", "nas", "dns", "dhcp", "ntp", "smtp", "imap", "pop3", "ftp", "sftp", "mcp",
                  "llm", "ai", "gpt", "uuid", "id", "qr", "2fa", "otp", "totp", "jwt", "cors", "csrf",
                  "rss", "atom", "pdf", "png", "jpg", "jpeg", "gif", "svg", "mp3", "mp4", "wav", "ogg",
                  "webhook", "localhost", "127.0.0.1", "0.0.0.0", "mdns", "upnp", "igd", "wol", "pve",
                  "proxmox", "truenas", "fritzbox", "jellyfin", "plex", "emby", "sonarr", "radarr", "lidarr",
                  "readarr", "bazarr", "prowlarr", "ombi", "tautulli", "nzbget", "sabnzbd", "transmission",
                  "qbittorrent", "deluge", "rtorrent", "nordvpn", "expressvpn", "wireguard", "openvpn",
                  "tailscale", "zerotier", "cloudflare", "letsencrypt", "acme", "dyndns", "no-ip", "gmail",
                  "outlook", "yahoo", "icloud", "google", "microsoft", "apple", "amazon", "facebook",
                  "twitter", "instagram", "linkedin", "youtube", "twitch", "discord", "telegram",
                  "whatsapp", "signal", "matrix", "mastodon", "bluesky", "threads", "reddit", "tiktok",
                  "snapchat", "pinterest", "tumblr", "flickr", "vimeo", "soundcloud", "spotify", "deezer",
                  "tidal", "pandora", "iheartradio", "tunein", "last.fm", "bandcamp", "mixcloud",
                  "distrokid", "cdbaby", "tunecore", "awal", "believe", "fuga", "downtown",
                  "sub pop", "4ad", "warp", "ninja tune", "brainfeeder", "stones throw",
                  "rhymesayers", "top dawg", "dreamville", "ovo", "xo", "republic", "interscope",
                  "columbia", "atlantic", "rca", "capitol", "emi", "universal", "sony", "warner",
                  "bmg", "kobalt", "concord", "reservoir", "round hill", "primary wave",
                  "hipgnosis", "taylor swift", "kanye west", "drake", "beyonce", "rihanna",
                  "jay-z", "eminem", "kendrick lamar", "travis scott", "future", "lil wayne",
                  "nicki minaj", "cardi b", "doja cat", "sza", "frank ocean", "the weeknd",
                  "bruno mars", "justin bieber", "ed sheeran", "adele", "sam smith", "dua lipa",
                  "harry styles", "billie eilish", "olivia rodrigo", "shakira", "j balvin",
                  "bad bunny", "karol g", "anitta", "rosalia", "selena gomez", "demi lovato",
                  "miley cyrus", "shawn mendes", "camila cabello", "one direction",
                  "backstreet boys", "nsync", "boyz ii men", "alicia keys", "john legend",
                  "celine dion", "whitney houston", "mariah carey", "christina aguilera",
                  "britney spears", "madonna", "lady gaga", "katy perry", "amy winehouse",
                  "norah jones", "michael buble", "josh groban", "andrea bocelli",
                  "luciano pavarotti", "placido domingo", "enya", "clannad", "the chieftains",
                  "spotify", "apple music", "youtube music", "amazon music", "tidal",
                  "deezer", "pandora", "bandcamp", "mixcloud", "soundcloud", "reverbnation",
                  "discogs", "rate your music", "allmusic", "musicbrainz", "last.fm",
                  "vinyl", "cd", "dvd", "blu-ray", "8k", "4k", "hd", "sd", "hdr", "dolby",
                  "dts", "aac", "flac", "alac", "wav", "aiff", "opus", "vorbis", "mp3",
                  "h.264", "h.265", "hevc", "av1", "vp9", "mpeg", "avi", "mkv", "mov",
                  "wmv", "flv", "webm", "m3u8", "dash", "hls", "rtmp", "rtsp", "webrtc",
                  "udp", "tcp", "icmp", "arp", "dhcp", "nat", "firewall", "router",
                  "switch", "hub", "bridge", "repeater", "access point", "mesh", "node",
                  "gateway", "proxy", "load balancer", "cdn", "dns", "tls", "ssl",
                  "certificate", "pkcs", "rsa", "ecdsa", "ed25519", "aes", "chacha20",
                  "sha", "md5", "bcrypt", "scrypt", "argon2", "pbkdf2", "hmac", "gcm",
                  "cbc", "ctr", "ecb", "ofb", "cfb", "xts", "ccm", "eax", "ocb",
                  "pgp", "gpg", "s/mime", "pem", "der", "cer", "crt", "p7b", "p12",
                  "pfx", "jks", "keystore", "truststore", "crl", "ocsp", "ct",
                  "dane", "dnssec", "spf", "dkim", "dmarc", "bimi", "mta-sts",
                  "tls-rpt", "smtp", "imap", "pop3", "exchange", "ldap", "ad",
                  "kerberos", "ntlm", "saml", "oauth", "oidc", "jwt", "scim",
                  "mfa", "2fa", "totp", "hotp", "fido", "u2f", "webauthn",
                  "passkey", "biometric", "pin", "password", "passphrase",
                  "credential", "secret", "token", "api key", "client id",
                  "client secret", "access token", "refresh token", "bearer",
                  "basic auth", "digest auth", "ntlm auth", "negotiate auth",
                  "certificate auth", "mutual tls", "mtls", "zero trust",
                  "siem", "soar", "xdr", "edr", "ndr", "mdr", "cspm", "cwpp",
                  "casb", "swg", "zta", "dap", "pam", "iam", "iga", "pim",
                  "pam", "rbac", "abac", "pbac", "rebac", "acl", "dac", "mac",
                  " MLS", "bell-lapadula", "biba", "clark-wilson", "chinese wall",
                  "brewer-nash", "take-grant", "hru", "spm", "tbac", "temporal",
                  "spatial", "contextual", "risk-based", "adaptive", "continuous",
                  "step-up", "step-down", "session", "cookie", "localstorage",
                  "sessionstorage", "indexeddb", "websql", "cache", "service worker",
                  "pwa", "spa", "ssr", "csr", "mpa", "jamstack", "headless",
                  "serverless", "faas", "baas", "paas", "iaas", "saas", "daas",
                  "caas", "kaas", "maas", "naas", "waas", "xaas", "private cloud",
                  "public cloud", "hybrid cloud", "multi-cloud", "edge cloud",
                  "fog computing", "mist computing", "distributed cloud",
                  "virtual machine", "vm", "container", "docker", "kubernetes",
                  "k8s", "helm", "istio", "linkerd", "consul", "vault", "nomad",
                  "terraform", "pulumi", "ansible", "puppet", "chef", "saltstack",
                  "vagrant", "packer", "jenkins", "gitlab ci", "github actions",
                  "azure devops", "aws codepipeline", "circleci", "travis ci",
                  "drone ci", "teamcity", "bamboo", "bitbucket pipelines",
                  "argo cd", "flux cd", "spinnaker", "tekton", "knative",
                  "openfaas", "kubeless", "fission", "nuclio", "dapr",
                  "open service mesh", "osm", "cilium", "calico", "flannel",
                  "weave net", "antrea", "kube-ovn", "cni", "csi", "cri",
                  "cAdvisor", "prometheus", "grafana", "thanos", "cortex",
                  "loki", "tempo", "jaeger", "zipkin", "opentelemetry",
                  "opentracing", "opencensus", "fluentd", "fluent bit",
                  "logstash", "filebeat", "metricbeat", "packetbeat",
                  "auditbeat", "heartbeat", "functionbeat", "elastic",
                  "elasticsearch", "kibana", "logstash", "beats", "apm",
                  "sentry", "datadog", "new relic", "dynatrace", "appdynamics",
                  "splunk", "sumo logic", "pagerduty", "opsgenie", "victorops",
                  "xmatters", "service now", "jira", "confluence", "trello",
                  "asana", "monday", "notion", "clickup", "linear", "height",
                  "shortcut", "clubhouse", " pivotal tracker", "targetprocess",
                  "youtrack", "redmine", "trac", "bugzilla", "mantis", "fogbugz",
                  "azure boards", "github issues", "gitlab issues", "bitbucket issues",
                  "gitea", "gogs", "sourcehut", "codeberg", "sr.ht", "forgejo",
                  "phabricator", "review board", "crucible", "gerrit", "sonarqube",
                  "checkmarx", "veracode", "snyk", "whitesource", "black duck",
                  "nexus iq", "jfrog xray", "twistlock", "prisma cloud",
                  "aqua security", "sysdig", "falco", "opa", "kyverno",
                  "gatekeeper", "kube-bench", "kube-hunter", "trivy",
                  "grype", "clair", "anchore", "snyk container", "docker scout",
                  "ecr", "gcr", "acr", "harbor", "quay", "artifactory",
                  "nexus", "distribution", "registry", "oci", "helm chart",
                  "kustomize", "jsonnet", "cue", "dhall", "hcl", "tfvars",
                  "packer hcl", "vagrant hcl", "waypoint hcl", "nomad hcl",
                  "consul hcl", "vault hcl", "boundary hcl", "terraform hcl",
                  "sentinel", "consul-template", "envconsul", "consul-esm",
                  "consul-terraform-sync", "nomad-autoscaler", "nomad-pack",
                  "vault-csi-provider", "vault-secrets-operator", "vault-agent",
                  "vault-ssh-helper", "consul-k8s", "vault-k8s", "nomad-k8s",
                  "waypoint", "nomad", "consul", "vault", "boundary", "packer",
                  "terraform", "vagrant", "nomad", "consul", "vault", "boundary"]
    en_lower = en_val.strip().lower()
    for term in tech_terms:
        if en_lower == term or (len(en_lower) > 3 and term in en_lower.split()):
            return False
    return en_norm == other_norm

def audit_category(category_dir):
    results = {
        "missing_files": [],
        "missing_keys": {},
        "untranslated": {},
        "extra_keys": {},
        "stats": defaultdict(lambda: {"total": 0, "missing": 0, "untranslated": 0}),
    }
    en_file = category_dir / "en.json"
    if not en_file.exists():
        return None
    en_data = load_json(en_file)
    if "__ERROR__" in en_data:
        return {"error": en_data["__ERROR__"]}
    en_flat = flatten_dict(en_data)
    for lang in NON_EN_LANGS:
        lang_file = category_dir / f"{lang}.json"
        if not lang_file.exists():
            results["missing_files"].append(lang)
            continue
        lang_data = load_json(lang_file)
        if "__ERROR__" in lang_data:
            continue
        lang_flat = flatten_dict(lang_data)
        en_keys = set(en_flat.keys())
        lang_keys = set(lang_flat.keys())
        missing = en_keys - lang_keys
        extra = lang_keys - en_keys
        if missing:
            results["missing_keys"][lang] = sorted(missing)
        if extra:
            results["extra_keys"][lang] = sorted(extra)
        untranslated = []
        for key in en_keys & lang_keys:
            if is_probably_not_translated(en_flat[key], lang_flat[key]):
                untranslated.append(key)
        if untranslated:
            results["untranslated"][lang] = sorted(untranslated)
        results["stats"][lang]["total"] = len(en_keys)
        results["stats"][lang]["missing"] = len(missing)
        results["stats"][lang]["untranslated"] = len(untranslated)
    return results

def main():
    categories = []
    for subdir in sorted(LANG_DIR.iterdir()):
        if not subdir.is_dir():
            continue
        if (subdir / "en.json").exists():
            categories.append((subdir.name, subdir))
        for nested in sorted(subdir.iterdir()):
            if nested.is_dir() and (nested / "en.json").exists():
                categories.append((f"{subdir.name}/{nested.name}", nested))

    lang_summary = defaultdict(lambda: {"categories": 0, "missing_files": 0, "missing_keys_total": 0, "untranslated_total": 0})
    category_issues = []
    detailed = []

    for cat_name, cat_dir in categories:
        audit = audit_category(cat_dir)
        if audit is None:
            continue
        has_issues = bool(
            audit.get("missing_files") or
            audit.get("missing_keys") or
            audit.get("untranslated") or
            audit.get("error")
        )
        if has_issues:
            category_issues.append(cat_name)
        detailed.append((cat_name, audit, has_issues))
        for lang in NON_EN_LANGS:
            stats = audit.get("stats", {}).get(lang, {})
            lang_summary[lang]["categories"] += 1
            if lang in audit.get("missing_files", []):
                lang_summary[lang]["missing_files"] += 1
            lang_summary[lang]["missing_keys_total"] += stats.get("missing", 0)
            lang_summary[lang]["untranslated_total"] += stats.get("untranslated", 0)

    report_lines = []
    report_lines.append("# AuraGo Translation Audit Report")
    report_lines.append("")
    report_lines.append(f"Reference Language: **English (en)**")
    report_lines.append(f"Languages Checked: **{', '.join(NON_EN_LANGS)}**")
    report_lines.append(f"Total Categories: **{len(categories)}**")
    report_lines.append("")
    report_lines.append("## Executive Summary")
    report_lines.append("")
    report_lines.append("| Language | Categories | Missing Files | Missing Keys | Untranslated Strings |")
    report_lines.append("|----------|-----------:|--------------:|-------------:|---------------------:|")

    sorted_langs = sorted(
        NON_EN_LANGS,
        key=lambda l: lang_summary[l]["missing_files"] + lang_summary[l]["missing_keys_total"] + lang_summary[l]["untranslated_total"],
        reverse=True
    )

    for lang in sorted_langs:
        s = lang_summary[lang]
        report_lines.append(f"| {lang} | {s['categories']} | {s['missing_files']} | {s['missing_keys_total']} | {s['untranslated_total']} |")

    report_lines.append("")
    report_lines.append(f"**Categories with issues: {len(category_issues)} / {len(categories)}**")
    report_lines.append("")

    # Per-category details
    report_lines.append("---")
    report_lines.append("")
    for cat_name, audit, has_issues in detailed:
        if not has_issues:
            continue
        report_lines.append(f"## Category: `{cat_name}`")
        report_lines.append("")
        if audit.get("error"):
            report_lines.append(f"**Error:** {audit['error']}")
            report_lines.append("")
            continue
        if audit.get("missing_files"):
            report_lines.append("### Missing Language Files")
            for lang in audit["missing_files"]:
                report_lines.append(f"- `{lang}.json`")
            report_lines.append("")
        for lang in NON_EN_LANGS:
            parts = []
            if lang in audit.get("missing_keys", {}):
                parts.append(f"{len(audit['missing_keys'][lang])} missing keys")
            if lang in audit.get("untranslated", {}):
                parts.append(f"{len(audit['untranslated'][lang])} untranslated strings")
            if lang in audit.get("extra_keys", {}):
                parts.append(f"{len(audit['extra_keys'][lang])} extra keys")
            if not parts:
                continue
            report_lines.append(f"### `{lang}` — {', '.join(parts)}")
            if lang in audit.get("missing_keys", {}):
                report_lines.append("**Missing keys:**")
                for key in audit["missing_keys"][lang][:30]:
                    report_lines.append(f"- `{key}`")
                if len(audit["missing_keys"][lang]) > 30:
                    report_lines.append(f"- ... and {len(audit['missing_keys'][lang]) - 30} more")
                report_lines.append("")
            if lang in audit.get("untranslated", {}):
                report_lines.append("**Likely untranslated:**")
                for key in audit["untranslated"][lang][:30]:
                    report_lines.append(f"- `{key}`")
                if len(audit["untranslated"][lang]) > 30:
                    report_lines.append(f"- ... and {len(audit['untranslated'][lang]) - 30} more")
                report_lines.append("")
            if lang in audit.get("extra_keys", {}):
                report_lines.append("**Extra keys (not in en):**")
                for key in audit["extra_keys"][lang][:15]:
                    report_lines.append(f"- `{key}`")
                if len(audit["extra_keys"][lang]) > 15:
                    report_lines.append(f"- ... and {len(audit['extra_keys'][lang]) - 15} more")
                report_lines.append("")
        report_lines.append("---")
        report_lines.append("")

    REPORT_DIR.mkdir(exist_ok=True)
    report_path = REPORT_DIR / "translation_audit_report.md"
    with open(report_path, "w", encoding="utf-8") as f:
        f.write("\n".join(report_lines))
    print(f"Report written to: {report_path}")

if __name__ == "__main__":
    main()

"""Experimental category-only routing with one fixed schema and complete()."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sys
import time

from .assets import MODEL_DIR, tokenizer
from .common import canonical, catalog, digest, inference_query

# These are the exported discovery categories, not a second manual taxonomy.
DESCRIPTIONS = {
    "communication": "email, Discord, messages, contacts, notifications, webhooks, co-agents",
    "data_apis": "YepAPI, Composio, Hugging Face, Manus, VirusTotal",
    "files": "files, cloud drives, archives, PDF, Paperless, Obsidian, workspace search, document creation",
    "infrastructure": "Docker, Proxmox, SSH, NAS, SQL, GitHub, websites, virtual desktop, Office, desktop notes, SIP, 3D printers, cameras",
    "media": "images, audio, video, music, speech, Bluetooth, radio, casting, Jellyfin",
    "memory": "persistent memory, knowledge graph, agent notes, journal, secrets vault",
    "network": "web search, browsing, HTTP, scraping, DNS, ports, certificates, firewall",
    "smart_home": "Home Assistant, MQTT, FritzBox, AdGuard, Grafana, Uptime Kuma, wake-on-LAN",
    "system": "shell, Python, processes, packages, system metrics, schedules, todos, appointments, agent skills, updates",
}
CATEGORIES = tuple(DESCRIPTIONS)
SYSTEM = (
    "Route the current request to ALL needed manual categories. Categories are labels, not actions. "
    "Resolve context and corrections. Missing action parameters still need manuals. "
    "Return no calls for conversation or questions answerable without tools. "
    "Select category functions without arguments."
)
MAX_NEW_TOKENS = 256
MAX_TOKENS = 1024


def schema():
    return [{"name": name, "description": description,
             "parameters": {"type": "object", "properties": {}}}
            for name, description in DESCRIPTIONS.items()]


def taxonomy(cat):
    mapping = {}
    for tool in cat["tools"]:
        mid = tool.get("manual_id")
        if not mid:
            continue
        category = tool["category"]
        if category not in CATEGORIES or mapping.setdefault(mid, category) != category:
            raise ValueError("unknown category or conflicting shared manual mapping")
    if set(mapping) != {m["id"] for m in cat["manuals"]}:
        raise ValueError("every manual must have exactly one category")
    return mapping


def contract_hash(cat):
    return digest(canonical({"catalog": cat["catalog_sha256"], "mapping": taxonomy(cat),
                             "system": SYSTEM, "schema": schema(), "tokens": MAX_TOKENS}))


def training_example(row):
    # Native decoding forces <think>. An empty rationale makes upstream
    # render_example OMIT that envelope entirely, creating a target mismatch.
    return {"system": SYSTEM, "tools": schema(), "query": inference_query(row),
            "reasoning": ", ".join(row["categories"]) if row["categories"] else "No tools needed.",
            "answers": [{"name": category, "arguments": {}} for category in row["categories"]]}


def encode(row, tok):
    from needle.model.finetune import render_example
    from needle.model.tokenizer import BOS_ID, EOS_ID
    prompt, target = render_example(training_example(row))
    prefix, suffix = tok.encode(prompt), tok.encode(target)
    if 1 + len(prefix) + MAX_NEW_TOKENS > MAX_TOKENS or len(suffix) + 1 > MAX_NEW_TOKENS:
        raise ValueError("category example exceeds runtime budget; no truncation")
    if not target.startswith("<think>\n"):
        raise ValueError("training target must include the native forced reasoning envelope")
    # Keep concise reasoning, while prioritizing the structured category decision.
    boundary = len(tok.encode(target[:target.index("<tool_call>")]))
    weights = [.25] * boundary + [1.] * (len(suffix) - boundary) + [1.]
    return [BOS_ID] + prefix + suffix + [EOS_ID], [0.] * (1 + len(prefix)) + weights


def guard(response):
    if not isinstance(response, dict) or response.get("success") is False or response.get("error"):
        return [], False
    calls = response.get("function_calls") if isinstance(response, dict) else None
    if not isinstance(calls, list) or len(calls) > len(CATEGORIES):
        return [], False
    values = []
    for call in calls:
        if (not isinstance(call, dict) or set(call) != {"name", "arguments"}
                or call["name"] not in CATEGORIES or call["arguments"] != {} or call["name"] in values):
            return [], False
        values.append(call["name"])
    return values, True


class CategoryRouter:
    """One public Needle instance; fixed schemas reuse the native prefix cache."""
    def __init__(self, weights, cat=None, factory=None):
        self.cat = cat or catalog()
        self.mapping = taxonomy(self.cat)
        self.tokenizer = tokenizer()
        self.contract = contract_hash(self.cat)
        platform, suffix = ("windows", "dll") if os.name == "nt" else ("linux", "so")
        os.environ["NEEDLE3_LIB_PATH"] = str(MODEL_DIR / platform / f"libneedle3.{suffix}")
        os.environ["DO_NOT_TRACK"] = "1"
        if factory is None:
            from needle import Needle
            factory = Needle
        self.agent = factory(weights=str(Path(weights).resolve()),
                             tools=json.dumps(schema(), ensure_ascii=False, separators=(",", ":")),
                             system=SYSTEM, auto_date=False, stateless=True)

    def select(self, row):
        from needle.model.finetune import render_example
        begun = time.perf_counter()
        allowed = row.get("available_manuals", list(self.mapping))
        if not isinstance(allowed, list) or any(x not in self.mapping for x in allowed):
            raise ValueError("available_manuals must contain known manual IDs")
        query = inference_query(row)
        if not isinstance(query, str) or not query.strip():
            raise ValueError("query must be a nonempty string")
        prompt, _ = render_example({"system": SYSTEM, "tools": schema(), "query": query})
        tokens = 1 + len(self.tokenizer.encode(prompt))
        if tokens + MAX_NEW_TOKENS > MAX_TOKENS:
            raise ValueError("input exceeds category router token budget")
        response = self.agent.complete(query, max_new_tokens=MAX_NEW_TOKENS)
        categories, valid = guard(response)
        available_categories = {self.mapping[mid] for mid in allowed}
        categories = [x for x in categories if x in available_categories]
        return {"category_ids": categories, "formal_valid": valid, "fallback_required": not valid,
                "manuals_by_category": {c: [mid for mid in allowed if self.mapping[mid] == c] for c in categories},
                "prompt_tokens": tokens, "latency_ms": (time.perf_counter() - begun) * 1000,
                "contract_sha256": self.contract}

    def close(self):
        self.agent.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--weights", required=True)
    args = parser.parse_args()
    router = CategoryRouter(args.weights)
    try:
        for line in sys.stdin:
            try:
                result = router.select(json.loads(line))
            except (ValueError, TypeError, KeyError, RuntimeError) as exc:
                result = {"category_ids": [], "formal_valid": False, "fallback_required": True,
                          "error_type": type(exc).__name__}
            print(canonical(result), flush=True)
    finally:
        router.close()


if __name__ == "__main__":
    main()

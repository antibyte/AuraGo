"""One schema and prompt contract for dataset compilation and complete()."""
from __future__ import annotations

import json
import re

from .common import inference_query

SYSTEM = "Select up to three available manual families needed to answer the current user request, in first-use order. Use context to resolve follow-ups and corrections. Missing execution parameters do not remove relevance. Return no calls when none apply. Manual functions are labels and take no arguments."
MAX_NEW_TOKENS = 256


def schemas(cat, candidates):
    manuals = {m["id"]: m for m in cat["manuals"]}
    if len(candidates) != len(set(candidates)) or set(candidates) - manuals.keys():
        raise ValueError("candidate IDs must be unique catalog families")
    result = []
    for mid in candidates:
        if not re.fullmatch(r"[a-z][a-z0-9_]*", mid):
            raise ValueError("manual ID is not a valid pseudo-function name")
        descriptions = [t["description"].split("\n")[0].split(". ")[0] for t in cat["tools"] if t.get("manual_id") == mid]
        # Deterministic short labels, versioned with the catalog. Full manuals stay in source evidence.
        description = "; ".join(dict.fromkeys(descriptions))[:180]
        result.append({"name": "manual_" + mid, "description": description,
                       "parameters": {"type": "object", "properties": {}, "required": [], "additionalProperties": False}})
    return result


def tools_json(cat, candidates):
    # Pass this exact string to Needle to avoid its default whitespace/ASCII serializer.
    return json.dumps(schemas(cat, candidates), ensure_ascii=False, separators=(",", ":"))


def example(row, cat, candidates, training=False):
    visible = [mid for mid in row["answers"] if mid in candidates]
    # A rationale mentioning missing labels leaks oracle information in training targets.
    reasoning = row["reasoning"] if set(row["all_manual_ids"]) <= set(candidates) else (
        "The available manuals cover only part of the request." if visible else "No available manual matches the request.")
    return {"system": SYSTEM, "query": inference_query(row), "tools": schemas(cat, candidates),
            "answers": [{"name": "manual_" + mid, "arguments": {}} for mid in visible], "reasoning": reasoning}


def encode_exact(example, tokenizer, maximum=2048):
    from needle.model.finetune import render_example
    from needle.model.tokenizer import BOS_ID, EOS_ID
    prompt, target = render_example(example)
    prefix, suffix = tokenizer.encode(prompt), tokenizer.encode(target)
    if len(prefix) + 1 + MAX_NEW_TOKENS > maximum or len(suffix) + 1 > MAX_NEW_TOKENS:
        raise ValueError("example exceeds the same prompt/completion budget used by the native runtime")
    ids = [BOS_ID] + prefix + suffix + [EOS_ID]
    if len(ids) > maximum:
        raise ValueError(f"fully rendered example has {len(ids)} tokens; limit is {maximum}; nothing was truncated")
    return ids, [0.0] * (len(prefix) + 1) + [1.0] * (len(suffix) + 1)


def validate_output(response, candidates, allowed):
    calls = response.get("function_calls") if isinstance(response, dict) else None
    if not isinstance(calls, list) or len(calls) > 3:
        return [], False
    result = []
    for call in calls:
        if not isinstance(call, dict) or set(call) - {"name", "arguments"}:
            return [], False
        name = call.get("name")
        if not isinstance(name, str) or not name.startswith("manual_") or call.get("arguments") != {}:
            return [], False
        mid = name[len("manual_"):]
        if mid not in candidates or mid not in allowed or mid in result:
            return [], False
        result.append(mid)
    return result, True

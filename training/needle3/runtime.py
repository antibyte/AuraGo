"""Local manual selection with complete(); pseudo-functions are never executed."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sys
import time

from .assets import MODEL_DIR, tokenizer
from .common import catalog, canonical, inference_query
from .serialization import MAX_NEW_TOKENS, SYSTEM, tools_json, validate_output


class Selector:
    def __init__(self, weights, cat=None, factory=None):
        self.cat = cat or catalog()
        self.known = {m["id"] for m in self.cat["manuals"]}
        self.weights = str(Path(weights).resolve())
        self.tokenizer = tokenizer()
        self.factory = factory
        self.agent = None
        self.candidates = None
        suffix, platform = ("dll", "windows") if sys.platform == "win32" else ("so", "linux")
        os.environ["NEEDLE3_LIB_PATH"] = str(MODEL_DIR / platform / f"libneedle3.{suffix}")
        os.environ["DO_NOT_TRACK"] = "1"

    def select(self, row, candidates, allowed=None):
        from needle.model.finetune import render_example
        allowed = self.known if allowed is None else set(allowed)
        if allowed - self.known or set(candidates) - allowed:
            raise ValueError("candidate outside the allowed catalog")
        serialized = tools_json(self.cat, candidates)
        prompt, _ = render_example({"system": SYSTEM, "tools": serialized, "query": inference_query(row)})
        count = 1 + len(self.tokenizer.encode(prompt))
        if count + MAX_NEW_TOKENS > 2048:
            raise ValueError("input plus completion reservation exceeds 2048 tokens")
        started = time.perf_counter()
        if self.candidates != candidates:
            self.close()
            if self.factory is None:
                from needle import Needle
                self.factory = Needle
            self.agent = self.factory(weights=self.weights, tools=serialized, system=SYSTEM, auto_date=False, stateless=True)
            self.candidates = list(candidates)
        initialized = time.perf_counter()
        response = self.agent.complete(inference_query(row), max_new_tokens=MAX_NEW_TOKENS)
        ids, valid = validate_output(response, candidates, allowed)
        return {"manual_ids": ids, "formal_valid": valid, "latency_ms": (time.perf_counter() - started) * 1000,
                "schema_setup_ms": (initialized - started) * 1000,
                "prompt_tokens": count, "confidence": None, "fallback_required": not valid}

    def close(self):
        if self.agent is not None:
            self.agent.close()
            self.agent = None
            self.candidates = None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--weights", required=True)
    parser.add_argument("--search-binary", required=True)
    parser.add_argument("--candidates", type=int, choices=[12, 16, 20], default=12)
    args = parser.parse_args()
    from .retrieval import Retriever
    cat = catalog()
    retriever, selector = Retriever(cat, args.search_binary), Selector(args.weights, cat)
    try:
        for line in sys.stdin:
            started = time.perf_counter()
            try:
                row = json.loads(line)
                allowed = row.get("available_manuals")
                candidates = retriever.retrieve(row, allowed, args.candidates)
                retrieved = time.perf_counter()
                result = {**selector.select(row, candidates, allowed), "candidates": candidates,
                          "retrieval_ms": (retrieved - started) * 1000,
                          "end_to_end_ms": (time.perf_counter() - started) * 1000}
            except (ValueError, TypeError, AttributeError, KeyError, RuntimeError, OSError) as exc:
                result = {"manual_ids": [], "formal_valid": False, "fallback_required": True, "error_type": type(exc).__name__}
            print(canonical({**result, "catalog_sha256": cat["catalog_sha256"]}), flush=True)
    finally:
        selector.close()
        retriever.close()


if __name__ == "__main__":
    main()

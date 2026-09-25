"""Exercise real complete() calls, availability guards and order sensitivity."""
from __future__ import annotations

import argparse
import platform
import time

from .assets import verify
from .common import catalog, file_digest, write_json
from .runtime import Selector

CASES = [
    ({"query": "Bitte lies meine ungelesenen E-Mails.", "context": []}, ["email"]),
    ({"query": "List the running Docker containers.", "context": []}, ["docker"]),
    ({"query": "Nein, sende ihn über Discord.", "context": ["Schicke den Statusbericht per E-Mail."]}, ["discord"]),
    ({"query": "Was ist sieben plus acht?", "context": []}, []),
]


def smoke(weights, output):
    verify()
    cat = catalog()
    selector = Selector(weights, cat)
    candidates = ["email", "discord", "docker", "mqtt", "home_assistant", "proxmox", "filesystem", "office_workbook", "workspace_search", "remote_control_shell", "manage_notes", "send_notification"]
    results = []
    try:
        for row, expected in CASES:
            for order in (candidates, list(reversed(candidates))):
                started = time.perf_counter()
                result = selector.select(row, order)
                if not result["formal_valid"]:
                    raise ValueError("native runtime returned invalid calls")
                results.append({"input": row, "expected": expected, "actual": result, "reversed": order != candidates,
                                "correct": result["manual_ids"] == expected, "total_ms": (time.perf_counter() - started) * 1000})
        with_empty = selector.select(CASES[0][0], [], [])
        if with_empty["manual_ids"]:
            raise ValueError("empty availability leaked a manual")
    finally:
        selector.close()
    report = {"platform": platform.platform(), "python": platform.python_version(), "model_sha256": file_digest(weights),
              "native_complete_verified": True, "cases": results, "empty_availability": with_empty,
              "exact_matches": sum(r["correct"] for r in results), "examples": len(results),
              "training_acceptance": False}
    write_json(output, report)
    print(f"Native complete(): {len(results)} valid cases, {report['exact_matches']} exact matches; empty availability respected")
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--weights", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    smoke(args.weights, args.out)

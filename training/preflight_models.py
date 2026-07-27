#!/usr/bin/env python3
"""Verify model access, license, template family, Unsloth install, and GGUF gate metadata."""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any

try:
    from train_unsloth import resolve_adapter
except ImportError:  # pragma: no cover - package import used by tests
    from training.train_unsloth import resolve_adapter


def parse_args() -> argparse.Namespace:
    root = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=root / "model_candidates.json")
    parser.add_argument("--model", action="append", default=[], help="candidate ID; repeat to select several")
    parser.add_argument(
        "--accept-license",
        action="append",
        default=[],
        help="record explicit review acceptance for a non-Apache candidate ID",
    )
    parser.add_argument("--report", type=Path, default=root / "model_preflight_report.json")
    parser.add_argument(
        "--skip-unsloth-import",
        action="store_true",
        help="metadata-only check; never valid as the final cloud-GPU preflight",
    )
    return parser.parse_args()


def load_candidates(path: Path, selected: set[str]) -> list[dict[str, Any]]:
    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SystemExit(f"cannot read candidate manifest: {exc}") from exc
    candidates = manifest.get("candidates")
    if not isinstance(candidates, list) or not candidates:
        raise SystemExit("candidate manifest has no candidates")
    known = {candidate.get("id") for candidate in candidates}
    unknown = sorted(selected - known)
    if unknown:
        raise SystemExit(f"unknown candidate IDs: {unknown}")
    return [candidate for candidate in candidates if not selected or candidate.get("id") in selected]


def card_license(info: Any) -> str:
    card_data = getattr(info, "card_data", None)
    if card_data is None:
        return ""
    if isinstance(card_data, dict):
        value = card_data.get("license")
    else:
        value = getattr(card_data, "license", "")
    if isinstance(value, list):
        return ",".join(str(item) for item in value)
    return str(value or "")


def preflight_candidate(
    candidate: dict[str, Any],
    *,
    accepted_licenses: set[str],
    skip_unsloth_import: bool,
) -> dict[str, Any]:
    try:
        from huggingface_hub import HfApi
        from transformers import AutoConfig
    except ImportError as exc:
        raise SystemExit("run `uv sync --project training --frozen` before model preflight") from exc

    model_id = str(candidate.get("id") or "")
    result: dict[str, Any] = {"id": model_id, "passed": False, "gates": {}, "errors": []}
    errors: list[str] = result["errors"]
    token = os.environ.get("HF_TOKEN")
    api = HfApi(token=token)
    try:
        info = api.model_info(model_id, files_metadata=False)
        result["revision"] = str(info.sha or "")
        result["hub_license"] = card_license(info)
        result["gated"] = bool(info.gated)
        files = api.list_repo_files(model_id, revision=info.sha, token=token)
        result["gates"]["access"] = bool(files)
    except Exception as exc:  # noqa: BLE001 - Hub clients expose several access errors
        result["gates"]["access"] = False
        errors.append(f"model access failed: {exc}")
        return result

    expected_license = str(candidate.get("license") or "")
    actual_license = str(result.get("hub_license") or "")
    license_matches = bool(actual_license) and (
        expected_license.lower() in actual_license.lower()
        or actual_license.lower() in expected_license.lower()
    )
    if not license_matches:
        errors.append(f"license mismatch: manifest={expected_license!r}, hub={actual_license!r}")
    review_required = candidate.get("license_review") == "required"
    review_passed = not review_required or model_id in accepted_licenses
    if not review_passed:
        errors.append("non-Apache license requires explicit --accept-license after human review")
    result["gates"]["license"] = license_matches and review_passed

    try:
        adapter = resolve_adapter(model_id)
        config = AutoConfig.from_pretrained(model_id, revision=result["revision"], token=token)
        result["adapter"] = adapter.name
        result["model_type"] = str(getattr(config, "model_type", "") or "")
        result["architectures"] = list(getattr(config, "architectures", []) or [])
        template_gate = bool(result["model_type"] and result["architectures"])
    except (Exception, SystemExit) as exc:  # resolve_adapter intentionally fails with SystemExit
        template_gate = False
        errors.append(f"template/config preflight failed: {exc}")
    result["gates"]["native_template_family"] = template_gate

    metadata_gate = bool(
        candidate.get("tool_calling")
        and candidate.get("unsloth_family_supported")
        and candidate.get("gguf_export")
    )
    if not metadata_gate:
        errors.append("candidate manifest does not approve tool calling, Unsloth family, and GGUF export")
    result["gates"]["candidate_metadata"] = metadata_gate

    if skip_unsloth_import:
        result["gates"]["unsloth_runtime"] = None
        result["metadata_only"] = True
    else:
        try:
            from unsloth import FastLanguageModel

            runtime_gate = callable(getattr(FastLanguageModel, "from_pretrained", None))
        except Exception as exc:  # noqa: BLE001 - CUDA/extension import errors are a failed gate
            runtime_gate = False
            errors.append(f"Unsloth runtime import failed: {exc}")
        result["gates"]["unsloth_runtime"] = runtime_gate

    required_gates = [value for value in result["gates"].values() if value is not None]
    result["passed"] = bool(required_gates) and all(required_gates) and not errors
    return result


def main() -> None:
    args = parse_args()
    candidates = load_candidates(args.manifest, set(args.model))
    results = [
        preflight_candidate(
            candidate,
            accepted_licenses=set(args.accept_license),
            skip_unsloth_import=args.skip_unsloth_import,
        )
        for candidate in candidates
    ]
    payload = {
        "schema_version": "1",
        "metadata_only": args.skip_unsloth_import,
        "all_passed": all(result["passed"] for result in results),
        "candidates": results,
    }
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps(payload, indent=2, ensure_ascii=False))
    if not payload["all_passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()

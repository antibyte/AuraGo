#!/usr/bin/env python3
"""Validate the complete AuraGo tool-calling training pack without a GPU."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterator

try:
    from jsonschema import Draft7Validator
except ImportError as exc:  # pragma: no cover - exercised by CLI environments
    raise SystemExit("jsonschema is required; install training/requirements.txt") from exc


SCHEMA_VERSION = "2.0"
EXPECTED_TOOLS = 202
EXPECTED_SCENARIOS = 5000
EXPECTED_CHALLENGE = 404
MAX_TOOLS = 20
MAX_SCHEMA_TOKENS = 6500
TOOL_TAG = re.compile(r"<tool_call>\s*(\{.*?\})\s*</tool_call>", re.DOTALL)
SECRET_PATTERNS = [
    re.compile(r"(?i)\b(?:api[_-]?key|token|password|secret)\s*[=:]\s*(?!example\b|\[redacted\])\S{8,}"),
    re.compile(r"\bsk-[A-Za-z0-9_-]{16,}\b"),
    re.compile(r"(?i)\bBearer\s+(?!\[redacted\])[A-Za-z0-9._~+/=-]{16,}"),
    re.compile(r"(?i)\b[A-Z]:\\Users\\[^\\\s]+"),
    re.compile(r"/(?:home|Users)/[^/\s]+"),
]


class ValidationFailure(RuntimeError):
    pass


@dataclass
class Stats:
    rows: int = 0
    languages: Counter[str] = field(default_factory=Counter)
    kinds: Counter[str] = field(default_factory=Counter)
    splits: Counter[str] = field(default_factory=Counter)
    target_counts: Counter[str] = field(default_factory=Counter)
    challenge_counts: Counter[str] = field(default_factory=Counter)
    operation_languages: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    call_summaries: dict[str, list[tuple[str, str, str]]] = field(default_factory=dict)
    ids_by_split: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    conversation_hashes: dict[str, str] = field(default_factory=dict)
    selection_prompts: int = 0
    named_selection_prompts: int = 0
    trace_rows: int = 0
    call_ids: set[str] = field(default_factory=set)
    family_splits: dict[str, str] = field(default_factory=dict)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--all", action="store_true", help="validate every generated and compatibility artifact")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent)
    return parser.parse_args()


def read_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValidationFailure(f"{path}: {exc}") from exc


def iter_jsonl(path: Path) -> Iterator[tuple[int, dict[str, Any]]]:
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                try:
                    row = json.loads(line)
                except json.JSONDecodeError as exc:
                    raise ValidationFailure(f"{path}:{line_no}: invalid JSON: {exc}") from exc
                if not isinstance(row, dict):
                    raise ValidationFailure(f"{path}:{line_no}: row is not an object")
                yield line_no, row
    except OSError as exc:
        raise ValidationFailure(f"{path}: {exc}") from exc


def canonical(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def load_catalog(root: Path) -> tuple[dict[str, dict[str, Any]], dict[str, Draft7Validator], dict[str, str]]:
    catalog = read_json(root / "tools_catalog.json")
    if catalog.get("schema_version") != SCHEMA_VERSION:
        raise ValidationFailure("tools_catalog.json has an unsupported schema version")
    tools = catalog.get("tools")
    if not isinstance(tools, list) or len(tools) != EXPECTED_TOOLS:
        raise ValidationFailure(f"catalog tool count is {len(tools or [])}, expected {EXPECTED_TOOLS}")
    by_name: dict[str, dict[str, Any]] = {}
    validators: dict[str, Draft7Validator] = {}
    tiers: dict[str, str] = {}
    for tool in tools:
        name = tool.get("name")
        if not isinstance(name, str) or not name or name in by_name:
            raise ValidationFailure(f"catalog contains invalid or duplicate tool name {name!r}")
        schema = tool.get("parameters")
        if not isinstance(schema, dict):
            raise ValidationFailure(f"catalog tool {name} has no parameter schema")
        Draft7Validator.check_schema(schema)
        by_name[name] = tool
        validators[name] = Draft7Validator(schema)
        tiers[name] = tool.get("tier", "")
    return by_name, validators, tiers


def validate_source_manifests(
    root: Path,
    tools: dict[str, dict[str, Any]],
) -> tuple[dict[str, dict[str, Any]], int]:
    tiers = read_json(root / "tool_tiers.json")
    assigned: dict[str, str] = {}
    for tier in ("core", "extended", "rare"):
        names = tiers.get(tier)
        if not isinstance(names, list):
            raise ValidationFailure(f"tool_tiers.json lacks {tier}")
        for name in names:
            if name in assigned:
                raise ValidationFailure(f"tool {name} is assigned to multiple tiers")
            assigned[name] = tier
    if set(assigned) != set(tools):
        missing = sorted(set(tools) - set(assigned))
        extra = sorted(set(assigned) - set(tools))
        raise ValidationFailure(f"tier coverage mismatch; missing={missing}, extra={extra}")

    contracts = read_json(root / "operation_contracts.json")
    if contracts.get("version") != 2:
        raise ValidationFailure("operation_contracts.json has an unsupported version")
    contract_tools = contracts.get("tools")
    if not isinstance(contract_tools, dict) or set(contract_tools) != set(tools):
        raise ValidationFailure("operation contract tool coverage differs from the catalog")
    operation_count = 0
    for name, contract in contract_tools.items():
        validator = Draft7Validator(tools[name]["parameters"])
        default_arguments = contract.get("default_arguments")
        if not isinstance(default_arguments, dict) or list(validator.iter_errors(default_arguments)):
            raise ValidationFailure(f"default operation fixture for {name} is not schema valid")
        operations = contract.get("operations") or []
        if not isinstance(operations, list):
            raise ValidationFailure(f"operation contract for {name} has invalid operations")
        expected_operations = [
            (operation.get("selector"), canonical(operation.get("value")))
            for operation in tools[name].get("operations") or []
        ]
        actual_operations: list[tuple[Any, str]] = []
        for fixture in operations:
            selector = fixture.get("selector")
            value = fixture.get("value")
            arguments = fixture.get("arguments")
            required_fields = fixture.get("required_fields")
            excluded_fields = fixture.get("excluded_fields")
            actual_operations.append((selector, canonical(value)))
            if not isinstance(arguments, dict) or list(validator.iter_errors(arguments)):
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} is not schema valid")
            if arguments.get(selector) != value:
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} lacks selector value")
            if not isinstance(required_fields, list) or selector not in required_fields:
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} lacks required_fields")
            if not isinstance(excluded_fields, list):
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} lacks excluded_fields")
            if len(required_fields) != len(set(required_fields)) or len(excluded_fields) != len(set(excluded_fields)):
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} repeats fields")
            if set(required_fields).intersection(excluded_fields):
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} conflicts on fields")
            if any(field not in arguments for field in required_fields):
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} misses required argument")
            if any(field in arguments for field in excluded_fields):
                raise ValidationFailure(f"operation fixture for {name} {selector}={value!r} includes excluded argument")
        if actual_operations != expected_operations:
            raise ValidationFailure(f"operation fixtures for {name} differ from the live catalog")
        operation_count += len(operations)
    return contract_tools, operation_count


def validate_messages(
    path: Path,
    line_no: int,
    row: dict[str, Any],
    tools: dict[str, dict[str, Any]],
    validators: dict[str, Draft7Validator],
) -> list[tuple[str, str, str]]:
    row_id = row.get("id")
    available_defs = row.get("tools")
    if not isinstance(available_defs, list) or not available_defs:
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: missing tool context")
    if len(available_defs) > MAX_TOOLS:
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: more than {MAX_TOOLS} tools")
    schema_tokens = (len(canonical(available_defs)) + 3) // 4
    if schema_tokens > MAX_SCHEMA_TOKENS:
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: {schema_tokens} schema tokens exceed {MAX_SCHEMA_TOKENS}")

    available: set[str] = set()
    for definition in available_defs:
        function = definition.get("function") if isinstance(definition, dict) else None
        name = function.get("name") if isinstance(function, dict) else None
        if not isinstance(name, str) or name in available or name not in tools:
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: invalid tool definition {name!r}")
        if canonical(function.get("parameters")) != canonical(tools[name]["parameters"]):
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: schema drift for {name}")
        available.add(name)

    messages = row.get("messages")
    if not isinstance(messages, list) or len(messages) < 3:
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: invalid messages")
    calls: list[tuple[str, str, str]] = []
    seen_ids: set[str] = set()
    index = 0
    while index < len(messages):
        message = messages[index]
        if not isinstance(message, dict):
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: message {index} is not an object")
        role = message.get("role")
        tool_calls = message.get("tool_calls") or []
        if tool_calls and role != "assistant":
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: tool_calls on role {role!r}")
        if role == "tool":
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: orphan role=tool message at {index}")
        if not tool_calls:
            index += 1
            continue

        expected_ids: list[str] = []
        for call in tool_calls:
            call_id = call.get("id")
            function = call.get("function") or {}
            name = function.get("name")
            arguments_raw = function.get("arguments")
            if not isinstance(call_id, str) or not call_id or call_id in seen_ids:
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: duplicate/empty call ID")
            if name not in available:
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: call to unavailable tool {name!r}")
            if not isinstance(arguments_raw, str):
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: arguments for {name} are not a JSON string")
            try:
                arguments = json.loads(arguments_raw)
            except json.JSONDecodeError as exc:
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: invalid arguments for {name}: {exc}") from exc
            if not isinstance(arguments, dict):
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: arguments for {name} are not an object")
            errors = sorted(validators[name].iter_errors(arguments), key=lambda item: list(item.path))
            if errors:
                detail = "; ".join(error.message for error in errors[:4])
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: schema-invalid {name}: {detail}")
            seen_ids.add(call_id)
            expected_ids.append(call_id)
            calls.append((call_id, name, canonical(arguments)))

        result_ids: list[str] = []
        cursor = index + 1
        while cursor < len(messages) and messages[cursor].get("role") == "tool":
            result = messages[cursor]
            result_id = result.get("tool_call_id")
            if result_id not in expected_ids or result_id in result_ids:
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: mismatched tool result {result_id!r}")
            if result.get("name") and result.get("name") != next(
                name for call_id, name, _ in calls if call_id == result_id
            ):
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: tool result name mismatch")
            result_ids.append(result_id)
            cursor += 1
        if result_ids != expected_ids:
            raise ValidationFailure(
                f"{path}:{line_no}: {row_id}: result IDs {result_ids} do not match calls {expected_ids}"
            )
        index = cursor

    expectations = row.get("expectations")
    if not isinstance(expectations, dict):
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: missing expectations")
    if bool(expectations.get("should_call")) != bool(calls):
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: should_call mismatch")
    expected_calls = expectations.get("calls") or []
    actual_without_ids = [(name, arguments) for _, name, arguments in calls]
    normalized_expected = [
        (call.get("name"), canonical(call.get("arguments"))) for call in expected_calls
    ]
    if actual_without_ids != normalized_expected:
        raise ValidationFailure(f"{path}:{line_no}: {row_id}: expected calls differ from messages")
    return calls


def validate_native_dataset(
    path: Path,
    tools: dict[str, dict[str, Any]],
    validators: dict[str, Draft7Validator],
    *,
    challenge: bool = False,
) -> Stats:
    stats = Stats()
    for line_no, row in iter_jsonl(path):
        stats.rows += 1
        row_id = row.get("id")
        if not isinstance(row_id, str) or not row_id:
            raise ValidationFailure(f"{path}:{line_no}: missing id")
        if row.get("schema_version") != SCHEMA_VERSION:
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: wrong schema version")
        if row_id in stats.call_summaries:
            raise ValidationFailure(f"{path}:{line_no}: duplicate id {row_id}")
        raw = canonical({"tools": row.get("tools"), "messages": row.get("messages")})
        digest = hashlib.sha256(raw.encode()).hexdigest()
        if digest in stats.conversation_hashes:
            raise ValidationFailure(
                f"{path}:{line_no}: rows {stats.conversation_hashes[digest]} and {row_id} are exact duplicates"
            )
        stats.conversation_hashes[digest] = row_id
        for pattern in SECRET_PATTERNS:
            match = pattern.search(raw)
            if match and "example" not in match.group(0).lower() and "[redacted]" not in match.group(0).lower():
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: possible secret or private path")

        calls = validate_messages(path, line_no, row, tools, validators)
        for call_id, _, _ in calls:
            if call_id in stats.call_ids:
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: globally duplicate call ID {call_id}")
            stats.call_ids.add(call_id)
        stats.call_summaries[row_id] = calls
        language = row.get("language")
        kind = row.get("kind")
        split = row.get("split")
        family = row.get("family")
        expectations = row.get("expectations") or {}
        target = expectations.get("target_tool")
        if challenge:
            if split != "challenge":
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: challenge row has split {split!r}")
            stats.languages[language] += 1
            stats.kinds[kind] += 1
            if target:
                stats.challenge_counts[target] += 1
            continue

        stats.languages[language] += 1
        stats.kinds[kind] += 1
        stats.splits[split] += 1
        stats.ids_by_split[split].add(row_id)
        if not isinstance(family, str) or not family:
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: missing scenario family")
        previous_split = stats.family_splits.get(family)
        if previous_split is not None and previous_split != split:
            raise ValidationFailure(
                f"{path}:{line_no}: family {family!r} crosses {previous_split} and {split}"
            )
        stats.family_splits[family] = split
        if target:
            stats.target_counts[target] += 1
        if str(row.get("source", "")).startswith("trace"):
            stats.trace_rows += 1
        if expectations.get("should_call") and target:
            stats.selection_prompts += 1
            user_text = "\n".join(
                str(message.get("content", ""))
                for message in row.get("messages", [])
                if message.get("role") == "user"
            ).lower()
            if target.lower() in user_text:
                stats.named_selection_prompts += 1
        selector = expectations.get("selector")
        if kind == "direct_success" and selector and language in {"de", "en"}:
            operation_key = f"{target}:{selector}:{canonical(expectations.get('value'))}"
            stats.operation_languages[operation_key].add(language)
    return stats


def validate_tagged_dataset(path: Path, expected: dict[str, list[tuple[str, str, str]]]) -> int:
    seen: set[str] = set()
    rows = 0
    for line_no, row in iter_jsonl(path):
        rows += 1
        row_id = row.get("id")
        if row_id not in expected or row_id in seen:
            raise ValidationFailure(f"{path}:{line_no}: unknown or duplicate id {row_id!r}")
        seen.add(row_id)
        parsed: list[tuple[str, str, str]] = []
        for message in row.get("messages", []):
            if message.get("tool_calls"):
                raise ValidationFailure(f"{path}:{line_no}: {row_id}: tagged row retained structured tool_calls")
            if message.get("role") != "assistant":
                continue
            content = str(message.get("content", ""))
            for match in TOOL_TAG.finditer(content):
                try:
                    payload = json.loads(match.group(1))
                except json.JSONDecodeError as exc:
                    raise ValidationFailure(f"{path}:{line_no}: {row_id}: invalid tool tag: {exc}") from exc
                arguments = payload.get("arguments")
                if not isinstance(arguments, dict):
                    raise ValidationFailure(f"{path}:{line_no}: {row_id}: tagged arguments are not an object")
                parsed.append((payload.get("id"), payload.get("name"), canonical(arguments)))
        if parsed != expected[row_id]:
            raise ValidationFailure(f"{path}:{line_no}: {row_id}: native/tagged roundtrip mismatch")
    if seen != set(expected):
        raise ValidationFailure(f"{path}: tagged IDs differ from native IDs")
    return rows


def validate_coverage(
    stats: Stats,
    tools: dict[str, dict[str, Any]],
    contracts: dict[str, dict[str, Any]],
    tiers: dict[str, str],
    operation_count: int,
) -> None:
    if stats.rows != EXPECTED_SCENARIOS:
        raise ValidationFailure(f"native dataset has {stats.rows} rows, expected {EXPECTED_SCENARIOS}")
    if stats.languages != Counter({"de": 3000, "en": 2000}):
        raise ValidationFailure(f"language distribution is {dict(stats.languages)}, expected de=3000/en=2000")
    expected_kinds = Counter(
        {
            "direct_success": 2250,
            "multi_call": 1000,
            "discover_invoke": 500,
            "tool_error": 375,
            "tool_recovery": 375,
            "clarification": 100,
            "disabled": 100,
            "safety_refusal": 100,
            "irrelevant": 100,
            "missing_parameters": 100,
        }
    )
    if stats.kinds != expected_kinds:
        raise ValidationFailure(f"scenario distribution differs: {dict(stats.kinds)}")
    for split, minimum in {"train": 3750, "validation": 350, "test": 350}.items():
        if stats.splits[split] < minimum:
            raise ValidationFailure(f"split {split} has only {stats.splits[split]} rows")
    if stats.trace_rows > EXPECTED_SCENARIOS * 0.20:
        raise ValidationFailure(f"trace rows exceed 20%: {stats.trace_rows}")
    if stats.selection_prompts:
        named_ratio = stats.named_selection_prompts / stats.selection_prompts
        if named_ratio > 0.10:
            raise ValidationFailure(
                f"{stats.named_selection_prompts}/{stats.selection_prompts} selection prompts name the target ({named_ratio:.1%})"
            )

    observed_operations = 0
    for name, tool_contract in contracts.items():
        minimum = {"core": 20, "extended": 10, "rare": 6}[tiers[name]]
        if stats.target_counts[name] < minimum:
            raise ValidationFailure(f"tool {name} has {stats.target_counts[name]} scenarios, minimum {minimum}")
        for operation in tool_contract.get("operations") or []:
            observed_operations += 1
            key = f"{name}:{operation['selector']}:{canonical(operation.get('value'))}"
            if stats.operation_languages[key] != {"de", "en"}:
                raise ValidationFailure(f"operation {key} lacks German and English direct coverage")
    if observed_operations != operation_count:
        raise ValidationFailure(f"validated {observed_operations} operations, expected {operation_count}")


def validate_challenge(stats: Stats, tools: dict[str, dict[str, Any]]) -> None:
    if stats.rows != EXPECTED_CHALLENGE:
        raise ValidationFailure(f"challenge set has {stats.rows} rows, expected {EXPECTED_CHALLENGE}")
    for name in tools:
        if stats.challenge_counts[name] != 2:
            raise ValidationFailure(f"challenge set has {stats.challenge_counts[name]} rows for {name}, expected 2")
    required_kinds = {
        "challenge_positive",
        "challenge_multi_call",
        "challenge_clarification",
        "challenge_disabled",
        "challenge_safety_refusal",
        "challenge_irrelevant",
        "challenge_missing_parameters",
    }
    missing = sorted(required_kinds - set(stats.kinds))
    if missing:
        raise ValidationFailure(f"challenge set lacks required scenario kinds: {missing}")


def validate_split_files(root: Path, expected: dict[str, set[str]]) -> None:
    for split in ("train", "validation", "test"):
        ids: set[str] = set()
        for line_no, row in iter_jsonl(root / f"dataset_native_fc_{split}.jsonl"):
            row_id = row.get("id")
            if row.get("split") != split or row_id in ids:
                raise ValidationFailure(f"split file {split}:{line_no} has invalid row {row_id!r}")
            ids.add(row_id)
        if ids != expected[split]:
            raise ValidationFailure(f"split file {split} does not match dataset_native_fc.jsonl")


def validate_compatibility_files(root: Path) -> None:
    for name in ("dataset_chatml.jsonl", "dataset_sharegpt.jsonl", "dataset_alpaca.jsonl"):
        rows = sum(1 for _ in iter_jsonl(root / name))
        if rows != EXPECTED_SCENARIOS:
            raise ValidationFailure(f"{name} has {rows} rows, expected {EXPECTED_SCENARIOS}")


def validate_hash_manifest(root: Path, operation_count: int) -> None:
    manifest = read_json(root / "dataset_manifest.json")
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise ValidationFailure("dataset_manifest.json has the wrong version")
    if manifest.get("operation_count") != operation_count:
        raise ValidationFailure("dataset_manifest.json operation count is stale")
    hashes = manifest.get("sha256")
    if not isinstance(hashes, dict) or not hashes:
        raise ValidationFailure("dataset_manifest.json has no artifact hashes")
    for name, expected in hashes.items():
        path = root / name
        try:
            actual = hashlib.sha256(path.read_bytes()).hexdigest()
        except OSError as exc:
            raise ValidationFailure(f"{path}: {exc}") from exc
        if actual != expected:
            raise ValidationFailure(f"{name} hash mismatch: {actual} != {expected}")


def main() -> None:
    args = parse_args()
    root = args.root.resolve()
    tools, validators, tiers = load_catalog(root)
    contracts, operation_count = validate_source_manifests(root, tools)
    native_stats = validate_native_dataset(root / "dataset_native_fc.jsonl", tools, validators)
    validate_coverage(native_stats, tools, contracts, tiers, operation_count)
    tagged_rows = validate_tagged_dataset(
        root / "dataset_tagged_fc.jsonl",
        native_stats.call_summaries,
    )
    challenge_stats = validate_native_dataset(
        root / "dataset_challenge_native_fc.jsonl",
        tools,
        validators,
        challenge=True,
    )
    validate_challenge(challenge_stats, tools)
    validate_hash_manifest(root, operation_count)
    if args.all:
        validate_split_files(root, native_stats.ids_by_split)
        validate_compatibility_files(root)
    print(
        "VALID "
        f"schema={SCHEMA_VERSION} tools={len(tools)} operations={operation_count} "
        f"native={native_stats.rows} tagged={tagged_rows} challenge={challenge_stats.rows} "
        f"splits={dict(native_stats.splits)} languages={dict(native_stats.languages)}"
    )


if __name__ == "__main__":
    try:
        main()
    except ValidationFailure as exc:
        print(f"INVALID: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc

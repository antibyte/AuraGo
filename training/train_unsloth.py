#!/usr/bin/env python3
"""Reproducible Unsloth SFT for AuraGo native tool calling."""

from __future__ import annotations

import argparse
import hashlib
import inspect
import json
import os
import subprocess
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class TemplateAdapter:
    name: str
    aliases: tuple[str, ...]
    instruction_marker: str
    response_marker: str
    target_modules: tuple[str, ...]


STANDARD_TARGETS = (
    "q_proj",
    "k_proj",
    "v_proj",
    "o_proj",
    "gate_proj",
    "up_proj",
    "down_proj",
)

ADAPTERS = (
    TemplateAdapter(
        name="qwen3",
        aliases=("qwen3",),
        instruction_marker="<|im_start|>user\n",
        response_marker="<|im_start|>assistant\n",
        target_modules=STANDARD_TARGETS,
    ),
    TemplateAdapter(
        name="functiongemma",
        aliases=("functiongemma",),
        instruction_marker="<start_of_turn>user\n",
        response_marker="<start_of_turn>model\n",
        target_modules=STANDARD_TARGETS,
    ),
    TemplateAdapter(
        name="lfm2.5",
        aliases=("lfm2.5", "lfm2_5", "lfm2-"),
        instruction_marker="<|im_start|>user\n",
        response_marker="<|im_start|>assistant\n",
        target_modules=("q_proj", "k_proj", "v_proj", "out_proj", "in_proj", "w1", "w2", "w3"),
    ),
    TemplateAdapter(
        name="smollm3",
        aliases=("smollm3",),
        instruction_marker="<|im_start|>user\n",
        response_marker="<|im_start|>assistant\n",
        target_modules=STANDARD_TARGETS,
    ),
)


def parse_args() -> argparse.Namespace:
    here = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--train-dataset",
        type=Path,
        default=here / "dataset_native_fc_train.jsonl",
        help="native function-calling training JSONL",
    )
    parser.add_argument(
        "--eval-dataset",
        type=Path,
        default=here / "dataset_native_fc_validation.jsonl",
        help="native function-calling validation JSONL",
    )
    parser.add_argument("--model", default="Qwen/Qwen3-1.7B", help="base model ID")
    parser.add_argument("--output-dir", type=Path, default=here / "outputs" / "aurago-tools-lora")
    parser.add_argument("--max-length", type=int, default=4096)
    parser.add_argument("--epochs", type=float, default=1.0)
    parser.add_argument("--max-steps", type=int, default=-1)
    parser.add_argument("--batch-size", type=int, default=1)
    parser.add_argument("--grad-accum", type=int, default=16)
    parser.add_argument("--learning-rate", type=float, default=2e-4)
    parser.add_argument("--lora-r", type=int, default=16)
    parser.add_argument("--lora-alpha", type=int, default=16)
    quantization = parser.add_mutually_exclusive_group()
    quantization.add_argument("--load-in-4bit", dest="load_in_4bit", action="store_true")
    quantization.add_argument("--no-4bit", dest="load_in_4bit", action="store_false")
    parser.set_defaults(load_in_4bit=True)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--eval-steps", type=int, default=50)
    parser.add_argument("--save-steps", type=int, default=50)
    parser.add_argument("--early-stopping-patience", type=int, default=3)
    parser.add_argument("--report-to", default="trackio", choices=("trackio",))
    parser.add_argument("--run-name", default="aurago-tool-calling")
    parser.add_argument("--push-to-hub", default="", help="private Hugging Face repository ID")
    parser.add_argument("--production", action="store_true", help="require a private Hub target and HF_TOKEN")
    parser.add_argument(
        "--estimated-cost-usd",
        type=float,
        default=0.0,
        help="reviewed cloud-GPU cost estimate; required and recorded for production runs",
    )
    parser.add_argument("--save-merged", action="store_true")
    parser.add_argument("--save-gguf", action="store_true", help="export Q4_K_M GGUF after training")
    parser.add_argument("--smoke", action="store_true", help="train two steps on 20/8 rows")
    parser.add_argument(
        "--validate-only",
        action="store_true",
        help="validate dataset structure without loading a model or GPU stack",
    )
    return parser.parse_args()


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                try:
                    row = json.loads(line)
                except json.JSONDecodeError as exc:
                    raise SystemExit(f"{path}:{line_no}: invalid JSON: {exc}") from exc
                validate_native_row(row, path, line_no)
                rows.append(row)
    except OSError as exc:
        raise SystemExit(f"cannot read {path}: {exc}") from exc
    if not rows:
        raise SystemExit(f"{path}: no rows")
    return rows


def validate_native_row(row: dict[str, Any], path: Path, line_no: int) -> None:
    if row.get("schema_version") != "2.0":
        raise SystemExit(f"{path}:{line_no}: unsupported schema_version")
    if not isinstance(row.get("id"), str) or not row["id"]:
        raise SystemExit(f"{path}:{line_no}: missing id")
    if not isinstance(row.get("tools"), list) or not row["tools"]:
        raise SystemExit(f"{path}:{line_no}: missing tools")
    if not isinstance(row.get("messages"), list) or not row["messages"]:
        raise SystemExit(f"{path}:{line_no}: missing messages")


def resolve_adapter(model_id: str) -> TemplateAdapter:
    normalized = model_id.lower()
    for adapter in ADAPTERS:
        if any(alias in normalized for alias in adapter.aliases):
            return adapter
    supported = ", ".join(adapter.name for adapter in ADAPTERS)
    raise SystemExit(f"unsupported model template for {model_id!r}; supported families: {supported}")


def messages_for_template(messages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Convert canonical OpenAI argument strings to HF template argument objects."""
    converted: list[dict[str, Any]] = []
    for message in messages:
        item = dict(message)
        calls = []
        for call in message.get("tool_calls") or []:
            normalized = {
                "id": call["id"],
                "type": "function",
                "function": dict(call["function"]),
            }
            arguments = normalized["function"].get("arguments")
            if isinstance(arguments, str):
                try:
                    normalized["function"]["arguments"] = json.loads(arguments)
                except json.JSONDecodeError as exc:
                    raise SystemExit(f"invalid canonical tool arguments: {exc}") from exc
            calls.append(normalized)
        if calls:
            item["tool_calls"] = calls
        converted.append(item)
    return converted


def render_row(tokenizer: Any, row: dict[str, Any]) -> str:
    try:
        return tokenizer.apply_chat_template(
            messages_for_template(row["messages"]),
            tools=row["tools"],
            tokenize=False,
            add_generation_prompt=False,
        )
    except Exception as exc:  # noqa: BLE001 - template errors need row context
        raise SystemExit(f"chat template failed for row {row['id']}: {exc}") from exc


def assert_response_markers(rendered: str, adapter: TemplateAdapter) -> None:
    if adapter.instruction_marker not in rendered:
        raise SystemExit(
            f"{adapter.name} template did not render the expected instruction marker "
            f"{adapter.instruction_marker!r}"
        )
    if adapter.response_marker not in rendered:
        raise SystemExit(
            f"{adapter.name} template did not render the expected response marker "
            f"{adapter.response_marker!r}"
        )


def validate_token_lengths(tokenizer: Any, rendered_rows: list[str], max_length: int) -> list[int]:
    encoded = tokenizer(rendered_rows, add_special_tokens=False)
    input_ids = encoded.get("input_ids") if isinstance(encoded, dict) else None
    if not isinstance(input_ids, list) or len(input_ids) != len(rendered_rows):
        raise SystemExit("tokenizer did not return one input_ids sequence per rendered row")
    lengths = [len(tokens) for tokens in input_ids]
    if lengths and max(lengths) > max_length:
        offenders = sum(length > max_length for length in lengths)
        raise SystemExit(
            f"{offenders} training rows exceed --max-length={max_length}; "
            f"maximum rendered length is {max(lengths)}"
        )
    return lengths


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def git_commit() -> str:
    try:
        return subprocess.run(
            ["git", "rev-parse", "HEAD"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return "unknown"


def write_run_manifest(
    path: Path,
    args: argparse.Namespace,
    adapter: TemplateAdapter,
    train_rows: int,
    eval_rows: int,
    *,
    model_revision: str = "",
    status: str,
) -> None:
    manifest = {
        "schema_version": "1",
        "status": status,
        "git_commit": git_commit(),
        "base_model": args.model,
        "model_revision": model_revision,
        "adapter": asdict(adapter),
        "train_dataset": str(args.train_dataset.resolve()),
        "train_dataset_sha256": sha256_file(args.train_dataset),
        "eval_dataset": str(args.eval_dataset.resolve()),
        "eval_dataset_sha256": sha256_file(args.eval_dataset),
        "train_rows": train_rows,
        "eval_rows": eval_rows,
        "seed": args.seed,
        "max_length": args.max_length,
        "load_in_4bit": args.load_in_4bit,
        "smoke": args.smoke,
        "estimated_cost_usd": args.estimated_cost_usd,
        "hyperparameters": {
            "epochs": args.epochs,
            "max_steps": args.max_steps,
            "batch_size": args.batch_size,
            "gradient_accumulation_steps": args.grad_accum,
            "learning_rate": args.learning_rate,
            "lora_r": args.lora_r,
            "lora_alpha": args.lora_alpha,
        },
    }
    path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


def sft_config_kwargs(args: argparse.Namespace) -> dict[str, Any]:
    kwargs: dict[str, Any] = {
        "output_dir": str(args.output_dir),
        "per_device_train_batch_size": args.batch_size,
        "per_device_eval_batch_size": 1,
        "gradient_accumulation_steps": args.grad_accum,
        "warmup_ratio": 0.03,
        "num_train_epochs": args.epochs,
        "max_steps": args.max_steps,
        "learning_rate": args.learning_rate,
        "logging_steps": 10,
        "optim": "adamw_8bit" if args.load_in_4bit else "adamw_torch_fused",
        "weight_decay": 0.01,
        "lr_scheduler_type": "cosine",
        "seed": args.seed,
        "eval_strategy": "steps",
        "eval_steps": args.eval_steps,
        "save_strategy": "steps",
        "save_steps": args.save_steps,
        "save_total_limit": 2,
        "load_best_model_at_end": True,
        "metric_for_best_model": "eval_loss",
        "greater_is_better": False,
        "report_to": args.report_to,
        "run_name": args.run_name,
        "dataset_text_field": "text",
        "packing": False,
    }
    return kwargs


def main() -> None:
    args = parse_args()
    adapter = resolve_adapter(args.model)
    train_rows = load_jsonl(args.train_dataset)
    eval_rows = load_jsonl(args.eval_dataset)
    train_ids = {row["id"] for row in train_rows}
    overlap = train_ids.intersection(row["id"] for row in eval_rows)
    if overlap:
        raise SystemExit(f"train/eval leakage: {len(overlap)} shared IDs")
    train_families = {str(row.get("family") or "") for row in train_rows}
    family_overlap = train_families.intersection(str(row.get("family") or "") for row in eval_rows)
    family_overlap.discard("")
    if family_overlap:
        raise SystemExit(f"train/eval leakage: {len(family_overlap)} shared scenario families")
    if args.smoke:
        train_rows = train_rows[:20]
        eval_rows = eval_rows[:8]
        args.max_steps = 2
        args.eval_steps = 1
        args.save_steps = 1
    if args.production:
        if not args.push_to_hub:
            raise SystemExit("--production requires --push-to-hub")
        if not os.environ.get("HF_TOKEN"):
            raise SystemExit("--production requires HF_TOKEN")
        if args.estimated_cost_usd <= 0:
            raise SystemExit("--production requires a positive reviewed --estimated-cost-usd")
    if args.validate_only:
        print(
            f"VALID model_family={adapter.name} train_rows={len(train_rows)} "
            f"eval_rows={len(eval_rows)} train_eval_overlap=0 family_overlap=0"
        )
        return

    # GPU stack stays lazy so --help and --validate-only work on CPU hosts.
    from datasets import Dataset
    from transformers import EarlyStoppingCallback
    from trl import SFTConfig, SFTTrainer
    from unsloth import FastLanguageModel
    from unsloth.chat_templates import train_on_responses_only

    model, tokenizer = FastLanguageModel.from_pretrained(
        model_name=args.model,
        max_seq_length=args.max_length,
        dtype=None,
        load_in_4bit=args.load_in_4bit,
    )
    if not getattr(tokenizer, "chat_template", None):
        raise SystemExit(f"{args.model} does not provide a native chat template")

    rendered_train = [render_row(tokenizer, row) for row in train_rows]
    rendered_eval = [render_row(tokenizer, row) for row in eval_rows]
    assert_response_markers(rendered_train[0], adapter)
    validate_token_lengths(tokenizer, rendered_train, args.max_length)
    validate_token_lengths(tokenizer, rendered_eval, args.max_length)

    model = FastLanguageModel.get_peft_model(
        model,
        r=args.lora_r,
        target_modules=list(adapter.target_modules),
        lora_alpha=args.lora_alpha,
        lora_dropout=0,
        bias="none",
        use_gradient_checkpointing="unsloth",
        random_state=args.seed,
    )
    train_dataset = Dataset.from_list(
        [{"id": row["id"], "text": text} for row, text in zip(train_rows, rendered_train, strict=True)]
    )
    eval_dataset = Dataset.from_list(
        [{"id": row["id"], "text": text} for row, text in zip(eval_rows, rendered_eval, strict=True)]
    )

    args.output_dir.mkdir(parents=True, exist_ok=True)
    model_revision = str(getattr(model.config, "_commit_hash", "") or "")
    manifest_path = args.output_dir / "aurago_run_manifest.json"
    write_run_manifest(
        manifest_path,
        args,
        adapter,
        len(train_rows),
        len(eval_rows),
        model_revision=model_revision,
        status="started",
    )

    config_kwargs = sft_config_kwargs(args)
    config_signature = inspect.signature(SFTConfig)
    if "max_length" in config_signature.parameters:
        config_kwargs["max_length"] = args.max_length
    elif "max_seq_length" in config_signature.parameters:
        config_kwargs["max_seq_length"] = args.max_length
    else:
        raise SystemExit("installed TRL SFTConfig exposes neither max_length nor max_seq_length")
    sft_config = SFTConfig(**config_kwargs)

    trainer_kwargs: dict[str, Any] = {
        "model": model,
        "train_dataset": train_dataset,
        "eval_dataset": eval_dataset,
        "args": sft_config,
        "callbacks": [EarlyStoppingCallback(early_stopping_patience=args.early_stopping_patience)],
    }
    trainer_signature = inspect.signature(SFTTrainer)
    if "processing_class" in trainer_signature.parameters:
        trainer_kwargs["processing_class"] = tokenizer
    elif "tokenizer" in trainer_signature.parameters:
        trainer_kwargs["tokenizer"] = tokenizer
    else:
        raise SystemExit("installed TRL SFTTrainer accepts neither processing_class nor tokenizer")
    trainer = SFTTrainer(**trainer_kwargs)
    trainer = train_on_responses_only(
        trainer,
        instruction_part=adapter.instruction_marker,
        response_part=adapter.response_marker,
    )

    stats = trainer.train()
    model.save_pretrained(str(args.output_dir))
    tokenizer.save_pretrained(str(args.output_dir))
    trainer.save_metrics("train", stats.metrics)
    trainer.save_state()

    if args.push_to_hub:
        from huggingface_hub import HfApi, create_repo

        create_repo(args.push_to_hub, private=True, exist_ok=True, token=os.environ.get("HF_TOKEN"))
        repository = HfApi(token=os.environ.get("HF_TOKEN")).repo_info(
            args.push_to_hub,
            repo_type="model",
        )
        if not repository.private:
            raise SystemExit(f"refusing to push training artifacts to public repository {args.push_to_hub}")
        model.push_to_hub(args.push_to_hub, token=os.environ.get("HF_TOKEN"))
        tokenizer.push_to_hub(args.push_to_hub, token=os.environ.get("HF_TOKEN"))
    if args.save_merged:
        merged = args.output_dir.parent / f"{args.output_dir.name}-merged"
        model.save_pretrained_merged(str(merged), tokenizer, save_method="merged_16bit")
    if args.save_gguf:
        gguf = args.output_dir.parent / f"{args.output_dir.name}-gguf"
        model.save_pretrained_gguf(str(gguf), tokenizer, quantization_method="q4_k_m")

    write_run_manifest(
        manifest_path,
        args,
        adapter,
        len(train_rows),
        len(eval_rows),
        model_revision=model_revision,
        status="completed",
    )
    print(f"Saved adapter and run manifest to {args.output_dir}")


if __name__ == "__main__":
    main()

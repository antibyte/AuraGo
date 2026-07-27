from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from training.evaluate_tool_calls import evaluate, parse_tagged_output
from training.train_unsloth import (
    ADAPTERS,
    assert_response_markers,
    messages_for_template,
    resolve_adapter,
    validate_token_lengths,
)


ROOT = Path(__file__).resolve().parent


class FakeTokenizer:
    def __call__(self, rows: list[str], *, add_special_tokens: bool) -> dict[str, list[list[int]]]:
        del add_special_tokens
        return {"input_ids": [list(range(len(row.split()))) for row in rows]}


class TrainingPipelineTests(unittest.TestCase):
    def test_all_candidate_adapters_resolve_and_unknown_fails(self) -> None:
        expected = {
            "google/functiongemma-270m-it": "functiongemma",
            "LiquidAI/LFM2.5-1.2B-Instruct": "lfm2.5",
            "Qwen/Qwen3-1.7B": "qwen3",
            "HuggingFaceTB/SmolLM3-3B": "smollm3",
        }
        self.assertEqual({resolve_adapter(model).name for model in expected}, {a.name for a in ADAPTERS})
        for model, family in expected.items():
            self.assertEqual(resolve_adapter(model).name, family)
        with self.assertRaises(SystemExit):
            resolve_adapter("unsupported/model")

    def test_native_arguments_are_objects_for_tokenizer_templates(self) -> None:
        messages = [
            {
                "role": "assistant",
                "tool_calls": [
                    {
                        "id": "call_1",
                        "type": "function",
                        "function": {"name": "demo", "arguments": '{"operation":"list"}'},
                    }
                ],
            }
        ]
        converted = messages_for_template(messages)
        self.assertEqual(
            converted[0]["tool_calls"][0]["function"]["arguments"],
            {"operation": "list"},
        )
        self.assertIsInstance(messages[0]["tool_calls"][0]["function"]["arguments"], str)

    def test_response_markers_and_token_limits_fail_closed(self) -> None:
        adapter = resolve_adapter("Qwen/Qwen3-1.7B")
        assert_response_markers(
            f"{adapter.instruction_marker}task{adapter.response_marker}answer",
            adapter,
        )
        with self.assertRaises(SystemExit):
            assert_response_markers("missing markers", adapter)
        self.assertEqual(validate_token_lengths(FakeTokenizer(), ["one two", "three"], 2), [2, 1])
        with self.assertRaises(SystemExit):
            validate_token_lengths(FakeTokenizer(), ["one two three"], 2)

    def test_tagged_roundtrip_parser_handles_nested_arguments(self) -> None:
        output = (
            '<tool_call>{"id":"call_1","name":"demo","arguments":'
            '{"nested":{"enabled":true}}}</tool_call>'
        )
        calls, parseable, errors = parse_tagged_output(output)
        self.assertTrue(parseable, errors)
        self.assertEqual(calls[0].arguments, {"nested": {"enabled": True}})
        _, parseable, errors = parse_tagged_output("<tool_call>{broken}</tool_call>")
        self.assertFalse(parseable)
        self.assertTrue(errors)

    def test_perfect_challenge_predictions_pass_every_gate(self) -> None:
        predictions: list[dict[str, object]] = []
        with (ROOT / "dataset_challenge_native_fc.jsonl").open(encoding="utf-8") as handle:
            for line in handle:
                row = json.loads(line)
                calls = [
                    {
                        "type": "function",
                        "function": {
                            "name": call["name"],
                            "arguments": call["arguments"],
                        },
                    }
                    for call in row["expectations"].get("calls", [])
                ]
                predictions.append({"id": row["id"], "tool_calls": calls})

        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "predictions.jsonl"
            path.write_text(
                "".join(json.dumps(row, separators=(",", ":")) + "\n" for row in predictions),
                encoding="utf-8",
            )
            result = evaluate(
                ROOT / "dataset_challenge_native_fc.jsonl",
                path,
                ROOT,
                apply_gates=True,
            )
        self.assertTrue(result.gates_passed, result.gate_failures)
        self.assertEqual(result.quality_score, 1.0)
        self.assertEqual(result.language_gap, 0.0)


if __name__ == "__main__":
    unittest.main()

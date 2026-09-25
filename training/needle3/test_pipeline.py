"""Offline contract tests; optional real tokenizer/checkpoint tests use the lock."""
from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from .budget import Budget
from .common import canonical, catalog, config, digest, inference_query, write_json, verify_files
from .data import expand_group, validate_row, corpus_report
from .evaluate import metrics, claim_final_test, paired_f2_advantage
from .retrieval import choose_k, reciprocal_rank
from .schedule import allocation, build_schedule, summary
from .serialization import validate_output, schemas


class PipelineTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cat, cls.cfg = catalog(), config()

    def test_exact_language_quotas_and_group_splits(self):
        groups = build_schedule(self.cat, self.cfg)
        report = summary(groups)
        self.assertEqual(report["groups"], 17500)
        self.assertEqual(report["requested_rows"], 70000)
        self.assertEqual(report["split_rows"]["train"]["de"], 28000)
        self.assertEqual(report["split_rows"]["validation"]["hi"], 125)
        self.assertEqual(report["challenge_rows"], 2800)
        self.assertEqual(len({g["group_id"] for g in groups}), 17500)
        self.assertEqual(summary(build_schedule(self.cat, self.cfg)), report)

    def test_planned_manual_coverage(self):
        from collections import Counter
        counts = Counter((mid, lang) for g in build_schedule() if g["split"] == "train" for mid in g["manual_ids"] for lang in ["de", "de", "en", g["language"]])
        for m in self.cat["manuals"]:
            for lang in self.cfg["languages"]:
                self.assertGreaterEqual(counts[m["id"], lang], self.cfg["minimum_manual_train"].get(lang, 4), (m["id"], lang))

    def test_context_is_part_of_search_query(self):
        row = {"query": "Nein, stattdessen per Discord.", "context": ["Schicke den Bericht per E-Mail."]}
        self.assertIn("Schicke den Bericht", inference_query(row))
        self.assertTrue(inference_query(row).endswith(row["query"]))
        with self.assertRaises(ValueError):
            inference_query({"query": "x", "context": ["x"] * 3})

    def test_planned_operations_include_large_shared_families(self):
        jobs = build_schedule()
        seen = {(r["tool"], r.get("selector", ""), str(r.get("value", ""))) for g in jobs if g["split"] == "train" for r in g["operation_refs"]}
        expected = {(t["name"], o.get("selector", ""), str(o.get("value", ""))) for t in self.cat["tools"] if t.get("manual_id") for o in t.get("operations") or [{}]}
        self.assertFalse(expected - seen, sorted(expected - seen))
        self.assertGreaterEqual(sum(g["variation"] != "natural" for g in jobs) * 2 / (4 * len(jobs)), .15)

    def test_empty_available_list_is_respected(self):
        self.assertEqual(reciprocal_rank([["email"], ["docker"]], set()), [])
        self.assertEqual(reciprocal_rank([["email", "email"], ["email", "docker"]], {"email"}), ["email"])

    def test_output_guard_rejects_unknown_arguments_duplicates_and_overflow(self):
        good = {"name": "manual_email", "arguments": {}}
        self.assertEqual(validate_output({"function_calls": [good]}, ["email"], {"email"}), (["email"], True))
        for calls in [[good, good], [good] * 4, [{"name": "manual_unknown", "arguments": {}}], [{"name": "manual_email", "arguments": {"to": "a"}}]]:
            self.assertEqual(validate_output({"function_calls": calls}, ["email"], {"email"}), ([], False))
        self.assertEqual(validate_output({"function_calls": [good]}, ["email"], set()), ([], False))
        self.assertEqual(validate_output({"function_calls": []}, [], set()), ([], True))
        self.assertEqual(validate_output({}, ["email"], {"email"}), ([], False))

    def test_candidate_order_is_preserved_in_runtime_schemas(self):
        self.assertEqual([s["name"] for s in schemas(self.cat, ["mqtt", "email"])], ["manual_mqtt", "manual_email"])
        with self.assertRaises(ValueError):
            schemas(self.cat, ["email", "email"])

    def test_missing_candidate_counts_as_retrieval_failure(self):
        row = {"id": "a", "gold": ["email"], "candidates": ["docker"]}
        result = metrics([row], {"a": {"manual_ids": [], "formal_valid": True}})
        self.assertEqual(result["retrieval_missed_manuals"], 1)
        self.assertEqual(result["recall_at_3"], 0)
        self.assertIsNone(result["conditional_selection_recall"])

    def test_invalid_empty_is_not_a_successful_abstention(self):
        row = {"id": "a", "gold": [], "candidates": []}
        self.assertEqual(metrics([row], {"a": {"manual_ids": [], "formal_valid": False}})["empty_accuracy"], 0)

    def test_shortlist_selection_is_validation_only_and_per_language(self):
        row = {"id": "a", "all_manual_ids": ["email"], "split": "test", "language": "de"}
        with self.assertRaises(ValueError):
            choose_k([row], {"a": ["email"]}, self.cfg)
        row["split"] = "validation"
        with self.assertRaises(ValueError):
            choose_k([row], {"a": ["email"]}, self.cfg)

    def test_budget_reserves_concurrently_and_retains_uncertain_calls(self):
        with tempfile.TemporaryDirectory() as tmp:
            ledger = Budget(Path(tmp) / "costs.sqlite", {"total": 1, "generate": 1})
            def reserve(_):
                try:
                    return ledger.reserve("generate", "test", .4)
                except RuntimeError:
                    return None
            with ThreadPoolExecutor(max_workers=4) as pool:
                ids = [x for x in pool.map(reserve, range(4)) if x]
            self.assertEqual(len(ids), 2)
            ledger.settle(ids[0], .1)
            self.assertEqual(ledger.summary()["generate"]["uncertain_requests"], 1)
            self.assertEqual(ledger.summary()["generate"]["accounted_usd"], .5)

    def test_price_overrun_latches_cost_gate(self):
        with tempfile.TemporaryDirectory() as tmp:
            ledger = Budget(Path(tmp) / "costs.sqlite", {"total": 1, "generate": 1})
            rid = ledger.reserve("generate", "test", .1)
            with self.assertRaises(RuntimeError):
                ledger.settle(rid, .2)
            with self.assertRaises(RuntimeError):
                ledger.reserve("generate", "test", .1)

    def test_model_key_budget_is_fifteen(self):
        self.assertEqual(self.cfg["budget_usd"]["total"], 15)
        self.assertEqual(sum(v for k, v in self.cfg["budget_usd"].items() if k != "total"), 15)
        self.assertFalse(self.cfg["runpod"]["launch_enabled"])

    def test_final_test_cannot_be_reselected(self):
        with tempfile.TemporaryDirectory() as tmp:
            claim_final_test(tmp, "data", "selection")
            with self.assertRaises(FileExistsError):
                claim_final_test(tmp, "data", "different selection")

    def test_artifact_manifest_rejects_path_escape(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(ValueError):
                verify_files(tmp, {"../outside": "x"})

    def test_empty_corpus_is_not_ready(self):
        report = corpus_report([], self.cat, self.cfg)
        self.assertFalse(report["structural_ready"])
        self.assertEqual(report["rows"], 0)

    def test_pilot_gate_binds_code_schedule_and_catalog(self):
        from .generate import require_current_pilot
        identity = {"catalog_sha256": "a", "schedule_sha256": "b", "code": {"data.py": "c"}}
        require_current_pilot({"full_generation_allowed": True, "experiment": identity}, identity)
        for pilot in ({"full_generation_allowed": True}, {"full_generation_allowed": False, "experiment": identity},
                      {"full_generation_allowed": True, "experiment": {**identity, "schedule_sha256": "changed"}}):
            with self.assertRaises(ValueError):
                require_current_pilot(pilot, identity)

    def test_source_audit_queue_covers_every_family_and_language(self):
        from .quality import audit_queue
        rows = [{"id": f"{mid}-{lang}", "language": lang, "all_manual_ids": [mid], "sources": {}, "query": "example", "context": [], "answers": [mid]}
                for mid in (m["id"] for m in self.cat["manuals"]) for lang in self.cfg["languages"]]
        selected = audit_queue(rows)
        indexed = {r["id"]: r for r in rows}
        self.assertEqual(len(selected), 640)
        self.assertEqual({indexed[r["row_id"]]["language"] for r in selected}, set(self.cfg["languages"]))
        self.assertEqual({mid for r in selected for mid in indexed[r["row_id"]]["all_manual_ids"]}, {m["id"] for m in self.cat["manuals"]})
        self.assertFalse(any(r["approved"] for r in selected))

    def test_lease_rejects_wrong_pod_volume_price_and_creation_time(self):
        from .runpod import validate_lease
        lease = {"pod_id": "pod1", "created_pod_id": "pod1", "run_id": "run1", "network_volume_id": "volume1",
                 "created_at_unix": 0, "maximum_seconds": 43200, "gpu_hourly_usd": .74}
        pod = {"id": "pod1", "name": "aurago-needle3-run1", "networkVolumeId": "volume1", "createdAt": "1970-01-01T00:00:00Z"}
        self.assertEqual(validate_lease(lease, pod, 100), 43200)
        for changed in ({**lease, "pod_id": "other"}, {**lease, "network_volume_id": "other"},
                        {**lease, "gpu_hourly_usd": 1.01}, {**lease, "gpu_hourly_usd": float("nan")},
                        {**lease, "maximum_seconds": 50000}, {**lease, "created_at_unix": 20}):
            with self.assertRaises(ValueError):
                validate_lease(changed, pod, 100)

    def test_expired_phase_does_not_spawn_process(self):
        from .controller import run_phase
        with patch("training.needle3.controller.subprocess.Popen") as popen:
            with self.assertRaises(TimeoutError):
                run_phase(["never-executed"], 0, "never-opened.log")
            popen.assert_not_called()

    def test_full_manual_retrieval_differs_from_prioritized_top_three(self):
        row = {"id": "a", "gold": ["a", "b", "c"], "all_manual_ids": ["a", "b", "c", "d"], "candidates": ["a", "b", "c"]}
        result = metrics([row], {"a": {"manual_ids": ["a", "b", "c"], "formal_valid": True}})
        self.assertEqual(result["recall_at_3"], 1)
        self.assertEqual(result["all_required_candidate_recall"], .75)

    def test_paired_comparison_keeps_translations_together(self):
        rows = [{"id": str(i), "group_id": str(i // 4), "gold": ["email"]} for i in range(16)]
        good = {r["id"]: {"manual_ids": ["email"]} for r in rows}
        empty = {r["id"]: {"manual_ids": []} for r in rows}
        self.assertFalse(paired_f2_advantage(rows, good, good, 100)["positive_advantage"])
        result = paired_f2_advantage(rows, good, empty, 100)
        self.assertEqual(result["groups"], 4)
        self.assertEqual(result["f2_delta_ci95"], [1, 1])

    def test_unaccepted_or_changed_corpus_cannot_publish(self):
        from .quality import publish_shards
        with tempfile.TemporaryDirectory() as tmp:
            for report in ({"ready_for_compilation": False}, {"ready_for_compilation": True, "corpus_sha256": "stale"}):
                with self.assertRaises(ValueError):
                    publish_shards([], report, tmp)
            self.assertFalse(list(Path(tmp).iterdir()))


@unittest.skipUnless(importlib.util.find_spec("needle") and importlib.util.find_spec("jax"), "locked ML environment not installed")
class MLContractTests(unittest.TestCase):
    def test_compiler_never_repairs_missing_heldout_candidates(self):
        from .compile import compile_rows
        from .assets import tokenizer, MODEL_DIR
        if not (MODEL_DIR / "needle/tokenizer/tokenizer.model").exists():
            self.skipTest("pinned tokenizer is not downloaded")
        rows = [{"id": split, "group_id": split, "split": split, "language": "de", "kind": "single", "challenge": False,
                 "answers": ["email"], "all_manual_ids": ["email"], "query": "Lies meine E-Mails.", "context": [],
                 "reasoning": "Email access needs its manual."} for split in ("validation", "test")]
        compiled = compile_rows(rows, catalog(), {r["id"]: ["docker"] for r in rows}, 12, tokenizer())
        for split in ("validation", "test"):
            row = compiled[split][0]
            self.assertEqual(row["candidates"], ["docker"])
            self.assertEqual(row["gold"], ["email"])
            self.assertEqual(row["example"]["answers"], [])

    def test_pilot_language_arms_have_identical_shuffled_scenario_batches(self):
        import numpy as np
        from .compile import pilot_pairs
        from .train import epoch_batches
        rows = [{"id": f"{g}-{i}", "group_id": str(g), "language": lang, "bucket": [512, 768, 256, 1024][i] + (g % 2) * 256}
                for g in range(12) for i, lang in enumerate(("de", "de", "en", "ja"))]
        left, right = pilot_pairs(rows)
        a = epoch_batches(left, 16, np.random.default_rng(42))
        b = epoch_batches(right, 16, np.random.default_rng(42))
        self.assertEqual(a, b)
        self.assertEqual([[left[i]["group_id"] for i in batch] for batch in a], [[right[i]["group_id"] for i in batch] for batch in b])
        self.assertTrue(all(r["bucket"] != 1024 for r in rows if r["language"] == "en"))

    def test_resume_restores_optimizer_rng_and_data_position(self):
        import jax.numpy as jnp
        import numpy as np
        import optax
        from .checkpoint import save, restore
        adapter = {("a", "kernel"): {"A": jnp.ones((2, 2)), "B": jnp.zeros((2, 2))}}
        optimizer = optax.adamw(.001)
        state = optimizer.init(adapter)
        gradients = {("a", "kernel"): {"A": jnp.full((2, 2), .3), "B": jnp.full((2, 2), .1)}}
        for _ in range(3):
            updates, state = optimizer.update(gradients, state, adapter)
            adapter = optax.apply_updates(adapter, updates)
        rng = np.random.default_rng(12)
        position = {"step": 3, "epoch": 1, "position": 4, "rng": rng.bit_generator.state, "batches": [[2, 1], [0, 3]]}
        with tempfile.TemporaryDirectory() as tmp:
            save(tmp, (adapter, state), position, {"data": "a"})
            restored, recovered = restore(tmp, (adapter, state), {"data": "a"})
            self.assertEqual(recovered, position)
            np.testing.assert_array_equal(restored[0][("a", "kernel")]["A"], adapter[("a", "kernel")]["A"])
            # The first step after resume must equal an uninterrupted fourth step.
            import jax
            expected_updates, expected_state = optimizer.update(gradients, state, adapter)
            actual_updates, actual_state = optimizer.update(gradients, restored[1], restored[0])
            for a, b in zip(jax.tree.leaves((expected_updates, expected_state)), jax.tree.leaves((actual_updates, actual_state))):
                np.testing.assert_array_equal(a, b)
            with self.assertRaises(ValueError):
                restore(tmp, (adapter, state), {"data": "changed"})

    def test_exact_tokenization_and_training_runtime_schema_string(self):
        from .assets import MODEL_DIR, tokenizer
        from .serialization import SYSTEM, tools_json, encode_exact
        from needle.model.finetune import render_example
        if not (MODEL_DIR / "needle" / "tokenizer" / "tokenizer.model").exists():
            self.skipTest("pinned tokenizer is not downloaded")
        cat = catalog()
        ex = {"system": SYSTEM, "tools": schemas(cat, ["email", "mqtt"]), "query": "Lies meine E-Mails.", "answers": [{"name": "manual_email", "arguments": {}}], "reasoning": "Email access needs its manual."}
        prompt, target = render_example(ex)
        self.assertIn(tools_json(cat, ["email", "mqtt"]), prompt)
        tok = tokenizer()
        ids, mask = encode_exact(ex, tok)
        self.assertEqual(len(ids), len(tok.encode(prompt)) + len(tok.encode(target)) + 2)
        self.assertEqual(mask[-1], 1)
        with self.assertRaises(ValueError):
            encode_exact(ex, tok, maximum=len(ids) - 1)


if __name__ == "__main__":
    unittest.main()

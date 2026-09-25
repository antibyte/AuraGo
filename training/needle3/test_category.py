"""Category coverage, independent partitions, guarded output and CPU bounds."""
from collections import Counter
from copy import deepcopy
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from .assets import MODEL_DIR, tokenizer
from .category_data import cases, validate
from .category_eval import metrics, select, finalize
from .category_router import CATEGORIES, MAX_NEW_TOKENS, MAX_TOKENS, CategoryRouter, encode, guard, schema, taxonomy, training_example
from .category_train import sampling_weights, train
from .category_reference import predictions
from .common import catalog, write_json


class CategoryTests(unittest.TestCase):
    def test_all_shared_manuals_have_one_discovery_category(self):
        cat = catalog()
        mapping = taxonomy(cat)
        self.assertEqual(set(mapping), {m["id"] for m in cat["manuals"]})
        self.assertEqual(mapping["mqtt"], "smart_home")
        self.assertEqual(mapping["email"], "communication")
        self.assertEqual(mapping["discord"], "communication")
        changed = deepcopy(cat)
        changed["tools"].append({"manual_id": "mqtt", "category": "network"})
        with self.assertRaisesRegex(ValueError, "conflicting"):
            taxonomy(changed)

    def test_translations_and_duplicate_queries_cannot_cross_partitions(self):
        cat = catalog()
        rows = cases(cat)
        validate(rows, cat)
        changed = deepcopy(rows)
        changed[1]["split"] = "holdout"
        with self.assertRaisesRegex(ValueError, "leaked"):
            validate(changed, cat)
        changed = deepcopy(rows)
        changed[-1]["query"] = next(r["query"] for r in rows if r["split"] == "holdout" and not r["context"])
        with self.assertRaisesRegex(ValueError, "leaked"):
            validate(changed, cat)

    def test_labels_must_follow_source_hashes_and_category_mapping(self):
        cat = catalog()
        rows = cases(cat)
        rows[0]["categories"] = ["files"]
        with self.assertRaisesRegex(ValueError, "source manuals"):
            validate(rows, cat)

    def test_no_three_category_cap_and_no_unknown_or_duplicate_output(self):
        def response(values):
            return {"function_calls": [{"name": c, "arguments": {}} for c in values]}
        self.assertEqual(guard(response(list(CATEGORIES))), (list(CATEGORIES), True))
        self.assertEqual(guard({"function_calls": []}), ([], True))
        for invalid in (response(["missing"]), response(["files", "files"]), response("files"),
                        {"function_calls": [], "success": False, "error": "truncated"},
                        {"function_calls": [{"name": "delete", "arguments": {}}]}, None):
            self.assertEqual(guard(invalid), ([], False))

    def test_balancing_limits_mechanical_data_and_preserves_negatives(self):
        rows = [r for r in cases(catalog()) if r["split"] == "train"]
        weights = sampling_weights(rows)
        masses = Counter()
        for row, weight in zip(rows, weights):
            masses[row["kind"]] += weight
        self.assertAlmostEqual(sum(weights), 1.)
        self.assertAlmostEqual(masses["catalog"], .15)
        self.assertAlmostEqual(masses["none"], .20)
        self.assertAlmostEqual(masses["multi"], .20)

    def test_all_required_metric_exposes_partial_multicategory_misses(self):
        rows = [{"id": "a", "categories": ["files", "media"]}, {"id": "b", "categories": []}]
        predictions = {"a": {"category_ids": ["files"], "formal_valid": True},
                       "b": {"category_ids": [], "formal_valid": True}}
        result = metrics(rows, predictions)
        self.assertEqual(result["recall"], .5)
        self.assertEqual(result["all_required"], 0.)
        self.assertEqual(result["empty_accuracy"], 1.)
        predictions["b"]["formal_valid"] = False
        result = metrics(rows, predictions)
        self.assertEqual(result["empty_accuracy"], 0.)
        self.assertEqual(result["exact"], 0.)

    def test_training_limits_fail_before_loading_or_allocating_models(self):
        for kwargs in ({"steps": 1001}, {"max_seconds": 1801}, {"batch": 1}, {"lr": .0003}):
            with self.assertRaises(ValueError):
                train("unused", **kwargs)

    def test_no_manual_reference_gate_and_empty_availability(self):
        row = {"id": "x"}
        probabilities = [[1.] * len(CATEGORIES)]
        self.assertEqual(predictions([row], probabilities, .5, {"email": "communication"}, [.1], .5)["x"]["category_ids"], [])
        self.assertEqual(predictions([row], probabilities, .5, {"email": "communication"}, [.9], .5)["x"]["category_ids"], ["communication"])
        row["available_manuals"] = []
        self.assertEqual(predictions([row], probabilities, .5, {"email": "communication"}, [.9], .5)["x"]["category_ids"], [])

    def test_baseline_cannot_be_disguised_as_a_trained_validation_winner(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            model = str(root / "baseline.cact")
            base = {"weights": model, "weights_sha256": "same"}
            write_json(root / "validation-baseline.json", base)
            write_json(root / "validation-step-0060.json", base)
            write_json(root / "reference-selection.json", {})
            write_json(model + ".json", {"adapter_sha256": None})
            with patch("training.needle3.category_eval.output_path", return_value=root), patch("training.needle3.category_eval.load"):
                with self.assertRaisesRegex(ValueError, "non-baseline"):
                    select(root)

    def test_existing_holdout_claim_prevents_any_repeat_inference(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "holdout.lock.json").write_text("{}", encoding="utf-8")
            selection = {"baseline": "base", "trained": "trained", "reference_selection_sha256": "hash", "evaluation_source_sha256": "hash"}
            with patch("training.needle3.category_eval.output_path", return_value=root), \
                    patch("training.needle3.category_eval.load", return_value=({"files": {"cases.jsonl": "hash"}}, [])), \
                    patch("training.needle3.category_eval.read_json", return_value=selection), \
                    patch("training.needle3.category_eval.file_digest", return_value="hash"), \
                    patch("training.needle3.category_eval.child_measure") as measure:
                with self.assertRaises(FileExistsError):
                    finalize(root)
                measure.assert_not_called()

    @unittest.skipUnless(importlib.util.find_spec("needle") and (MODEL_DIR / "needle/tokenizer/tokenizer.model").exists(), "locked local tokenizer required")
    def test_token_budget_context_and_serialization_match_inference(self):
        from needle.model.finetune import render_example
        tok = tokenizer()
        rows = cases(catalog())
        for row in rows:
            ids, mask = encode(row, tok)
            self.assertEqual(len(ids), len(mask))
            self.assertLessEqual(len(ids), MAX_TOKENS)
            prompt, target = render_example(training_example(row))
            self.assertTrue(target.startswith("<think>\n"))
            self.assertLessEqual(1 + len(tok.encode(prompt)) + MAX_NEW_TOKENS, MAX_TOKENS)
        with self.assertRaisesRegex(ValueError, "budget"):
            encode({"query": "large " * 4000, "categories": []}, tok)
        encode({"query": "Use all available tool areas.", "categories": list(CATEGORIES)}, tok)

    @unittest.skipUnless(importlib.util.find_spec("needle") and (MODEL_DIR / "needle/tokenizer/tokenizer.model").exists(), "locked local tokenizer required")
    def test_empty_availability_stays_empty_and_schema_is_initialized_once(self):
        calls = []
        class Fake:
            def __init__(self, **kwargs):
                calls.append(kwargs)
            def complete(self, query, **kwargs):
                return {"function_calls": [{"name": c, "arguments": {}} for c in ["communication", "files"]]}
            def close(self):
                pass
        router = CategoryRouter("unused.cact", factory=Fake)
        result = router.select({"query": "Send the file", "available_manuals": []})
        self.assertEqual(result["category_ids"], [])
        result = router.select({"query": "Send the file", "available_manuals": ["email"]})
        self.assertEqual(result["category_ids"], ["communication"])
        self.assertEqual(result["manuals_by_category"], {"communication": ["email"]})
        self.assertEqual(len(calls), 1)
        with self.assertRaises(ValueError):
            router.select({"query": "x", "available_manuals": ["invented"]})
        router.close()


if __name__ == "__main__":
    unittest.main()

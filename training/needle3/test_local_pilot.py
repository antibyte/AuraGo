"""Guard the independence and bounds of the small CPU training experiment."""
from collections import Counter
from copy import deepcopy
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from .common import catalog, file_digest, write_json, write_jsonl
from .local_pilot import load_pack, output_path, train, validate_splits
from .local_pilot_data import pilot_rows
from .local_pilot_eval import best_trained, choose_search, search_predictions


class LocalPilotTests(unittest.TestCase):
    def test_a_failed_training_arm_cannot_be_replaced_by_the_untrained_model(self):
        def model(step, score):
            return {"step": step, "validation": {"overall": {"f2": score, "recall_at_3": score, "empty_accuracy": 0}}}
        baseline = model(0, .9)
        self.assertEqual(best_trained([baseline, model(80, .2), model(240, .3)])["step"], 240)
        with self.assertRaisesRegex(ValueError, "no completed training"):
            best_trained([baseline])

    def test_translations_and_old_diagnostics_stay_in_one_partition(self):
        rows = pilot_rows(catalog())
        validate_splits(rows)
        self.assertEqual(Counter(r["split"] for r in rows), {"train": 116, "validation": 24, "holdout": 48})
        for row in rows:
            if row["id"].startswith(("natural-", "context-")):
                self.assertEqual(row["split"], "train")
        changed = deepcopy(rows)
        changed[1]["split"] = "holdout"
        with self.assertRaisesRegex(ValueError, "leaked"):
            validate_splits(changed)
        changed = deepcopy(rows)
        changed[-1]["query"], changed[-1]["context"] = rows[0]["query"], rows[0]["context"]
        with self.assertRaisesRegex(ValueError, "leaked"):
            validate_splits(changed)

    def test_holdout_is_not_allowed_to_choose_reference_threshold(self):
        with self.assertRaisesRegex(ValueError, "validation only"):
            choose_search([r for r in pilot_rows(catalog()) if r["split"] == "holdout"])
        row = {"id": "x", "candidates": ["docker", "email"], "embedding_scores": {"docker": .9, "email": .7}}
        self.assertEqual(search_predictions([row], 3, .8)["x"]["manual_ids"], ["docker"])
        self.assertEqual(search_predictions([row], 1, .95)["x"]["manual_ids"], [])

    def test_cpu_limits_are_checked_before_loading_ml_or_starting_work(self):
        for kwargs in ({"steps": 0}, {"steps": 1001}, {"max_seconds": 1801}, {"max_seconds": 0}, {"lr": .01}):
            with self.assertRaises(ValueError):
                train("unused", **kwargs)
        with self.assertRaises(ValueError):
            output_path("training/needle3/data/not-an-accepted-pilot")

    def test_changed_heldout_file_is_rejected_before_evaluation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = root / "cases.jsonl"
            write_jsonl(path, [{"id": "original"}])
            write_json(root / "manifest.json", {"files": {"cases.jsonl": file_digest(path)}})
            write_jsonl(path, [{"id": "changed"}])
            with patch("training.needle3.local_pilot.output_path", return_value=root):
                with self.assertRaisesRegex(ValueError, "changed"):
                    load_pack(root)


if __name__ == "__main__":
    unittest.main()

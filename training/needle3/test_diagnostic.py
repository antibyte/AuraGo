"""Prevent optimistic or contaminated development measurements."""
import unittest
from pathlib import Path
import tempfile
from unittest.mock import patch

from .common import catalog, file_digest, inference_query, write_json, write_jsonl
from .diagnostic import make_tasks, read_cached_cases, validate_cases
from .diagnostic_fixtures import catalog_probes, fixture_rows
from .latency import measure, paired_cases, timing


class DiagnosticTests(unittest.TestCase):
    def test_latency_uses_distinct_scenarios_and_excludes_empty_availability(self):
        selected = paired_cases(fixture_rows())
        self.assertEqual(len(selected), 58)
        self.assertEqual(len({r["group_id"] for r in selected}), 58)
        self.assertEqual(sum(r["language"] == "de" for r in selected), 29)
        self.assertFalse(any(r["kind"] == "unavailable" for r in selected))

    def test_latency_ceiling_is_strict_and_checks_tail_not_just_median(self):
        self.assertFalse(timing([100, 200, 600])["all_observed_under_600_ms"])
        self.assertEqual(timing([100, 200, 600])["under_600_ms"], 2)
        self.assertTrue(timing([599.9])["all_observed_under_600_ms"])
        for values in ([], [float("nan")], [-1]):
            with self.assertRaises(ValueError):
                timing(values)

    def test_latency_includes_retrieval_and_rejects_search_drift(self):
        from unittest.mock import Mock
        row = {"query": "Read email", "candidates": ["email"]}
        retriever, selector = Mock(), Mock()
        retriever.retrieve.return_value = ["email"]
        selector.select.return_value = {"latency_ms": 400, "schema_setup_ms": 100}
        with patch("training.needle3.latency.time.perf_counter", side_effect=[0, .2, .61]):
            result = measure(row, retriever, selector)
        self.assertEqual(result["end_to_end_ms"], 610)
        self.assertEqual(result["retrieval_ms"], 200)
        self.assertEqual(result["completion_ms"], 300)
        retriever.retrieve.return_value = ["docker"]
        with self.assertRaisesRegex(ValueError, "search differs"):
            measure(row, retriever, selector)

    def test_resume_rejects_modified_cases_and_configuration(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            identity = {"model": "pinned-model"}
            write_jsonl(output / "cases.jsonl", [{"id": "original"}])
            write_json(output / "manifest.json", {"identity": identity,
                       "cases_sha256": file_digest(output / "cases.jsonl")})
            self.assertEqual(read_cached_cases(output, identity), [{"id": "original"}])
            with self.assertRaisesRegex(ValueError, "inputs changed"):
                read_cached_cases(output, {"model": "other-model"})
            write_jsonl(output / "cases.jsonl", [{"id": "changed"}])
            with self.assertRaisesRegex(ValueError, "cases changed"):
                read_cached_cases(output, identity)

    def test_catalog_probes_cover_every_family_in_both_main_languages(self):
        cat = catalog()
        rows = catalog_probes(cat)
        validate_cases(rows, cat)
        expected = {(m["id"], lang) for m in cat["manuals"] for lang in ("de", "en")}
        self.assertEqual({(r["gold"][0], r["language"]) for r in rows}, expected)
        self.assertEqual(len(rows), len(expected))
        self.assertTrue(all("<nil>" not in r["query"] for r in rows))
        self.assertTrue(all(" Then: " not in r["query"] and " Danach: " not in r["query"] for r in rows))

    def test_translation_siblings_remain_in_the_same_scenario(self):
        rows = fixture_rows()
        validate_cases(rows, catalog())
        reference = {r["group_id"]: r for r in rows if r["language"] == "en"}
        for row in rows:
            self.assertEqual(row["gold"], reference[row["group_id"]]["gold"])
        self.assertEqual(len({r["language"] for r in rows}), 16)

    def test_missing_gold_is_injected_only_into_named_oracle_arm(self):
        row = {"id": "probe", "group_id": "probe", "query": "Email it.", "context": [],
               "gold": ["email"], "candidates": ["docker", "mqtt"],
               "ranking": ["docker", "mqtt", "email"], "language": "de", "cohort": "natural_challenge"}
        tasks = {r["variant"]: r for r in make_tasks([row])}
        self.assertNotIn("email", tasks["retrieved12"]["candidates"])
        self.assertNotIn("email", tasks["reverse12"]["candidates"])
        self.assertIn("email", tasks["oracle12"]["candidates"])
        for task in tasks.values():
            self.assertEqual(inference_query(task), "Email it.")

    def test_sealed_split_and_unavailable_gold_are_rejected(self):
        row = fixture_rows()[0]
        with self.assertRaisesRegex(ValueError, "sealed"):
            validate_cases([{**row, "split": "test"}], catalog())
        with self.assertRaisesRegex(ValueError, "available"):
            validate_cases([{**row, "available_manuals": []}], catalog())


if __name__ == "__main__":
    unittest.main()

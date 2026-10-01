"""Synthetic SQLite checks for the operator memory repair script (stdlib only)."""

from contextlib import closing, redirect_stdout
import hashlib
import io
import json
from pathlib import Path
import re
import sqlite3
import tempfile
import unittest

import repair_memory_metadata as repair


REVIEWED = "2026-09-01 12:00:00"
OVERWRITTEN = "2026-09-02 12:00:00"
LATER = "2026-09-03 12:00:00"


class MemoryMetadataRepairTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.path = self.root / "short_term.db"
        previous_root = repair.REPORTS_ROOT
        repair.REPORTS_ROOT = self.root / "reports"
        self.addCleanup(setattr, repair, "REPORTS_ROOT", previous_root)
        self.db = sqlite3.connect(self.path)
        self.db.row_factory = sqlite3.Row
        self.db.isolation_level = None
        self.addCleanup(self.db.close)
        # Read the production table definitions, rather than maintaining a parallel schema.
        source = (Path(__file__).resolve().parents[1] / "internal/memory/short_term_init.go").read_text(encoding="utf-8")
        for table in ("memory_meta", "memory_curation_events", "memory_conflicts"):
            schema = re.search(r"CREATE TABLE IF NOT EXISTS " + table + r" \(.*?\);", source, re.S)
            self.assertIsNotNone(schema, table)
            self.db.executescript(schema.group())

    def seed(self, doc_id, action="confirm"):
        status = repair.STATUS_ACTIONS[action]
        self.db.execute("""INSERT INTO memory_meta
            (doc_id, verification_status, source_type, extraction_confidence, source_reliability,
             last_reviewed_at, last_event_at, archived_at, archived_reason, review_note, protected)
            VALUES (?, ?, 'user', 0.99, 0.98, ?, ?, ?, ?, 'human review', 1)""",
            (doc_id, status, REVIEWED, REVIEWED, REVIEWED if action == "archive" else None, "obsolete" if action == "archive" else ""))
        self.event(doc_id, action, REVIEWED)

    def event(self, doc_id, action, at, dry_run=False):
        self.db.execute("""INSERT INTO memory_curation_events
            (doc_id, action, actor, previous_status, new_status, timestamp, reason, dry_run)
            VALUES (?, ?, 'admin', 'unverified', ?, ?, 'synthetic review', ?)""",
            (doc_id, action, repair.STATUS_ACTIONS.get(action, "confirmed"), at, dry_run))

    def damage(self, doc_id):
        self.db.execute("""UPDATE memory_meta SET verification_status='unverified', source_type='memory_analysis',
            extraction_confidence=0.85, source_reliability=0.70, last_event_at=? WHERE doc_id=?""", (OVERWRITTEN, doc_id))

    def baseline(self, name="baseline.sqlite"):
        return repair.backup(self.db, repair.REPORTS_ROOT / name)

    def metadata(self, doc_id):
        return dict(self.db.execute("SELECT * FROM memory_meta WHERE doc_id=?", (doc_id,)).fetchone())

    def run_main(self, *args):
        with redirect_stdout(io.StringIO()):
            self.assertEqual(repair.main(["--db", str(self.path), *map(str, args)]), 0)

    def test_preview_backup_rehearsal_and_repeated_application(self):
        self.db.execute("PRAGMA journal_mode=WAL")
        self.seed("archived", "archive")
        self.seed("confirmed")
        baseline = self.baseline()
        self.damage("archived")
        self.damage("confirmed")
        before = {doc_id: self.metadata(doc_id) for doc_id in ("archived", "confirmed")}
        plan_path = repair.REPORTS_ROOT / "preview.json"
        self.run_main("--plan", plan_path, "--baseline", baseline)
        plan = json.loads(plan_path.read_text(encoding="utf-8"))
        self.assertEqual(len(plan["proposals"]), 2)
        self.assertEqual(plan["review_required"], [])
        self.assertEqual(before, {doc_id: self.metadata(doc_id) for doc_id in before})
        self.assertEqual(len(list(repair.REPORTS_ROOT.glob("*-backup.sqlite"))), 0)
        self.run_main("--apply", "--plan", plan_path)
        for doc_id, status in (("archived", "archived"), ("confirmed", "confirmed")):
            row = self.metadata(doc_id)
            self.assertEqual(row["verification_status"], status)
            self.assertEqual(row["source_type"], "user")
            self.assertEqual(row["extraction_confidence"], 0.99)
            self.assertEqual(row["source_reliability"], 0.98)
            self.assertEqual(row["review_note"], "human review")
            self.assertEqual(row["protected"], 1)
        saved_path, = repair.REPORTS_ROOT.glob("*-backup.sqlite")
        with closing(repair.connect(saved_path)) as saved:
            self.assertEqual(dict(saved.execute("SELECT * FROM memory_meta WHERE doc_id='confirmed'").fetchone()), before["confirmed"])
        rehearsal_path, = repair.REPORTS_ROOT.glob("*-rehearsal.sqlite")
        with closing(repair.connect(rehearsal_path)) as rehearsal:
            self.assertEqual(rehearsal.execute("SELECT verification_status FROM memory_meta WHERE doc_id='confirmed'").fetchone()[0], "confirmed")
        after = self.metadata("confirmed")
        self.run_main("--apply", "--plan", plan_path)
        self.assertEqual(self.metadata("confirmed"), after)
        self.assertEqual(self.db.execute("SELECT COUNT(*) FROM memory_curation_events WHERE action='metadata_repair'").fetchone()[0], 2)
        self.run_main("--plan", repair.REPORTS_ROOT / "after.json", "--baseline", baseline)
        self.assertEqual(json.loads((repair.REPORTS_ROOT / "after.json").read_text())["proposals"], [])

    def test_read_only_preview_does_not_change_database_bytes(self):
        self.seed("doc")
        self.damage("doc")
        self.db.close()
        before = hashlib.sha256(self.path.read_bytes()).digest()
        self.run_main("--plan", repair.REPORTS_ROOT / "preview.json")
        self.assertEqual(hashlib.sha256(self.path.read_bytes()).digest(), before)

    def test_later_decisions_dry_runs_and_conflicts_are_respected(self):
        for doc_id in ("later-unverify", "open-conflict", "later-resolved", "dry-run", "stale-review", "unknown-decision"):
            self.seed(doc_id)
            self.damage(doc_id)
        self.event("later-unverify", "unverify", LATER)
        self.db.execute("UPDATE memory_meta SET last_reviewed_at=? WHERE doc_id='later-unverify'", (LATER,))
        self.event("later-unverify", "confirm", LATER, dry_run=True)
        self.event("dry-run", "archive", LATER, dry_run=True)
        self.db.execute("UPDATE memory_meta SET last_reviewed_at=? WHERE doc_id='stale-review'", (LATER,))
        self.db.execute("""INSERT INTO memory_curation_events
            (doc_id, action, actor, previous_status, new_status, timestamp)
            VALUES ('unknown-decision', 'legacy_manual_edit', 'admin', 'confirmed', 'unverified', ?)""", (LATER,))
        self.db.execute("UPDATE memory_meta SET last_reviewed_at=? WHERE doc_id='unknown-decision'", (LATER,))
        for doc_id, status in (("open-conflict", "open"), ("later-resolved", "resolved")):
            self.db.execute("""INSERT INTO memory_conflicts(doc_id_left, doc_id_right, conflict_key, status, detected_at, resolved_at)
                VALUES (?, 'other', 'language', ?, ?, ?)""", (doc_id, status, LATER, LATER if status == "resolved" else ""))
        plan = repair.make_plan(self.db)
        self.assertEqual([item["doc_id"] for item in plan["proposals"]], ["dry-run"])
        reviews = {item["doc_id"] for item in plan["review_required"]}
        self.assertTrue({"open-conflict", "later-resolved", "stale-review"} <= reviews)

    def test_later_protection_preserves_last_effective_confirmation(self):
        self.seed("doc")
        self.db.execute("""INSERT INTO memory_curation_events
            (doc_id, action, actor, previous_status, new_status, timestamp)
            VALUES ('doc', 'protect', 'admin', 'confirmed', 'confirmed', ?)""", (LATER,))
        self.db.execute("UPDATE memory_meta SET last_reviewed_at=?, last_event_at=? WHERE doc_id='doc'", (LATER, LATER))
        baseline = self.baseline()
        self.damage("doc")
        self.db.execute("UPDATE memory_meta SET last_event_at='2026-09-04 12:00:00' WHERE doc_id='doc'")
        with closing(repair.connect(baseline)) as saved:
            plan = repair.make_plan(self.db, [(baseline, saved)])
            self.assertEqual(plan["review_required"], [])
            self.assertEqual(plan["proposals"][0]["changes"]["verification_status"], "confirmed")
            self.assertEqual(repair.apply_plan(self.db, plan, [(baseline, saved)])["applied"], 1)
        self.assertEqual(self.metadata("doc")["protected"], 1)

    def test_archival_reconstruction_and_ambiguous_reactivation(self):
        self.seed("missing-time", "archive")
        self.db.execute("UPDATE memory_meta SET archived_at=NULL WHERE doc_id='missing-time'")
        self.seed("reactivated", "archive")
        self.damage("reactivated")
        self.event("reactivated", "confirm", LATER)
        self.db.execute("UPDATE memory_meta SET last_reviewed_at=? WHERE doc_id='reactivated'", (LATER,))
        self.seed("clock-reset", "archive")
        self.db.execute("UPDATE memory_curation_events SET timestamp=? WHERE doc_id='clock-reset'", (LATER,))
        self.damage("clock-reset")
        self.event("clock-reset", "confirm", REVIEWED)
        self.db.execute("UPDATE memory_meta SET archived_at=?, last_reviewed_at=? WHERE doc_id='clock-reset'", (LATER, REVIEWED))
        plan = repair.make_plan(self.db)
        self.assertEqual([item["doc_id"] for item in plan["proposals"]], ["missing-time"])
        self.assertEqual(plan["proposals"][0]["changes"]["archived_at"], REVIEWED)
        self.assertEqual({item["doc_id"] for item in plan["review_required"]}, {"reactivated", "clock-reset"})

    def test_backup_identity_and_disagreement_require_review(self):
        self.seed("doc")
        first = self.baseline("first.sqlite")
        self.db.execute("UPDATE memory_meta SET extraction_confidence=0.96 WHERE doc_id='doc'")
        second = self.baseline("second.sqlite")
        self.damage("doc")
        with closing(repair.connect(first)) as one, closing(repair.connect(second)) as two:
            plan = repair.make_plan(self.db, [(first, one), (second, two)])
            self.assertEqual(plan["proposals"], [])
            self.assertIn("disagree", plan["review_required"][0]["reasons"][0])
        with closing(repair.connect(second, True)) as foreign:
            foreign.execute("UPDATE memory_curation_events SET actor='other-installation'")
        with closing(repair.connect(second)) as foreign:
            plan = repair.make_plan(self.db, [(second, foreign)])
            self.assertEqual(plan["proposals"][0]["changes"], {"verification_status": "confirmed"})
            self.assertTrue(plan["review_required"])

    def test_intervening_metadata_decisions_and_conflicts_are_skipped(self):
        for doc_id in ("metadata", "decision", "conflict", "unchanged"):
            self.seed(doc_id)
            self.damage(doc_id)
        plan = repair.make_plan(self.db)
        self.db.execute("UPDATE memory_meta SET protected=0 WHERE doc_id='metadata'")
        self.event("decision", "unverify", LATER)
        self.db.execute("""INSERT INTO memory_conflicts(doc_id_left, doc_id_right, conflict_key)
            VALUES ('conflict', 'other', 'language')""")
        result = repair.apply_plan(self.db, plan)
        self.assertEqual(result["applied"], 1)
        self.assertEqual(result["skipped"], 3)
        self.assertEqual(self.metadata("unchanged")["verification_status"], "confirmed")
        for doc_id in ("metadata", "decision", "conflict"):
            self.assertEqual(self.metadata(doc_id)["verification_status"], "unverified")

    def test_failed_audit_write_rolls_back_all_repairs(self):
        for doc_id in ("one", "two"):
            self.seed(doc_id)
            self.damage(doc_id)
        plan = repair.make_plan(self.db)
        self.db.execute("""CREATE TRIGGER reject_repair_log BEFORE INSERT ON memory_curation_events
            WHEN NEW.action='metadata_repair' BEGIN SELECT RAISE(ABORT, 'synthetic audit failure'); END""")
        with self.assertRaises(sqlite3.Error):
            repair.apply_plan(self.db, plan)
        for doc_id in ("one", "two"):
            self.assertEqual(self.metadata(doc_id)["verification_status"], "unverified")

    def test_edited_plan_cannot_introduce_unproven_changes(self):
        self.seed("doc")
        self.damage("doc")
        plan = repair.make_plan(self.db)
        plan["proposals"][0]["changes"]["extraction_confidence"] = 1.0
        self.assertEqual(repair.apply_plan(self.db, plan)["applied"], 0)
        plan["proposals"][0]["changes"]["protected"] = 0
        with self.assertRaises(ValueError):
            repair.apply_plan(self.db, plan)
        self.assertEqual(self.metadata("doc")["verification_status"], "unverified")

    def test_artifacts_cannot_escape_reports(self):
        with self.assertRaises(ValueError):
            repair.write_report(self.root / "outside.json", {})
        with self.assertRaises(ValueError):
            repair.backup(self.db, self.root / "outside.sqlite")
        self.assertFalse((self.root / "outside.json").exists())

    def test_partial_confirmation_remains_visible_and_continues_with_backup(self):
        self.seed("doc")
        baseline = self.baseline()
        self.damage("doc")
        first = repair.make_plan(self.db)
        self.assertEqual(first["proposals"][0]["changes"], {"verification_status": "confirmed"})
        self.assertEqual(repair.apply_plan(self.db, first)["applied"], 1)
        waiting = repair.make_plan(self.db)
        self.assertEqual(waiting["proposals"], [])
        self.assertEqual(waiting["review_required"][0]["doc_id"], "doc")
        audit = json.loads(self.db.execute("SELECT reason FROM memory_curation_events WHERE action='metadata_repair'").fetchone()[0])
        self.assertEqual(audit["version"], 2)
        self.assertTrue(audit["remaining"])
        with closing(repair.connect(baseline)) as saved:
            second = repair.make_plan(self.db, [(baseline, saved)])
            self.assertEqual(second["review_required"], [])
            self.assertEqual(set(second["proposals"][0]["changes"]), set(repair.QUALITY_FIELDS))
            self.assertEqual(repair.apply_plan(self.db, second, [(baseline, saved)])["applied"], 1)
            self.assertEqual(repair.apply_plan(self.db, second, [(baseline, saved)])["applied"], 0)
        self.assertEqual(self.metadata("doc")["source_type"], "user")

    def test_partial_repair_rejects_new_decisions_conflicts_and_bad_history(self):
        for change in ("curation", "conflict", "audit"):
            with self.subTest(change=change):
                doc = change
                self.seed(doc)
                baseline = self.baseline(change + ".sqlite")
                self.damage(doc)
                repair.apply_plan(self.db, repair.make_plan(self.db))
                if change == "curation":
                    self.event(doc, "protect", LATER)
                    self.db.execute("UPDATE memory_meta SET last_reviewed_at=? WHERE doc_id=?", (LATER, doc))
                elif change == "conflict":
                    self.db.execute("INSERT INTO memory_conflicts(doc_id_left,doc_id_right,conflict_key) VALUES (?, 'other', 'language')", (doc,))
                else:
                    self.db.execute("UPDATE memory_curation_events SET reason='unproven repair' WHERE doc_id=? AND action='metadata_repair'", (doc,))
                with closing(repair.connect(baseline)) as saved:
                    plan = repair.make_plan(self.db, [(baseline, saved)])
                    self.assertNotIn(doc, [item["doc_id"] for item in plan["proposals"]])
                    self.assertIn(doc, [item["doc_id"] for item in plan["review_required"]])

    def test_legacy_status_repair_requires_exact_evidence_chain(self):
        self.seed("doc")
        baseline = self.baseline()
        self.damage("doc")
        first = repair.make_plan(self.db)
        repair.apply_plan(self.db, first)
        self.db.execute("UPDATE memory_curation_events SET reason=? WHERE action='metadata_repair'",
                        ("repair fields: verification_status; evidence: " + first["proposals"][0]["evidence_hash"],))
        with closing(repair.connect(baseline)) as saved:
            self.assertEqual(len(repair.make_plan(self.db, [(baseline, saved)])["proposals"]), 1)
        self.db.execute("UPDATE memory_curation_events SET reason=reason || 'changed' WHERE action='metadata_repair'")
        with closing(repair.connect(baseline)) as saved:
            plan = repair.make_plan(self.db, [(baseline, saved)])
            self.assertEqual(plan["proposals"], [])
            self.assertTrue(plan["review_required"])


if __name__ == "__main__":
    unittest.main()

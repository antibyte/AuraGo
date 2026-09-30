#!/usr/bin/env python3
"""Preview and explicitly apply evidence-backed AuraGo memory metadata repairs."""

import argparse
from contextlib import ExitStack, closing
from datetime import datetime, timezone
import hashlib
import json
import math
import os
from pathlib import Path
import sqlite3
import sys
import uuid


REPORTS_ROOT = Path(__file__).resolve().parents[1] / "reports"
REPAIR_FIELDS = {
    "verification_status", "archived_at", "archived_reason",
    "source_type", "source_reliability", "extraction_confidence",
}
QUALITY_FIELDS = ("source_type", "source_reliability", "extraction_confidence")
STATUS_ACTIONS = {"archive": "archived", "confirm": "confirmed", "unverify": "unverified"}


def connect(path, writable=False):
    path = Path(path).resolve(strict=True)
    db = sqlite3.connect(path.as_uri() + ("?mode=rw" if writable else "?mode=ro"), uri=True, timeout=5)
    db.row_factory = sqlite3.Row
    db.isolation_level = None
    return db


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False, allow_nan=False).encode("utf-8")).hexdigest()


def timestamp(value):
    if not value:
        return None
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        return parsed.replace(tzinfo=timezone.utc) if parsed.tzinfo is None else parsed.astimezone(timezone.utc)
    except (TypeError, ValueError):
        return None


def evidence(db, doc_id):
    events = [dict(row) for row in db.execute(
        "SELECT * FROM memory_curation_events WHERE doc_id=? AND dry_run=0 ORDER BY id", (doc_id,))]
    conflicts = [dict(row) for row in db.execute(
        "SELECT * FROM memory_conflicts WHERE doc_id_left=? OR doc_id_right=? ORDER BY id", (doc_id, doc_id))]
    return {"events": events, "conflicts": conflicts}


def status_decisions(events):
    return [event for event in events if event["action"] in STATUS_ACTIONS
            or event["previous_status"] != event["new_status"]]


def backup_quality(row, proof, baselines, changes):
    matches = []
    decisions = status_decisions(proof["events"])
    archive_time = changes.get("archived_at", row["archived_at"]) or ""
    expected_status = decisions[-1]["new_status"] if decisions else None
    if archive_time:
        expected_status = "archived"
    for path, db in baselines:
        saved = db.execute("SELECT * FROM memory_meta WHERE doc_id=?", (row["doc_id"],)).fetchone()
        if saved is None:
            continue
        saved = dict(saved)
        # Matching IDs alone cannot establish that this is the same curated fact.
        if (not row["last_reviewed_at"] or row["last_reviewed_at"] != proof["events"][-1]["timestamp"]
                or saved["last_reviewed_at"] != row["last_reviewed_at"]
                or evidence(db, row["doc_id"])["events"] != proof["events"]
                or saved["protected"] != row["protected"] or saved["keep_forever"] != row["keep_forever"]
                or (saved["archived_at"] or "") != archive_time
                or saved["verification_status"] != expected_status
                or not timestamp(saved["last_event_at"]) or not timestamp(row["last_event_at"])
                or timestamp(saved["last_event_at"]) > timestamp(row["last_event_at"])):
            continue
        values = {key: saved[key] for key in QUALITY_FIELDS}
        if not isinstance(values["source_type"], str) or not values["source_type"].strip() or values["source_type"] == "memory_analysis":
            continue
        if any(not isinstance(values[key], (int, float)) or not math.isfinite(values[key])
               or not 0 <= values[key] <= 1 for key in QUALITY_FIELDS[1:]):
            continue
        matches.append({"path": str(path), "row_hash": digest(saved), "values": values})
    if len({digest(match["values"]) for match in matches}) > 1:
        return None, matches, "matching backups disagree on provenance or confidence"
    return (matches[0]["values"] if matches else None), matches, ""


def propose(row, proof, baselines):
    events = proof["events"]
    decisions = status_decisions(events)
    decision = decisions[-1] if decisions else None
    status = (row["verification_status"] or "unverified").strip().lower()
    archived_at = (row["archived_at"] or "").strip()
    changes, reasons, review = {}, [], []

    if archived_at and status != "archived":
        archive_time = timestamp(archived_at)
        if archive_time is None:
            return None, ["archive timestamp cannot be parsed"]
        if decision and decision["new_status"] != "archived":
            decision_time = timestamp(decision["timestamp"])
            archive_events = [event for event in decisions if event["new_status"] == "archived" and timestamp(event["timestamp"]) == archive_time]
            if (decision_time is None or decision_time >= archive_time
                    or any(event["id"] < decision["id"] for event in archive_events)):
                return None, ["later reactivation decision conflicts with retained archive timestamp"]
        changes["verification_status"] = "archived"
        reasons.append("retained archive timestamp proves archival")
    elif status == "archived" and not archived_at:
        if (not decision or decision["action"] != "archive" or not timestamp(decision["timestamp"])
                or decision["new_status"] != "archived" or not events
                or row["last_reviewed_at"] != events[-1]["timestamp"]):
            return None, ["archived status has no unambiguous archive-time evidence"]
        changes.update(archived_at=decision["timestamp"], archived_reason=decision["reason"] or "")
        reasons.append("last effective archive event supplies timestamp and reason")
    elif status == "unverified" and row["source_type"] == "memory_analysis" and decision and decision["action"] == "confirm":
        if (decision["new_status"] != "confirmed" or not timestamp(decision["timestamp"])
                or row["last_reviewed_at"] != events[-1]["timestamp"]):
            return None, ["confirmation does not match the last recorded review"]
        for conflict in proof["conflicts"]:
            times = [timestamp(conflict[key]) for key in ("detected_at", "resolved_at") if conflict[key]]
            if (conflict["status"] != "resolved" or any(value is None for value in times)
                    or any(value >= timestamp(decision["timestamp"]) for value in times)):
                return None, ["open or later conflict prevents restoring confirmation"]
        changes["verification_status"] = "confirmed"
        reasons.append("last effective confirmation survived in the curation ledger")

    if (not changes and status == "unverified" and row["source_type"] == "memory_analysis"
            and decision and decision["new_status"] != "unverified"):
        return None, ["last status decision cannot be reconstructed unambiguously"]

    backups = []
    if row["source_type"] == "memory_analysis" and events:
        if not archived_at and status != "archived" and any(conflict["status"] != "resolved" for conflict in proof["conflicts"]):
            return None, ["unresolved conflicts require review before provenance repair"]
        values, backups, ambiguity = backup_quality(row, proof, baselines, changes)
        if ambiguity:
            return None, [ambiguity]
        if values:
            changes.update({key: value for key, value in values.items() if value != row[key]})
            if any(key in changes for key in QUALITY_FIELDS):
                reasons.append("matching backup proves provenance and confidence")
        elif changes or baselines:
            review.append("provenance and confidence need a matching pre-damage backup")
    if not changes:
        return None, review
    return {
        "doc_id": row["doc_id"], "expected": row, "changes": changes,
        "evidence_hash": digest(proof), "reasons": reasons, "backups": backups,
        "events": [{key: event[key] for key in ("id", "timestamp", "action", "actor", "new_status")} for event in events],
        "conflict_ids": [conflict["id"] for conflict in proof["conflicts"]],
    }, review


def make_plan(db, baselines=()):
    plan = {"version": 1, "proposals": [], "review_required": []}
    rows = db.execute("""SELECT * FROM memory_meta WHERE
        (COALESCE(archived_at,'') != '' AND COALESCE(verification_status,'') != 'archived')
        OR (verification_status='archived' AND COALESCE(archived_at,'')='')
        OR (source_type='memory_analysis' AND verification_status='unverified') ORDER BY doc_id""")
    for record in rows:
        row = dict(record)
        proposal, review = propose(row, evidence(db, row["doc_id"]), baselines)
        if proposal:
            plan["proposals"].append(proposal)
        if review:
            plan["review_required"].append({"doc_id": row["doc_id"], "reasons": review})
    return plan


def apply_plan(db, plan, baselines=()):
    if plan.get("version") != 1:
        raise ValueError("unsupported repair plan version")
    result = {"applied": 0, "skipped": 0, "items": []}
    db.execute("BEGIN IMMEDIATE")
    try:
        for item in plan["proposals"]:
            changes = item["changes"]
            if not changes or not set(changes) <= REPAIR_FIELDS or item["expected"]["doc_id"] != item["doc_id"]:
                raise ValueError("invalid repair fields or document identity")
            current = db.execute("SELECT * FROM memory_meta WHERE doc_id=?", (item["doc_id"],)).fetchone()
            current = dict(current) if current else None
            outcome = "skipped_changed"
            if current and all(current[key] == value for key, value in changes.items()):
                outcome = "skipped_already_applied"
            elif current == item["expected"]:
                proof = evidence(db, item["doc_id"])
                verified, _ = propose(current, proof, baselines)
                if (digest(proof) == item["evidence_hash"] and verified
                        and verified["changes"] == changes and verified["backups"] == item["backups"]):
                    fields = sorted(changes)
                    update = db.execute("UPDATE memory_meta SET " + ", ".join(key + "=?" for key in fields)
                               + ", last_event_at=CURRENT_TIMESTAMP WHERE doc_id=?",
                               [changes[key] for key in fields] + [item["doc_id"]])
                    if update.rowcount != 1:
                        raise ValueError("repair update did not affect exactly one expected row")
                    db.execute("""INSERT INTO memory_curation_events
                        (doc_id, action, actor, previous_status, new_status, reason, dry_run)
                        VALUES (?, 'metadata_repair', 'operator_repair', ?, ?, ?, 0)""",
                        (item["doc_id"], current["verification_status"], changes.get("verification_status", current["verification_status"]),
                         "repair fields: " + ", ".join(fields) + "; evidence: " + item["evidence_hash"]))
                    outcome = "applied"
            result["applied" if outcome == "applied" else "skipped"] += 1
            result["items"].append({"doc_id": item["doc_id"], "outcome": outcome, "fields": sorted(changes)})
        db.commit()
    except BaseException:
        db.rollback()
        raise
    return result


def report_path(path):
    path = Path(path).resolve()
    if not path.is_relative_to(REPORTS_ROOT.resolve()):
        raise ValueError("repair artifacts must stay under reports/")
    path.parent.mkdir(parents=True, exist_ok=True)
    return path


def write_report(path, value):
    path = report_path(path)
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w", encoding="utf-8") as output:
        json.dump(value, output, ensure_ascii=False, indent=2, allow_nan=False)
        output.write("\n")


def backup(db, path):
    path = report_path(path)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    os.close(fd)
    with closing(sqlite3.connect(path)) as target:
        db.backup(target)
        if target.execute("PRAGMA quick_check").fetchone()[0] != "ok":
            raise ValueError("repair backup failed SQLite integrity check")
    return path


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", required=True, type=Path, help="existing short_term.db")
    parser.add_argument("--baseline", action="append", default=[], type=Path, help="pre-damage SQLite backup; repeat to cross-check")
    parser.add_argument("--plan", type=Path, help="preview JSON under reports/; required with --apply")
    parser.add_argument("--apply", action="store_true", help="apply reviewed plan after backup and rehearsal")
    args = parser.parse_args(argv)
    if args.apply and (args.plan is None or args.baseline):
        parser.error("--apply requires --plan; baseline paths come from the reviewed plan")
    run_id = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex[:8]
    prefix = REPORTS_ROOT / ("memory-repair-" + run_id)
    plan_path = report_path(args.plan or (str(prefix) + "-preview.json"))
    with ExitStack() as stack:
        db = stack.enter_context(closing(connect(args.db, args.apply)))
        if args.apply:
            plan = json.loads(plan_path.read_text(encoding="utf-8"))
            if Path(plan["database"]).resolve() != args.db.resolve():
                raise ValueError("repair plan belongs to a different database path")
            baseline_paths = [Path(path) for path in plan["baseline_databases"]]
        else:
            baseline_paths = [path.resolve(strict=True) for path in args.baseline]
        baselines = []
        for path in baseline_paths:
            if path.resolve() == args.db.resolve():
                raise ValueError("the current database cannot be its own pre-damage baseline")
            baseline_db = stack.enter_context(closing(connect(path)))
            baseline_db.execute("BEGIN")
            baselines.append((path, baseline_db))
        if not args.apply:
            db.execute("BEGIN")
            plan = make_plan(db, baselines)
            db.rollback()
            plan.update(database=str(args.db.resolve()), baseline_databases=[str(path) for path in baseline_paths], generated_at=run_id)
            write_report(plan_path, plan)
            print(f"Preview: {len(plan['proposals'])} repairs, {len(plan['review_required'])} review items; {plan_path}")
            return 0
        backup_path = backup(db, str(prefix) + "-backup.sqlite")
        with closing(connect(backup_path)) as saved:
            rehearsal_path = backup(saved, str(prefix) + "-rehearsal.sqlite")
        with closing(connect(rehearsal_path, True)) as rehearsal_db:
            rehearsal = apply_plan(rehearsal_db, plan, baselines)
            repeated = apply_plan(rehearsal_db, plan, baselines)
            if repeated["applied"] or rehearsal_db.execute("PRAGMA integrity_check").fetchone()[0] != "ok":
                raise ValueError("repair rehearsal failed integrity or idempotence check")
        write_report(str(prefix) + "-rehearsal.json", {"first": rehearsal, "repeat": repeated})
        applied = apply_plan(db, plan, baselines)
        write_report(str(prefix) + "-applied.json", {"backup": str(backup_path), "rehearsal": str(rehearsal_path), "result": applied})
        print(f"Applied: {applied['applied']}, skipped: {applied['skipped']}; {prefix}-applied.json")
        return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, sqlite3.Error, ValueError, KeyError) as error:
        print(f"Memory repair failed: {error}", file=sys.stderr)
        sys.exit(1)

"""Transactional cost reservations; uncertain requests keep their reservation."""
from __future__ import annotations

from contextlib import contextmanager
from decimal import Decimal, ROUND_CEILING
from pathlib import Path
import sqlite3
import uuid

from .common import config


def micros(usd):
    return int((Decimal(str(usd)) * 1_000_000).to_integral_value(rounding=ROUND_CEILING))


class Budget:
    def __init__(self, path, limits=None):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.limits = {k: micros(v) for k, v in (limits or config()["budget_usd"]).items()}
        with self.connection() as db:
            db.execute("CREATE TABLE IF NOT EXISTS requests (id TEXT PRIMARY KEY, stage TEXT NOT NULL, model TEXT NOT NULL, reserved INTEGER NOT NULL, charged INTEGER, status TEXT NOT NULL, created TEXT DEFAULT CURRENT_TIMESTAMP)")
            db.execute("CREATE TABLE IF NOT EXISTS faults (reason TEXT NOT NULL)")

    @contextmanager
    def connection(self):
        db = sqlite3.connect(self.path, timeout=30)
        try:
            with db:
                db.execute("BEGIN IMMEDIATE")
                yield db
        finally:
            db.close()

    def reserve(self, stage, model, upper_bound_usd):
        amount = micros(upper_bound_usd)
        if stage not in self.limits or stage == "total" or amount <= 0:
            raise ValueError("invalid budget reservation")
        with self.connection() as db:
            if db.execute("SELECT 1 FROM faults LIMIT 1").fetchone():
                raise RuntimeError("cost accounting is latched closed; reconcile the ledger first")
            total = db.execute("SELECT COALESCE(SUM(COALESCE(charged,reserved)),0) FROM requests").fetchone()[0]
            subtotal = db.execute("SELECT COALESCE(SUM(COALESCE(charged,reserved)),0) FROM requests WHERE stage=?", (stage,)).fetchone()[0]
            if total + amount > self.limits["total"] or subtotal + amount > self.limits[stage]:
                raise RuntimeError(f"{stage} budget exhausted; no request was sent")
            request_id = uuid.uuid4().hex
            db.execute("INSERT INTO requests(id,stage,model,reserved,status) VALUES(?,?,?,?,?)", (request_id, stage, model, amount, "pending"))
            return request_id

    def settle(self, request_id, cost_usd):
        actual = micros(cost_usd)
        if actual < 0:
            raise ValueError("negative API cost")
        with self.connection() as db:
            row = db.execute("SELECT reserved,status FROM requests WHERE id=?", (request_id,)).fetchone()
            if row is None or row[1] != "pending":
                raise ValueError("unknown or already settled reservation")
            db.execute("UPDATE requests SET charged=?,status='settled' WHERE id=?", (actual, request_id))
            exceeded = actual > row[0]
            if exceeded:
                db.execute("INSERT INTO faults VALUES ('actual cost exceeded reserved maximum')")
        if exceeded:
            raise RuntimeError("API cost exceeded its conservative reservation; stop and inspect pricing")

    def summary(self):
        with self.connection() as db:
            rows = db.execute("SELECT stage,COUNT(*),SUM(COALESCE(charged,reserved)),SUM(status='pending') FROM requests GROUP BY stage").fetchall()
        return {stage: {"requests": count, "accounted_usd": amount / 1e6, "uncertain_requests": pending} for stage, count, amount, pending in rows}

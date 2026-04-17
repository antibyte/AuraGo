import sys
import json
import os
import re
from datetime import datetime, timedelta

LEVEL_PATTERNS = {
    "error": re.compile(r'\b(ERROR|FATAL|CRITICAL|SEVERE|ERR)\b', re.IGNORECASE),
    "warning": re.compile(r'\b(WARN|WARNING|WRN)\b', re.IGNORECASE),
    "info": re.compile(r'\b(INFO|INFORMATION)\b', re.IGNORECASE),
    "debug": re.compile(r'\b(DEBUG|DBG|TRACE)\b', re.IGNORECASE),
}

TIMESTAMP_PATTERNS = [
    re.compile(r'(\d{4}-\d{2}-\d{2}[\sT]\d{2}:\d{2}:\d{2})'),
    re.compile(r'(\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2})'),
    re.compile(r'(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})'),
]

def _parse_timestamp(line):
    for pat in TIMESTAMP_PATTERNS:
        m = pat.search(line)
        if m:
            for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S", "%d/%m/%Y %H:%M:%S", "%b %d %H:%M:%S"):
                try:
                    return datetime.strptime(m.group(1), fmt)
                except ValueError:
                    continue
    return None

def _detect_level(line):
    for level, pat in LEVEL_PATTERNS.items():
        if pat.search(line):
            return level
    return "unknown"

def _parse_time_filter(since):
    if not since:
        return None
    m = re.match(r'(\d+)\s*(m|h|d|w)', since.lower().strip())
    if not m:
        return None
    val, unit = int(m.group(1)), m.group(2)
    delta_map = {"m": timedelta(minutes=val), "h": timedelta(hours=val), "d": timedelta(days=val), "w": timedelta(weeks=val)}
    return datetime.now() - delta_map.get(unit, timedelta())

def {{.FunctionName}}(log_path, operation="summary", pattern=None, since=None, max_results=100):
    """{{.Description}}"""
    if not os.path.isabs(log_path):
        log_path = os.path.abspath(log_path)
    if not os.path.exists(log_path):
        return {"status": "error", "message": f"File not found: {log_path}"}

    try:
        with open(log_path, "r", encoding="utf-8", errors="replace") as f:
            lines = f.readlines()
    except Exception as e:
        return {"status": "error", "message": str(e)}

    cutoff = _parse_time_filter(since)
    max_results = int(max_results)
    parsed = []
    for line in lines:
        ts = _parse_timestamp(line)
        level = _detect_level(line)
        if cutoff and ts and ts < cutoff:
            continue
        parsed.append({"line": line.rstrip(), "level": level, "timestamp": ts})

    if operation == "summary":
        counts = {}
        for entry in parsed:
            counts[entry["level"]] = counts.get(entry["level"], 0) + 1
        return {
            "status": "success",
            "result": {
                "total_lines": len(lines),
                "filtered_lines": len(parsed),
                "level_counts": counts,
                "time_range": {
                    "earliest": str(parsed[0]["timestamp"]) if parsed and parsed[0]["timestamp"] else None,
                    "latest": str(parsed[-1]["timestamp"]) if parsed and parsed[-1]["timestamp"] else None,
                },
            },
        }

    elif operation == "errors":
        errors = [e for e in parsed if e["level"] in ("error", "fatal")]  [:max_results]
        return {"status": "success", "result": {"count": len(errors), "errors": [e["line"] for e in errors]}}

    elif operation == "search":
        if not pattern:
            return {"status": "error", "message": "Pattern is required for search operation"}
        regex = re.compile(pattern, re.IGNORECASE)
        matches = [e for e in parsed if regex.search(e["line"])][:max_results]
        return {"status": "success", "result": {"count": len(matches), "matches": [e["line"] for e in matches]}}

    elif operation == "tail":
        tail_lines = parsed[-max_results:]
        return {"status": "success", "result": {"lines": [e["line"] for e in tail_lines]}}

    elif operation == "count_by_level":
        counts = {}
        for entry in parsed:
            counts[entry["level"]] = counts.get(entry["level"], 0) + 1
        return {"status": "success", "result": counts}

    else:
        return {"status": "error", "message": f"Unknown operation: {operation}. Use: summary, errors, search, tail, count_by_level"}

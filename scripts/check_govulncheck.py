#!/usr/bin/env python3
"""Check a successful govulncheck -json source/symbol scan, failing closed.

GO-2026-6452 currently omits the fixed version. Excelize v2.11.0 contains
upstream fix 93f0b3caed37f21ef5079e3259c6c21dcfe68453 (tag is two commits ahead):
https://github.com/qax-os/excelize/releases/tag/v2.11.0
https://github.com/qax-os/excelize/commit/93f0b3caed37f21ef5079e3259c6c21dcfe68453
TestWorkbookRejectsInvalidSharedStringIndex exercises the exploit through
GetCellValue, GetRows and the editor before CI runs this gate. This exact-version
exception can be removed once the Go vulnerability database records the fix.
All other called-symbol findings remain blocking. Scanner errors and malformed
reports fail the gate as well.
"""

import json
import sys
from pathlib import Path


REVIEWED = ("GO-2026-6452", "github.com/xuri/excelize/v2", "v2.11.0")


def assess(source):
    decoder = json.JSONDecoder()
    offset = 0
    configured = False
    has_sbom = False
    blocked, reviewed, uncalled = set(), set(), set()
    while offset < len(source):
        if source[offset].isspace():
            offset += 1
            continue
        record, offset = decoder.raw_decode(source, offset)
        if not isinstance(record, dict) or len(record) != 1:
            raise ValueError("invalid govulncheck record")
        kind, data = next(iter(record.items()))
        if not isinstance(data, dict):
            raise ValueError("invalid govulncheck payload")
        if kind == "config":
            if configured or data.get("protocol_version") != "v1.0.0" or data.get("scanner_name") != "govulncheck" or data.get("scan_level") != "symbol" or data.get("scan_mode") != "source":
                raise ValueError("expected one govulncheck source/symbol scan")
            configured = True
        elif kind == "SBOM":
            has_sbom = bool(data.get("modules"))
        elif kind == "finding":
            trace = data.get("trace")
            if not data.get("osv") or not isinstance(trace, list) or not trace or not isinstance(trace[0], dict) or not trace[0].get("module"):
                raise ValueError("incomplete vulnerability finding")
            frame = trace[0]
            item = (data["osv"], frame["module"], frame.get("version", ""))
            if not frame.get("function"):
                uncalled.add(item)
            elif item == REVIEWED:
                reviewed.add(item)
            else:
                blocked.add(item)
        elif kind not in {"osv", "progress"}:
            raise ValueError(f"unexpected govulncheck record: {kind}")
    if not configured or not has_sbom:
        raise ValueError("missing scan configuration or dependency inventory")
    return blocked, reviewed, uncalled - blocked - reviewed


def main():
    try:
        blocked, reviewed, uncalled = assess(Path(sys.argv[1]).read_text(encoding="utf-8"))
    except (IndexError, OSError, ValueError, TypeError) as exc:
        print(f"Invalid vulnerability scan: {exc}", file=sys.stderr)
        return 2
    for label, items in (("BLOCKED", blocked), ("Reviewed database false positive", reviewed), ("Not called", uncalled)):
        for advisory, module, version in sorted(items):
            print(f"{label}: {advisory} {module}@{version} https://pkg.go.dev/vuln/{advisory}")
    print(f"Vulnerability gate: {len(blocked)} blocking findings; {len(reviewed)} exact-version database corrections")
    return 1 if blocked else 0


if __name__ == "__main__":
    sys.exit(main())

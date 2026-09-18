import json
import unittest

from check_govulncheck import REVIEWED, assess


def report(*findings, **config):
    records = [
        {"config": {"protocol_version": "v1.0.0", "scanner_name": "govulncheck", "scan_level": "symbol", "scan_mode": "source", **config}},
        {"SBOM": {"modules": [{"path": "aurago"}]}},
        *({"finding": finding} for finding in findings),
    ]
    return "\n".join(json.dumps(record) for record in records)


def finding(advisory=REVIEWED[0], module=REVIEWED[1], version=REVIEWED[2], function="GetCellValue"):
    return {"osv": advisory, "trace": [{"module": module, "version": version, "function": function}]}


class VulnerabilityGateTests(unittest.TestCase):
    def test_clean_scan(self):
        self.assertEqual(assess(report()), (set(), set(), set()))

    def test_only_exact_reviewed_advisory_and_version_are_exempt(self):
        self.assertEqual(assess(report(finding()))[:2], (set(), {REVIEWED}))
        for override in ({"version": "v2.10.1"}, {"version": "v2.11.1"}, {"module": "example.com/other"}, {"advisory": "GO-2026-NEW"}):
            with self.subTest(override=override):
                blocked, reviewed, _ = assess(report(finding(**override)))
                self.assertEqual(len(blocked), 1)
                self.assertFalse(reviewed)

    def test_other_findings_still_fail_alongside_exception(self):
        blocked, reviewed, _ = assess(report(finding(), finding("GO-2026-6443", "google.golang.org/grpc", "v1.82.1")))
        self.assertEqual(blocked, {("GO-2026-6443", "google.golang.org/grpc", "v1.82.1")})
        self.assertEqual(reviewed, {REVIEWED})

    def test_module_only_findings_retain_upstream_symbol_scan_behavior(self):
        self.assertEqual(len(assess(report(finding("GO-OTHER", function="")))[2]), 1)
        blocked, _, uncalled = assess(report(finding("GO-OTHER", function=""), finding("GO-OTHER")))
        self.assertEqual(len(blocked), 1)
        self.assertFalse(uncalled)

    def test_invalid_or_weaker_scans_fail_closed(self):
        for source in ("", "{}", "[]", "{", report() + "{", report(scan_level="module"), report(scan_mode="binary"), report(protocol_version="unknown"), report() + '{"error": {"message": "scan failed"}}', report({"osv": "GO-OTHER", "trace": []}), '{"config": {}}'):
            with self.subTest(source=source):
                with self.assertRaises((ValueError, TypeError)):
                    assess(source)


if __name__ == "__main__":
    unittest.main()

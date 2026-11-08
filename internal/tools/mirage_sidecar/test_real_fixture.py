import json
import os
import unittest

import server


class RealFixtureTest(unittest.TestCase):
    def test_real_output_is_formatted(self):
        path = os.path.join(os.path.dirname(__file__), "fixture_real.json")
        with open(path, encoding="utf-8") as fh:
            result = json.load(fh)
        text = server.format_description(result)
        self.assertNotEqual(text, "No content could be described.")
        self.assertTrue(text.strip())
        # Real shape: scene dict, object ids shared per kind, text under "string".
        self.assertIn("Scene: computer screen", text)
        self.assertIn("text label (white) at left x12", text)
        self.assertIn("menu bar part of software window", text)
        self.assertIn('"Code Studio"', text)


if __name__ == "__main__":
    unittest.main()

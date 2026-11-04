from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[1]


class IntegrationAuthenticationTests(unittest.TestCase):
    def test_workflow_authentication_matches_fixture(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        fixture = (ROOT / ".github/fixtures/mihomo.yaml").read_text()
        secret_match = re.search(r"^secret: (\S+)$", fixture, re.MULTILINE)
        self.assertIsNotNone(secret_match)
        secret = secret_match[1]
        headers = re.findall(r'-H "Authorization: ([^"]+)"', workflow)
        self.assertEqual(headers, ["Bearer $GH_TOKEN", "Bearer " + secret])
        self.assertIn("MIOCLI_INTEGRATION_SECRET=" + secret, workflow)
        self.assertIn("GH_TOKEN: ${{ github.token }}", workflow)


if __name__ == "__main__":
    unittest.main()

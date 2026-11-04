from pathlib import Path
import hashlib
import os
import re
import subprocess
import tempfile
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


class COSReleaseTests(unittest.TestCase):
    def setUp(self):
        self.workflow = (ROOT / ".github/workflows/release.yml").read_text()

    def test_uploads_in_release_workflow_with_stable_last(self):
        self.assertFalse((ROOT / ".github/workflows/publish-s3.yml").exists())
        self.assertFalse((ROOT / "scripts/publish_s3.py").exists())
        self.assertIn("needs: draft", self.workflow)
        self.assertIn("environment: s3-release", self.workflow)
        self.assertIn("cancel-in-progress: false", self.workflow)
        self.assertIn("!contains(github.ref_name, '-')", self.workflow)
        self.assertIn("!contains(github.ref_name, '+')", self.workflow)
        action = "TencentCloud/cos-action@5ee3807d5ff6fddadbbb1d3e658c0690863de8ec"
        self.assertEqual(self.workflow.count(action), 2)
        self.assertEqual(self.workflow.count("clean: 'false'"), 2)
        self.assertLess(self.workflow.index("local_path: cos-files"), self.workflow.index("local_path: cos-stable"))
        self.assertNotIn("continue-on-error", self.workflow)
        for name in ("S3_BUCKET", "S3_REGION", "S3_KEY_PREFIX", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"):
            self.assertIn("${{ secrets." + name + " }}", self.workflow)
        self.assertNotIn("S3_ENDPOINT", self.workflow)
        self.assertNotIn("vars.", self.workflow)

    def test_prepares_paths_and_rejects_bad_input(self):
        script = self.workflow.split("      - name: Prepare COS files\n", 1)[1].split("        run: |\n", 1)[1]
        script = script.split("      - name:", 1)[0]
        script = "\n".join(line[10:] for line in script.splitlines())
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            assets = root / "release-assets"
            assets.mkdir()
            for name in ("install.sh", "install.ps1"):
                (assets / name).write_text(name)
            for system in ("linux", "darwin", "windows"):
                for arch in ("amd64", "arm64"):
                    extension = "zip" if system == "windows" else "tar.gz"
                    archive = assets / f"miocli_1.2.3_{system}_{arch}.{extension}"
                    archive.write_bytes(b"test package")
                    checksum = assets / (archive.name + ".sha256")
                    checksum.write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + "  " + archive.name + "\n")
            env = dict(os.environ, RELEASE_TAG="v1.2.3", S3_BUCKET="example-123456",
                       S3_REGION="ap-example", S3_KEY_PREFIX="public/", S3_ACCESS_KEY_ID="test",
                       S3_SECRET_ACCESS_KEY="test", GITHUB_OUTPUT=str(root / "outputs"))
            result = subprocess.run(["bash", "-e", "-c", script], cwd=root, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            releases = root / "cos-files/miocli/releases/v1.2.3"
            self.assertEqual(sorted(path.name for path in releases.iterdir()), sorted(path.name for path in assets.iterdir()))
            self.assertEqual(len(list(releases.iterdir())), 14)
            self.assertFalse((root / "cos-files/miocli/stable.txt").exists())
            self.assertEqual((root / "cos-stable/miocli/stable.txt").read_text(), "v1.2.3\n")
            self.assertIn("remote_path=public\n", (root / "outputs").read_text())
            for overrides in ({"S3_REGION": ""}, {"S3_KEY_PREFIX": "../private"}, {"RELEASE_TAG": "v1.2.3-rc1"}):
                result = subprocess.run(["bash", "-e", "-c", script], cwd=root, env=env | overrides,
                                        capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
            checksum.write_text("0" * 64 + "  " + archive.name + "\n")
            result = subprocess.run(["bash", "-e", "-c", script], cwd=root, env=env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()

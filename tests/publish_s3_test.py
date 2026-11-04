import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("publish_s3", ROOT / "scripts/publish_s3.py")
publisher = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(publisher)


class WorkflowConfigurationTests(unittest.TestCase):
    def test_s3_configuration_uses_secrets(self):
        workflow = (ROOT / ".github/workflows/publish-s3.yml").read_text()
        for name in ("S3_ENDPOINT", "S3_BUCKET", "S3_KEY_PREFIX",
                     "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"):
            with self.subTest(name=name):
                self.assertIn("${{ secrets." + name + " }}", workflow)
        self.assertIn("AWS_DEFAULT_REGION: ${{ secrets.S3_REGION || 'us-east-1' }}", workflow)
        self.assertNotIn("vars.", workflow)
        self.assertIn("environment: s3-release", workflow)


class FakeStorage:
    def __init__(self, previous=None, failure=None):
        self.previous = previous
        self.failure = failure
        self.events = []

    def event(self, operation, key):
        self.events.append((operation, key))
        if self.failure == (operation, key):
            raise publisher.PublishError("simulated storage failure")

    def immutable(self, key, path):
        self.event("upload", key)

    def verify_public(self, key, path):
        self.event("verify", key)

    def stable(self, directory):
        self.event("stable", "miocli/stable.txt")
        return self.previous, '"etag"' if self.previous else None

    def alias(self, key, path):
        self.event("alias", key)

    def promote(self, path, etag):
        self.event("promote", "miocli/stable.txt")
        self.previous = path.read_text().strip()


class PublicationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.objects = {}
        for name in ("install.sh", "install.ps1", "archive.tar.gz"):
            path = self.directory / name
            path.write_text(name)
            self.objects[f"miocli/releases/v1.2.3/{name}"] = path

    def test_pointer_last(self):
        storage = FakeStorage()
        publisher.publish(storage, "v1.2.3", self.directory, self.objects)
        self.assertEqual(storage.previous, "v1.2.3")
        operations = [operation for operation, _ in storage.events]
        self.assertEqual(operations[:6], ["upload"] * 3 + ["verify"] * 3)
        self.assertEqual(operations[-2:], ["promote", "verify"])

    def test_failure_before_promotion_preserves_stable(self):
        for failure in (
            ("upload", "miocli/releases/v1.2.3/archive.tar.gz"),
            ("verify", "miocli/releases/v1.2.3/archive.tar.gz"),
            ("alias", "miocli/install.ps1"),
            ("verify", "miocli/install.sh"),
        ):
            with self.subTest(failure=failure):
                storage = FakeStorage("v1.2.2", failure)
                with self.assertRaises(publisher.PublishError):
                    publisher.publish(storage, "v1.2.3", self.directory, self.objects)
                self.assertEqual(storage.previous, "v1.2.2")
                self.assertNotIn(("promote", "miocli/stable.txt"), storage.events)

    def test_rerun_and_older_release_do_not_regress_stable(self):
        for previous in ("v1.2.3", "v1.2.10", "v2.0.0"):
            storage = FakeStorage(previous)
            publisher.publish(storage, "v1.2.3", self.directory, self.objects)
            self.assertEqual(storage.previous, previous)
            self.assertFalse(any(operation in ("alias", "promote") for operation, _ in storage.events))

    def test_rerun_does_not_hide_stale_public_index(self):
        storage = FakeStorage("v1.2.3", ("verify", "miocli/stable.txt"))
        with self.assertRaises(publisher.PublishError):
            publisher.publish(storage, "v1.2.3", self.directory, self.objects)
        self.assertEqual(storage.previous, "v1.2.3")

    def test_immutable_conflict_and_identical_retry(self):
        storage = publisher.Storage("https://storage.example.test", "public", "")
        path = self.directory / "archive.tar.gz"
        with patch.object(storage, "head", return_value={"Metadata": {"sha256": "wrong"}, "ContentLength": path.stat().st_size}):
            with self.assertRaises(publisher.PublishError):
                storage.immutable("file", path)
        with patch.object(storage, "head", return_value={"Metadata": {"sha256": publisher.sha256(path)}, "ContentLength": path.stat().st_size}), \
                patch.object(storage, "api") as api:
            storage.immutable("file", path)
            api.assert_not_called()

    def test_missing_object_is_not_permission_failure(self):
        storage = publisher.Storage("https://storage.example.test", "public", "")
        with patch.object(storage, "api", side_effect=publisher.PublishError("aws failed: (404)")):
            self.assertIsNone(storage.head("file"))
        with patch.object(storage, "api", side_effect=publisher.PublishError("aws failed: (403)")):
            with self.assertRaises(publisher.PublishError):
                storage.head("file")

    def test_conditional_writes(self):
        storage = publisher.Storage("https://storage.example.test", "public", "distribution")
        path = self.directory / "install.sh"
        with patch.object(storage, "head", return_value=None), patch.object(storage, "api") as api:
            storage.immutable("file", path)
            self.assertIn("--if-none-match", api.call_args.args)
            storage.promote(path, '"previous"')
            self.assertIn("--if-match", api.call_args.args)
            storage.promote(path, None)
            self.assertIn("--if-none-match", api.call_args.args)

    def test_manifest_and_configuration_validation(self):
        manifest = publisher.parse_manifest(ROOT / "internal/core/mihomo-release.json")
        self.assertEqual(len(manifest["assets"]), 6)
        for change in (
            lambda data: data.update(version="../tag"),
            lambda data: data["assets"].pop(),
            lambda data: data["assets"][0].update(name="../core.gz"),
            lambda data: data["assets"][0].update(sha256="invalid"),
            lambda data: data["assets"][1].update(os="linux", arch="amd64"),
        ):
            data = json.loads(json.dumps(manifest))
            change(data)
            path = self.directory / "manifest.json"
            path.write_text(json.dumps(data))
            with self.assertRaises(publisher.PublishError):
                publisher.parse_manifest(path)
        for tag in ("../v1.2.3", "v1.2.3-rc1", "v01.2.3", "v1.2.3\n"):
            with self.assertRaises(publisher.PublishError):
                publisher.version_tuple(tag)
        for endpoint, bucket, prefix in (
            ("http://storage.test", "public", ""),
            ("https://storage.test", "public", "../private"),
            ("https://storage.test", "../public", ""),
        ):
            with self.assertRaises(publisher.PublishError):
                publisher.Storage(endpoint, bucket, prefix)


class GatheringTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.manifest = publisher.parse_manifest(ROOT / "internal/core/mihomo-release.json")
        self.core_bytes = b"upstream archive"
        digest = hashlib.sha256(self.core_bytes).hexdigest()
        for asset in self.manifest["assets"]:
            asset["sha256"] = digest
        names = {"install.sh", "install.ps1"}
        for system, arch in publisher.PLATFORMS:
            extension = ".zip" if system == "windows" else ".tar.gz"
            name = f"miocli_1.2.3_{system}_{arch}{extension}"
            names.update((name, name + ".sha256"))
        self.release = {"tag_name": "v1.2.3", "draft": False, "prerelease": False,
                        "assets": [{"name": name} for name in names]}
        self.bad_checksum = False
        self.bad_core = False

    def command(self, *args):
        if args[:2] == ("gh", "api"):
            if "/license?" in args[2]:
                return json.dumps({"encoding": "base64", "content": base64.b64encode(b"upstream license").decode()})
            if "MetaCubeX" in args[2]:
                return json.dumps({"tag_name": self.manifest["version"], "draft": False, "prerelease": False,
                                   "assets": [{"name": asset["name"], "digest": "sha256:" + asset["sha256"]}
                                              for asset in self.manifest["assets"]]})
            return json.dumps(self.release)
        destination = Path(args[args.index("--dir") + 1])
        names = [args[index + 1] for index, argument in enumerate(args) if argument == "--pattern"]
        for name in names:
            path = destination / name
            if "MetaCubeX/mihomo" in args:
                path.write_bytes(b"corrupt" if self.bad_core else self.core_bytes)
            elif name.endswith(".sha256"):
                archive_name = name.removesuffix(".sha256")
                digest = hashlib.sha256(archive_name.encode()).hexdigest()
                if self.bad_checksum:
                    digest = "0" * 64
                path.write_text(digest + "  " + archive_name + "\n")
            else:
                path.write_text(name)
        return ""

    def gather(self):
        with patch.object(publisher, "run", side_effect=self.command):
            return publisher.gather("v1.2.3", "example/miocli", self.directory, self.manifest)

    def test_complete_release(self):
        objects = self.gather()
        self.assertEqual(len(objects), 22)
        source = objects[f'mihomo/releases/{self.manifest["version"]}/SOURCE.txt'].read_text()
        self.assertIn(self.manifest["version"] + ".tar.gz", source)

    def test_incomplete_release(self):
        self.release["assets"] = []
        with self.assertRaisesRegex(publisher.PublishError, "missing"):
            self.gather()

    def test_draft_rejected(self):
        self.release["draft"] = True
        with self.assertRaisesRegex(publisher.PublishError, "formally published"):
            self.gather()

    def test_bad_cli_checksum(self):
        self.bad_checksum = True
        with self.assertRaisesRegex(publisher.PublishError, "checksum"):
            self.gather()

    def test_bad_core_checksum(self):
        self.bad_core = True
        with self.assertRaisesRegex(publisher.PublishError, "checksum"):
            self.gather()


if __name__ == "__main__":
    unittest.main()

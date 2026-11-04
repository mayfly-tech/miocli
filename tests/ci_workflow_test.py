from pathlib import Path
import hashlib
import json
import os
import re
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import zipfile


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


def workflow_script(workflow, name):
    step = workflow.split("      - name: " + name + "\n", 1)[1]
    script = step.split("        run: |\n", 1)[1]
    lines = []
    for line in script.splitlines():
        if line and not line.startswith("          "):
            break
        lines.append(line[10:])
    return "\n".join(lines)


class ReleaseWorkflowTests(unittest.TestCase):
    def setUp(self):
        self.workflow = (ROOT / ".github/workflows/release.yml").read_text()

    def job(self, name):
        return re.search(r"^  " + re.escape(name) + r":\n(.*?)(?=^  [\w-]+:\n|\Z)",
                         self.workflow, re.MULTILINE | re.DOTALL)[1]

    def run_script(self, name, root, env):
        return subprocess.run(["bash", "-e", "-o", "pipefail", "-c", workflow_script(self.workflow, name)],
                              cwd=root, env=dict(os.environ) | env, capture_output=True, text=True)

    def test_release_please_configuration(self):
        config = json.loads((ROOT / "release-please-config.json").read_text())
        manifest = json.loads((ROOT / ".release-please-manifest.json").read_text())
        self.assertEqual(manifest, {".": "0.1.1"})
        self.assertEqual(config["packages"], {
            ".": {"release-type": "go", "include-component-in-tag": False, "include-v-in-tag": True}
        })
        self.assertRegex(config["bootstrap-sha"], r"^[0-9a-f]{40}$")
        for option in ("release-as", "skip-github-release", "draft", "prerelease"):
            self.assertNotIn(option, config)
            self.assertNotIn(option, config["packages"]["."])
        self.assertIn("on:\n  push:\n    branches:\n      - main\n", self.workflow)
        self.assertNotIn("    tags:", self.workflow)
        release = self.job("release-please")
        self.assertIn("uses: googleapis/release-please-action@v4", release)
        self.assertIn("config-file: release-please-config.json", release)
        self.assertIn("manifest-file: .release-please-manifest.json", release)
        for permission in ("contents", "issues", "pull-requests"):
            self.assertIn(permission + ": write", release)
        for output in ("release_created", "tag_name", "sha"):
            self.assertIn(output + ": ${{ steps.release.outputs." + output + " }}", release)
        self.assertNotIn("token:", release)
        self.assertIn("concurrency:\n  group: release\n", self.workflow)
        self.assertIn("cancel-in-progress: false", self.workflow)

    def test_release_jobs_use_exact_commit_and_ci_gate(self):
        preflight = self.job("preflight")
        self.assertIn("needs: release-please", preflight)
        self.assertIn("if: needs.release-please.outputs.release_created == 'true'", preflight)
        self.assertIn("uses: ./.github/workflows/ci.yml", preflight)
        self.assertIn("ref: ${{ needs.release-please.outputs.sha }}", preflight)
        ci = (ROOT / ".github/workflows/ci.yml").read_text()
        self.assertIn("workflow_call:\n    inputs:\n      ref:", ci)
        self.assertEqual(ci.count("ref: ${{ inputs.ref || github.sha }}"), ci.count("uses: actions/checkout@"))
        for name in ("build", "installers"):
            job = self.job(name)
            self.assertIn("needs: [release-please, preflight]", job)
            self.assertIn("ref: ${{ needs.release-please.outputs.sha }}", job)
            self.assertNotIn("contents: write", job)
        build = self.job("build")
        self.assertIn("COMMIT: ${{ needs.release-please.outputs.sha }}", build)
        self.assertIn("RELEASE_TAG: ${{ needs.release-please.outputs.tag_name }}", build)
        targets = re.findall(r"- goos: (\w+)\n\s+goarch: (\w+)\n\s+extension: (.+)\n\s+archive: (.+)", build)
        self.assertEqual(targets, [
            (system, arch, '".exe"' if system == "windows" else '""', "zip" if system == "windows" else "tar.gz")
            for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")
        ])
        upload = self.job("upload")
        self.assertIn("needs: [release-please, build, installers]", upload)
        self.assertIn("contents: write", upload)
        self.assertIn("pattern: release-*", upload)
        self.assertIn("working-directory: release-assets", upload)
        self.assertIn("RELEASE_TAG: ${{ needs.release-please.outputs.tag_name }}", upload)
        for removed in ("COS", "cos-action", "S3_", "s3-release", "stable.txt", "action-gh-release", "github.ref_name"):
            self.assertNotIn(removed, self.workflow)
        self.assertNotIn("continue-on-error", self.workflow)
        for pattern in ("*.tar.gz", "*.zip", "*.sha256"):
            self.assertIn("            dist/" + pattern + "\n", self.workflow)

    def test_installer_repository_configuration(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("install.sh", "install.ps1"):
                shutil.copyfile(ROOT / name, root / name)
            result = self.run_script("Configure installer repository", root, {"GITHUB_REPOSITORY": "example/miocli"})
            self.assertEqual(result.returncode, 0, result.stderr)
            for name in ("install.sh", "install.ps1"):
                configured = (root / "dist" / name).read_text()
                self.assertIn("example/miocli", configured)
                self.assertNotIn("__MIOCLI_REPOSITORY__", configured)
                for removed in ("S3", "s3Root", "Test-AvailabilityFailure"):
                    self.assertNotIn(removed, configured)

    def test_build_packages_metadata_licenses_and_checksums(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tools = root / "bin"
            tools.mkdir()
            for name in ("README.md", "README_EN.md", "LICENSE"):
                shutil.copyfile(ROOT / name, root / name)
            go = tools / "go"
            go.write_text(
                '#!/bin/sh\nset -eu\n'
                'if [ "$1" = env ] && [ "$2" = GOPATH ]; then printf "%s\\n" "$MOCK_GOPATH"; exit 0; fi\n'
                '[ "$1" = build ]\nprintf "%s\\n" "$@" > "$BUILD_LOG"\n'
                'while [ "$#" -gt 0 ]; do\n'
                '  if [ "$1" = -o ]; then printf "binary\\n" > "$2"; exit 0; fi\n'
                '  shift\n'
                'done\nexit 1\n'
            )
            go.chmod(0o700)
            licenses = tools / "go-licenses"
            licenses.write_text(
                '#!/bin/sh\nset -eu\n[ "$1" = save ]\n[ "$2" = ./cmd/miocli ]\n'
                'directory="${3#--save_path=}"\nmkdir -p "$directory"\n'
                'printf "third-party license\\n" > "$directory/LICENSE"\n'
            )
            licenses.chmod(0o700)
            env = {"PATH": str(tools) + os.pathsep + os.environ["PATH"], "MOCK_GOPATH": str(root),
                   "BUILD_LOG": str(root / "build.log"), "COMMIT": "a" * 40, "RELEASE_TAG": "v1.2.3"}
            for system in ("linux", "darwin", "windows"):
                for arch in ("amd64", "arm64"):
                    with self.subTest(system=system, arch=arch):
                        extension = "zip" if system == "windows" else "tar.gz"
                        executable = "miocli.exe" if system == "windows" else "miocli"
                        result = self.run_script("Build and package", root, env | {
                            "GOOS": system, "GOARCH": arch, "EXTENSION": ".exe" if system == "windows" else "",
                            "ARCHIVE": extension
                        })
                        self.assertEqual(result.returncode, 0, result.stderr)
                        name = f"miocli_1.2.3_{system}_{arch}"
                        archive = root / "dist" / f"{name}.{extension}"
                        if extension == "zip":
                            with zipfile.ZipFile(archive) as package:
                                members = package.namelist()
                        else:
                            with tarfile.open(archive) as package:
                                members = package.getnames()
                        for member in (executable, "README.md", "README_EN.md", "LICENSE", "THIRD_PARTY_LICENSES/LICENSE"):
                            self.assertIn(name + "/" + member, members)
                        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                        self.assertEqual(Path(str(archive) + ".sha256").read_text(), digest + "  " + archive.name + "\n")
                        log = (root / "build.log").read_text()
                        self.assertIn("miocli/internal/cli.version=1.2.3", log)
                        self.assertIn("miocli/internal/cli.commit=" + "a" * 40, log)
                        self.assertRegex(log, r"miocli/internal/cli.buildDate=\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ")
            for tag in ("main", "v1.2", "../v1.2.3"):
                result = self.run_script("Build and package", root, env | {"RELEASE_TAG": tag})
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Tag must be a semantic version", result.stdout)

    def test_verifies_all_assets_before_uploading_to_existing_release(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tools = root / "bin"
            tools.mkdir()
            gh = tools / "gh"
            gh.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$UPLOAD_LOG"\nexit "${UPLOAD_STATUS:-0}"\n')
            gh.chmod(0o700)
            for name in ("install.sh", "install.ps1"):
                (root / name).write_text(name)
            expected = ["install.sh", "install.ps1"]
            for system in ("linux", "darwin", "windows"):
                for arch in ("amd64", "arm64"):
                    extension = "zip" if system == "windows" else "tar.gz"
                    archive = root / f"miocli_1.2.3_{system}_{arch}.{extension}"
                    archive.write_bytes(b"test package")
                    checksum = root / (archive.name + ".sha256")
                    checksum.write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + "  " + archive.name + "\n")
                    expected.extend((archive.name, checksum.name))
            log = root / "upload.log"
            env = {"PATH": str(tools) + os.pathsep + os.environ["PATH"], "RELEASE_TAG": "v1.2.3",
                   "GITHUB_REPOSITORY": "example/miocli", "UPLOAD_LOG": str(log)}
            result = self.run_script("Verify and upload release assets", root, env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(len(expected), 14)
            self.assertEqual(log.read_text().splitlines(),
                             ["release", "upload", "v1.2.3"] + expected + ["--repo", "example/miocli", "--clobber"])
            result = self.run_script("Verify and upload release assets", root, env | {"UPLOAD_STATUS": "1"})
            self.assertNotEqual(result.returncode, 0)
            log.unlink()
            for tag in ("main", "v1.2", "../v1.2.3"):
                result = self.run_script("Verify and upload release assets", root, env | {"RELEASE_TAG": tag})
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(log.exists())
            for name in expected:
                asset = root / name
                original = asset.read_bytes()
                for missing in (True, False):
                    with self.subTest(asset=name, missing=missing):
                        if missing:
                            asset.unlink()
                        else:
                            asset.write_bytes(b"")
                        result = self.run_script("Verify and upload release assets", root, env)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertIn("Missing or empty release asset", result.stdout)
                        self.assertFalse(log.exists())
                        asset.write_bytes(original)
            checksum.write_text("0" * 64 + "  " + archive.name + "\n")
            result = self.run_script("Verify and upload release assets", root, env)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(log.exists())


if __name__ == "__main__":
    unittest.main()

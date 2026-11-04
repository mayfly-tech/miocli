#!/usr/bin/env python3
"""Publish verified, immutable release objects, then promote the stable entry point."""

import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import urlparse


PUBLIC_ROOT = "https://s3.usemayfly.tech/public"
VERSION = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)")
PLATFORMS = {(system, arch) for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")}


class PublishError(Exception):
    pass


def version_tuple(version):
    match = VERSION.fullmatch(version)
    if not match:
        raise PublishError("Expected a stable semantic version tag.")
    return tuple(int(part) for part in match.groups())


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def run(*args):
    result = subprocess.run(args, check=False, capture_output=True, text=True)
    if result.returncode:
        raise PublishError(f"{args[0]} failed: {result.stderr.strip()}")
    return result.stdout


def parse_manifest(path):
    release = json.loads(path.read_text())
    version_tuple(release["version"])
    seen = set()
    for asset in release["assets"]:
        platform = (asset["os"], asset["arch"])
        if platform not in PLATFORMS or platform in seen:
            raise PublishError("Invalid or duplicate pinned Mihomo platform.")
        seen.add(platform)
        extension = ".zip" if asset["os"] == "windows" else ".gz"
        prefix = f'mihomo-{asset["os"]}-{asset["arch"]}-'
        names = {prefix + release["version"] + extension}
        if asset["arch"] == "amd64":
            names.add(prefix + "v1-" + release["version"] + extension)
        if asset["name"] not in names or asset["format"] != extension:
            raise PublishError("Invalid pinned Mihomo filename or format.")
        if not re.fullmatch(r"[0-9a-f]{64}", asset["sha256"]):
            raise PublishError("Invalid pinned Mihomo digest.")
    if seen != PLATFORMS:
        raise PublishError("Pinned Mihomo release must cover all six platforms.")
    return release


class Storage:
    def __init__(self, endpoint, bucket, prefix):
        parsed = urlparse(endpoint)
        if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise PublishError("S3 endpoint must be an anonymous HTTPS endpoint URL.")
        if not re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", bucket):
            raise PublishError("Invalid S3 bucket.")
        if prefix and not re.fullmatch(r"[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*/?", prefix):
            raise PublishError("Invalid S3 key prefix.")
        self.endpoint = endpoint
        self.bucket = bucket
        self.prefix = prefix.rstrip("/")

    def api(self, operation, key, *args):
        key = "/".join(filter(None, (self.prefix, key)))
        return run("aws", "--endpoint-url", self.endpoint, "s3api", operation, "--bucket", self.bucket, "--key", key, *args)

    def head(self, key):
        try:
            return json.loads(self.api("head-object", key))
        except PublishError as error:
            if re.search(r"\((404|NoSuchKey|NotFound)\)", str(error)):
                return None
            raise

    def immutable(self, key, path):
        digest = sha256(path)
        existing = self.head(key)
        if existing is not None:
            if existing.get("Metadata", {}).get("sha256") != digest or existing.get("ContentLength") != path.stat().st_size:
                raise PublishError(f"Immutable object conflict: {key}")
            return
        self.api("put-object", key, "--body", str(path), "--if-none-match", "*",
                 "--metadata", f"sha256={digest}", "--cache-control", "public,max-age=31536000,immutable")

    def verify_public(self, key, path):
        with tempfile.TemporaryDirectory(prefix="miocli-verify-") as directory:
            downloaded = Path(directory) / "object"
            run("curl", "--fail", "--silent", "--show-error", "--proto", "=https",
                "--connect-timeout", "15", "--max-time", "180",
                PUBLIC_ROOT + "/" + key, "--output", str(downloaded))
            if sha256(downloaded) != sha256(path):
                raise PublishError(f"Public download verification failed: {key}")

    def stable(self, directory):
        existing = self.head("miocli/stable.txt")
        if existing is None:
            return None, None
        path = directory / "previous-stable.txt"
        response = json.loads(self.api("get-object", "miocli/stable.txt", str(path)))
        previous = path.read_text().rstrip("\r\n")
        version_tuple(previous)
        return previous, response["ETag"]

    def alias(self, key, path):
        self.api("put-object", key, "--body", str(path), "--cache-control", "no-cache,max-age=0,must-revalidate",
                 "--content-type", "text/plain")

    def promote(self, path, etag):
        condition = ("--if-match", etag) if etag else ("--if-none-match", "*")
        self.api("put-object", "miocli/stable.txt", "--body", str(path), *condition,
                 "--cache-control", "no-cache,max-age=0,must-revalidate", "--content-type", "text/plain")


def gather(tag, repository, directory, manifest):
    release = json.loads(run("gh", "api", f"repos/{repository}/releases/tags/{tag}"))
    if release["tag_name"] != tag or release["draft"] or release["prerelease"]:
        raise PublishError("Only formally published stable releases can be synchronized.")
    expected = {"install.sh", "install.ps1"}
    for system, arch in PLATFORMS:
        extension = ".zip" if system == "windows" else ".tar.gz"
        name = f"miocli_{tag[1:]}_{system}_{arch}{extension}"
        expected.update((name, name + ".sha256"))
    available = {asset["name"] for asset in release["assets"]}
    if not expected.issubset(available):
        raise PublishError("Release is missing installers, archives, or checksums.")
    cli_dir = directory / "cli"
    cli_dir.mkdir()
    patterns = [argument for name in sorted(expected) for argument in ("--pattern", name)]
    run("gh", "release", "download", tag, "--repo", repository, "--dir", str(cli_dir), *patterns)
    objects = {}
    for name in sorted(expected):
        path = cli_dir / name
        if not path.is_file() or path.stat().st_size == 0:
            raise PublishError(f"Missing or empty release asset: {name}")
        if name.endswith(".sha256"):
            match = re.fullmatch(r"([0-9a-fA-F]{64})[ \t]+(\S+)\r?\n?", path.read_text())
            if not match or match[2] != name.removesuffix(".sha256") or sha256(cli_dir / match[2]) != match[1].lower():
                raise PublishError(f"Invalid miocli archive checksum: {name}")
        if name in ("install.sh", "install.ps1") and b"__MIOCLI_REPOSITORY__" in path.read_bytes():
            raise PublishError("Release installer repository is not configured.")
        objects[f"miocli/releases/{tag}/{name}"] = path
    core_dir = directory / "mihomo"
    core_dir.mkdir()
    core_tag = manifest["version"]
    core_release = json.loads(run("gh", "api", f"repos/MetaCubeX/mihomo/releases/tags/{core_tag}"))
    if core_release["draft"] or core_release["prerelease"] or core_release["tag_name"] != core_tag:
        raise PublishError("Pinned Mihomo is not an official stable release.")
    assets = {asset["name"]: asset for asset in core_release["assets"]}
    for asset in manifest["assets"]:
        name = asset["name"]
        if name not in assets or assets[name].get("digest") != "sha256:" + asset["sha256"]:
            raise PublishError(f"Official Mihomo digest does not match pinned manifest: {name}")
        run("gh", "release", "download", core_tag, "--repo", "MetaCubeX/mihomo",
            "--pattern", name, "--dir", str(core_dir))
        path = core_dir / name
        if sha256(path) != asset["sha256"]:
            raise PublishError(f"Mihomo archive checksum mismatch: {name}")
        objects[f"mihomo/releases/{core_tag}/{name}"] = path
    license_info = json.loads(run("gh", "api", f"repos/MetaCubeX/mihomo/license?ref={core_tag}"))
    if license_info.get("encoding") != "base64":
        raise PublishError("Cannot obtain upstream Mihomo license.")
    license_path = core_dir / "LICENSE"
    license_path.write_bytes(base64.b64decode(license_info["content"]))
    if not license_path.stat().st_size:
        raise PublishError("Empty Mihomo license.")
    objects[f"mihomo/releases/{core_tag}/LICENSE"] = license_path
    source_path = core_dir / "SOURCE.txt"
    source_path.write_text(
        f"Unmodified official Mihomo {core_tag} binaries; see LICENSE for GPL-3.0 terms.\n"
        f"Corresponding upstream source: https://github.com/MetaCubeX/mihomo/tree/{core_tag}\n"
        f"Source archive: https://github.com/MetaCubeX/mihomo/archive/refs/tags/{core_tag}.tar.gz\n"
    )
    objects[f"mihomo/releases/{core_tag}/SOURCE.txt"] = source_path
    return objects


def publish(storage, tag, directory, objects):
    version_tuple(tag)
    for key, path in objects.items():
        storage.immutable(key, path)
    for key, path in objects.items():
        storage.verify_public(key, path)
    previous, etag = storage.stable(directory)
    if previous is not None and version_tuple(previous) >= version_tuple(tag):
        if previous == tag:
            for name in ("install.sh", "install.ps1"):
                storage.verify_public("miocli/" + name, objects[f"miocli/releases/{tag}/{name}"])
        pointer = directory / "stable.txt"
        pointer.write_text(previous + "\n")
        storage.verify_public("miocli/stable.txt", pointer)
        print(f"Verified {tag}; stable remains {previous}.")
        return
    for name in ("install.sh", "install.ps1"):
        path = objects[f"miocli/releases/{tag}/{name}"]
        storage.alias("miocli/" + name, path)
        storage.verify_public("miocli/" + name, path)
    pointer = directory / "stable.txt"
    pointer.write_text(tag + "\n")
    storage.promote(pointer, etag)
    storage.verify_public("miocli/stable.txt", pointer)
    print(f"Published and publicly verified {tag} at {PUBLIC_ROOT}/miocli.")


def main():
    tag = os.environ["RELEASE_TAG"]
    version_tuple(tag)
    repository = os.environ["GITHUB_REPOSITORY"]
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise PublishError("Invalid GitHub repository.")
    manifest = parse_manifest(Path(__file__).resolve().parents[1] / "internal/core/mihomo-release.json")
    storage = Storage(os.environ["S3_ENDPOINT"], os.environ["S3_BUCKET"], os.environ.get("S3_KEY_PREFIX", ""))
    with tempfile.TemporaryDirectory(prefix="miocli-publish-") as temporary:
        directory = Path(temporary)
        objects = gather(tag, repository, directory, manifest)
        publish(storage, tag, directory, objects)


if __name__ == "__main__":
    try:
        main()
    except (PublishError, KeyError, ValueError, OSError) as error:
        raise SystemExit(f"miocli S3 publication failed: {error}") from error

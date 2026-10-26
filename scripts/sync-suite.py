#!/usr/bin/env python3
"""Sync the canonical C++ sample suite and pin every vendored file hash."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent


def files(directory):
    return sorted(path for path in directory.rglob("*")
                  if path.is_file() and "__pycache__" not in path.parts and path.suffix != ".pyc")


def hashes(directory):
    return {path.relative_to(directory).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in files(directory)}


def check():
    lock = json.loads((ROOT / "suite.lock").read_text(encoding="utf-8"))
    if hashes(ROOT / "sample-suite") != lock["sha256"]:
        raise SystemExit("suite drift: sync a reviewed canonical commit; do not edit vendored files")
    print("PASS canonical suite lock")


def sync(source):
    source = Path(source).resolve()
    suite = source / "sample-suite"
    if not suite.is_dir():
        raise SystemExit(f"canonical suite not found: {suite}")
    commit = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    dirty = subprocess.check_output(["git", "-C", str(source), "status", "--porcelain", "--", "sample-suite"], text=True)
    if dirty:
        raise SystemExit("canonical sample-suite has uncommitted changes; sync a reviewed clean commit")
    target = ROOT / "sample-suite"
    if target.exists():
        shutil.rmtree(target)
    target.mkdir(parents=True)
    for path in files(suite):
        dest = target / path.relative_to(suite)
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(path, dest)
    lock = {"repo": "lmctl-ai/lmcode-cpp", "commit": commit, "sha256": hashes(target)}
    (ROOT / "suite.lock").write_text(json.dumps(lock, indent=2) + "\n", encoding="utf-8", newline="\n")
    print(f"Pinned suite at {commit}")


if __name__ == "__main__":
    if len(sys.argv) == 2 and sys.argv[1] == "--check":
        check()
    elif len(sys.argv) <= 2:
        sync(sys.argv[1] if len(sys.argv) == 2 else ROOT.parent / "lmcode-cpp")
    else:
        raise SystemExit("usage: sync-suite.py [SOURCE_REPO] | --check")

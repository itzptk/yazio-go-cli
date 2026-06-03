#!/usr/bin/env python3
"""Shared semver helpers for release scripts."""

from __future__ import annotations

import os
import subprocess
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
VERSION_FILE = ROOT / "VERSION"


@dataclass(frozen=True, order=True)
class StableCore:
    major: int
    minor: int
    patch: int

    def __str__(self) -> str:
        return f"{self.major}.{self.minor}.{self.patch}"


@dataclass(frozen=True)
class BetaTag:
    base: StableCore
    beta: int


def stable_core(version: str) -> StableCore:
    core = version.split("-", 1)[0].split("+", 1)[0]
    parts = core.split(".")
    if len(parts) != 3 or not all(part.isdigit() for part in parts):
        raise ValueError(f"invalid semver core: {version}")
    return StableCore(*(int(part) for part in parts))


def has_prerelease(version: str) -> bool:
    return "-" in version or "+" in version


def bump_patch(version: str) -> StableCore:
    core = stable_core(version)
    return StableCore(core.major, core.minor, core.patch + 1)


def compare_stable_cores(left: str | StableCore, right: str | StableCore) -> int:
    left_core = stable_core(left) if isinstance(left, str) else left
    right_core = stable_core(right) if isinstance(right, str) else right
    return (left_core > right_core) - (left_core < right_core)


def parse_stable_tag(tag: str) -> StableCore | None:
    if not tag.startswith("v") or "-" in tag:
        return None
    try:
        return stable_core(tag[1:])
    except ValueError:
        return None


def parse_beta_tag(tag: str) -> BetaTag | None:
    if not tag.startswith("v") or "-beta." not in tag:
        return None
    base_part, beta_part = tag[1:].split("-beta.", 1)
    if not beta_part.isdigit():
        return None
    try:
        return BetaTag(base=stable_core(base_part), beta=int(beta_part))
    except ValueError:
        return None


def list_git_tags() -> list[str]:
    output = subprocess.check_output(["git", "tag", "--list"], cwd=ROOT, text=True)
    return [line.strip() for line in output.splitlines() if line.strip()]


def find_latest_stable_tag(tags: list[str]) -> str | None:
    stable = sorted(
        ({"tag": tag, "core": parse_stable_tag(tag)} for tag in tags),
        key=lambda item: item["core"] or StableCore(0, 0, 0),
        reverse=True,
    )
    for entry in stable:
        if entry["core"] is not None:
            return entry["tag"]
    return None


def append_github_output(key: str, value: str) -> None:
    path = os.environ.get("GITHUB_OUTPUT")
    if not path:
        print(f"{key}={value}")
        return
    with open(path, "a", encoding="utf-8") as handle:
        handle.write(f"{key}={value}\n")


def read_version() -> str:
    version = VERSION_FILE.read_text(encoding="utf-8").strip()
    if not version:
        raise ValueError("VERSION file is empty")
    stable_core(version)
    return version


def write_version(version: str) -> bool:
    stable_core(version)
    current = VERSION_FILE.read_text(encoding="utf-8").strip() if VERSION_FILE.exists() else ""
    if current == version:
        return False
    VERSION_FILE.write_text(f"{version}\n", encoding="utf-8")
    return True

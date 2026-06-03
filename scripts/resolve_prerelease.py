#!/usr/bin/env python3
"""Resolve the next development prerelease tag (vX.Y.Z-beta.N)."""

from __future__ import annotations

import sys

from release_version import (
    append_github_output,
    bump_patch,
    compare_stable_cores,
    find_latest_stable_tag,
    has_prerelease,
    list_git_tags,
    parse_beta_tag,
    parse_stable_tag,
    read_version,
    stable_core,
)


def resolve_target_base(version: str, tags: list[str]) -> str:
    core = stable_core(version)

    if has_prerelease(version):
        return str(core)

    latest_stable_tag = find_latest_stable_tag(tags)
    latest_stable_core = parse_stable_tag(latest_stable_tag) if latest_stable_tag else None

    if latest_stable_core is None:
        return str(bump_patch(str(core)))

    if compare_stable_cores(core, latest_stable_core) > 0:
        return str(core)

    return str(bump_patch(str(latest_stable_core)))


def resolve_next_beta(target_base: str, tags: list[str]) -> int:
    target = stable_core(target_base)
    max_beta = 0
    for tag in tags:
        parsed = parse_beta_tag(tag)
        if parsed and parsed.base == target:
            max_beta = max(max_beta, parsed.beta)
    return max_beta + 1


def main() -> None:
    write_output = "--github-output" in sys.argv[1:]
    version = read_version()
    tags = list_git_tags()
    target_base = resolve_target_base(version, tags)
    beta = resolve_next_beta(target_base, tags)
    resolved_version = f"{target_base}-beta.{beta}"
    tag = f"v{resolved_version}"

    outputs = {
        "version": resolved_version,
        "tag": tag,
        "name": f"yazio-go-cli {resolved_version}",
        "target_base": target_base,
    }

    for key, value in outputs.items():
        if write_output:
            append_github_output(key, value)
        else:
            print(f"{key}={value}")


if __name__ == "__main__":
    main()

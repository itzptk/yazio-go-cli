#!/usr/bin/env python3
"""Resolve the previous release tag for GitHub release notes."""

from __future__ import annotations

import argparse

from release_version import compare_stable_cores, list_git_tags, parse_beta_tag, parse_stable_tag, append_github_output


def resolve_previous_beta(current_tag: str, tags: list[str]) -> str | None:
    current = parse_beta_tag(current_tag)
    if current is None:
        raise ValueError(f"invalid beta release tag: {current_tag}")

    candidates = sorted(
        (
            {"tag": tag, "parsed": parse_beta_tag(tag)}
            for tag in tags
        ),
        key=lambda entry: entry["parsed"].beta if entry["parsed"] else -1,
        reverse=True,
    )
    for entry in candidates:
        parsed = entry["parsed"]
        if parsed and parsed.base == current.base and parsed.beta < current.beta:
            return entry["tag"]
    return None


def resolve_previous_stable(current_tag: str, tags: list[str]) -> str | None:
    current_core = parse_stable_tag(current_tag)
    if current_core is None:
        raise ValueError(f"invalid stable release tag: {current_tag}")

    candidates = sorted(
        (
            {"tag": tag, "core": parse_stable_tag(tag)}
            for tag in tags
        ),
        key=lambda entry: entry["core"] or current_core,
        reverse=True,
    )
    for entry in candidates:
        core = entry["core"]
        if core and compare_stable_cores(core, current_core) < 0:
            return entry["tag"]
    return None


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--channel", choices=["beta", "stable"], required=True)
    parser.add_argument("--current-tag", required=True)
    parser.add_argument("--github-output", action="store_true")
    args = parser.parse_args()

    tags = list_git_tags()
    previous = (
        resolve_previous_beta(args.current_tag, tags)
        if args.channel == "beta"
        else resolve_previous_stable(args.current_tag, tags)
    )

    if args.github_output:
        append_github_output("previous_tag", previous or "")
    else:
        print(f"previous_tag={previous or ''}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Align VERSION to a release version."""

from __future__ import annotations

import sys

from release_version import append_github_output, stable_core, write_version


def main() -> None:
    args = [arg for arg in sys.argv[1:] if arg != "--github-output"]
    write_output = "--github-output" in sys.argv[1:]
    if len(args) != 1:
        print("Usage: update_release_version.py <version> [--github-output]", file=sys.stderr)
        raise SystemExit(1)

    version = args[0]
    stable_core(version)
    changed = write_version(version)

    if write_output:
        append_github_output("changed", "true" if changed else "false")
    elif not changed:
        print("VERSION already matches release version.")


if __name__ == "__main__":
    main()

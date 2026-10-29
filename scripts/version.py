#!/usr/bin/env python3
"""Keep release versions consistent across the Mattermost and npm manifests."""

import argparse
import json
import os
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent.parent
VERSION_PATTERN = re.compile(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?")
MANIFESTS = ("plugin.json", "webapp/package.json", "webapp/package-lock.json")


def validate_version(value):
    match = VERSION_PATTERN.fullmatch(value)
    if not match:
        raise ValueError("Use a version such as 0.1.1 or 0.2.0-rc.1, without a v prefix.")
    if match[4] and any(part.isdigit() and len(part) > 1 and part.startswith("0") for part in match[4].split(".")):
        raise ValueError("Numeric prerelease identifiers must not have leading zeroes.")
    return value


def read_manifests(root):
    return [(root / name, json.loads((root / name).read_text())) for name in MANIFESTS]


def current_version(root=ROOT, tag=None):
    manifests = read_manifests(root)
    versions = [data["version"] for _, data in manifests]
    versions.append(manifests[-1][1]["packages"][""]["version"])
    version = validate_version(versions[0])
    if any(value != version for value in versions):
        raise ValueError("Manifest versions differ; run the version command before releasing.")
    if tag is not None and tag != f"v{version}":
        raise ValueError(f"Tag must be v{version}, matching the manifests.")
    return version


def set_version(value, root=ROOT):
    validate_version(value)
    manifests = read_manifests(root)
    # Validate all input files before modifying any of them.
    if "" not in manifests[-1][1].get("packages", {}):
        raise ValueError("npm lockfile has no root package; regenerate it with the deps command.")
    for _, data in manifests:
        data["version"] = value
    manifests[-1][1]["packages"][""]["version"] = value
    for path, data in manifests:
        path.write_text(json.dumps(data, indent=2) + "\n")


def bump_version(part, root=ROOT):
    if part not in ("major", "minor", "patch"):
        raise ValueError("Choose major, minor, or patch.")
    version = current_version(root)
    if "-" in version:
        raise ValueError("Automatic releases require a stable X.Y.Z base version.")
    major, minor, patch = map(int, version.split("."))
    if part == "major":
        major, minor, patch = major + 1, 0, 0
    elif part == "minor":
        minor, patch = minor + 1, 0
    else:
        patch += 1
    value = f"{major}.{minor}.{patch}"
    set_version(value, root)
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--check", action="store_true", help="Check manifest consistency without changing files.")
    mode.add_argument("--bump", choices=("major", "minor", "patch"), help="Increment the current version and update all manifests.")
    parser.add_argument("--tag", help="Also require a matching release tag, including its v prefix.")
    args = parser.parse_args()
    try:
        if args.check:
            print(current_version(tag=args.tag))
        elif args.bump:
            if args.tag is not None:
                parser.error("--tag requires --check")
            print(bump_version(args.bump))
        else:
            if args.tag is not None:
                parser.error("--tag requires --check")
            value = os.environ.get("VERSION", "")
            set_version(value)
            print(f"Updated plugin and npm versions to {value}.")
    except (ValueError, KeyError, OSError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()

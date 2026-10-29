#!/usr/bin/env python3
"""Verify the manifest and executables inside a generated plugin archive."""

import json
import sys
import tarfile


def check(path):
    with tarfile.open(path, "r:gz") as archive:
        manifests = [member for member in archive.getmembers() if member.name.count("/") == 1 and member.name.endswith("/plugin.json")]
        if len(manifests) != 1:
            raise SystemExit("Expected one plugin manifest under the plugin ID directory")
        manifest = json.load(archive.extractfile(manifests[0]))
        plugin_id = manifest["id"]
        assert manifests[0].name == f"{plugin_id}/plugin.json"
        for executable in manifest["server"]["executables"].values():
            member = archive.getmember(f"{plugin_id}/{executable}")
            assert member.size > 0 and member.mode & 0o111
            assert archive.extractfile(member).read(4) == b"\x7fELF"
        member = archive.getmember(f"{plugin_id}/{manifest['webapp']['bundle_path']}")
        assert member.size > 0
        archive.getmember(f"{plugin_id}/licenses/dependencies/inventory.json")
        for member in archive.getmembers():
            assert member.name.startswith(f"{plugin_id}/")
            assert ".." not in member.name.split("/")
            assert member.isfile()
    print(f"Validated {path}")


if __name__ == "__main__":
    for filename in sys.argv[1:]:
        check(filename)

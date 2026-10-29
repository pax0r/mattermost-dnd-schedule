#!/usr/bin/env python3
"""Prepare checksums and installation notes for an already verified bundle."""

import argparse
import hashlib
import json
from pathlib import Path
import tarfile

from version import ROOT, current_version


def prepare_release(root=ROOT, tag=None):
    version = current_version(root, tag)
    manifest = json.loads((root / "plugin.json").read_text())
    plugin_id = manifest["id"]
    bundle = root / "dist" / f"{plugin_id}-{version}.tar.gz"
    with tarfile.open(bundle, "r:gz") as archive:
        embedded = json.load(archive.extractfile(f"{plugin_id}/plugin.json"))
    if embedded["version"] != version or embedded["id"] != plugin_id:
        raise ValueError("Bundle manifest does not match the source; rebuild before releasing.")
    checksum = hashlib.sha256(bundle.read_bytes()).hexdigest()
    checksum_path = bundle.with_name(bundle.name + ".sha256")
    checksum_path.write_text(f"{checksum}  {bundle.name}\n")
    notes = f"""## Scheduled DND {version}

Requires Mattermost {manifest['min_server_version']} or later on Linux amd64/arm64.

### Installation or upgrade

1. Download **{bundle.name}** from this release's assets. GitHub's automatic
   source-code archives are not installable plugin bundles.
2. Optionally verify it with `sha256sum -c {checksum_path.name}` after downloading
   the matching checksum file into the same directory.
3. As a system administrator, open **System Console → Plugins → Plugin Management**,
   upload the `.tar.gz` file, and enable **Scheduled DND**. Enable plugin uploads
   in server configuration if required by your deployment policy.
4. Reload the desktop/browser client, then open **main menu → Scheduled DND**.

Manual uploads require `PluginSettings.EnableUploads=true`. Deployments enforcing
`RequirePluginSignature` disable web uploads and need administrator-approved
signed distribution. The SHA-256 checksum is not a plugin signature.

The same plugin ID is retained on upgrades so user configuration remains in
plugin KV storage. Existing timed DND expires naturally during an upgrade.

The bundle passes automated Go race tests, frontend tests, TypeScript checking,
and archive validation. Live Mattermost/HA behavior requires the acceptance
walkthrough in `docs/ui.md`; automated builds do not certify that environment.
"""
    (root / "dist/release-notes.md").write_text(notes)
    return bundle, checksum_path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", help="Require this tag to match the version being packaged.")
    args = parser.parse_args()
    try:
        bundle, checksum = prepare_release(tag=args.tag)
    except (ValueError, KeyError, OSError, tarfile.TarError) as error:
        parser.error(str(error))
    print(bundle)
    print(checksum)


if __name__ == "__main__":
    main()

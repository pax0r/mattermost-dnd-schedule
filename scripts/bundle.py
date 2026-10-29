#!/usr/bin/env python3
"""Build the server targets declared in the manifest and package a plugin bundle."""

import io
import json
import os
from pathlib import Path
import subprocess
import tarfile


ROOT = Path(__file__).resolve().parent.parent


def main():
    manifest = json.loads((ROOT / "plugin.json").read_text())
    plugin_id = manifest["id"]
    version = manifest["version"]
    webapp = ROOT / manifest["webapp"]["bundle_path"]
    if not webapp.is_file():
        raise SystemExit("Missing webapp bundle; run npm run build in webapp first.")

    subprocess.run(["go", "mod", "download"], cwd=ROOT, check=True)
    for platform, executable in manifest["server"]["executables"].items():
        target_os, target_arch = platform.split("-", 1)
        output = ROOT / executable
        output.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(
            ["go", "build", "-mod=readonly", "-trimpath", "-ldflags=-s -w", "-o", str(output), "./server"],
            cwd=ROOT,
            env={**os.environ, "CGO_ENABLED": "0", "GOOS": target_os, "GOARCH": target_arch},
            check=True,
        )

    output_dir = ROOT / "dist"
    output_dir.mkdir(exist_ok=True)
    destination = output_dir / f"{plugin_id}-{version}.tar.gz"
    temporary = destination.with_suffix(".tmp")
    sources = ["plugin.json", "LICENSE", "NOTICE", "README.md", "docs/research.md", "docs/ui.md", "licenses/Apache-2.0.txt", manifest["webapp"]["bundle_path"], *manifest["server"]["executables"].values()]
    sources += [str(p.relative_to(ROOT)) for p in sorted((ROOT / "licenses/dependencies").glob("**/*")) if p.is_file()]
    if not (ROOT / "licenses/dependencies").is_dir():
        raise SystemExit("Dependency license inventory missing; run python3 scripts/licenses.py first.")
    with tarfile.open(temporary, "w:gz", format=tarfile.PAX_FORMAT) as archive:
        for source in sources:
            path = ROOT / source
            info = archive.gettarinfo(str(path), arcname=f"{plugin_id}/{source}")
            info.uid = info.gid = 0
            info.uname = info.gname = ""
            info.mtime = int(os.environ.get("SOURCE_DATE_EPOCH", "0"))
            info.mode = 0o755 if source in manifest["server"]["executables"].values() else 0o644
            archive.addfile(info, io.BytesIO(path.read_bytes()))
    temporary.replace(destination)
    print(destination)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Collect upstream notices for Go packages actually linked into the plugin."""

import json
from pathlib import Path
import shutil
import subprocess


ROOT = Path(__file__).resolve().parent.parent


def objects(text):
    decoder = json.JSONDecoder()
    remaining = text.lstrip()
    while remaining:
        value, end = decoder.raw_decode(remaining)
        yield value
        remaining = remaining[end:].lstrip()


def main():
    result = subprocess.run(["go", "list", "-mod=readonly", "-deps", "-json", "./server"], cwd=ROOT, capture_output=True, text=True, check=True)
    modules = {}
    for package in objects(result.stdout):
        module = package.get("Module")
        if module and not module.get("Main"):
            modules[module["Path"]] = module
    destination = ROOT / "licenses/dependencies"
    if destination.exists():
        shutil.rmtree(destination)
    destination.mkdir(parents=True)
    inventory = []
    for name, module in sorted(modules.items()):
        directory = Path(module["Dir"])
        target = destination / name.replace("/", "_")
        target.mkdir(parents=True, exist_ok=True)
        notices = []
        for source in directory.iterdir():
            if source.is_file() and source.name.upper().startswith(("LICENSE", "LICENCE", "COPYING", "NOTICE", "AUTHORS")):
                shutil.copyfile(source, target / source.name)
                notices.append(source.name)
        if not notices:
            raise SystemExit(f"No root license notice found for {name}; inspect upstream before distributing.")
        inventory.append({"module": name, "version": module.get("Version"), "notices": notices})
    goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
    (destination / "go").mkdir()
    for name in ("LICENSE", "PATENTS"):
        if (goroot / name).is_file():
            shutil.copyfile(goroot / name, destination / "go" / name)
    (destination / "inventory.json").write_text(json.dumps(inventory, indent=2) + "\n")


if __name__ == "__main__":
    main()

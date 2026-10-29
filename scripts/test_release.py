import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

from release import prepare_release
from version import bump_version, current_version, set_version, validate_version


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / "webapp").mkdir()
        self.write("plugin.json", {"id": "example.dnd", "version": "0.1.0", "min_server_version": "11.7.0"})
        self.write("webapp/package.json", {"version": "0.1.0", "private": True})
        self.write("webapp/package-lock.json", {"version": "0.1.0", "packages": {"": {"version": "0.1.0"}, "node_modules/example": {"version": "2.0.0"}}})

    def write(self, name, value):
        (self.root / name).write_text(json.dumps(value))

    def bundle_fixture(self, embedded_version="0.1.0"):
        # Release metadata tests need only a manifest; executable/layout checks
        # belong to check_bundle.py, which the release build runs beforehand.
        path = self.root / "dist/example.dnd-0.1.0.tar.gz"
        path.parent.mkdir()
        with tarfile.open(path, "w:gz") as archive:
            payload = json.dumps({"id": "example.dnd", "version": embedded_version}).encode()
            member = tarfile.TarInfo("example.dnd/plugin.json")
            member.size = len(payload)
            archive.addfile(member, io.BytesIO(payload))
        return path

    def test_version_updates_all_roots_without_changing_dependency_versions(self):
        set_version("0.2.0-rc.1", self.root)
        self.assertEqual(current_version(self.root, "v0.2.0-rc.1"), "0.2.0-rc.1")
        lock = json.loads((self.root / "webapp/package-lock.json").read_text())
        self.assertEqual(lock["packages"]["node_modules/example"]["version"], "2.0.0")
        self.assertTrue(json.loads((self.root / "webapp/package.json").read_text())["private"])

    def test_invalid_version_does_not_modify_files(self):
        original = (self.root / "plugin.json").read_bytes()
        for value in ("", "v1.2.3", "1.2", "01.2.3", "1.2.3-rc.01", "1.2.3/../../x", "1.2.3\n"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                set_version(value, self.root)
        self.assertEqual((self.root / "plugin.json").read_bytes(), original)
        self.assertEqual(validate_version("1.2.3-rc.0"), "1.2.3-rc.0")

    def test_inconsistent_versions_and_wrong_tags_are_rejected(self):
        with self.assertRaises(ValueError):
            current_version(self.root, "v0.2.0")
        self.write("webapp/package.json", {"version": "0.2.0"})
        with self.assertRaises(ValueError):
            current_version(self.root)

    def test_release_checksum_and_installation_notes(self):
        path = self.bundle_fixture()
        bundle, checksum = prepare_release(self.root, "v0.1.0")
        self.assertEqual(bundle, path)
        self.assertEqual(checksum.read_text(), f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n")
        notes = (self.root / "dist/release-notes.md").read_text()
        self.assertIn(path.name, notes)
        self.assertIn("11.7.0", notes)
        self.assertIn("source-code archives are not installable", notes)

    def test_old_embedded_bundle_version_is_rejected(self):
        self.bundle_fixture("0.0.9")
        with self.assertRaises(ValueError):
            prepare_release(self.root)
        self.assertFalse((self.root / "dist/release-notes.md").exists())

    def test_first_release_without_any_prior_tag_or_release(self):
        for part, expected in (("patch", "0.0.1"), ("minor", "0.1.0"), ("major", "1.0.0")):
            with self.subTest(part=part):
                set_version("0.0.0", self.root)
                self.assertEqual(bump_version(part, self.root), expected)
                self.assertEqual(current_version(self.root), expected)

    def test_bump_resets_lower_components_and_preserves_next_release_base(self):
        for part, expected in (("patch", "1.2.4"), ("minor", "1.3.0"), ("major", "2.0.0")):
            with self.subTest(part=part):
                set_version("1.2.3", self.root)
                self.assertEqual(bump_version(part, self.root), expected)
        set_version("0.0.0", self.root)
        self.assertEqual(bump_version("patch", self.root), "0.0.1")
        self.assertEqual(bump_version("patch", self.root), "0.0.2")

    def test_invalid_bump_or_prerelease_base_does_not_change_manifests(self):
        with self.assertRaises(ValueError):
            bump_version("invalid", self.root)
        self.assertEqual(current_version(self.root), "0.1.0")
        set_version("1.0.0-rc.1", self.root)
        with self.assertRaises(ValueError):
            bump_version("patch", self.root)
        self.assertEqual(current_version(self.root), "1.0.0-rc.1")


if __name__ == "__main__":
    unittest.main()

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from version import set_version


SCRIPT = Path(__file__).with_name("commit-release.sh").resolve()


class ReleaseGitTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.remote = self.root / "remote.git"
        self.work = self.root / "work"
        subprocess.run(["git", "init", "--bare", "-q", str(self.remote)], check=True)
        subprocess.run(["git", "init", "-q", "-b", "main", str(self.work)], check=True)
        self.git("config", "user.name", "Test developer")
        self.git("config", "user.email", "test@example.invalid")
        self.git("remote", "add", "origin", str(self.remote))
        (self.work / "webapp").mkdir()
        for name in ("plugin.json", "webapp/package.json"):
            (self.work / name).write_text(json.dumps({"version": "0.0.0"}))
        (self.work / "webapp/package-lock.json").write_text(json.dumps({"version": "0.0.0", "packages": {"": {"version": "0.0.0"}}}))
        (self.work / "source.txt").write_text("tested source")
        self.git("add", ".")
        self.git("commit", "-qm", "Initial source; no release exists")
        self.git("push", "-q", "origin", "main")
        self.base = self.git("rev-parse", "HEAD").stdout.strip()
        self.output = self.root / "output"
        set_version("0.0.1", self.work)

    def git(self, *args, cwd=None, check=True):
        return subprocess.run(["git", *args], cwd=cwd or self.work, text=True, capture_output=True, check=check)

    def commit_release(self):
        return subprocess.run(["bash", str(SCRIPT)], cwd=self.work, text=True, capture_output=True,
                              env={**os.environ, "RELEASE_BRANCH": "main", "RELEASE_TAG": "v0.0.1",
                                   "BASE_SHA": self.base, "GITHUB_OUTPUT": str(self.output)})

    def test_first_release_pushes_matching_commit_and_tag(self):
        result = self.commit_release()
        self.assertEqual(result.returncode, 0, result.stderr)
        head = self.git("rev-parse", "HEAD").stdout.strip()
        self.assertNotEqual(head, self.base)
        self.assertEqual(self.git("rev-parse", "HEAD^").stdout.strip(), self.base)
        for ref in ("refs/heads/main", "refs/tags/v0.0.1"):
            self.assertEqual(self.git("ls-remote", "origin", ref).stdout.split()[0], head)
        self.assertIn(f"commit={head}", self.output.read_text())

    def test_retry_reuses_matching_remote_tag_without_another_commit(self):
        self.assertEqual(self.commit_release().returncode, 0)
        released = self.git("rev-parse", "HEAD").stdout.strip()
        self.git("reset", "--hard", self.base)
        set_version("0.0.1", self.work)
        result = self.commit_release()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("rev-parse", "HEAD").stdout.strip(), self.base)
        self.assertIn(f"commit={released}", self.output.read_text())

    def test_branch_race_rejects_both_branch_and_tag_updates(self):
        other = self.root / "other"
        subprocess.run(["git", "clone", "-q", "--branch", "main", str(self.remote), str(other)], check=True)
        self.git("config", "user.name", "Another developer", cwd=other)
        self.git("config", "user.email", "other@example.invalid", cwd=other)
        (other / "source.txt").write_text("new source pushed during build")
        self.git("add", "source.txt", cwd=other)
        self.git("commit", "-qm", "Concurrent update", cwd=other)
        self.git("push", "-q", "origin", "main", cwd=other)
        advanced = self.git("rev-parse", "HEAD", cwd=other).stdout.strip()
        result = self.commit_release()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("ls-remote", "origin", "refs/heads/main").stdout.split()[0], advanced)
        self.assertEqual(self.git("ls-remote", "origin", "refs/tags/v0.0.1").stdout, "")
        # Even with the failed attempt's local tag, a retry cannot claim that
        # an unpublished tag exists remotely and proceed to publication.
        self.git("reset", "--hard", self.base)
        set_version("0.0.1", self.work)
        self.assertNotEqual(self.commit_release().returncode, 0)

    def test_colliding_tag_is_never_moved(self):
        self.git("tag", "v0.0.1", self.base)
        self.git("push", "-q", "origin", "refs/tags/v0.0.1")
        result = self.commit_release()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("ls-remote", "origin", "refs/tags/v0.0.1").stdout.split()[0], self.base)
        self.assertEqual(self.git("ls-remote", "origin", "refs/heads/main").stdout.split()[0], self.base)


if __name__ == "__main__":
    unittest.main()

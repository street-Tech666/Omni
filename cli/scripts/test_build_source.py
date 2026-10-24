"""Tests unsigned staging guards; compiler is mocked, native builds are separate."""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile
from build_source import build

class StagingTests(unittest.TestCase):
    def test_existing_destination_is_never_changed(self):
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp)
            keep = out / "personal.txt"
            keep.write_text("preserve")
            with self.assertRaises(ValueError):
                build(out, "unused")
            self.assertEqual(keep.read_text(), "preserve")
            self.assertEqual(list(out.iterdir()), [keep])

    def test_failed_compile_remains_explicitly_incomplete(self):
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / "staging"
            with patch("build_source.subprocess.run", side_effect=subprocess.CalledProcessError(1, "go")):
                with self.assertRaises(subprocess.CalledProcessError):
                    build(out, "unused")
            self.assertTrue((out / "INCOMPLETE.txt").is_file())
            self.assertFalse((out / "source-manifest.json").exists())

    def test_exact_six_unsigned_targets_and_digests(self):
        def compiler(args, **kwargs):
            Path(args[args.index("-o")+1]).write_bytes(b"mock-source-binary")
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / "staging"
            with patch("build_source.subprocess.run", side_effect=compiler):
                build(out, "unused")
            manifest = json.loads((out / "source-manifest.json").read_text())
            self.assertFalse(manifest["signed"])
            self.assertFalse(manifest["published"])
            self.assertEqual({a["target"] for a in manifest["artifacts"]}, {s+"/"+a for s in ["darwin","linux","windows"] for a in ["amd64","arm64"]})
            for item in manifest["artifacts"]:
                archive = out / item["file"]
                self.assertEqual(item["sha256"], hashlib.sha256(archive.read_bytes()).hexdigest())
                filename = "evolution.exe" if item["target"].startswith("windows/") else "evolution"
                with zipfile.ZipFile(archive) as z:
                    self.assertEqual(set(z.namelist()), {filename, "README.md", "UNSIGNED.txt"})
            self.assertFalse((out / "INCOMPLETE.txt").exists())

if __name__ == "__main__":
    unittest.main()
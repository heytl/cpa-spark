import sys
import unittest
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from validate_tag import validate
from release_artifacts import source_version
from release_notes import render


class ReleaseWorkflowTests(unittest.TestCase):
    def test_tag_must_be_numeric_and_match_source(self):
        self.assertEqual(validate('v' + source_version()), 'v' + source_version())
        for bad in ('0.1.2', 'v01.1.2', 'v0.1.2-beta', 'v999.0.0', 'v0.1.2\nmalicious=value'):
            with self.assertRaises(ValueError):
                validate(bad)

    def test_notes_and_store_pr_use_actual_identity_and_all_assets(self):
        notes, pr = render('fixture-owner/cpa-spark')
        for suffix in ('linux_amd64', 'linux_arm64', 'darwin_amd64', 'darwin_arm64', 'windows_amd64'):
            self.assertIn(f'cpa-spark_{source_version()}_{suffix}.zip', notes)
            self.assertIn(f'cpa-spark_{source_version()}_{suffix}.zip', pr)
        self.assertIn('fixture-owner/cpa-spark/releases/latest/download/registry.json', notes)
        self.assertIn('/releases/tag/v' + source_version(), pr)
        self.assertIn('checksums.txt', pr)
        self.assertNotIn('OWNER', notes)

    def test_invalid_repository_rejected(self):
        for repo in ('', 'https://github.com/owner/repo', 'owner/repo/extra', 'owner/repo\ntext'):
            with self.assertRaises(ValueError):
                render(repo)


if __name__ == '__main__':
    unittest.main()

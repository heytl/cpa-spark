import sys
import unittest
import re
import tempfile
import subprocess
import json
from unittest.mock import patch
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from validate_tag import validate
from release_artifacts import source_version
from release_notes import render
from release_notes import changelog_section
from build_metadata import metadata
import release_notes


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
        self.assertIn('## 中文', notes)
        self.assertIn('## English', notes)
        self.assertIn('/README.en.md', notes)
        self.assertIn('/releases/latest)', notes)
        self.assertIn('/releases)', notes)
        self.assertNotIn('## 0.1.6', notes)

    def test_development_names_do_not_change_plugin_version(self):
        values = metadata('', 'a' * 40)
        self.assertEqual(values['version'], source_version())
        self.assertEqual(values['artifact_version'], source_version() + '-dev.aaaaaaa')
        self.assertEqual(metadata('v' + source_version(), '')['artifact_version'], source_version())
        for sha in ('', 'short', '../unsafe', 'a' * 40 + '\n'):
            with self.assertRaises(ValueError):
                metadata('', sha)

    def test_both_changelog_entries_are_required(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'CHANGELOG.md').write_text(f'## {source_version()}\n\n- 中文修复\n', encoding='utf-8')
            with patch.object(release_notes, 'ROOT', root):
                with self.assertRaisesRegex(ValueError, 'CHANGELOG.en.md'):
                    render('owner/repo')
                (root / 'CHANGELOG.en.md').write_text('## 99.0.0\n- Wrong version\n', encoding='utf-8')
                with self.assertRaisesRegex(ValueError, 'CHANGELOG.en.md'):
                    render('owner/repo')

    def test_empty_and_duplicate_version_entries_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'CHANGELOG.en.md'
            for content in ('## 1.2.3\n\n## 1.2.4\n- Other\n',
                            '## 1.2.3\n- First\n## 1.2.3\n- Duplicate\n'):
                path.write_text(content, encoding='utf-8')
                with self.assertRaises(ValueError):
                    changelog_section(path, '1.2.3')

    def test_bilingual_documentation_stays_linked_and_versioned(self):
        root = Path(__file__).resolve().parents[1]
        zh = (root / 'README.md').read_text(encoding='utf-8')
        en = (root / 'README.en.md').read_text(encoding='utf-8')
        self.assertIn('(README.en.md)', zh)
        self.assertIn('(README.md)', en)
        self.assertIn(source_version(), zh)
        self.assertIn(source_version(), en)
        versions = []
        for name in ('CHANGELOG.md', 'CHANGELOG.en.md'):
            content = (root / name).read_text(encoding='utf-8')
            found = re.findall(r'^## (\d+\.\d+\.\d+(?:-[A-Za-z0-9.]+)?)(?=\s|$)', content, re.M)
            self.assertEqual(len(found), len(set(found)), name)
            versions.append(set(found))
        self.assertEqual(versions[0], versions[1])

    def test_store_logo_uses_the_actual_repository_asset(self):
        root = Path(__file__).resolve().parents[1]
        command = [sys.executable, str(root / 'scripts/store_registry.py'),
                   '--repository', 'https://github.com/fixture-owner/cpa-spark', '--author', 'fixture-owner']
        entry = json.loads(subprocess.check_output(command))['plugins'][0]
        self.assertEqual(entry['logo'], 'https://raw.githubusercontent.com/fixture-owner/cpa-spark/main/logo.svg')
        self.assertEqual((root / 'logo.svg').read_bytes(), (root / 'web/logo.svg').read_bytes())
        overridden = json.loads(subprocess.check_output(command + ['--logo', 'https://example.com/brand.svg']))['plugins'][0]
        self.assertEqual(overridden['logo'], 'https://example.com/brand.svg')

    def test_invalid_repository_rejected(self):
        for repo in ('', 'https://github.com/owner/repo', 'owner/repo/extra', 'owner/repo\ntext'):
            with self.assertRaises(ValueError):
                render(repo)


if __name__ == '__main__':
    unittest.main()

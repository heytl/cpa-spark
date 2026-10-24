"""Generated frontend bytes must match on Windows as well as Unix."""
import contextlib
import io
import os
from pathlib import Path
import runpy
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


class WebBuildTests(unittest.TestCase):
    def test_windows_newlines_produce_the_committed_assets(self):
        generated = {}

        def windows_write(path, data, encoding=None, errors=None, newline=None):
            # TextIOWrapper translates bare newlines to CRLF by default on Windows.
            effective_newline = '\r\n' if newline is None else newline
            translated = data.replace('\n', effective_newline or '\n')
            generated[path.as_posix()] = translated.encode(encoding or 'utf-8')
            return len(data)

        def write_bytes(path, data):
            generated[path.as_posix()] = data
            return len(data)

        previous = Path.cwd()
        try:
            os.chdir(ROOT)
            with patch.object(Path, 'write_text', windows_write), \
                    patch.object(Path, 'write_bytes', write_bytes), \
                    patch('sys.argv', ['scripts/build_web.py']), \
                    patch('subprocess.run'), contextlib.redirect_stdout(io.StringIO()):
                runpy.run_path(str(ROOT / 'scripts/build_web.py'), run_name='__main__')
        finally:
            os.chdir(previous)
        self.assertEqual(set(generated), {
            'web/index.html', 'web/app.js', 'web/style.css', 'web/third-party-notices.txt', 'web/logo.svg', 'web/sidebar-icon.svg'
        })
        for name, content in generated.items():
            self.assertEqual(content, (ROOT / name).read_bytes(), name)


if __name__ == '__main__':
    unittest.main()

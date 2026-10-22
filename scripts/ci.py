"""Cross-platform CI entry point; fake hosts only, no OAuth/model requests."""
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GENERATED = ['web/index.html', 'web/app.js', 'web/style.css', 'web/third-party-notices.txt']

def run(args, env=None):
    subprocess.run(args, cwd=ROOT, env=env, check=True)

def main():
    before = {p: (ROOT / p).read_bytes() for p in GENERATED}
    run([sys.executable, 'scripts/build_web.py'])
    changed = [p for p in GENERATED if (ROOT / p).read_bytes() != before[p]]
    if changed:
        raise SystemExit('Commit regenerated frontend assets: ' + ', '.join(changed))
    run([os.environ.get('GO', 'go'), 'test', '-race', '-timeout', '120s', './...'])
    run(['node', '--check', 'web/app.js'])
    run(['node', '--test', 'tests/cpa_session.test.cjs', 'tests/task_controls.test.cjs', 'tests/host_theme.test.cjs', 'tests/i18n.test.cjs', 'tests/account_timeline.test.cjs', 'tests/model_catalog.test.cjs'])
    run(['node', 'tests/editor_dom.cjs'])
    run(['node', 'tests/editor_dom.cjs'], {**os.environ, 'CPA_QA_UNAVAILABLE': '1'})
    run([sys.executable, '-m', 'unittest', 'discover', '-s', 'tests', '-p', 'test_*.py'])

if __name__ == '__main__':
    main()

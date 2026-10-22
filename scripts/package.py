"""Package tracked source only. Native libraries belong in platform release ZIPs."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess
import zipfile
from release_artifacts import ROOT, source_version


def package(output):
    listing = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT)
    files = [Path(name) for name in listing.decode('utf-8').split('\0') if name]
    if not files:
        raise ValueError('Stage or commit source files before packaging')
    forbidden_parts = {'.git', 'node_modules', 'dist', 'release', '.build', 'data', 'plugins', '__pycache__'}
    forbidden_suffixes = {'.so', '.dll', '.dylib', '.db', '.pyc', '.log', '.zip'}
    for rel in files:
        if any(p in forbidden_parts for p in rel.parts) or rel.suffix in forbidden_suffixes or rel.name.startswith('.env') and rel.name != '.env.example':
            raise ValueError('Untrack runtime or generated file before packaging: ' + str(rel))
    output = Path(output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    checks = []
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
        for rel in sorted(files):
            file = ROOT / rel
            if file.is_symlink() or not file.is_file():
                raise ValueError('Expected a regular source file: ' + str(rel))
            raw = file.read_bytes()
            archive.writestr('cpa-spark/' + rel.as_posix(), raw)
            checks.append(hashlib.sha256(raw).hexdigest() + '  ' + rel.as_posix())
        archive.writestr('cpa-spark/SHA256SUMS.txt', '\n'.join(checks) + '\n')
    return {'archive': str(output), 'files': len(files), 'bytes': output.stat().st_size}


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('output', nargs='?', default=str(ROOT / 'dist' / ('cpa-spark-' + source_version() + '-source.zip')))
    args = parser.parse_args()
    print(json.dumps(package(args.output)))

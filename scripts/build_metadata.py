"""Resolve source and downloadable artifact versions without changing source files."""
import argparse
from pathlib import Path
import re
from release_artifacts import source_version
from validate_tag import validate


def metadata(release_tag, sha):
    version = source_version()
    if release_tag:
        validate(release_tag)
        artifact_version = version
    else:
        if not re.fullmatch(r'[0-9a-f]{40}', sha):
            raise ValueError('Development builds require a full Git commit SHA')
        artifact_version = f'{version}-dev.{sha[:7]}'
    return {'version': version, 'artifact_version': artifact_version}


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--release-tag', default='')
    parser.add_argument('--sha', default='')
    parser.add_argument('--github-output', type=Path, required=True)
    args = parser.parse_args()
    try:
        values = metadata(args.release_tag, args.sha)
    except ValueError as error:
        parser.error(str(error))
    with args.github_output.open('a', encoding='utf-8', newline='\n') as output:
        for key, value in values.items():
            output.write(f'{key}={value}\n')

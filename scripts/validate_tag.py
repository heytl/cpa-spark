"""Validate the store's numeric vX.Y.Z tag against the single source version."""
import argparse
import re
from pathlib import Path
from release_artifacts import source_version


def validate(tag):
    if not re.fullmatch(r'v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)', tag):
        raise ValueError('Store releases require a stable numeric vX.Y.Z tag')
    if tag[1:] != source_version():
        raise ValueError('Release tag must match internal/warmup/types.go Version')
    return tag


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('tag')
    parser.add_argument('--github-output', type=Path)
    args = parser.parse_args()
    try:
        tag = validate(args.tag)
    except ValueError as error:
        parser.error(str(error))
    if args.github_output:
        with args.github_output.open('a', encoding='utf-8', newline='\n') as output:
            output.write('tag=' + tag + '\n')
            output.write('version=' + tag[1:] + '\n')
    print(tag)

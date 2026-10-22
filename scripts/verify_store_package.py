"""Verify checksum/layout/architecture before loading a downloaded release library."""
import argparse,re
from pathlib import Path
from release_artifacts import TARGETS,verify,source_version
p=argparse.ArgumentParser();p.add_argument('--directory',required=True);p.add_argument('--tag',required=True);p.add_argument('--target',required=True,choices=TARGETS);p.add_argument('--extract-to',required=True);a=p.parse_args()
if not re.fullmatch(r'v\d+\.\d+\.\d+',a.tag):p.error('Expected stable numeric tag vX.Y.Z')
if a.tag[1:]!=source_version():p.error('Check out the exact release tag before verification')
raw=verify(a.directory,a.tag[1:],a.target);out=Path(a.extract_to);out.mkdir(parents=True,exist_ok=True);library=out/f'cpa-spark.{TARGETS[a.target]}';library.write_bytes(raw);print(library)

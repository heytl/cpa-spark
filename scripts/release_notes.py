"""Build release notes and an official-store PR draft from actual repo identity."""
import argparse
import re
from pathlib import Path
from release_artifacts import ROOT, TARGETS, source_version
from validate_tag import validate


def changelog_section(path, version):
    try:
        changes = path.read_text(encoding='utf-8')
    except FileNotFoundError as error:
        raise ValueError(f'Missing required changelog: {path.name}') from error
    sections = re.findall(r'^## ' + re.escape(version) + r'(?:[ \t]+[^\n]*)?\n(.*?)(?=^## |\Z)', changes, re.M | re.S)
    if len(sections) != 1 or not sections[0].strip():
        raise ValueError(f'{path.name} requires exactly one nonempty {version} section')
    return sections[0].strip()


def render(repository):
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('Expected GitHub owner/repo')
    version = source_version()
    tag = validate('v' + version)
    url = 'https://github.com/' + repository
    base = url + '/releases/download/' + tag
    chinese = changelog_section(ROOT / 'CHANGELOG.md', version)
    english = changelog_section(ROOT / 'CHANGELOG.en.md', version)
    packages = '\n'.join(f'- [cpa-spark_{version}_{target}.zip]({base}/cpa-spark_{version}_{target}.zip)' for target in TARGETS)
    notes = f'''# CPA Spark {tag}

[中文](#中文) · [English](#english)

[最新稳定版 / Latest stable]({url}/releases/latest) · [全部版本 / All releases]({url}/releases) · [Tags]({url}/tags)

## 中文

{chinese}

安装：下载匹配 CPA 服务器 OS/CPU 的 ZIP，解压并导入动态库。Docker 持久化和使用方法见 [中文说明]({url}/blob/{tag}/README.md)。

每个动态库均在匹配 OS/CPU 的 runner 上构建，并通过假 CPA Host C ABI 加载测试；这不替代真实 CPA 和账号验收。

自定义商店源：`{url}/releases/latest/download/registry.json`。当前版本尚未上架官方插件商店。

## English

{english}

Installation: download the ZIP matching the CPA server's OS/CPU, extract it and import the library. See the [English README]({url}/blob/{tag}/README.en.md) for Docker persistence and usage.

Each library is built and loaded against a fake CPA Host on its matching OS/CPU. These checks do not replace validation with a real CPA instance and account.

Custom store source: `{url}/releases/latest/download/registry.json`. This plugin is not yet listed in the official store.

## 下载 / Downloads

{packages}
- [SHA-256 checksums]({base}/checksums.txt)
- [Registry]({base}/registry.json)

ZIP 内仅含一个根目录动态库。Each ZIP contains a single root-level plugin library.
'''
    pr = f'''## Add CPA Spark

Repository: {url}
Latest stable release: {url}/releases/tag/{tag}
Plugin ID: `cpa-spark`

Adds account-specific and group-specific warmup schedules, one-off warmup, execution history, and planned quota-window visualization for CLIProxyAPI. Reuses CPA host callbacks for account/model execution.

Release assets:

{packages}
- [checksums.txt]({base}/checksums.txt)
- [Registry entry]({base}/registry.json)

All five targets pass native ABI load tests and archive/checksum/architecture validation in GitHub Actions. Before opening this PR, publish the draft, wait for Verify Release packages to pass, and confirm installation on an actual CPA instance.

This file is a draft for the author. The store PR should update only registry.json with the plugin object from the generated registry entry. No binaries or checksums belong in the store repository.
'''
    return notes, pr


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--repository', required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--store-pr-output', type=Path)
    args = parser.parse_args()
    try:
        notes, pr = render(args.repository)
    except ValueError as error:
        parser.error(str(error))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(notes, encoding='utf-8', newline='\n')
    if args.store_pr_output:
        args.store_pr_output.parent.mkdir(parents=True, exist_ok=True)
        args.store_pr_output.write_text(pr, encoding='utf-8', newline='\n')

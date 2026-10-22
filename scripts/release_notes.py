"""Build release notes and an official-store PR draft from actual repo identity."""
import argparse
import re
from pathlib import Path
from release_artifacts import ROOT, TARGETS, source_version
from validate_tag import validate


def render(repository):
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('Expected GitHub owner/repo')
    version = source_version()
    tag = validate('v' + version)
    url = 'https://github.com/' + repository
    base = url + '/releases/download/' + tag
    changes = (ROOT / 'CHANGELOG.md').read_text(encoding='utf-8')
    section = re.search(r'^## ' + re.escape(version) + r'[^\n]*\n(.*?)(?=^## |\Z)', changes, re.M | re.S)
    if not section:
        raise ValueError('Version requires a CHANGELOG.md section')
    packages = '\n'.join(f'- [cpa-spark_{version}_{target}.zip]({base}/cpa-spark_{version}_{target}.zip)' for target in TARGETS)
    notes = f'''# CPA Spark {tag}

{section[1].strip()}

## 安装包

{packages}
- [checksums.txt]({base}/checksums.txt)

每个平台 ZIP 根目录只有 `cpa-spark.so`、`cpa-spark.dylib` 或 `cpa-spark.dll`。
每个动态库均在匹配 OS/CPU 的 GitHub runner 上构建并通过假 CPA Host C ABI 加载测试。
这些测试不代表真实 OAuth 账号或目标 CPA 版本的完整集成验收。

在 CPA 中先添加自定义商店源：
`{url}/releases/latest/download/registry.json`

手动安装和 Docker 持久化步骤见 [README]({url}/blob/{tag}/README.md)。
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

All five targets pass native ABI load tests and archive/checksum/architecture validation in GitHub Actions. Before opening this PR, publish the draft, wait for Verify published store packages to pass, and confirm installation on an actual CPA instance.

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

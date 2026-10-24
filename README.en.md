# CPA Spark · 星火

<img src="logo.svg" width="80" height="80" alt="CPA Spark">

[简体中文](README.md) | **English**

[Latest stable](https://github.com/heytl/cpa-spark/releases/latest) · [All releases](https://github.com/heytl/cpa-spark/releases) · [Changelog](CHANGELOG.en.md)

[![CI](https://github.com/heytl/cpa-spark/actions/workflows/ci.yml/badge.svg)](https://github.com/heytl/cpa-spark/actions/workflows/ci.yml)

**Account Warmup & Scheduling**

CPA Spark is a native [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) plugin for independent account and group warmup schedules. Scheduling runs on the server and continues after the browser closes.

The current development version is **0.1.8**. The plugin is not yet listed in the official CPA store. A successful request does not prove that a quota window has started; window visualization is an estimate based on schedules, while actual resets are controlled by the upstream service.

## Features

- Individual accounts, multiple accounts and static groups, with independent weekdays, times, timezones, models and reasoning settings.
- A union of selected account models: an empty or failed catalog does not block models available to other accounts.
- Prefer Codex Luna / Gemini Flash and the lowest available reasoning consumption for new tasks, using CPA's reported capabilities.
- Search bundled IANA timezones; default to `Asia/Shanghai`.
- Merge identical account/request configurations at the same scheduled instant. Serialize each account, with global concurrency of 1–8 and a maximum of 2 per provider.
- Persist configuration, execution results and idempotency claims in SQLite. Skip disabled, expired or unavailable accounts at execution.
- Run one-off account/group warmup without creating a scheduled task.
- Visualize 24/48-hour planned windows; account details show the past 24 hours of executions and the next 24 hours of plans.
- Delete individual records, filtered history or all history while retaining active executions.
- Follow CPA's language and theme. Preserve user-defined names/descriptions; missing translations fall back to complete English messages.

## Requirements

| Item | Requirement |
| --- | --- |
| CPA | C ABI 1, RPC schema ≥ 6, `management_api`, `host.auth.list` and `host.model.execute` |
| Providers | Codex and Antigravity; CPA and the upstream service enforce model access |
| Build tools | Go 1.24+, a CGO C compiler, Python 3.12+, Node.js 22+ |
| State directory | Writable, persistent local storage for one plugin instance; NFS and multi-node sharing are unsupported |

Running the plugin requires neither Python/Node nor an external CDN. Frontend assets are embedded in the library with `go:embed`. A `no-plugin` CPA build cannot load this plugin.

## Install in Docker Compose CPA

Import and enable the **library matching the CPA server's platform** in CPA's plugin manager. For ARM64 Linux, use `cpa-spark-v0.1.8.so`, not the source ZIP.

To copy it manually, run the following in your CPA Compose project directory:

```bash
mkdir -p plugins/linux/arm64
cp /your/download/directory/cpa-spark-v0.1.8.so plugins/linux/arm64/cpa-spark.so
```

Ensure Compose mounts the plugin directory into the container, then enable it in CPA configuration:

```yaml
plugins:
  enabled: true
  dir: "./plugins"
  configs:
    cpa-spark:
      enabled: true
      priority: 10
      state_dir: "./plugins/state/cpa-spark"
```

`state_dir` is relative to CPA's working directory inside the container and must be writable and persistent. Reload the plugin or restart CPA, then open **CPA Spark** from the plugin resource menu. The default URL is `/v0/resource/plugins/cpa-spark/status`; include your reverse proxy's deployment prefix when applicable.

The plugin silently reuses CPA's remembered same-origin session. **No Management key entry is needed in the plugin.** Select “Remember password” when logging into CPA; return to CPA to log in again after logout or expiration. The plugin does not persist another copy of the key or write it to SQLite, URLs or logs.

See the [integration guide](docs/INTEGRATION.md) (Chinese) for interfaces and deployment checks.

## Usage

1. Review CPA accounts under Accounts & Groups and create groups if needed.
2. Create a task and choose accounts, weekdays, times, timezone, model and reasoning.
3. Review the server's next five scheduled occurrences and save.
4. Use account/group one-off warmup for an immediate check. It consumes normal account quota.
5. Inspect per-account execution results and the account timeline's rolling 24-hour history and plan.

First startup does not create sample accounts, enable tasks or send model requests. Account login and OAuth refresh are managed by CPA.

## Build from source

```bash
npm ci --ignore-scripts
python3 scripts/build_web.py
go mod download
python3 scripts/ci.py
make build
```

Versioned downloads are written to `dist/downloads/PLATFORM/cpa-spark-vVERSION.so` (`.dylib` on macOS, `.dll` on Windows). Internal ABI checks and store ZIPs retain the canonical `cpa-spark.so` filename. Usually only the library is needed for installation. Build for the actual server OS/CPU; a Linux library cannot run directly on macOS or Windows.

For a native build with ABI verification on the matching platform:

```bash
python3 scripts/build_native.py --target linux_arm64
```

Other targets are `linux_amd64`, `darwin_amd64`, `darwin_arm64` and `windows_amd64`. The script rejects a different OS/CPU instead of treating cross-compilation as native verification.

## Automated builds and Releases

CI checks, builds and loads the plugin on five native platforms, producing downloadable installation packages:

| Type | Example |
| --- | --- |
| Development package | `cpa-spark_0.1.8-dev.a976b6b_linux_arm64.zip` |
| Stable package | `cpa-spark_0.1.8_linux_arm64.zip` |
| Actions package bundle | `cpa-spark_0.1.8-dev.a976b6b_packages.zip` |

Development filenames identify the source commit; the plugin UI retains the source version without an automatic version bump. The outer Actions ZIP contains five platform ZIPs and `checksums.txt`. Extract the platform ZIP before importing its library. Native diagnostic artifacts also include the version and ABI verification receipt.

To release:

1. Set the stable version in `internal/warmup/types.go` and add matching entries to both `CHANGELOG.md` and `CHANGELOG.en.md`. Commit the changes.
2. Create and push a matching Tag, such as `v0.1.8`. The workflow creates a **Release draft** by default.
3. To publish through Actions, open **Actions → Release → Run workflow**, enter an existing Tag and select `publish`. The default `draft` mode leaves it unpublished. An existing draft can be verified again and published; an already published Release is rejected.
4. The `publish` mode publishes the draft as the latest stable release only after all five native builds, tests, packaging and exact Release asset download/load checks succeed. Published assets are then verified again. Draft mode runs the draft download checks too.

Each Release includes Chinese and English notes, five platform downloads, checksums and version navigation. Use All releases for older versions. GitHub's branch/Tag selector chooses the workflow code; release creation also requires the matching Tag input. A missing changelog entry in either language blocks publication.

Third-party Actions are pinned by commit SHA. Normal CI uses read-only repository permissions; only Release creation/publication jobs use write permission. No personal token, CPA key or OAuth credentials are required. A rerun reuses an existing draft without replacing its assets or notes. Published tags and assets are not replaced.

Development bundles do not generate store registration files. Stable Release packages include `registry.json` and store submission material; drafts do not become the store's latest installation source. Real CPA acceptance is still required before publication. See the [release and store guide](docs/PLUGIN_STORE.md) (Chinese).

## Development and directories

```bash
make dev      # Local fake CPA; no real OAuth credentials or upstream requests
make preview  # Build a self-contained static preview and check interactions
make test     # Run the same complete checks as CI
```

| Directory | Contents |
| --- | --- |
| `cmd/plugin` | CPA C ABI library entry point |
| `cmd/dev` | Local fake host |
| `internal/warmup` | Models, SQLite, scheduling, host callbacks and management API |
| `web` | Frontend sources, locales and generated embedded assets |
| `design` | Page templates and offline preview generator |
| `scripts` | Frontend generation, checks, builds, packaging and release material |
| `tests` | Frontend rules, interactions, builds and native ABI checks |
| `docs` | Interfaces, integration, localization and release guidance |

Edit the frontend source and regenerate rather than directly changing `web/index.html`, `web/app.js` or `web/style.css`. Do not commit static previews, libraries, databases, logs or dependency directories.

- [Contributing](CONTRIBUTING.md) (Chinese)
- [Localization maintenance](docs/I18N.md) (Chinese)
- [Management API](docs/API.md)
- [Changelog](CHANGELOG.en.md)

## Execution semantics and validation scope

A new process starts scheduling from future minutes; missed requests are not replayed by default. Skip missing DST times and use the first occurrence of repeated times. Deduplicate the same UTC instant, account and request configuration; different configurations execute separately.

An already dispatched request is not falsely reported as cancelled. Interrupted in-flight results become unknown after restart and are not automatically replayed. SQLite write failures stop automatic dispatch. CPA's own routing/retry settings can still affect network attempt counts.

History defaults to 30 days, configurable to 7/30/90 days. Deleting visible history does not delete deduplication claims. History uses server-side pagination, with 50 rows by default and 20/50/100 options; the existing date filter is unchanged. Bounded claim archiving, configuration import, notifications and multi-node coordination are not yet implemented.

Linux amd64/arm64, macOS Intel/Apple Silicon and Windows amd64 have passed native GitHub CI builds, tests, library loading and packaging. Check the corresponding Actions run for later changes. Fake-host tests do not replace real CPA acceptance. Account plans use explicit metadata and Codex JWT claims; unknown plans and windows are not guessed.

## Licensing and acknowledgments

A project-wide source license has not yet been selected. Public visibility does not grant additional usage rights; the author will determine the license. See [third-party notices](docs/third-party/README.md) for icons. Go/npm dependencies retain their own licenses.

Native interfaces follow the [CPA plugin specification](https://help.router-for.me/cn/plugin/development.html). ABI/host calls are based on official CPA examples; session compatibility follows the official management UI's storage format. See [source revisions and licenses](docs/SOURCES.md).

After a push to `main`, successful five-platform CI creates a missing source-version tag and explicitly calls the Release draft workflow. Existing tags are never moved.

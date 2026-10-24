# Changelog

[简体中文](CHANGELOG.md) | **English**

## 0.1.8 — 2026-10-24

- Add server-side history pagination with 20/50/100 rows (50 by default), retain the existing date picker and delete filtered records across all pages. Query home summaries, execution details and account timelines independently.

- Use “Delete” for history actions and “Failed” for individual failures; display all trigger times with a full tooltip and make upcoming task titles plain text.

- Add “First +5h” to generate same-day times at five-hour intervals from the first time.
- Keep the account timeline in its own centered dialog while viewing execution details; returning or closing details preserves the timeline and scroll position.
- Default history to individual account executions, with a task view toggle. Deduplicate shared requests and apply deletion to the selected view.
- Open a task's execution history from its home-page title; retain the dedicated edit action.
- Name standalone libraries `cpa-spark-vVERSION.so` as required by CPA, separated into platform directories; keep canonical filenames inside store packages.

## 0.1.7 — 2026-10-22

- Use a transparent gray icon in CPA navigation while retaining the colored C “Spark Dial” logo on pages and in the store.

- Adopt the C “Spark Dial” SVG logo for the header, settings, favicon, README, native metadata and plugin-store entry.

- Version all Actions artifacts and platform ZIPs. Development downloads include the source commit; stable downloads use the release version.
- Add manual draft/publish Release modes, bilingual release notes and explicit five-platform verification before automatic publication.
- Provide Chinese and English README files with language links and version navigation.
- Fix Windows CI compiler setup, generated CSS line endings and SQLite drive-letter file URIs.
- Rework CPA session compatibility and the native bridge against official CPA sources; document source revisions and include upstream notices in the library.
- Refine group color swatches, remove the outer selection ring and center checkmarks. Use the Lucide Settings 2 icon and localize the brand subtitle.
- Offer the union of selected account models; one empty or failed account catalog no longer blocks models from other accounts. Disabled and unavailable accounts are skipped at execution.
- Show executions from the past 24 hours and plans for the next 24 hours in account timelines. Deduplicate merged executions and retain actual per-account times.
- Remove the connection banner. Connect silently, report errors through shared feedback and retry automatically. Small development changes retain the current version.
- Display the build-generated version tag beside the project name in the header and settings, following the active theme without translation.

## 0.1.6 — 2026-10-22

- Share one brand component between the header and settings; fix labels compressing controls.
- Replace partial Chinese text replacement with a centralized catalog, stable keys and named parameters. Check coverage, key references and parameters at build time; missing translations fall back to complete English messages.
- Fix mixed-language pages, dialogs, dropdowns and feedback while preserving user-defined names, descriptions and input values.

## 0.1.5 — 2026-10-22

- Replace timeline “+1 day / +2 days” labels with full dates above midnight ticks; align time labels on a separate row.

## 0.1.4 — 2026-10-22

- Simplify window visualization: remove repeated dates and model/account metadata, move the current time beside the timezone, add a continuous time guide and reduce row height.
- Import CPA page, card, overlay, text and border colors; synchronize theme changes and retain standalone theme fallbacks.
- Fix light surfaces leaking into dark mode and unify dialogs, filters, statuses and timelines across themes.
- Left-align account warmup actions and their column heading.

## 0.1.3 — 2026-10-22

- Expand group colors to seven, with more spacing and a clear selection indicator.
- Fix model refresh leaving task creation disabled. Bind server validation to the current configuration and support retrying failures.
- Redesign window visualization with separate date/time/current-time regions, adaptive labels and fixed target/summary areas.

## 0.1.2 — 2026-10-21

- Read account plans using official CPA host.auth callbacks and the explicit Codex / Antigravity plan-field distinctions used by cpa-plugin-key-policy. Check list metadata first, then host.auth.get_runtime and host.auth.get.
- Support nested credential metadata and Codex JWT claims without exposing OAuth credentials or inferring plans from OAuth type, model capabilities or file names. Unknown plans remain unidentified.
- Align typography with CPA and fix overflowing current-time labels, day offsets and action-column alignment.
- Add plan compatibility and credential non-disclosure tests. Linux amd64 passed native ABI checks; ARM64 was cross-compiled, with real CPA acceptance still pending at that point.
- Add five-platform CI, numeric tag validation, Release drafts, actual-asset verification and generated plugin-store submission material.

## 0.1.1 — 2026-10-21

- Center all editor and detail dialogs.
- Use shared custom dropdowns, searchable IANA timezones and UTC offsets.
- Generate editable default task and group names.
- Delete individual, filtered or all history while retaining active executions and deduplication claims.
- Add one-off warmup for accounts and groups without saving a scheduled task.
- Filter accounts by enabled status and list enabled accounts first.
- Unify save, delete, queue and completion feedback; display the latest settings save time.
- Update branding, typography, window visualization and upcoming triggers.

## Unreleased: plugin-store integration

- Separate model catalog loading from execution availability so paused or unavailable targets can still expose models and reasoning choices. Distinguish loading, failure and empty selections.
- Center the task editor and remove the top-right status/help/theme area.
- Load models and reasoning capabilities from CPA, preferring Luna / Gemini Flash and the lowest available effort, including native Gemini budgets.
- Add 24-hour time dropdowns, presets, deduplication and 596 searchable IANA timezone entries; default to Shanghai.
- Move row menus into an independent overlay to prevent clipping.
- Reuse CPA WebUI's remembered same-origin login instead of asking for a Management key; handle logout, session changes and unauthorized responses.
- Supply required author/repository metadata in local builds and test CPA's registration gate.
- Package platform ZIPs and checksums with binary architecture and native ABI receipt checks.
- Introduce five-platform native build, draft and published-download verification workflows; GitHub execution was still pending at this historical stage.
- Persist state under plugins/state/cpa-spark within the official Docker Compose plugin mount.
- Generate store registration material; the plugin is not yet listed.

## 0.1.0-alpha.2

- Adopt CPA Spark (星火) as the official name; use warmup only as a feature description.
- Align directories, Go module, plugin ID, library names, API routes, configuration, state paths and CI artifacts under cpa-spark.
- Update the README, page introduction, about dialog and plugin resource menu.
- Remain in Alpha pending integration with a real CPA instance and accounts.

## 0.1.0-alpha.1

- Introduce server-side scheduling and SQLite persistence.
- Support account/group tasks, duplicate merging, concurrency limits, pause controls and execution records.
- Build Linux amd64 and verify Go race checks and the native ABI with a fake host.

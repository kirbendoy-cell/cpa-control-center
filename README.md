[English](./README.md) | [简体中文](./README.zh-CN.md)

# CPA Control Center

Desktop operations tool for CPA-managed Codex auth pools.

This project starts from a very practical observation: once an auth pool grows beyond a small scale, relying on a browser tab or manually visiting `localhost` to manage it becomes fragmented, easy to ignore, and easy to miss anomalies. `CPA Control Center` wraps the existing CPA management APIs into a focused desktop app. It requires no extra deployment, and once you connect it to your CPA instance, you can scan, inspect anomalies, run maintenance, and export results from one place.

## Acknowledgement and Intended Backend

- This project is explicitly inspired by and borrows workflow ideas from [`fantasticjoe/cpa-warden`](https://github.com/fantasticjoe/cpa-warden).
- This desktop tool is intended to be used with [`router-for-me/CLIProxyAPI`](https://github.com/router-for-me/CLIProxyAPI) as the CPA backend that exposes the management endpoints consumed by the app.

## Overview

- Native desktop app built with Wails, Go, Vue 3, and TypeScript
- Only requires `Base URL` and `Management Token`
- Scans Codex auth pools through CPA management endpoints
- Automatically classifies accounts into `Normal`, `401 Invalid`, `Quota Limited`, `Recovered`, and `Error`
- Shows confirmation dialogs before maintenance actions to avoid mistakes
- Supports scan history, paginated details, live task logs, and CSV/JSON export
- Built-in bilingual interface: English and Simplified Chinese

## Who This Is For

This project is a good fit if:

- you have already deployed CPA and enabled management endpoints
- you maintain a Codex-focused auth pool, with possible expansion to other channels later
- you want a desktop app that works immediately without any extra deployment
- you want scanning, maintenance, logs, and exports inside one tool

This project is not currently focused on:

- creating or importing auth files from the GUI
- running OAuth login flows inside the desktop app
- becoming a general-purpose admin panel for every CPA capability

## What Problem It Solves

The pain point in large auth pools is usually not total failure. It is mixed operational states:

- some accounts are already `401 Invalid`
- some accounts hit quota limits
- some accounts were disabled earlier and are now recoverable
- some probe failures are only temporary network or upstream issues

The app is designed around two goals:

1. Give you a fast, reliable picture of current pool health.
2. Let you run controlled, repeatable maintenance on the latest scan result.

## What It Can Do

### 1. Connect to CPA

You only need:

- `Base URL`
- `Management Token`

The app can test connectivity before saving, and the connection test itself does not trigger a scan.

### 2. Scan the Pool

When you click **Scan Now**, the app will:

1. load the full auth inventory from CPA
2. apply your configured `targetType` and `provider` filters
3. probe matching accounts concurrently
4. write the latest local snapshot
5. record a scan history entry for later inspection

### 3. Apply Unified State Classification

Scan results are normalized into a compact set of operationally useful states:

- `Normal`
- `401 Invalid`
- `Quota Limited`
- `Recovered`
- `Error`

The dashboard, account list, maintenance rules, and export logic all use this same state model.

### 4. Run Maintenance

When you click **Run Maintenance**, the app first performs a fresh scan and then applies your configured rules:

- delete `401 Invalid` accounts
- `disable` or `delete` quota-limited accounts
- automatically re-enable recovered accounts

All destructive actions require confirmation first.

### 5. Review History and Logs

The app keeps:

- the current account snapshot
- recent scan history
- paginated scan details
- live task logs and progress events

If you enable **Detailed Logs** in Settings, you will also see per-account probe and maintenance messages.

### 6. Export Problem Sets

You can export the current:

- `401 Invalid` accounts
- `Quota Limited` accounts

Formats:

- JSON
- CSV

## Core Capabilities

- desktop-first operations workflow
- dashboard health overview and recent scan history
- real-time account table with search, filter, and pagination
- single-account probe, disable/enable, and delete actions
- live scan and maintenance progress in the task log stream
- automatic retries for recoverable transient probe failures
- local persistence with `settings.json`, `state.db`, and `app.log`
- Windows build support and a completed macOS build pipeline
- GitHub Actions-based automated builds and tag-based Release publishing

## Real Workflow

If you want the shortest possible mental model, it is this:

1. Save CPA connection settings.
2. Click **Scan Now**.
3. Review the dashboard and recent scan history.
4. Open scan details if you need to inspect specific accounts.
5. Click **Run Maintenance** when the latest scan matches your intent.
6. Export `401` or `Quota Limited` results when you need to hand them off elsewhere.

## Page Structure

### Dashboard

- pool health overview
- recent scan history
- scan details drawer with backend pagination
- one-click scan and one-click maintenance entry points

### Accounts

- real-time account table
- full-dataset search before pagination
- state and provider filters
- single-account actions

### Logs

- real-time task stream
- current progress is visible whether detailed logs are enabled or not
- enabling detailed logs exposes per-account messages

### Settings

- CPA connection parameters
- language switching
- concurrency and timeout settings
- retry count
- quota-handling strategy
- export directory
- log verbosity

## CPA Endpoints Used

This app intentionally stays focused and depends on only a small set of CPA management endpoints:

- `GET /v0/management/auth-files`
- `POST /v0/management/api-call`
- `DELETE /v0/management/auth-files?name=...`
- `PATCH /v0/management/auth-files/status`

Pool health probing goes through CPA and targets:

- `https://chatgpt.com/backend-api/wham/usage`

## Default Behavior

The default configuration is:

| Setting | Default |
| --- | --- |
| Locale | normalized system locale (`en-US` / `zh-CN`) |
| Target type | `codex` |
| Probe workers | `40` |
| Action workers | `20` |
| Timeout | `15s` |
| Retries | `1` |
| Quota action | `disable` |
| Delete 401 | enabled |
| Auto re-enable recovered accounts | enabled |
| Detailed logs | disabled |

## Retry Model

Retries are split into two layers:

- request-level retries for outer request failures and transient CPA errors (`408`, `429`, `5xx`)
- probe-level retries for recoverable probe anomalies such as temporary upstream issues or incomplete response payloads

The app does not blindly retry final business outcomes such as:

- `401 Invalid`
- `Quota Limited`
- clearly missing account metadata

## Local Data Storage

The app stores local state under your OS user configuration directory in:

`CPA Control Center/`

Typical contents:

- `settings.json`
- `state.db`
- `app.log`
- `exports/`

The current implementation keeps the latest snapshot and the most recent `30` scan runs.

## Project Structure

```text
cpa-control-center/
├─ frontend/                     # Vue 3 + TypeScript frontend
├─ internal/backend/             # CPA client, state store, task orchestration
├─ build/                        # Wails build assets and platform packaging config
├─ scripts/build-macos.sh        # macOS build helper
├─ .github/workflows/            # CI / Release workflows
├─ app.go                        # Wails binding layer
├─ main.go                       # shared entry point
├─ platform_options_windows.go   # Windows window configuration
├─ platform_options_darwin.go    # macOS window configuration
└─ wails.json                    # Wails project configuration
```

## Development and Build

### Requirements

- Go `1.24+`
- Node.js `22+` recommended
- Wails CLI `v2.11.0`

### Development Mode

```bash
wails dev
```

### Windows Build

```bash
wails build
```

### macOS Build

Run this on a Mac or a `macos-latest` GitHub Actions runner:

```bash
bash ./scripts/build-macos.sh
```

## Safety Notes

- This is an operations tool, not a demo page. Maintenance actions can delete or disable accounts.
- Review the latest scan result before running maintenance.
- If you are validating a new CPA environment, it is safer to turn off `delete401` first and observe results.
- Detailed logs are great for troubleshooting large pools, but they can become noisy.

## Current Scope

This project currently focuses on:

- managing existing CPA auth files
- health probing and maintenance for Codex pools
- Windows-first user experience while also providing a macOS build path

It does not currently include:

- auth import wizards
- in-app login / OAuth acquisition
- multi-node orchestration
- advanced analytics beyond local snapshots and history views

## FAQ

### Does it open a browser?

No. It is a Wails desktop application and opens in its own native window.

### Is it a full CPA admin panel?

No. It is a focused desktop operations tool for auth-pool health and maintenance.

### Does “Run Maintenance” scan first?

Yes. Maintenance always starts from a fresh scan and then applies your maintenance rules.

### Can scan details handle large result sets?

Yes. Scan details are paginated on the backend and are not loaded all at once into the drawer.

### Is macOS supported?

The build path is already in place. Production macOS artifacts should be generated on a Mac or a `macos-latest` GitHub Actions runner.

## Roadmap

- scheduled tasks for automatic pool scanning and automatic maintenance
- configurable threshold-based rules, for example disabling accounts once they fall below a defined percentage
- support more auth-channel maintenance
- add richer statistics and trend views
- add signing / notarization workflows when external distribution requires it

## Current Status

The app is already usable for real CPA pool operations, but it is still an evolving practical tool. The current priority remains reliability, clarity, and maintainability instead of feature sprawl.

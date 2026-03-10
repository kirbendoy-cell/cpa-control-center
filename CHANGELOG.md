# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

## v1.3.0

### Highlights

- Rebuilt the desktop shell into window-driven `wide`, `desktop`, and `compact` layout modes so the app now expands on large screens and stays readable on smaller windows without the old fixed-canvas scaling behavior.
- Reworked the dashboard, sidebar, account/log/settings layouts, and scan detail drawer to follow the new shell modes with tighter compact layouts and better internal scrolling behavior.
- Updated the pool health donut to size from its container, keep the chart centered across shell modes, and stay stable during first render and resize changes.
- Changed startup window sizing on Windows and macOS to prefer the best desktop size while shrinking to the current screen work area when the display is smaller.

### Notes

- The app no longer relies on whole-window scale transforms for primary layout behavior.
- Smaller screens now prioritize vertical scrolling and readable panel density over fitting the entire dashboard into a single static viewport.
- Startup window sizing uses the operating system work area when available, so the first window should open closer to the best usable size on both Windows and macOS.

## v1.2.0

### Highlights

- Added an in-app scheduler that can trigger recurring `Scan` or `Maintain` runs while the desktop app is open.
- Added `Full` and `Incremental` scan modes with configurable incremental batch size.
- Reduced large-pool setup pressure by merging connection validation and inventory sync into a single remote fetch during **Test & Save**.
- Extended the settings UI with scheduler mode, cron expression, next-run status, last-run result details, and advanced parameter help popovers.
- Adjusted task completion refresh handling so manual and scheduled runs refresh the UI once instead of duplicating large-pool reloads.

### Notes

- Scheduled tasks use local system time and standard 5-field cron expressions.
- The scheduler does not replay missed runs after the app restarts.
- Incremental scans prioritize `Pending` accounts first, then the oldest last-probed records.
- The default retry count is now `3`.

## v1.1.0

### Highlights

- Added inventory-first startup for large pools, so first-time connections can sync tracked auth records before the first full scan.
- Moved the account table and scan details to backend pagination to reduce frontend pressure on pools with thousands of auth files.
- Stabilized dashboard startup and donut rendering to address blank first-load states and improve large-pool reliability.
- Improved retry handling and retry visibility for transient probe failures.

### Notes

- Existing local settings and state are preserved across upgrades.
- macOS users may need to right-click the app and choose `Open` on first launch.
- This release focuses on large-pool startup, inventory sync, dashboard stability, and paged data loading.

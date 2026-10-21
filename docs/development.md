# Development guide

[README](../README.md) · [中文介绍](../README.zh-CN.md) · [Windows packaging](packaging.md)

## Environment and setup

Development targets Windows 10/11 with Go 1.27.0+, Node.js 22.12+ or a compatible newer version, pnpm 12.14.0, and WebView2 Runtime. Build templates for other platforms do not indicate supported PasteWin builds.

Install the pinned package manager with `npm install --global pnpm@12.14.0` if needed. Wails development and production builds use pnpm with `--frozen-lockfile`. After changing dependencies, update and commit `frontend/pnpm-lock.yaml`. The frontend's `pnpm-workspace.yaml` sets a seven-day minimum release age for dependency resolution; pnpm 12 reads this setting from YAML, not `.npmrc`.

The project uses Wails **v3.0.0-beta.25**, React 18, TypeScript, Vite, lucide-react, and the pure-Go `modernc.org/sqlite` driver. Exact dependencies are recorded in [go.mod](../go.mod) and [pnpm-lock.yaml](../frontend/pnpm-lock.yaml).

From the repository root:

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.25
wails3 doctor
wails3 dev
```

If `wails3` is not on PATH, add the `bin` directory under `go env GOPATH`, or run:

```powershell
& "$(go env GOPATH)\bin\wails3.exe" dev
```

Exit an existing PasteWin instance through settings or the tray before launching a development build. The single-instance mechanism can otherwise activate the old process, and Windows can prevent replacement of its executable.

## Build and UI preview

```powershell
wails3 build
```

The production build generates TypeScript bindings, builds the frontend, and produces `bin/pastewin.exe`. Go embeds `frontend/dist`, so a fresh checkout needs a build before running Go tests.

For a browser-only UI preview:

```powershell
cd frontend
pnpm install --frozen-lockfile
pnpm run dev
```

Open `http://127.0.0.1:9245/?preview=1`. Preview mode uses in-memory sample data and does not access the system clipboard. Sample data is limited to development builds. Browser preview cannot exercise native hotkeys or paste into another application.

## Automated checks

Run from the repository root after the initial build:

```powershell
go test ./...
go vet ./...
pnpm --dir frontend run build
pnpm --dir frontend test
```

`pnpm --dir frontend run format` formats frontend source and tests in place.

Browser tests use installed Microsoft Edge in headless mode and synthetic data. They start a preview server on port 9246, test 360 × 680 and 320 × 420 viewports, and write results to `.work/browser-results`. Coverage includes panel reopening, delayed capture, selection, stale requests, pagination, dialogs, and copy/paste target state.

Go tests cover text preservation, deduplication, favorites, capacity, search, pagination, persistence, image conversion and limits, database migration, image cleanup, stale capture jobs, resource paths, and isolated startup registration.

Native image integration tests are skipped by default. These commands **replace the system clipboard with synthetic content**. Run the packages separately to avoid clipboard contention:

```powershell
$env:PASTEWIN_TEST_CLIPBOARD = '1'
try {
    go test . -run TestImageCaptureAndCopyIntegration -v
    go test ./internal/native -run TestImageClipboardIntegration -v
} finally {
    Remove-Item Env:PASTEWIN_TEST_CLIPBOARD
}
```

For DIB parser fuzzing:

```powershell
go test ./internal/clipimage -run '^$' -fuzz FuzzDecodeDIB -fuzztime 15s
```

## Native validation and test data

Select an independent data directory before starting the app, from the repository root:

```powershell
$env:PASTEWIN_DATA_DIR = Join-Path (Get-Location) '.work\manual-test-data'
wails3 dev
```

This isolates history storage, but not the single-instance identifier or global hotkey. Remove the variable after testing to restore the default path for subsequent launches:

```powershell
Remove-Item Env:PASTEWIN_DATA_DIR
```

Useful desktop checks with synthetic content:

1. Copy Chinese and multiline text in Notepad, open PasteWin with the hotkey, search, favorite, and paste at the original cursor position.
2. Pause and resume capture, hide and reopen the panel, then exit and restart to check persistence.
3. Copy a screenshot, a browser image, and a transparent PNG; paste into Paint or an image-capable editor. Check that pasting does not submit a message.
4. Inspect thumbnails, mixed text/image previews, long text, empty states, keyboard controls, and dialogs at both supported panel sizes.
5. For startup changes, check actual Windows sign-out/sign-in behavior. Registry tests alone do not cover the Windows session lifecycle.

## Source layout

| Path | Purpose |
| --- | --- |
| `main.go` | Windows, tray, hotkey, single instance, lifecycle |
| `clipboard_service.go` | Wails service and capture/copy/paste coordination |
| `capture_pipeline.go` | Bounded capture queue and background image processing |
| `image_assets.go` | Image resources addressed by history ID |
| `internal/native/` | Windows clipboard notifications, formats, focus, and input injection |
| `internal/history/` | SQLite storage, search, capacity, and persistence tests |
| `internal/clipimage/` | PNG/DIB validation, conversion, pixel hashes, and thumbnails |
| `internal/startup/` | Current-user startup registration and isolated tests |
| `frontend/src/` | React UI, service adapter, and development preview |
| `frontend/src/i18n.tsx`, `messages.ts`, `messages-errors.ts` | Language state, typed UI translations, and known application error translations |
| `frontend/bindings/` | TypeScript bindings generated by Wails |
| `Taskfile.yml`, `build/` | Development and build configuration |

## Implementation overview

The compact panel uses a light Liquid Glass-inspired palette, pill-shaped category controls, a floating action toolbar, and opaque-enough content cards. Wails enables the native Acrylic backdrop with a light window theme; on older Windows versions its backend falls back to blur-behind. Desktop translucency depends on the OS and transparency settings. CSS provides reduced-transparency and forced-colors treatments. Keep blur on controls rather than every history card to limit compositing work. Verify both 360 × 680 and 320 × 420 layouts, and use a native window to check the backdrop; browser preview only shows the frontend layers.

Run browser regression tests after production builds finish: binding generation can trigger Vite hot reload and replace test fixtures if a build runs concurrently.

The UI supports `zh-CN` (default) and `en`. The preference is stored under `language` in the existing SQLite `settings` table, without a schema migration. The frontend loads it before displaying the panel and changes language only after saving succeeds. The tray labels update on the UI thread. Browser preview uses a separate localStorage preference and does not change the native setting.

Chinese UI messages act as typed translation keys, with named placeholders for values. English count labels include singular forms, and timestamps and counts use the selected locale. Known backend errors and their wrappers are translated at the display boundary, while unknown diagnostics and clipboard content remain unchanged. Language regression tests cover persistence, failed saves, unchanged selection, keyboard switching, and both panel sizes.

Clipboard capture uses `AddClipboardFormatListener` notifications rather than idle polling. Text preserves whitespace and line breaks. Exact text duplicates merge while retaining favorite status. Images are deduplicated by normalized RGBA pixels and dimensions; mixed records also include the text hash.

Clipboard reads copy bounded data before a single background worker decodes images. The pipeline keeps at most one pending snapshot in addition to the active task. A newer pending snapshot replaces an older one under load. Pause/resume, clearing records that are not favorites, and shutdown invalidate stale capture tasks.

Supported DIB inputs include 1/4/8-bit palettes and 16/24/32-bit RGB/bitfields. Compressed DIB data, external color profiles, and malformed inputs are rejected, with fallback to another available format. Raw image format buffers total at most 64 MiB per capture. PNG normalization produces 8-bit RGBA without source metadata or professional color profiles. Clipboard image output provides both PNG and sRGB DIBV5 with explicit alpha.

SQLite stores images and thumbnails in `clip_images` with cascading deletion. The version-1 migration preserves older text history, favorites, and pause state. Deleted space is reused; database and WAL size can exceed the stored image-content limit.

Lists return 50 records per page with text previews of up to 180 characters. Full text is fetched on selection. Images load on demand through `/history-image/<id>/thumbnail.png` and `/history-image/<id>/image.png`, with `no-store` caching, using the Wails resource handler.

Automatic paste restores the previous target window and injects Ctrl+V after shortcut keys are released. Errors remain visible so the user can copy and paste manually. The hidden panel retains its WebView for faster reopening; memory measurements need to include associated WebView2 processes.

The app's About version comes from `build/windows/info.json`. See the [packaging guide](packaging.md) for installer version metadata. Agent-specific editing constraints remain in [AGENTS.md](../AGENTS.md).

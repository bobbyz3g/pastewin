# PasteWin

English | [简体中文](README.zh-CN.md)

PasteWin is a local clipboard history tool for Windows. Save copied text and images, then use a keyboard shortcut to find and paste them back into your application. Your history stays on your computer. No account is required.

PasteWin is an early-stage project. The application supports English and Simplified Chinese. The installer currently uses Chinese UI text.

## Features

- **Text and image history** — keep plain text and static bitmap images across restarts, with duplicate detection.
- **Quick access** — open a compact floating panel with **Ctrl+Shift+V**, search text, and select a record.
- **Glass appearance** — a translucent window with pill-shaped navigation, capture status at the top, and a floating action toolbar at the bottom. Search counts appear above results. Readable cards keep clipboard content clear. Background effects vary with Windows version and system settings.
- **Paste into your application** — paste at the cursor in the window you were using before opening PasteWin.
- **Favorites and cleanup** — keep useful records, delete individual items, or clear records that are not favorites.
- **Background operation** — record from the tray, pause capture, or enable optional startup at Windows sign-in.
- **Local storage** — save history on disk without uploading clipboard content.

## Install and run

PasteWin runs on **Windows 10/11** and requires **Microsoft Edge WebView2 Runtime**.

[Build from source](#build-from-source) to produce `bin/pastewin.exe`, or [build an installer](docs/packaging.md) to produce `bin/pastewin-<version>-<arch>-setup.exe`.

- **Portable executable:** run `pastewin.exe`. History is stored in your Windows user profile, not beside the executable.
- **Installer:** run the setup executable and choose an installation directory and optional desktop and Start menu shortcuts. The default directory is `%LOCALAPPDATA%\Programs\PasteWin`. Installation applies to the current user. The finish page offers a checked-by-default option to launch PasteWin when you click Finish. If WebView2 is missing, the installer runs Microsoft's bootstrapper, which requires internet access.

When upgrading a running installation, click OK in the installer prompt to close that copy of PasteWin and continue. If the application cannot exit, the installer stops replacing files and offers a retry. Exit PasteWin through settings or the tray before uninstalling or replacing a portable executable manually. Upgrades and uninstall preserve clipboard history. The executable and installer are currently unsigned.

## Quick start

1. Start PasteWin, then copy text or an image in another application.
2. Place the cursor where you want to paste and press **Ctrl+Shift+V**.
3. Search or select a record with **↑ / ↓**.
4. Press **Enter** to paste into the original window. Double-clicking a record also pastes it.

PasteWin sends **Ctrl+V** only. It does not send Enter or submit a message. If no target window is available, you can still copy a record and paste it manually.

| Shortcut | Action |
| --- | --- |
| Ctrl+Shift+V | Open the history panel |
| ↑ / ↓ | Select a record |
| Enter | Paste the selected record |
| Ctrl+Enter | Copy the selected record without pasting |
| Delete | Delete the selected record; in a nonempty search field or another text field, edit text instead |
| Esc | Close a dialog or menu; otherwise clear the search, then hide the panel |
| Shift+F10 / Menu key | Open the context menu for a focused record |

Right-click a record to favorite, unfavorite, or delete it. Use the content preview to inspect full text or an image. If a record contains both text and an image, the default action uses the text; the preview provides separate image actions. Search matches text only. Delete does not remove records while a dialog is open.

To change the language, open **设置** (Settings) and select **English** or **简体中文** under **语言** (Language). The interface and tray menu update immediately, and your choice is saved for the next launch. The default is Simplified Chinese. Clipboard content is not translated; system diagnostics may use the Windows language.

Closing the panel keeps PasteWin running in the tray. Choose **Quit app** in settings or **Exit** in the tray menu to stop it. **Start at sign-in** is off by default; when enabled, PasteWin starts in the tray without opening the panel. After moving a portable executable, enable startup again to register its new path. Windows startup settings or organization policies can still block automatic startup.

## Data and limitations

History, images, and favorites are stored in `%APPDATA%\PasteWin\history.db`. Storage is **local and unencrypted**. PasteWin honors supported clipboard exclusion flags, but cannot identify every password or sensitive item. Pause capture when needed.

- Capture starts with new copy events after launch. Existing clipboard content and items copied while paused are not imported.
- History holds up to **500 records**, with up to **1 MiB of UTF-8 text per record**. The oldest records that are not favorites are removed when needed. If all 500 records are favorites, new content is not retained.
- Images are limited to **16 million pixels** and **20 MiB per normalized PNG**. Images and thumbnails together can use up to **256 MiB** of stored content. Old images that are not favorites are removed to make room; if space is still unavailable, the new image is rejected with a message. SQLite database and journal files can be larger and do not shrink immediately after deletion.
- Plain text and static PNG / uncompressed DIB bitmaps are supported. Copying an image file in File Explorer is not the same as copying bitmap content. File history, animated images, OCR, rich HTML/RTF content, image editing, and custom shortcuts are not supported.
- Rapid copying can skip pending image captures, with a message in the panel.
- Automatic paste depends on the target application's input support and Windows permissions. Administrator windows and some editors may require manual paste. Image paste requires an application that accepts images.

## Build from source

On Windows, install **Go 1.27.0+**, **Node.js 22.12+ or a compatible newer version**, **pnpm 12.14.0**, and **WebView2 Runtime**. Run from the repository root:

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.25
wails3 doctor
wails3 build
```

The executable is written to `bin/pastewin.exe`. For live development, run `wails3 dev`.

PasteWin uses Go, Wails v3, React, TypeScript, and SQLite. See the [development guide](docs/development.md) for setup, UI preview, tests, and source layout, and the [Windows packaging guide](docs/packaging.md) for installer builds and options.

## Feedback and contributions

Report bugs or propose features through [GitHub Issues](https://github.com/bobbyz3g/pastewin/issues). For bugs, include your Windows version, PasteWin version, reproduction steps, and expected behavior. Use sample content instead of private clipboard data.

Contributions are welcome. The [development guide](docs/development.md) explains how to run and validate changes locally.

## License

PasteWin is licensed under the [MIT License](LICENSE). Third-party dependencies retain their respective licenses.

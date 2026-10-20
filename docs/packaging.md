# Windows packaging

[README](../README.md) · [中文介绍](../README.zh-CN.md) · [Development guide](development.md)

## Build an installer

Set up the [development environment](development.md), then install [NSIS 3](https://nsis.sourceforge.io/Download). Run from the repository root:

```powershell
wails3 task package
```

To specify the NSIS compiler or target ARM64:

```powershell
powershell -NoProfile -File build/windows/package.ps1 -NsisPath 'C:\Tools\NSIS\makensis.exe'
wails3 task package ARCH=arm64
```

ARM64 output needs runtime validation on an ARM64 device. The packaging script accepts `-Arch`, `-NsisPath`, and `-OutputDir`.

[package.ps1](../build/windows/package.ps1) stages a production build in `.work/package-<arch>` and writes `bin/pastewin-<version>-<arch>-setup.exe`. It does not overwrite `bin/pastewin.exe`. Packaging also retrieves the WebView2 bootstrapper.

The filename version comes from `fixed.file_version` in [info.json](../build/windows/info.json), which also supplies the frontend About version. Release version metadata is maintained in that file and [config.yml](../build/config.yml). Installer behavior is implemented in the project's [NSIS script](../build/windows/nsis/project.nsi). The executable and installer currently have no code signature.

## Installation, upgrades, and removal

The Chinese-language wizard installs for the current user, using `%LOCALAPPDATA%\Programs\PasteWin` by default. It allows a different directory and independent desktop and Start menu shortcut choices. If WebView2 is missing, its bootstrapper needs internet access.

The finish page includes a checked-by-default “启动 PasteWin” option. Clicking Finish launches the installed executable when selected; clearing it leaves the application closed. Silent installation does not launch the application.

An upgrade reuses the registered directory and shortcut choices, which the user can change. If the executable is in use, the installer offers to close the matching PasteWin process and continue. It checks the full executable path and requests normal shutdown through the hidden main-thread window in Wails v3.0.0-beta.25, waiting up to ten seconds. It does not forcibly terminate an unresponsive process or close another installation. If shutdown or file access fails, the user can retry or cancel; replacement requires a successful write-access check. A copy failure triggers an attempt to restore the old executable.

Changing the install directory removes only the registered old program and uninstall files. Unregistered portable copies are not scanned or removed. Installing into a portable copy's directory can replace that executable.

Users can uninstall through Windows Installed Apps or `uninstall-pastewin.exe` in the installation directory. Upgrades and uninstall preserve `%APPDATA%\PasteWin\history.db` and WebView settings, as well as unrelated files in the installation directory.

Installation does not enable startup automatically. Same-directory upgrades preserve it. A directory change updates startup only when it points to the old installed executable; uninstall removes startup only when it points to the executable being removed. A separate portable copy's registration is retained.

## Silent installation

The installer accepts `/S`, `/DESKTOP=0|1`, `/STARTMENU=0|1`, and `/D=<directory>`. Append these arguments to the setup executable:

```text
/S /DESKTOP=0 /STARTMENU=1 /D=C:\Apps\PasteWin
```

`/D=` must be the last argument. NSIS expects the directory without embedded quote characters, even when it contains spaces. Omitted shortcut options preserve the previous choices, or default to enabled on first install. A locked executable makes silent installation return a nonzero exit code.

## Installer validation

```powershell
powershell -NoProfile -File build/windows/test-installer.ps1 -NsisPath 'C:\Tools\NSIS\makensis.exe'
```

The script uses a random application identifier, isolated directories, dummy executables, and isolated startup registry keys. It checks installation, same-directory upgrades, directory migration, locked files, shortcut choices, startup migration, uninstall, and protection of separate portable copies. A clipboard-free window fixture checks normal shutdown, exact executable-path matching, replacement after shutdown, silent-install refusal, and the timeout for an unresponsive application. It does not use the real clipboard database.

The automated checks complement a production installer build and visual checks of the directory, shortcut, and finish wizard pages. Recheck the hidden-window shutdown contract when upgrading Wails. Windows sign-in behavior requires a separate session-level check.

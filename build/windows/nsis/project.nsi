Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!include "nsDialogs.nsh"

!ifndef APP_ID
!define APP_ID "dev.pastewin.app"
!endif
!ifndef APP_NAME
!define APP_NAME "PasteWin"
!endif
!define EXE "pastewin.exe"
!define UNINSTALLER "uninstall-pastewin.exe"
!define KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_ID}"
!ifndef RUN_KEY
!define RUN_KEY "Software\Microsoft\Windows\CurrentVersion\Run"
!endif
!ifndef VERSION
!define VERSION "0.3.2"
!endif
!ifndef ARCH
!define ARCH "amd64"
!endif
!ifndef BINARY
!error "BINARY must point to the production executable"
!endif
!ifndef OUTPUT
!error "OUTPUT must specify the installer path"
!endif

Name "${APP_NAME}"
OutFile "${OUTPUT}"
InstallDir ""
RequestExecutionLevel user
ManifestDPIAware true
SetCompressor /SOLID lzma
VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "FileDescription" "${APP_NAME} Installer"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "Copyright 2026 PasteWin"
!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
Page custom ShortcutPage ShortcutLeave
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "启动 PasteWin"
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Var OldDir
Var DesktopChoice
Var MenuChoice
Var DesktopBox
Var MenuBox
Var OldRenamed
Var InstallMutex
Var StartupCommand

; One installer/uninstaller at a time, including during directory migration.
!macro InitShared
  SetShellVarContext current
  SetRegView 64
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\${APP_ID}.installer") p .r0 ?e'
  Pop $1
  StrCpy $InstallMutex $0
  ${If} $1 = 183
    MessageBox MB_OK|MB_ICONEXCLAMATION "另一个安装或卸载程序正在运行。" /SD IDOK
    SetErrorLevel 1618
    Quit
  ${EndIf}
!macroend

Function .onInit
  !insertmacro InitShared
  ${IfNot} ${AtLeastWin10}
    MessageBox MB_OK|MB_ICONSTOP "需要 Windows 10 或更新版本。" /SD IDOK
    SetErrorLevel 1633
    Quit
  ${EndIf}
!if "${ARCH}" == "amd64"
  ${IfNot} ${IsNativeAMD64}
!else if "${ARCH}" == "arm64"
  ${IfNot} ${IsNativeARM64}
!else
  !error "Unsupported ARCH"
!endif
    MessageBox MB_OK|MB_ICONSTOP "请选择与系统架构匹配的安装包（${ARCH}）。" /SD IDOK
    SetErrorLevel 1633
    Quit
  ${EndIf}
  ReadRegStr $OldDir HKCU "${KEY}" "InstallLocation"
  ; NSIS has already populated INSTDIR when /D was supplied.
  ${If} $INSTDIR == ""
    ${If} $OldDir != ""
      StrCpy $INSTDIR $OldDir
    ${Else}
      StrCpy $INSTDIR "$LOCALAPPDATA\Programs\${APP_NAME}"
    ${EndIf}
  ${EndIf}
  StrCpy $DesktopChoice ${BST_CHECKED}
  StrCpy $MenuChoice ${BST_CHECKED}
  ; Remember choices on upgrade. CLI switches override them for deployment.
  ClearErrors
  ReadRegDWORD $0 HKCU "${KEY}" "DesktopShortcut"
  ${IfNot} ${Errors}
    StrCpy $DesktopChoice $0
  ${EndIf}
  ClearErrors
  ReadRegDWORD $0 HKCU "${KEY}" "StartMenuShortcut"
  ${IfNot} ${Errors}
    StrCpy $MenuChoice $0
  ${EndIf}
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/DESKTOP=" $1
  ${IfNot} ${Errors}
    StrCpy $DesktopChoice $1
  ${EndIf}
  ClearErrors
  ${GetOptions} $0 "/STARTMENU=" $1
  ${IfNot} ${Errors}
    StrCpy $MenuChoice $1
  ${EndIf}
FunctionEnd

Function ShortcutPage
  !insertmacro MUI_HEADER_TEXT "快捷方式" "选择需要创建的快捷方式。"
  nsDialogs::Create 1018
  Pop $0
  ${NSD_CreateLabel} 0 0 100% 28u "仅为当前 Windows 用户安装。升级会替换旧版程序，保留剪贴板历史。"
  Pop $0
  ${NSD_CreateCheckbox} 0 40u 100% 14u "创建桌面快捷方式"
  Pop $DesktopBox
  ${NSD_SetState} $DesktopBox $DesktopChoice
  ${NSD_CreateCheckbox} 0 64u 100% 14u "创建开始菜单快捷方式"
  Pop $MenuBox
  ${NSD_SetState} $MenuBox $MenuChoice
  nsDialogs::Show
FunctionEnd

Function ShortcutLeave
  ${NSD_GetState} $DesktopBox $DesktopChoice
  ${NSD_GetState} $MenuBox $MenuChoice
FunctionEnd

; Rename alone can succeed for a running image. Test write access first so an
; upgrade cannot strand a still-running old version under another filename.
!include "close-running.nsh"
!macro CheckWritable PATH PREFIX
  ${If} ${FileExists} "${PATH}"
    ${Do}
      System::Call 'kernel32::CreateFileW(w "${PATH}", i 0x40000000, i 0, p 0, i 3, i 0, p 0) p .r0'
      ${If} $0 != -1
        System::Call 'kernel32::CloseHandle(p r0)'
        ${ExitDo}
      ${EndIf}
!if "${PREFIX}" != "uninstall"
      MessageBox MB_OKCANCEL|MB_ICONQUESTION "程序文件正在使用或无法写入：$\r$\n${PATH}$\r$\n点击“确定”关闭此目录中正在运行的 PasteWin 并继续安装。" /SD IDCANCEL IDCANCEL ${PREFIX}cancel
      Push "${PATH}"
      Call CloseRunningApp
      Pop $0
      ${If} $0 = 0
        Goto ${PREFIX}retry
      ${EndIf}
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "未能关闭程序或访问文件。请确认程序可以响应、安装目录可写，并使用相同用户权限重试。" /SD IDCANCEL IDRETRY ${PREFIX}retry
      ${PREFIX}cancel:
!else
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "无法删除程序文件：$\r$\n${PATH}$\r$\n请从托盘菜单退出 PasteWin 后重试，并确认目录可写。" /SD IDCANCEL IDRETRY ${PREFIX}retry
!endif
      SetErrorLevel 5
      Quit
      ${PREFIX}retry:
    ${Loop}
  ${EndIf}
!macroend

Function EnsureWebView
  ReadRegStr $0 HKLM "Software\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 != ""
  ${AndIf} $0 != "0.0.0.0"
    Return
  ${EndIf}
  ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 != ""
  ${AndIf} $0 != "0.0.0.0"
    Return
  ${EndIf}
  DetailPrint "正在安装 Microsoft WebView2 Runtime（需要联网）…"
  SetOutPath "$PLUGINSDIR"
  File "MicrosoftEdgeWebview2Setup.exe"
  ExecWait '"$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $0
  ${If} $0 != 0
  ${AndIf} $0 != 3010
    MessageBox MB_OK|MB_ICONSTOP "WebView2 安装失败（$0）。请联网安装 Microsoft WebView2 Runtime 后重试。" /SD IDOK
    SetErrorLevel 1603
    Quit
  ${EndIf}
FunctionEnd

Section "安装"
  InitPluginsDir
  Call EnsureWebView
  ${If} $OldDir != ""
  ${AndIf} $OldDir != $INSTDIR
    ; Never overwrite an unrelated executable at the new destination.
    ${If} ${FileExists} "$INSTDIR\${EXE}"
      MessageBox MB_OK|MB_ICONSTOP "目标目录已有 pastewin.exe。请选择空目录或原安装目录。" /SD IDOK
      SetErrorLevel 80
      Quit
    ${EndIf}
  ${EndIf}
  !insertmacro CheckWritable "$INSTDIR\${EXE}" target
  ${If} $OldDir != ""
  ${AndIf} $OldDir != $INSTDIR
    !insertmacro CheckWritable "$OldDir\${EXE}" source
  ${EndIf}
  ${If} $OldDir == ""
    StrCpy $OldDir $INSTDIR
  ${EndIf}
  ReadRegStr $StartupCommand HKCU "${RUN_KEY}" "${APP_ID}"
  ${If} $StartupCommand == '$\"$OldDir\${EXE}$\" --autostart'
    StrLen $0 '$\"$INSTDIR\${EXE}$\" --autostart'
    ${If} $0 > 260
      MessageBox MB_OK|MB_ICONSTOP "目标路径过长，无法保留开机自启设置，请选择较短的安装目录。" /SD IDOK
      SetErrorLevel 206
      Quit
    ${EndIf}
  ${EndIf}
  ${If} ${FileExists} "$OldDir\pastewin.previous.exe"
    MessageBox MB_OK|MB_ICONSTOP "旧目录存在 pastewin.previous.exe 备份。请先恢复或移走它，然后重试。" /SD IDOK
    SetErrorLevel 80
    Quit
  ${EndIf}
  SetOutPath "$PLUGINSDIR\payload"
  File /oname=pastewin.exe "${BINARY}"
  ClearErrors
  CreateDirectory "$INSTDIR"
  WriteUninstaller "$INSTDIR\${UNINSTALLER}"
  ${If} ${Errors}
    MessageBox MB_OK|MB_ICONSTOP "无法写入安装目录，请选择当前用户可写的目录。" /SD IDOK
    SetErrorLevel 5
    Quit
  ${EndIf}
  StrCpy $OldRenamed 0
  ${If} ${FileExists} "$OldDir\${EXE}"
    ClearErrors
    Rename "$OldDir\${EXE}" "$OldDir\pastewin.previous.exe"
    ${If} ${Errors}
      MessageBox MB_OK|MB_ICONSTOP "无法替换旧版程序，请退出 PasteWin 后重试。" /SD IDOK
      SetErrorLevel 5
      Quit
    ${EndIf}
    StrCpy $OldRenamed 1
  ${EndIf}
  ClearErrors
  CopyFiles /SILENT "$PLUGINSDIR\payload\${EXE}" "$INSTDIR\${EXE}"
  ${If} ${Errors}
    Delete "$INSTDIR\${EXE}"
    ${If} $OldRenamed = 1
      Rename "$OldDir\pastewin.previous.exe" "$OldDir\${EXE}"
    ${EndIf}
    MessageBox MB_OK|MB_ICONSTOP "安装失败，已尝试恢复旧版程序。请检查磁盘空间和目录权限。" /SD IDOK
    SetErrorLevel 1603
    Quit
  ${EndIf}
  ; Only migrate a startup entry owned by the installation being replaced.
  ${If} $OldDir != $INSTDIR
  ${AndIf} $StartupCommand == '$\"$OldDir\${EXE}$\" --autostart'
    ClearErrors
    WriteRegStr HKCU "${RUN_KEY}" "${APP_ID}" '$\"$INSTDIR\${EXE}$\" --autostart'
    ${If} ${Errors}
      Delete "$INSTDIR\${EXE}"
      ${If} $OldRenamed = 1
        Rename "$OldDir\pastewin.previous.exe" "$OldDir\${EXE}"
      ${EndIf}
      MessageBox MB_OK|MB_ICONSTOP "无法更新开机自启路径，已尝试恢复旧版程序。" /SD IDOK
      SetErrorLevel 5
      Quit
    ${EndIf}
  ${EndIf}
  Delete "$OldDir\pastewin.previous.exe"
  ${If} $OldDir != $INSTDIR
    Delete "$OldDir\${UNINSTALLER}"
    ; Only remove an empty directory: never recursively delete user files.
    RMDir "$OldDir"
  ${EndIf}
  SetOutPath "$INSTDIR"
  Delete "$DESKTOP\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}.lnk"
  ${If} $DesktopChoice = ${BST_CHECKED}
    CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${EXE}"
  ${EndIf}
  ${If} $MenuChoice = ${BST_CHECKED}
    CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${EXE}"
  ${EndIf}
  WriteRegStr HKCU "${KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${KEY}" "Publisher" "PasteWin"
  WriteRegStr HKCU "${KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${KEY}" "DisplayIcon" "$INSTDIR\${EXE}"
  WriteRegStr HKCU "${KEY}" "UninstallString" '$\"$INSTDIR\${UNINSTALLER}$\"'
  WriteRegStr HKCU "${KEY}" "QuietUninstallString" '$\"$INSTDIR\${UNINSTALLER}$\" /S'
  WriteRegDWORD HKCU "${KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${KEY}" "NoRepair" 1
  WriteRegDWORD HKCU "${KEY}" "DesktopShortcut" $DesktopChoice
  WriteRegDWORD HKCU "${KEY}" "StartMenuShortcut" $MenuChoice
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "${KEY}" "EstimatedSize" $0
SectionEnd

Function un.onInit
  !insertmacro InitShared
FunctionEnd

Section "Uninstall"
  !insertmacro CheckWritable "$INSTDIR\${EXE}" uninstall
  ReadRegStr $0 HKCU "${RUN_KEY}" "${APP_ID}"
  ${If} $0 == '$\"$INSTDIR\${EXE}$\" --autostart'
    ClearErrors
    DeleteRegValue HKCU "${RUN_KEY}" "${APP_ID}"
    ${If} ${Errors}
      SetErrorLevel 5
      Quit
    ${EndIf}
  ${EndIf}
  ClearErrors
  Delete "$INSTDIR\${EXE}"
  ${If} ${Errors}
    SetErrorLevel 5
    Quit
  ${EndIf}
  ; A copied stale uninstaller must not unregister a newer installation.
  ReadRegStr $0 HKCU "${KEY}" "InstallLocation"
  ${If} $0 == $INSTDIR
    Delete "$DESKTOP\${APP_NAME}.lnk"
    Delete "$SMPROGRAMS\${APP_NAME}.lnk"
    DeleteRegKey HKCU "${KEY}"
  ${EndIf}
  Delete "$INSTDIR\${UNINSTALLER}"
  RMDir "$INSTDIR"
  ; APPDATA\PasteWin and WebView profile are intentionally preserved.
SectionEnd

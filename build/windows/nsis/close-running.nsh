; Wails v3.0.0-beta.25 routes WM_CLOSE on this hidden window to App.Quit,
; including service shutdown. WM_CLOSE on the visible panel only hides it.
; Recheck this contract when upgrading Wails.
; Input: executable path on the stack. Output: 0 only after a matching process
; exits; 1 if not found, inaccessible, or still running after ten seconds.
Function CloseRunningApp
  Exch $R0
  Push $R1
  Push $R2
  Push $R3
  Push $R4
  Push $R5
  Push $R6
  StrCpy $R1 0
  StrCpy $R6 1
  ${Do}
    System::Call 'user32::FindWindowExW(p 0, p R1, w "WailsWebviewWindow", w "__wails_hidden_mainthread") p .R1'
    ${If} $R1 = 0
      ${ExitDo}
    ${EndIf}
    System::Call 'user32::GetWindowThreadProcessId(p R1, *i .R2)'
    ; Keep the process handle open through the wait, so PID reuse cannot cause
    ; us to wait on another process. No terminate-process access is requested.
    System::Call 'kernel32::OpenProcess(i 0x101000, i 0, i R2) p .R3'
    ${If} $R3 != 0
      StrCpy $R4 ${NSIS_MAX_STRLEN}
      System::Call 'kernel32::QueryFullProcessImageNameW(p R3, i 0, w .R5, *i R4) i .R4'
      ${If} $R4 != 0
      ${AndIf} $R5 == $R0
        System::Call 'user32::PostMessageW(p R1, i 0x10, p 0, p 0) i .R4'
        ${If} $R4 != 0
          System::Call 'kernel32::WaitForSingleObject(p R3, i 10000) i .R4'
          ${If} $R4 = 0
            StrCpy $R6 0
          ${EndIf}
        ${EndIf}
        System::Call 'kernel32::CloseHandle(p R3)'
        ${ExitDo}
      ${EndIf}
      System::Call 'kernel32::CloseHandle(p R3)'
    ${EndIf}
  ${Loop}
  StrCpy $R0 $R6
  Pop $R6
  Pop $R5
  Pop $R4
  Pop $R3
  Pop $R2
  Pop $R1
  Exch $R0
FunctionEnd

param([Parameter(Mandatory)][string]$NsisPath)
$ErrorActionPreference = 'Stop'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$NsisPath = (Resolve-Path -LiteralPath $NsisPath).Path
$testId = 'PasteWin-InstallerTest-' + [guid]::NewGuid().ToString('N')
$testRoot = Join-Path $repo ".work\$testId"
$oldDir = Join-Path $testRoot 'old path 中文'
$newDir = Join-Path $testRoot 'new path 中文'
$key = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\$testId"
$runKey = "Software\$testId-Run"
$runPath = "HKCU:\$runKey"
$desktopLink = Join-Path ([Environment]::GetFolderPath('Desktop')) "$testId.lnk"
$menuLink = Join-Path ([Environment]::GetFolderPath('Programs')) "$testId.lnk"
New-Item -ItemType Directory -Force $testRoot | Out-Null

function Assert([bool]$Condition, [string]$Message) {
    if (!$Condition) { throw $Message }
}
function Run-Installer([string]$Exe, [string]$Arguments, [int]$Expected = 0) {
    $process = Start-Process -FilePath $Exe -ArgumentList $Arguments -WindowStyle Hidden -PassThru
    if (!$process.WaitForExit(30000)) {
        Stop-Process -Id $process.Id -Force
        throw "Installer timed out: $Exe"
    }
    Assert ($process.ExitCode -eq $Expected) "Expected exit $Expected, got $($process.ExitCode): $Exe"
}
function Compile-Fixture([string]$Version, [string]$Contents) {
    $payload = Join-Path $testRoot "payload-$Version.exe"
    [IO.File]::WriteAllText($payload, $Contents)
    $setup = Join-Path $testRoot "setup-$Version.exe"
    & $NsisPath '/V2' '/INPUTCHARSET' 'UTF8' "/DAPP_ID=$testId" "/DAPP_NAME=$testId" "/DRUN_KEY=$runKey" "/DVERSION=$Version" "/DBINARY=$payload" "/DOUTPUT=$setup" "$repo\build\windows\nsis\project.nsi"
    if ($LASTEXITCODE -ne 0) { throw 'Fixture compilation failed.' }
    return $setup
}

try {
    # Use a separate registry identity, shortcut names and dummy payloads. Never
    # launch PasteWin or read/change the user's clipboard database in this test.
    $v1 = Compile-Fixture '0.1.0' 'version-one'
    $v2 = Compile-Fixture '0.2.0' 'version-two'
    Run-Installer $v1 "/S /DESKTOP=0 /STARTMENU=0 /D=$oldDir"
    Assert ((Get-Content "$oldDir\pastewin.exe" -Raw) -eq 'version-one') 'Initial payload missing'
    Assert (!(Test-Path $desktopLink) -and !(Test-Path $menuLink)) 'Unchecked shortcuts were created'
    Assert ((Get-ItemProperty $key).InstallLocation -eq $oldDir) 'Custom directory not registered'
    Set-Content "$oldDir\keep.txt" 'unrelated file'
    Set-Content "$oldDir\history.db" 'synthetic user data'
    Write-Host 'PASS: custom directory, no shortcuts'
    Assert (!(Test-Path $runPath)) 'Fresh install enabled startup'
    New-Item -Path $runPath -Force | Out-Null
    $oldCommand = '"' + $oldDir + '\pastewin.exe" --autostart'
    $newCommand = '"' + $newDir + '\pastewin.exe" --autostart'
    New-ItemProperty -Path $runPath -Name $testId -Value $oldCommand -PropertyType String | Out-Null
    New-ItemProperty -Path $runPath -Name 'unrelated' -Value 'keep' -PropertyType String | Out-Null

    $lock = [IO.File]::Open("$oldDir\pastewin.exe", 'Open', 'Read', 'None')
    try { Run-Installer $v2 "/S /D=$newDir" 5 } finally { $lock.Dispose() }
    Assert ((Get-Content "$oldDir\pastewin.exe" -Raw) -eq 'version-one') 'Locked upgrade changed old payload'
    Assert (!(Test-Path "$newDir\pastewin.exe")) 'Locked upgrade installed new payload'
    Assert ((Get-ItemProperty $key).InstallLocation -eq $oldDir) 'Locked upgrade changed registration'
    Assert ((Get-ItemPropertyValue $runPath $testId) -eq $oldCommand) 'Locked upgrade changed startup'
    Write-Host 'PASS: locked old binary blocks upgrade without damaging installation'

    Run-Installer $v2 '/S'
    Assert ((Get-Content "$oldDir\pastewin.exe" -Raw) -eq 'version-two') 'Same-directory upgrade failed'
    Assert (!(Test-Path "$oldDir\pastewin.previous.exe")) 'Old binary backup left behind'
    Assert ((Get-ItemProperty $key).DisplayVersion -eq '0.2.0') 'Version not updated'
    Assert (!(Test-Path $desktopLink) -and !(Test-Path $menuLink)) 'Shortcut preference not preserved'
    Write-Host 'PASS: same-directory upgrade, remembered choices, old binary removed'
    Assert ((Get-ItemPropertyValue $runPath $testId) -eq $oldCommand) 'Same-directory upgrade lost startup'

    Run-Installer $v2 "/S /DESKTOP=1 /STARTMENU=1 /D=$newDir"
    Assert (!(Test-Path "$oldDir\pastewin.exe")) 'Old location still has binary'
    Assert (!(Test-Path "$oldDir\uninstall-pastewin.exe")) 'Old uninstaller left behind'
    Assert (Test-Path "$oldDir\keep.txt") 'Unrelated old file deleted'
    Assert ((Get-Content "$oldDir\history.db" -Raw).Trim() -eq 'synthetic user data') 'Data deleted during migration'
    Assert ((Get-Content "$newDir\pastewin.exe" -Raw) -eq 'version-two') 'Migration lost binary'
    Assert ((Get-ItemProperty $key).InstallLocation -eq $newDir) 'Migration registration incorrect'
    $shell = New-Object -ComObject WScript.Shell
    foreach ($link in @($desktopLink, $menuLink)) {
        Assert (Test-Path $link) 'Selected shortcut missing'
        Assert ($shell.CreateShortcut($link).TargetPath -eq "$newDir\pastewin.exe") 'Shortcut target incorrect'
    }
    Write-Host 'PASS: directory migration, both shortcuts, unrelated files and data preserved'
    Assert ((Get-ItemPropertyValue $runPath $testId) -eq $newCommand) 'Migration left stale startup path'

    Run-Installer $v2 '/S /DESKTOP=0 /STARTMENU=1'
    Assert (!(Test-Path $desktopLink) -and (Test-Path $menuLink)) 'Shortcut choices not independent'
    Set-Content "$newDir\keep.txt" 'retain after uninstall'
    $uninstaller = Join-Path $testRoot 'uninstall-test.exe'
    Copy-Item "$newDir\uninstall-pastewin.exe" $uninstaller
    Run-Installer $uninstaller "/S _?=$newDir"
    Assert (!(Test-Path "$newDir\pastewin.exe")) 'Uninstall left binary'
    Assert (!(Test-Path $key)) 'Uninstall left registration'
    Assert (!(Test-Path $desktopLink) -and !(Test-Path $menuLink)) 'Uninstall left shortcuts'
    Assert (Test-Path "$newDir\keep.txt") 'Uninstall deleted unrelated file'
    Assert (Test-Path "$oldDir\history.db") 'Uninstall deleted data'
    Assert ($null -eq (Get-ItemProperty $runPath).$testId) 'Uninstall left startup registration'
    Assert ((Get-ItemPropertyValue $runPath 'unrelated') -eq 'keep') 'Uninstall changed unrelated startup'
    Write-Host 'PASS: independent shortcut choices, uninstall preserves unrelated files and data'

    # A portable copy's registration must survive installation and uninstall elsewhere.
    New-ItemProperty -Path $runPath -Name $testId -Value $oldCommand -PropertyType String | Out-Null
    Run-Installer $v2 "/S /DESKTOP=0 /STARTMENU=0 /D=$newDir"
    Assert ((Get-ItemPropertyValue $runPath $testId) -eq $oldCommand) 'Install changed portable startup'
    Copy-Item "$newDir\uninstall-pastewin.exe" $uninstaller -Force
    Run-Installer $uninstaller "/S _?=$newDir"
    Assert ((Get-ItemPropertyValue $runPath $testId) -eq $oldCommand) 'Uninstall changed portable startup'
    Write-Host 'PASS: startup migration, cleanup and portable-copy ownership'
} finally {
    # These names include a fresh GUID and are exclusively owned by this test.
    Remove-Item -LiteralPath $desktopLink, $menuLink -Force -ErrorAction SilentlyContinue
    if (Test-Path $key) { Remove-Item -LiteralPath $key -Recurse -Force }
    if (Test-Path $runPath) { Remove-Item -LiteralPath $runPath -Force }
    $resolved = [IO.Path]::GetFullPath($testRoot)
    $allowed = [IO.Path]::GetFullPath((Join-Path $repo '.work')) + '\'
    if (!$resolved.StartsWith($allowed, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe test cleanup path' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

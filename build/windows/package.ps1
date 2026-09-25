param(
    [ValidateSet('amd64', 'arm64')][string]$Arch = 'amd64',
    [string]$NsisPath,
    [string]$OutputDir = 'bin'
)
$ErrorActionPreference = 'Stop'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if (!$NsisPath) {
    $compiler = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($compiler) { $NsisPath = $compiler.Source }
    else {
        $candidates = @(
            "${env:ProgramFiles(x86)}\NSIS\makensis.exe",
            "$env:ProgramFiles\NSIS\makensis.exe",
            (Join-Path $repo '.work\tools\nsis-3.12\makensis.exe')
        )
        $NsisPath = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    }
}
if (!$NsisPath -or !(Test-Path -LiteralPath $NsisPath)) {
    throw 'Install NSIS 3 (https://nsis.sourceforge.io/Download), or pass -NsisPath <makensis.exe>.'
}
$NsisPath = (Resolve-Path -LiteralPath $NsisPath).Path
$wails = Get-Command wails3.exe -ErrorAction SilentlyContinue
if (!$wails) {
    $gopath = & go env GOPATH
    if ($LASTEXITCODE -ne 0) { throw 'Go is required.' }
    $wailsPath = Join-Path $gopath 'bin\wails3.exe'
} else { $wailsPath = $wails.Source }
if (!(Test-Path -LiteralPath $wailsPath)) { throw 'Install the project version of Wails CLI first (see README).' }

Push-Location $repo
try {
    # Stage separately so packaging does not overwrite a running development EXE.
    & $wailsPath build "ARCH=$Arch" "BIN_DIR=.work/package-$Arch"
    if ($LASTEXITCODE -ne 0) { throw 'Production build failed.' }
    & $wailsPath generate webview2bootstrapper -dir (Join-Path $repo 'build\windows\nsis')
    if ($LASTEXITCODE -ne 0) { throw 'WebView2 bootstrapper download failed.' }
    $version = (Get-Content build\windows\info.json -Raw | ConvertFrom-Json).fixed.file_version
    if ($version -notmatch '^\d+\.\d+\.\d+$') { throw "Expected a three-part version, got $version" }
    if (![IO.Path]::IsPathRooted($OutputDir)) { $OutputDir = Join-Path $repo $OutputDir }
    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
    $output = Join-Path $OutputDir "pastewin-$version-$Arch-setup.exe"
    & $NsisPath '/V3' '/INPUTCHARSET' 'UTF8' "/DARCH=$Arch" "/DVERSION=$version" "/DBINARY=$repo\.work\package-$Arch\pastewin.exe" "/DOUTPUT=$output" "$repo\build\windows\nsis\project.nsi"
    if ($LASTEXITCODE -ne 0) { throw 'NSIS compilation failed.' }
    Write-Host "Installer: $output"
} finally { Pop-Location }

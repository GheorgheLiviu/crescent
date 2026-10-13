# Installs Ghidra (ghidra-sre.org), the NSA's reverse engineering tool, from its GitHub releases.
# Ghidra has no winget package: it ships as a zip, so this unpacks it into Program Files and adds
# a Start menu shortcut. It needs Java 21 or newer and installs Temurin 21 if none is found.
# Safe to run twice (it replaces Ghidra with the latest version).
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# Java installed a moment ago isn't on this window's PATH yet, so look in its usual folders too.
$jdks = Get-ChildItem "$env:ProgramFiles\Eclipse Adoptium", "$env:ProgramFiles\Java", "$env:ProgramFiles\Microsoft" -Directory -Filter 'jdk-*' -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -match '^jdk-(\d+)' -and [int]$Matches[1] -ge 21 }
if (-not $jdks) {
    Write-Host 'Installing Java 21 (Temurin), which Ghidra needs...'
    winget install --id EclipseAdoptium.Temurin.21.JDK --exact --silent --accept-package-agreements --accept-source-agreements
}

$release = Invoke-RestMethod 'https://api.github.com/repos/NationalSecurityAgency/ghidra/releases/latest'
$asset = $release.assets | Where-Object { $_.name -like 'ghidra_*_PUBLIC_*.zip' } | Select-Object -First 1
if (-not $asset) { Write-Host 'Could not find the Ghidra download.'; exit 1 }

Write-Host "Downloading $($asset.name)..."
$zip = Join-Path $env:TEMP $asset.name
$tmp = Join-Path $env:TEMP 'ghidra-unpack'
Invoke-WebRequest $asset.browser_download_url -OutFile $zip
Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
Expand-Archive $zip $tmp
$dest = "$env:ProgramFiles\Ghidra"
Remove-Item $dest -Recurse -Force -ErrorAction SilentlyContinue
# The zip holds one folder, ghidra_<version>_PUBLIC.
Move-Item (Get-ChildItem $tmp -Directory | Select-Object -First 1).FullName $dest
Remove-Item $zip, $tmp -Recurse -Force

$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut("$env:ProgramData\Microsoft\Windows\Start Menu\Programs\Ghidra.lnk")
$link.TargetPath = "$dest\ghidraRun.bat"
$link.WorkingDirectory = $dest
$link.IconLocation = "$dest\support\ghidra.ico"
$link.WindowStyle = 7  # minimized, so the launcher's console window stays out of the way
$link.Save()
Write-Host "Ghidra $($release.tag_name) is installed. Open it from the Start menu."

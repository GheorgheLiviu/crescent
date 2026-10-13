# Crescent: run the page on this PC.
#   .\setup.cmd         open the page at http://localhost:8765 (close this window to stop it)
#   .\setup.cmd -Dev    first install what building and testing need (Go, Node, the test browser),
#                       then run the tests, then open the page
param([switch]$Dev, [int]$Port = 8765)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Have($cmd) { [bool](Get-Command $cmd -CommandType Application -ErrorAction SilentlyContinue) }
function Check($what) { if ($LASTEXITCODE -ne 0) { throw "$what failed (exit code $LASTEXITCODE)." } }
# The Path this window started with can be older than the tools installed since; read it fresh.
function Refresh-Path { $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User") }
Refresh-Path
# Windows ships a "python" that only opens the Microsoft Store, so check that it really runs.
function Find-Python {
  foreach ($cmd in "python", "py") {
    if (Have $cmd) { & $cmd -c "import sys" 2>$null; if ($LASTEXITCODE -eq 0) { return $cmd } }
  }
  return $null
}
# Installs a tool with winget when it's missing.
function Need($cmd, $id) {
  if (Have $cmd) { return }
  if (-not (Have winget)) { throw "$cmd is missing and winget isn't available to install it. Install $id by hand." }
  Write-Host "Installing $id..."
  winget install --id $id --exact --silent --accept-package-agreements --accept-source-agreements
  # 0x8A150061 and 0x8A15002B: already installed, which is fine.
  if ($LASTEXITCODE -ne 0 -and @(-1978335135, -1978335189) -notcontains $LASTEXITCODE) { Check "Installing $id" }
  Refresh-Path
  if (-not (Have $cmd)) { throw "$id was installed, but $cmd isn't found yet. Open a new window and run setup again." }
}

if ($Dev) {
  Need go GoLang.Go
  Need node OpenJS.NodeJS.LTS
  Need git Git.Git  # tests/run.sh runs in Git Bash
  Push-Location tests
  npm install --no-audit --no-fund; Check "npm install"
  npx playwright install chromium; Check "Installing the test browser"
  Pop-Location
  $sh = Join-Path (Split-Path (Split-Path (Get-Command git).Source)) "bin\sh.exe"
  & $sh tests/run.sh; Check "The tests"
  Write-Host "Setup done. To rebuild the installer after changing installer/, see docs/DEVELOPING.md, Build the installer."
}

$url = "http://localhost:$Port/"
$py = Find-Python
if ($py) {
  Write-Host "Crescent page: $url (close this window to stop it)"
  Start-Process $url
  & $py -m http.server $Port --bind 127.0.0.1 --directory web
} else {
  # Without Python the page still works opened straight from the file.
  Write-Host "Python isn't installed, so the page opens straight from its file."
  Start-Process (Join-Path $PSScriptRoot "web\index.html")
}

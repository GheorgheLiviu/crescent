# Installs REA (rea.tools), the reverse-engineering tools for AI agents, with npm.
# Needs Node.js 22 or newer: tick Node.js LTS on the page. Safe to run twice (it updates REA).

# Node installed a moment ago isn't on this window's PATH yet, so add its usual folders.
$env:Path = "$env:ProgramFiles\nodejs;$env:APPDATA\npm;$env:Path"
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Write-Host 'Node.js is not installed, so REA was not. Tick Node.js LTS and run this again.'
    exit 1
}

npm install --global rea-agents
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host 'REA is installed. To connect it to Claude, Cursor or another agent, run: npx rea-agents setup'

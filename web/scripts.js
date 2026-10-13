// Built by tools/build_scripts.py from scripts/scripts.json. Don't edit it here.
window.SCRIPT_LIBRARY = [
 {
  "id": "show-file-extensions",
  "name": "Show file extensions",
  "description": "Shows file extensions like .pdf and .exe, and hidden files, in File Explorer (Finder on a Mac).",
  "ro": {
   "name": "Afișează extensiile fișierelor",
   "description": "Afișează extensii precum .pdf și .exe, și fișierele ascunse, în File Explorer (Finder pe Mac)."
  },
  "windows": {
   "name": "show-file-extensions.ps1",
   "kind": "ps1",
   "body": "# Show file extensions and hidden files in File Explorer, for the signed-in user.\n# Crescent runs this as administrator after all apps are installed.\n\n$key = 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced'\n\n# 0 shows extensions such as .pdf and .exe (1 hides them).\nNew-ItemProperty -Path $key -Name HideFileExt -Value 0 -PropertyType DWord -Force | Out-Null\n# 1 shows hidden files and folders (2 hides them).\nNew-ItemProperty -Path $key -Name Hidden -Value 1 -PropertyType DWord -Force | Out-Null\n\n# Restart File Explorer so the change shows now. Windows starts it again by itself.\nStop-Process -Name explorer -Force -ErrorAction SilentlyContinue\nWrite-Host 'File extensions and hidden files are now shown.'\n"
  },
  "mac": {
   "name": "show-file-extensions.sh",
   "kind": "sh",
   "body": "# Shows file extensions (like .pdf) and hidden files in Finder\ndefaults write NSGlobalDomain AppleShowAllExtensions -bool true\ndefaults write com.apple.finder AppleShowAllFiles -bool true\nkillall Finder\n"
  }
 },
 {
  "id": "dark-mode",
  "name": "Dark mode",
  "description": "Switches the system and apps to dark mode.",
  "ro": {
   "name": "Mod întunecat",
   "description": "Trece sistemul și aplicațiile în modul întunecat."
  },
  "windows": {
   "name": "dark-mode.ps1",
   "kind": "ps1",
   "body": "# Dark mode for Windows and apps, for the signed-in user.\n# Crescent runs this as administrator after all apps are installed.\n\n$key = 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize'\nif (-not (Test-Path $key)) { New-Item -Path $key | Out-Null }\n\n# 0 means dark. Apps: Settings, File Explorer and other apps. System: the taskbar and Start.\nNew-ItemProperty -Path $key -Name AppsUseLightTheme -Value 0 -PropertyType DWord -Force | Out-Null\nNew-ItemProperty -Path $key -Name SystemUsesLightTheme -Value 0 -PropertyType DWord -Force | Out-Null\n\nWrite-Host 'Dark mode is on. Apps that are already open switch the next time they start.'\n"
  },
  "mac": {
   "name": "dark-mode.sh",
   "kind": "sh",
   "body": "# Switches the Mac and apps to dark mode. The Mac may ask to let Crescent control System Events.\nosascript -e 'tell application \"System Events\" to tell appearance preferences to set dark mode to true'\n"
  }
 },
 {
  "id": "monthly-printer-test",
  "name": "Monthly printer test page",
  "description": "Prints a colour test page on the 1st of every month, so the ink or toner doesn't dry out. It finds the printer by itself each time.",
  "ro": {
   "name": "Pagină de test lunară la imprimantă",
   "description": "Tipărește o pagină de test color pe 1 ale fiecărei luni, ca să nu se usuce cerneala sau tonerul. Găsește singur imprimanta de fiecare dată."
  },
  "windows": {
   "name": "monthly-printer-test.ps1",
   "kind": "ps1",
   "body": "# Monthly printer test page, so the ink or toner doesn't dry out.\n# Adds a Task Scheduler task that prints one page of colour bars on the 1st\n# of every month at 12:00, or the next time the PC is on, on the default printer (or the first real\n# one if the default only makes files). Safe to run twice: it replaces the task.\n# Crescent runs this as administrator after all apps are installed.\n\n$dir = Join-Path $env:ProgramData 'Crescent'\n$page = Join-Path $dir 'printer-test.ps1'\nNew-Item -ItemType Directory -Force -Path $dir | Out-Null\n\n# The page: the date and five colour bars (cyan, magenta, yellow, black, grey). The printer is picked\n# each time it runs, so a new printer is used without setting anything up again.\n@'\nAdd-Type -AssemblyName System.Drawing\n$doc = New-Object System.Drawing.Printing.PrintDocument\n$doc.DocumentName = 'Monthly printer test'\n# The default printer, unless there is none or it only makes files (PDF, XPS, OneNote, fax), which\n# would open a Save dialog: then the first real printer. No real printer yet: try again next month.\n$virtual = 'PDF|XPS|OneNote|Fax|Send To'\nif (-not $doc.PrinterSettings.IsValid -or $doc.PrinterSettings.PrinterName -match $virtual) {\n  $real = [System.Drawing.Printing.PrinterSettings]::InstalledPrinters | Where-Object { $_ -notmatch $virtual } | Select-Object -First 1\n  if (-not $real) { exit 0 }\n  $doc.PrinterSettings.PrinterName = $real\n}\n$doc.add_PrintPage({\n  param($s, $e)\n  $g = $e.Graphics\n  $font = New-Object System.Drawing.Font('Arial', 14)\n  $g.DrawString('Monthly printer test - ' + (Get-Date -Format 'yyyy-MM-dd'), $font, [System.Drawing.Brushes]::Black, 50, 40)\n  $colors = @(\n    @('Cyan', [System.Drawing.Color]::FromArgb(0, 174, 239)),\n    @('Magenta', [System.Drawing.Color]::FromArgb(236, 0, 140)),\n    @('Yellow', [System.Drawing.Color]::FromArgb(255, 242, 0)),\n    @('Black', [System.Drawing.Color]::Black),\n    @('Grey', [System.Drawing.Color]::Gray)\n  )\n  $y = 90\n  foreach ($c in $colors) {\n    $g.FillRectangle((New-Object System.Drawing.SolidBrush($c[1])), 50, $y, 700, 120)\n    $g.DrawString($c[0], $font, [System.Drawing.Brushes]::Black, 55, $y + 125)\n    $y += 160\n  }\n  $e.HasMorePages = $false\n})\n$doc.Print()\n'@ | Set-Content -Path $page -Encoding UTF8\n\n# The task runs as the signed-in user (printers belong to the user), only while they're signed in.\n$user = [Security.SecurityElement]::Escape([Security.Principal.WindowsIdentity]::GetCurrent().Name)\n$taskArgs = [Security.SecurityElement]::Escape(\"-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `\"$page`\"\")\n$months = ('January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December' | ForEach-Object { \"<$_ />\" }) -join ''\n$xml = @\"\n<?xml version=\"1.0\" encoding=\"UTF-16\"?>\n<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\n  <RegistrationInfo><Description>Prints a colour test page each month so the printer's ink or toner doesn't dry out. Added by Crescent.<\/Description><\/RegistrationInfo>\n  <Triggers>\n    <CalendarTrigger>\n      <StartBoundary>2026-01-01T12:00:00<\/StartBoundary>\n      <ScheduleByMonth><DaysOfMonth><Day>1<\/Day><\/DaysOfMonth><Months>$months<\/Months><\/ScheduleByMonth>\n    <\/CalendarTrigger>\n  <\/Triggers>\n  <Principals><Principal id=\"Author\"><UserId>$user<\/UserId><LogonType>InteractiveToken<\/LogonType><RunLevel>LeastPrivilege<\/RunLevel><\/Principal><\/Principals>\n  <Settings>\n    <StartWhenAvailable>true<\/StartWhenAvailable>\n    <DisallowStartIfOnBatteries>false<\/DisallowStartIfOnBatteries>\n    <StopIfGoingOnBatteries>false<\/StopIfGoingOnBatteries>\n    <ExecutionTimeLimit>PT10M<\/ExecutionTimeLimit>\n  <\/Settings>\n  <Actions Context=\"Author\"><Exec><Command>powershell.exe<\/Command><Arguments>$taskArgs<\/Arguments><\/Exec><\/Actions>\n<\/Task>\n\"@\n# schtasks reads task XML reliably only as UTF-16 with a byte order mark.\n$xmlFile = Join-Path $env:TEMP 'crescent-printer-task.xml'\nSet-Content -Path $xmlFile -Value $xml -Encoding Unicode\nschtasks /Create /TN 'Crescent\\Monthly printer test' /XML $xmlFile /F\n$ok = $LASTEXITCODE -eq 0\nRemove-Item $xmlFile -ErrorAction SilentlyContinue\nif (-not $ok) { Write-Host \"Couldn't add the monthly printer test task.\"; exit 1 }\n\nWrite-Host 'Monthly printer test is on: a colour page prints on the 1st of every month, on the default printer.'\nWrite-Host 'To print one now, run:' \"powershell -ExecutionPolicy Bypass -File `\"$page`\"\"\nWrite-Host 'To turn it off, delete \"Crescent\\Monthly printer test\" in Task Scheduler.'\n"
  },
  "mac": null
 },
 {
  "id": "install-rea",
  "name": "Install REA (AI reverse engineering)",
  "description": "Installs REA from rea.tools, which lets AI agents such as Claude take apart apps and programs. Needs Node.js LTS ticked. Connect it to your agent afterwards with: npx rea-agents setup",
  "ro": {
   "name": "Instalează REA (reverse engineering cu AI)",
   "description": "Instalează REA de pe rea.tools, care le permite agenților AI precum Claude să analizeze aplicații și programe. Are nevoie de Node.js LTS bifat. După aceea îl conectezi la agent cu: npx rea-agents setup"
  },
  "windows": {
   "name": "install-rea.ps1",
   "kind": "ps1",
   "body": "# Installs REA (rea.tools), the reverse-engineering tools for AI agents, with npm.\n# Needs Node.js 22 or newer: tick Node.js LTS on the page. Safe to run twice (it updates REA).\n\n# Node installed a moment ago isn't on this window's PATH yet, so add its usual folders.\n$env:Path = \"$env:ProgramFiles\\nodejs;$env:APPDATA\\npm;$env:Path\"\nif (-not (Get-Command npm -ErrorAction SilentlyContinue)) {\n    Write-Host 'Node.js is not installed, so REA was not. Tick Node.js LTS and run this again.'\n    exit 1\n}\n\nnpm install --global rea-agents\nif ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\nWrite-Host 'REA is installed. To connect it to Claude, Cursor or another agent, run: npx rea-agents setup'\n"
  },
  "mac": {
   "name": "install-rea.sh",
   "kind": "sh",
   "body": "# Installs REA (rea.tools), the reverse-engineering tools for AI agents, with npm.\n# Needs Node.js 22 or newer: tick Node.js LTS on the page. Safe to run twice (it updates REA).\n\n# Homebrew's node@22 is not linked into bin, so add its folder too.\nexport PATH=\"/opt/homebrew/opt/node@22/bin:/usr/local/opt/node@22/bin:/opt/homebrew/bin:/usr/local/bin:$PATH\"\nif ! command -v npm >/dev/null; then\n  echo 'Node.js is not installed, so REA was not. Tick Node.js LTS and run this again.'\n  exit 1\nfi\n\n# Install into the user's folder, so it needs no administrator password.\nnpm install --global --prefix \"$HOME/.npm-global\" rea-agents || exit $?\necho \"REA is installed in ~/.npm-global/bin. To connect it to Claude, Cursor or another agent, run: npx rea-agents setup\"\n"
  }
 },
 {
  "id": "install-ghidra",
  "name": "Install Ghidra (reverse engineering)",
  "description": "Installs Ghidra, the free reverse engineering tool from the NSA, which takes apart programs to show how they work. Also installs the Java it needs.",
  "ro": {
   "name": "Instalează Ghidra (reverse engineering)",
   "description": "Instalează Ghidra, unealta gratuită de reverse engineering de la NSA, care desface programele ca să vezi cum funcționează. Instalează și Java de care are nevoie."
  },
  "windows": {
   "name": "install-ghidra.ps1",
   "kind": "ps1",
   "body": "# Installs Ghidra (ghidra-sre.org), the NSA's reverse engineering tool, from its GitHub releases.\n# Ghidra has no winget package: it ships as a zip, so this unpacks it into Program Files and adds\n# a Start menu shortcut. It needs Java 21 or newer and installs Temurin 21 if none is found.\n# Safe to run twice (it replaces Ghidra with the latest version).\n$ErrorActionPreference = 'Stop'\n$ProgressPreference = 'SilentlyContinue'\n\n# Java installed a moment ago isn't on this window's PATH yet, so look in its usual folders too.\n$jdks = Get-ChildItem \"$env:ProgramFiles\\Eclipse Adoptium\", \"$env:ProgramFiles\\Java\", \"$env:ProgramFiles\\Microsoft\" -Directory -Filter 'jdk-*' -ErrorAction SilentlyContinue |\n    Where-Object { $_.Name -match '^jdk-(\\d+)' -and [int]$Matches[1] -ge 21 }\nif (-not $jdks) {\n    Write-Host 'Installing Java 21 (Temurin), which Ghidra needs...'\n    winget install --id EclipseAdoptium.Temurin.21.JDK --exact --silent --accept-package-agreements --accept-source-agreements\n}\n\n$release = Invoke-RestMethod 'https://api.github.com/repos/NationalSecurityAgency/ghidra/releases/latest'\n$asset = $release.assets | Where-Object { $_.name -like 'ghidra_*_PUBLIC_*.zip' } | Select-Object -First 1\nif (-not $asset) { Write-Host 'Could not find the Ghidra download.'; exit 1 }\n\nWrite-Host \"Downloading $($asset.name)...\"\n$zip = Join-Path $env:TEMP $asset.name\n$tmp = Join-Path $env:TEMP 'ghidra-unpack'\nInvoke-WebRequest $asset.browser_download_url -OutFile $zip\nRemove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue\nExpand-Archive $zip $tmp\n$dest = \"$env:ProgramFiles\\Ghidra\"\nRemove-Item $dest -Recurse -Force -ErrorAction SilentlyContinue\n# The zip holds one folder, ghidra_<version>_PUBLIC.\nMove-Item (Get-ChildItem $tmp -Directory | Select-Object -First 1).FullName $dest\nRemove-Item $zip, $tmp -Recurse -Force\n\n$shell = New-Object -ComObject WScript.Shell\n$link = $shell.CreateShortcut(\"$env:ProgramData\\Microsoft\\Windows\\Start Menu\\Programs\\Ghidra.lnk\")\n$link.TargetPath = \"$dest\\ghidraRun.bat\"\n$link.WorkingDirectory = $dest\n$link.IconLocation = \"$dest\\support\\ghidra.ico\"\n$link.WindowStyle = 7  # minimized, so the launcher's console window stays out of the way\n$link.Save()\nWrite-Host \"Ghidra $($release.tag_name) is installed. Open it from the Start menu.\"\n"
  },
  "mac": {
   "name": "install-ghidra.sh",
   "kind": "sh",
   "body": "# Installs Ghidra (ghidra-sre.org), the NSA's reverse engineering tool, with Homebrew.\n# Homebrew also installs the Java it needs. Safe to run twice.\n\nexport PATH=\"/opt/homebrew/bin:/usr/local/bin:$PATH\"\nif ! command -v brew >/dev/null; then\n  echo 'Homebrew is not installed, so Ghidra was not.'\n  exit 1\nfi\n\nbrew install --cask ghidra || exit $?\necho 'Ghidra is installed. Run it with: ghidraRun'\n"
  }
 }
];

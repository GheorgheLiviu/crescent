# Monthly printer test page, so the ink or toner doesn't dry out.
# Adds a Task Scheduler task that prints one page of colour bars on the 1st
# of every month at 12:00, or the next time the PC is on, on the default printer (or the first real
# one if the default only makes files). Safe to run twice: it replaces the task.
# Crescent runs this as administrator after all apps are installed.

$dir = Join-Path $env:ProgramData 'Crescent'
$page = Join-Path $dir 'printer-test.ps1'
New-Item -ItemType Directory -Force -Path $dir | Out-Null

# The page: the date and five colour bars (cyan, magenta, yellow, black, grey). The printer is picked
# each time it runs, so a new printer is used without setting anything up again.
@'
Add-Type -AssemblyName System.Drawing
$doc = New-Object System.Drawing.Printing.PrintDocument
$doc.DocumentName = 'Monthly printer test'
# The default printer, unless there is none or it only makes files (PDF, XPS, OneNote, fax), which
# would open a Save dialog: then the first real printer. No real printer yet: try again next month.
$virtual = 'PDF|XPS|OneNote|Fax|Send To'
if (-not $doc.PrinterSettings.IsValid -or $doc.PrinterSettings.PrinterName -match $virtual) {
  $real = [System.Drawing.Printing.PrinterSettings]::InstalledPrinters | Where-Object { $_ -notmatch $virtual } | Select-Object -First 1
  if (-not $real) { exit 0 }
  $doc.PrinterSettings.PrinterName = $real
}
$doc.add_PrintPage({
  param($s, $e)
  $g = $e.Graphics
  $font = New-Object System.Drawing.Font('Arial', 14)
  $g.DrawString('Monthly printer test - ' + (Get-Date -Format 'yyyy-MM-dd'), $font, [System.Drawing.Brushes]::Black, 50, 40)
  $colors = @(
    @('Cyan', [System.Drawing.Color]::FromArgb(0, 174, 239)),
    @('Magenta', [System.Drawing.Color]::FromArgb(236, 0, 140)),
    @('Yellow', [System.Drawing.Color]::FromArgb(255, 242, 0)),
    @('Black', [System.Drawing.Color]::Black),
    @('Grey', [System.Drawing.Color]::Gray)
  )
  $y = 90
  foreach ($c in $colors) {
    $g.FillRectangle((New-Object System.Drawing.SolidBrush($c[1])), 50, $y, 700, 120)
    $g.DrawString($c[0], $font, [System.Drawing.Brushes]::Black, 55, $y + 125)
    $y += 160
  }
  $e.HasMorePages = $false
})
$doc.Print()
'@ | Set-Content -Path $page -Encoding UTF8

# The task runs as the signed-in user (printers belong to the user), only while they're signed in.
$user = [Security.SecurityElement]::Escape([Security.Principal.WindowsIdentity]::GetCurrent().Name)
$taskArgs = [Security.SecurityElement]::Escape("-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$page`"")
$months = ('January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December' | ForEach-Object { "<$_ />" }) -join ''
$xml = @"
<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>Prints a colour test page each month so the printer's ink or toner doesn't dry out. Added by Crescent.</Description></RegistrationInfo>
  <Triggers>
    <CalendarTrigger>
      <StartBoundary>2026-01-01T12:00:00</StartBoundary>
      <ScheduleByMonth><DaysOfMonth><Day>1</Day></DaysOfMonth><Months>$months</Months></ScheduleByMonth>
    </CalendarTrigger>
  </Triggers>
  <Principals><Principal id="Author"><UserId>$user</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>
  <Settings>
    <StartWhenAvailable>true</StartWhenAvailable>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT10M</ExecutionTimeLimit>
  </Settings>
  <Actions Context="Author"><Exec><Command>powershell.exe</Command><Arguments>$taskArgs</Arguments></Exec></Actions>
</Task>
"@
# schtasks reads task XML reliably only as UTF-16 with a byte order mark.
$xmlFile = Join-Path $env:TEMP 'crescent-printer-task.xml'
Set-Content -Path $xmlFile -Value $xml -Encoding Unicode
schtasks /Create /TN 'Crescent\Monthly printer test' /XML $xmlFile /F
$ok = $LASTEXITCODE -eq 0
Remove-Item $xmlFile -ErrorAction SilentlyContinue
if (-not $ok) { Write-Host "Couldn't add the monthly printer test task."; exit 1 }

Write-Host 'Monthly printer test is on: a colour page prints on the 1st of every month, on the default printer.'
Write-Host 'To print one now, run:' "powershell -ExecutionPolicy Bypass -File `"$page`""
Write-Host 'To turn it off, delete "Crescent\Monthly printer test" in Task Scheduler.'

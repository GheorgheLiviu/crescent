# Dark mode for Windows and apps, for the signed-in user.
# Crescent runs this as administrator after all apps are installed.

$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize'
if (-not (Test-Path $key)) { New-Item -Path $key | Out-Null }

# 0 means dark. Apps: Settings, File Explorer and other apps. System: the taskbar and Start.
New-ItemProperty -Path $key -Name AppsUseLightTheme -Value 0 -PropertyType DWord -Force | Out-Null
New-ItemProperty -Path $key -Name SystemUsesLightTheme -Value 0 -PropertyType DWord -Force | Out-Null

Write-Host 'Dark mode is on. Apps that are already open switch the next time they start.'

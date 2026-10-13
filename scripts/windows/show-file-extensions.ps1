# Show file extensions and hidden files in File Explorer, for the signed-in user.
# Crescent runs this as administrator after all apps are installed.

$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced'

# 0 shows extensions such as .pdf and .exe (1 hides them).
New-ItemProperty -Path $key -Name HideFileExt -Value 0 -PropertyType DWord -Force | Out-Null
# 1 shows hidden files and folders (2 hides them).
New-ItemProperty -Path $key -Name Hidden -Value 1 -PropertyType DWord -Force | Out-Null

# Restart File Explorer so the change shows now. Windows starts it again by itself.
Stop-Process -Name explorer -Force -ErrorAction SilentlyContinue
Write-Host 'File extensions and hidden files are now shown.'

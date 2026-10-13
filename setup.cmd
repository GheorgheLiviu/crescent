@echo off
rem Double-click to open the Crescent page. "setup.cmd -Dev" also sets up building and testing.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup.ps1" %*

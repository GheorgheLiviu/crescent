# Crescent.exe

One generic Windows program. The web page appends the chosen app list to the end of the file
(`<JSON {"apps":[{"id","name","icon","office","url","detect","fixed"}],"autoUpdate","scripts"}><uint32 LE length>"AUTOINS1"`;
fields in [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md#payload)), and at run time
the program reads it back and runs `winget install --id <id> --exact --silent` for each app.

It opens a small installer window (Ready, Installing with a progress bar, then "All set") drawn from
`ui.html` in the page's colours, using Microsoft Edge WebView2, which Windows 11 and current Windows 10
already have. No console window appears. Without WebView2 it falls back to plain Windows message boxes.

On the Ready screen the user picks where apps go: Windows' usual place, one folder for all apps (a
whole drive such as D:\ becomes D:\Program Files), or a folder for a single app. Each app gets its own
subfolder, passed to winget as `--location`. Apps whose setup ignores a folder are marked `fixed` by
the page (`fixedLocation` in `web/apps.js`, rebuilt with `python3 tools/location_support.py --write`); Store apps,
Microsoft 365 and makers' own setups are always fixed. The folder picker is Windows' own
(`folder_windows.go`).

The page can also add scripts to run after all apps are installed (`script.go`):
`"scripts":[{"name","kind":"ps1"|"cmd","body"}]`, in the order shown on the page (the project's script
library from `../scripts/`, then the user's own). Invalid ones are dropped and at most 20 are kept. The
Ready screen lists them with a checkbox each, on by default (`aiStart(locations, runScripts)`); the
last screen offers "Run scripts" for any that were switched off or skipped by Cancel
(`aiRunScripts`, once). Each script is written to a temporary folder of its own (PowerShell with a
UTF-8 byte order mark, a batch file with `chcp 65001` first, so accented letters work) and runs as
administrator in its own console window, one after another, with the Path and other variables the new
apps set, from the user's home folder. Cancel never closes a script's window; it skips the scripts
that haven't started. Crescent continues when a window closes and reports each exit code.
Without WebView2, one message box asks whether to run them all.

Only Windows scripts go into this installer; the library's Mac scripts (`.sh`) are kept for a later Mac
version.

The embedded manifest requests administrator rights, so Windows asks for permission once at launch.

## Rebuild

```sh
go install github.com/tc-hib/go-winres@v0.3.3
go-winres make --in winres/winres.json --arch amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o Crescent-base.exe .
{ printf 'window.CRESCENT_EXE_B64="'; base64 -w0 Crescent-base.exe; printf '";\n'; } > ../web/installer-base.js
```

App icons for the page and the window come from `web/icons.js`, built by `python3 tools/build_icons.py`.

The exe is unsigned, so SmartScreen warns on first run. Code signing must happen before the list is
appended or the signature breaks; a signed release would need the list stored another way.

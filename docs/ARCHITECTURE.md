# How Crescent fits together

Back to [README.md](../README.md). How to build and test: [README.md](../README.md).

## The big picture

This is the Windows path. The Mac path (page switch, `Crescent.app`, Homebrew, the window
in a browser tab) is in [mac-plan.md](mac-plan.md).

```
 Web page: web/index.html + icons.js + scripts.js + installer-base.js
   |  the user picks apps, templates, scripts and the weekly updates switch
   v
 buildExe() in the page
   |  payload JSON {apps, autoUpdate, scripts?}
   |  output = base exe + JSON (UTF-8) + uint32 LE JSON length + "AUTOINS1"
   v
 Crescent.exe   (plain .exe in a browser; .zip with the exe inside the Claude viewer)
   |  the user runs it; the manifest makes Windows ask for admin once
   v
 installer/main_windows.go
   |  reads its own file (os.Executable), finds the trailer, cleans apps and scripts
   |  looks for winget on PATH (if missing: offers the Store page for App Installer)
   v
 Window: installer/ui.html in WebView2   (no WebView2: message boxes instead)
   |  Ready screen: folders per app, scripts on or off  ->  aiStart(locations, runScripts)
   v
 Installer.Run, one app at a time, in order (maker downloads last)
   |  winget install ... --source winget         normal apps
   |  winget install ... --source msstore        12-character Store IDs
   |  winget install ... --override /configure   Microsoft 365
   |  curl download + signature check + setup    Samsung Magician ("Your turn")
   v
 Weekly update task (only if switched on and not cancelled)
   v
 Ticked scripts, one after another, each in its own console window, as admin
   v
 Done screen: summary, notes, "Run scripts" for any that didn't run
```

## The pieces

| Piece | Files | Job |
|---|---|---|
| Page | `web/index.html` | Catalog, templates, scripts list, notices; builds and downloads the exe. |
| Icons | `web/icons.js` (from `tools/build_icons.py`) | `window.APP_ICONS = {wingetId: dataURI}`. The page puts each icon into the payload, so the exe carries them. |
| Script library | `scripts/`, `web/scripts.js` (from `tools/build_scripts.py`) | `window.SCRIPT_LIBRARY = [{id, name, description, windows:{name, kind, body}, mac}]`. |
| Base program | `web/installer-base.js`, `installer/Crescent-base.exe` | The Go program as base64, decoded once by the page. |
| Installer logic | `installer/main.go`, `direct.go`, `script.go` | Payload, winget calls, maker downloads, scripts, weekly task. Shared by all platforms. |
| Windows parts | `installer/main_windows.go`, `folder_windows.go`, `proc_windows.go`, `script_windows.go` | Window, bindings, folder picker, signature check, registry, script consoles. |
| Window | `installer/ui.html` | Embedded with `//go:embed`, shown with WebView2 (`github.com/jchv/go-webview2`). |
| Test-only build | `installer/main_other.go`, `proc_other.go`, `script_other.go` | `//go:build !windows`: plain text output, so the tests can run the program on Linux. |

## Payload

The page sends this JSON (built in `buildExe` and `selectedApps` in `index.html`):

```json
{
  "apps": [
    { "id": "Valve.Steam", "name": "Steam", "icon": "data:image/png;base64,..." },
    { "id": "9NKSQGP7F2NH", "name": "WhatsApp", "icon": "...", "fixed": true },
    { "id": "Microsoft.Office", "name": "Microsoft 365 (Word, Excel)", "office": ["Word", "Excel"], "icon": "...", "fixed": true },
    { "id": "Samsung.Magician", "name": "Samsung Magician", "fixed": true,
      "url": "https://download.semiconductor.samsung.com/.../Samsung_Magician_Installer_Official_9.0.1.950.exe",
      "detect": "Samsung Magician" }
  ],
  "autoUpdate": false,
  "scripts": [ { "name": "dark-mode.ps1", "kind": "ps1", "body": "..." } ]
}
```

- `apps` is in selection order (starter apps first). `icon` is left out when there is none.
- `fixed` is sent for apps whose folder can't be chosen (see below).
- `url` and `detect` are sent only for maker downloads (`"downloads"` in `web/apps.js`).
- Office picks (Word, Excel, PowerPoint, Outlook, OneNote, Access) become one `Microsoft.Office` entry with an `office` list.
- `autoUpdate` is always sent by the page.
- `scripts` holds the ticked scripts in list order, at most 20. It is left out when none is ticked.
- The Go side (`Payload`, `App`, `Script` in `main.go` and `script.go`) ignores unknown keys. The old
  single `script` key from earlier today is ignored, and a test checks that.

## Trailer format

```
<base exe bytes><JSON, UTF-8><uint32 little-endian: JSON length><"AUTOINS1">
```

The output is `base length + JSON length + 12` bytes. In Go, `readPayload` reports "no app list
found in this installer" when the magic is missing, and "the app list in this installer is
damaged" for a bad length or bad JSON. `parsePayload` turns these into a message that ends "Build a
new one from the Crescent page", and also fails when no valid apps remain.

Because the list is appended after the build, the exe can't be code-signed as it is. Signing would
need the list stored somewhere else.

## Cleaning in the installer

The installer checks everything again, because the trailer can be edited:

- **Apps** (`cleanApps`): ID must match `^[A-Za-z0-9][A-Za-z0-9.+_-]{1,99}$` or the app is dropped. A
  `url` must pass `validDirectURL` (https, no user info, no port, host in the allowlist, path ending
  `.exe`) or the app is dropped. Empty name becomes the ID. `office` keeps only Word, Excel,
  PowerPoint. `icon` is kept only for `data:image/png;base64,` or `data:image/svg+xml;base64,`.
  `detect` is cleared if over 100 characters or containing `\` or NUL. Store IDs, `Microsoft.Office`
  and `url` apps are always `fixed`. `url` apps move to the end.
- **Scripts** (`cleanScripts`): see Scripts below.

## The window and its bindings

`main_windows.go` creates a WebView2 window titled "Crescent", 680 x 640 scaled for DPI, fixed
size, centered, with its data folder in `%TEMP%\Crescent-WebView2`. Before loading `ui.html`
it sets:

```
window.AI = { apps: [{id, name, icon, manual, fixed}], autoUpdate, scripts: [{name, kind, body, lines}] }
```

`manual` is true for maker downloads. Without `window.AI`, `ui.html` shows demo data (used by the
tests and previews).

**Page calls Go** (bound functions):

| Binding | What it does |
|---|---|
| `aiStart(locations, runScripts)` | Starts the install, once. `locations[i]` is the folder for app i or `""`; `runScripts[i]` is the checkbox for script i. Both arguments are always passed. |
| `aiPickFolder(title, start)` | Opens Windows' folder picker (after the call returns, through `w.Dispatch`), then calls `window.ui.picked(path)`. Ignored after Install is pressed. |
| `aiCancel()` | Cancel: stops the running winget or curl and skips the rest. |
| `aiRunScripts()` | The done screen's "Run scripts": runs the scripts that didn't run, once. |
| `aiClose()` | Closes the window. |

**Go calls the page** (`window.ui.*`): `progress(done, total, status)`,
`outcome(i, "installed" | "skipped" | "failed" | "cancelled" | "waiting")`, `log(line)`,
`done({installed, skipped, failed[], notes[], cancelled, scriptsRan[]})`, `scriptStarted(i)`,
`scriptFinished({index, note, ok})`, `scriptsDone()`, `picked(path)`.

**Screens:** "Ready to install" (app list, install location, "After installing" scripts list,
"Weekly updates will be turned on" when set), "Installing your apps" (progress, Show details log,
Cancel), and the done screen titled "All set", "Finished, with some problems" or "Installation
stopped".

**Fallbacks:** if the payload is bad, a message box shows the error. If winget is missing, a Yes/No
box offers `ms-windows-store://pdp/?productid=9NBLGGH4NNS1` (App Installer). If WebView2 can't start,
a Yes/No box lists the apps, they install with no progress display and no folder choice, one Yes/No
box asks whether to run all scripts (the per-script ticks don't exist there), and a "Done." box
shows the summary.

## How each app installs

Every app gets one call:

```
winget install --id <ID> --exact --silent --disable-interactivity --source <winget|msstore>
               --accept-package-agreements --accept-source-agreements [extras]
```

`--source msstore` is used for 12-character Store IDs (WhatsApp `9NKSQGP7F2NH`, Tobii Experience
`9NK75KF67S2N`). Extras, in this order:

- `--location <folder>` when a folder was chosen.
- **Microsoft 365** (`Microsoft.Office` with at least one picked app):
  `--override "/configure \"%TEMP%\crescent-office.xml\""`. The XML asks for
  `O365HomePremRetail`, 64-bit, Current channel, the system language, no UI, EULA accepted. It
  excludes the unpicked Word, Excel or PowerPoint, and always excludes Access, Groove, Lync, OneNote,
  Outlook, OutlookForWindows, Publisher, Teams and Bing.
- **C++ Build Tools** (`Microsoft.VisualStudio.2022.BuildTools`): `--override "--quiet --wait
  --norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"`.
- **PostgreSQL** (`PostgreSQL.PostgreSQL.*`): `--custom "--superpassword <random 20 characters>"`.
  After a successful install, `PostgreSQL password.txt` (host, port 5432, user postgres, password)
  goes to `~\OneDrive\Desktop`, else `~\Desktop`, else `~`. If none works, the password goes in the
  summary notes.

**Exit codes:** 0 is "installed". `0x8A150061` and `0x8A15002B` are "skipped" (already installed).
Anything else is "failed", logged in hex. winget's output is cleaned (no ANSI codes, spinners or
progress bars) and shown in the log.

**Maker downloads** (`direct.go`, today only Samsung Magician):

1. If `detect` matches the start of a `DisplayName` under the Uninstall registry keys (HKLM 64-bit,
   HKLM 32-bit, HKCU), the app is skipped.
2. `%SystemRoot%\System32\curl.exe` downloads the file into `%TEMP%\Crescent-*`, https only, at
   most 5 redirects, and the final URL must still be on the allowlist
   (`download.semiconductor.samsung.com`). HTTP 404 or 410 says "the download link no longer works".
3. `verifySignature` runs `WinVerifyTrustEx`. Any trusted signer passes; it doesn't check for Samsung.
4. The setup opens in its own window and the app shows "Your turn" (`waiting`). Inno Setup exit codes
   2 and 5 mean the user closed it early.

**Cancel** kills the current winget or curl and skips the apps not started. It never kills a maker's
setup or a script; it waits for them. Closing the window counts as Cancel.

**Summary:** `Installed: N   Already there: N   Failed: N`, plus `(stopped early)` after Cancel.

## Install location

On the Ready screen the user keeps Windows' usual folders, picks one folder for all apps, or uses
Change on a single app. Each app gets its own subfolder named after the app (not added twice if the
chosen folder already ends with that name). A whole drive such as `D:\` becomes `D:\Program Files`.
Both `ui.html` and Go (`validFolder`) accept only drive-letter paths up to 200 characters, with no
characters Windows forbids, no `..`, no trailing `\`, dot or space, and no network paths. Go passes
the folder as `--location`.

**Apps that can't use a custom folder, and why:**

- **30 catalog apps** in `"fixedLocation"` in `web/apps.js`, for example Google Chrome, Discord,
  Spotify, PowerToys, Windows Terminal, Docker Desktop, C++ Build Tools, AnyDesk, the Runtimes group,
  Logitech G HUB and Corsair iCUE. The list is built by
  `tools/location_support.py` from each app's newest winget manifest. winget's `--location` only
  works when the installer takes a folder: Inno, NSIS and portable installers do; MSI, WiX and other
  EXE installers only when the manifest names a location switch (`InstallLocation`); MSIX and appx
  never do. Chrome is listed by hand because its installer ignores the folder.
- **Store apps** (12-character IDs): the Store decides where they go.
- **Microsoft 365**: Click-to-Run installs to its own place.
- **Maker downloads** (Samsung Magician): the user chooses in its own setup ("You choose in its own
  setup").

The page marks these with `fixed`, and the installer forces `fixed` again for the last three kinds.
The Ready screen shows "Usual place only" for them and hides the location box when no app can move.

## Scripts

- **Sources:** the project's library (`scripts/` built into `scripts.js`, all unticked) followed by
  the user's own (typed or opened from a `.ps1`, `.cmd`, `.bat` or `.txt` file, ticked when added).
- **Rules** (page and `script.go` both enforce them): kind `ps1` or `cmd`; at most 20 scripts; body
  not blank, at most 3,000,000 bytes of UTF-8 (the page also caps at 1,000,000 characters), no NUL; name
  matches `^[A-Za-z0-9][A-Za-z0-9 ._()-]{0,59}$`, isn't a device name (CON, PRN, AUX, NUL, COM0 to
  COM9, LPT0 to LPT9), and ends `.ps1` for PowerShell or `.cmd`/`.bat` for Command Prompt. A bad
  name becomes `script.ps1` or `script.cmd`.
- **Running** (`script.go`, `script_windows.go`): each script is written to its own
  `%TEMP%\Crescent-script-*` folder (CRLF line endings; PowerShell gets a UTF-8 BOM; batch files
  start with `@chcp 65001 >nul`), then started with `CreateProcess` in a new console titled
  "Crescent: <name>":
  - PowerShell: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File <file>`
  - Batch: `cmd.exe /d /s /c ""<file>""`
- They run elevated (the installer is), from the user's home folder, with environment variables
  re-read from the registry so the new apps' Path entries are there.
- They run one after another after all apps (and after the weekly task step). Cancel skips scripts
  that haven't started and never closes a running one. Notes report "finished", the exit code, or
  "Couldn't run".
- The installer window shows each script's file name, not the library's friendly name.

## Weekly updates

Only when `autoUpdate` is on and the install wasn't cancelled. `scheduleUpdates` writes a task XML
(UTF-16 LE with BOM) to `%TEMP%`, runs `schtasks /Create /TN "Crescent\Weekly app updates"
/XML <file> /F`, then deletes the XML. The task runs every Sunday at 12:00 (or at the next chance if
the PC was off), on battery too, only with network, for at most 4 hours, at the highest available
level. It runs `cmd.exe /c start "Crescent updates" /min "<winget path>" upgrade --all
--silent --accept-package-agreements --accept-source-agreements`. The note on the done screen says
how to remove it.

## Limits in one place

| Limit | Value | Where |
|---|---|---|
| App ID | `^[A-Za-z0-9][A-Za-z0-9.+_-]{1,99}$` | `ID_RE` in `index.html`, `idPattern` in `main.go` |
| Store ID | `^[0-9A-Z]{12}$` | `fixedPlace` in `index.html`, `storeID` in `main.go` |
| Scripts per installer | 20 (also the most own scripts) | `SCRIPT_MAX`, `maxScripts` |
| Script size | 3,000,000 bytes of UTF-8; the page also caps at 1,000,000 characters and refuses files over 12 MB before reading them | `SCRIPT_MAX_BYTES` and `SCRIPT_MAX_CHARS` in `index.html`, `script.go` |
| Script name | 60 characters, see rules above | `SCRIPT_NAME_RE`, `validScriptName` |
| Install folder | drive-letter path, 200 characters | `validFolder` in `ui.html` and `main.go` |
| `detect` | 100 characters, no `\` or NUL | `cleanApps` |
| Maker download hosts | `download.semiconductor.samsung.com` | `direct.go` |
| Number of apps, icon size | no limit | |

When you change a rule, change it on both sides (page and Go) and in `tools/build_scripts.py` for
script rules.

## Test-only build

On Linux the same Go code builds with a text UI (`main_other.go`; macOS builds the real Mac
installer, `main_darwin.go`): it installs every app,
prints the summary, tries the weekly task when `autoUpdate` is on (it fails off Windows), then runs
all scripts with `sh`
(or `pwsh` for `.ps1`). The signature check always passes and `isInstalled` always returns false
there. `tests/e2e.test.mjs` uses this build with a stand-in `winget` script.

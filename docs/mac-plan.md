# Crescent on Mac

Back to [README.md](../README.md). Built on 2026-10-12. **Never run on a real Mac** yet.
Everything below compiles and the page side is tested; what happens on a Mac is not.

## What the visitor sees

The page has a **Windows | Mac** switch next to ENG | RO. It starts on Mac when the browser runs on
a Mac and remembers the choice. On Mac:

- Only the 70 apps with a Homebrew package show (`"mac"` in `web/apps.js`). Windows-only apps
  (Everything, PowerToys, Notepad++, the runtimes...) are hidden, and templates keep only their Mac
  apps. Switching to Mac takes Windows-only apps out of the selection and says so.
- "Add by ID" takes a Homebrew name (`firefox`, `node@22`, `cask:visual-studio-code`).
- The library scripts with a Mac version (`scripts/mac/`) show; your own scripts are shell
  scripts (`.sh`).
- The download is `Crescent.zip`, holding `Crescent.app`.

## How the Mac app works

| Part | File | What it does |
| --- | --- | --- |
| The app | built in `buildMacApp()` in `web/index.html` | `Crescent.app/Contents/` with `Info.plist`, `MacOS/Crescent` (the program, mode 755) and `Resources/apps.json` (the app list). The list is a file, not appended to the program, because that would break the program's signature. |
| The program | `installer/main_darwin.go`, built by `python tools/build_mac.py` into `web/installer-mac.js` | One universal program for Apple silicon and Intel. Go signs the Apple silicon half ad hoc, which those Macs require. The page loads `installer-mac.js` (18 MB) only when Mac is picked. |
| The window | `installer/ui.html` | The same window as on Windows, opened as a **browser tab**: the program serves it from this Mac only (`127.0.0.1`, a random port and a secret path) and opens it with `open`. Clicks go back with `fetch`; progress comes as Server-Sent Events. `Info.plist` has `LSUIElement`, so no Dock icon bounces. Closing the tab doesn't stop an install; the program quits when nothing runs and no tab has been open for 90 seconds, or when Finish is pressed. |
| Homebrew | `installBrew` in `main_darwin.go` | When the Mac has no Homebrew, it downloads Homebrew's official `.pkg` from GitHub, checks it with `spctl` (signed by a developer Apple knows), and installs it with the Mac's own password dialog. |
| Apps | `installer/brew.go` | `brew install --cask NAME` or `--formula NAME` per app; "already installed" or "already an App at" counts as already there. Every app goes to its usual place (Applications), so the window has no folder choice. |
| Password | `askpass` in `main_darwin.go` | Some casks (`.pkg` based) and scripts need `sudo`. A `sudo` wrapper and `SUDO_ASKPASS` make it ask in a dialog, since an app opened from Finder has no terminal. |
| Scripts | `installer/script_darwin.go` | Run as the user, one after another, with bash (or the shell in their `#!` line). Their output goes to Show details; they have no window of their own. |
| Weekly updates | `scheduleBrewUpdates` in `brew.go` | A launchd job, `~/Library/LaunchAgents/com.crescent.updates.plist`, runs `brew upgrade` on Sundays at 12:00. |

## Apple's warning

The app isn't signed by an Apple developer (it stays unsigned, as on Windows). On
macOS 15 and later the visitor opens it once, chooses Done, then System Settings, Privacy &
Security, Open Anyway. The page says so. Signing and notarising needs the Apple Developer Program
($99 a year).

## Left out on purpose

- Choosing an install folder (Homebrew's `--appdir`): everything goes to Applications.
- Special handling for PostgreSQL (no password file; the notice tells you to start it with
  `brew services`) and Microsoft 365 (Word, Excel and PowerPoint are three separate apps).
- A Mac version of the monthly printer script.
- An app icon (the Mac shows a generic one).

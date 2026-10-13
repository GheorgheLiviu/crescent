# Tests

Run everything with one command from the crescent folder:

```sh
sh tests/run.sh
```

It runs the installer program's Go tests (and checks the Windows build), then the browser tests:

- `page.test.mjs`: the web page (picking apps, quick setups, Microsoft 365, notices, the scripts
  list and editor, and the installer file it builds).
- `installer-window.test.mjs`: the installer window with the Windows side mocked (install location,
  the scripts list, progress, Cancel, the summary).
- `e2e.test.mjs`: the page builds an installer and the program runs it against a stand-in winget,
  then runs the ticked scripts in order. Skipped on Windows.

Needs Go and Node 22. The first time, run `npm install` and `npx playwright install chromium` in
this folder. Nothing here runs on real Windows; that still needs a manual try.

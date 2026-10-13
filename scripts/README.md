# Scripts

Scripts people can tick on the Crescent page. The installer runs the ticked ones as
administrator after all apps are installed, one after another, each in its own window.

## Add a script

1. Put the file in `windows/`: a PowerShell script (`.ps1`) or a Command Prompt script (`.cmd` or
   `.bat`). Save it as UTF-8. Keep it short, commented, and safe to run twice.
2. Add an entry to `scripts.json`. The order there is the order on the page:

   ```json
   {
     "id": "dark-mode",
     "name": "Dark mode",
     "description": "Switches Windows and apps to dark mode.",
     "windows": "windows/dark-mode.ps1",
     "mac": null
   }
   ```

   - `id`: lowercase letters, digits and `-`, up to 40 characters, not used by another entry.
   - `name` and `description`: what the page shows.
   - `ro` (optional): `{"name": "...", "description": "..."}`, shown when the page is in Romanian.
   - `windows` and `mac`: the file inside this folder, or `null`. Mac scripts (`.sh` in `mac/`)
     show on the Mac page and run as the user, not as administrator; each page shows only the
     scripts that have its version.
3. From the crescent folder, run:

   ```sh
   python3 tools/build_scripts.py
   ```

   It checks every entry and writes `web/scripts.js`, which the page loads.

File names may use letters, digits, spaces and `. _ ( ) -`, up to 60 characters. A script can be
up to 1,000,000 characters (3,000,000 bytes).

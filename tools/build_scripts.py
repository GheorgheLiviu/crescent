#!/usr/bin/env python3
"""Build web/scripts.js from the project's script library in scripts/.

scripts/scripts.json is a list of entries like
  {"id": "dark-mode", "name": "Dark mode", "description": "...",
   "ro": {"name": "Mod întunecat", "description": "..."},
   "windows": "windows/dark-mode.ps1", "mac": null}
where "windows" and "mac" are file paths inside scripts/ (or null). The kind comes from the file's
extension: .ps1 is PowerShell, .cmd and .bat are Command Prompt, .sh is a Mac shell script.

The page loads the result as window.SCRIPT_LIBRARY. Run from the crescent folder:
  python3 tools/build_scripts.py
"""
import json, os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LIB = os.path.join(ROOT, "scripts")
OUT = os.path.join(ROOT, "web", "scripts.js")

MAX_BYTES = 3000000
ID_RE = re.compile(r"^[a-z0-9-]{1,40}$")
# Same file name rule as the page and the installer program.
NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9 ._()-]{0,59}$")
DEVICE_RE = re.compile(r"^(con|prn|aux|nul|com[0-9]|lpt[0-9])$", re.I)
KINDS = {"windows": {".ps1": "ps1", ".cmd": "cmd", ".bat": "cmd"}, "mac": {".sh": "sh"}}
KEYS = {"id", "name", "description", "ro", "windows", "mac"}


def load_variant(entry, platform, errors):
    rel = entry.get(platform)
    where = f"{entry.get('id', '?')}: {platform}"
    if rel is None:
        return None
    if not isinstance(rel, str) or not rel.strip():
        errors.append(f"{where} must be a file path inside scripts/ or null")
        return None
    path = os.path.normpath(os.path.join(LIB, rel))
    if os.path.commonpath([path, LIB]) != LIB:
        errors.append(f"{where}: {rel} is outside the scripts folder")
        return None
    name = os.path.basename(path)
    ext = os.path.splitext(name)[1].lower()
    kind = KINDS[platform].get(ext)
    if not kind:
        want = ", ".join(KINDS[platform])
        errors.append(f"{where}: {name} must end in {want}")
        return None
    if not NAME_RE.match(name):
        errors.append(f"{where}: {name} isn't a usable file name (letters, digits, spaces and . _ ( ) -, up to 60 characters)")
        return None
    if DEVICE_RE.match(name.split(".")[0].strip()):
        errors.append(f"{where}: {name} is a name Windows keeps for devices. Pick another name")
        return None
    if not os.path.isfile(path):
        errors.append(f"{where}: {rel} doesn't exist")
        return None
    data = open(path, "rb").read()
    if len(data) > MAX_BYTES:
        errors.append(f"{where}: {rel} is {len(data)} bytes; the most is {MAX_BYTES}")
        return None
    try:
        body = data.decode("utf-8")
    except UnicodeDecodeError as e:
        errors.append(f"{where}: {rel} isn't UTF-8 ({e}). Save it as UTF-8")
        return None
    body = body.removeprefix("\ufeff").replace("\r\n", "\n")
    if "\0" in body:
        errors.append(f"{where}: {rel} has a NUL character")
        return None
    if not body.strip():
        errors.append(f"{where}: {rel} is empty")
        return None
    return {"name": name, "kind": kind, "body": body}


def build():
    try:
        entries = json.load(open(os.path.join(LIB, "scripts.json"), encoding="utf-8"))
    except (OSError, ValueError) as e:
        sys.exit(f"Couldn't read scripts/scripts.json: {e}")
    if not isinstance(entries, list):
        sys.exit("scripts/scripts.json must be a list of scripts")
    errors, seen, out = [], set(), []
    for i, e in enumerate(entries):
        if not isinstance(e, dict):
            errors.append(f"entry {i + 1} must be an object")
            continue
        sid = e.get("id")
        if not isinstance(sid, str) or not ID_RE.match(sid):
            errors.append(f"entry {i + 1}: id must be 1 to 40 lowercase letters, digits or -")
        elif sid in seen:
            errors.append(f"entry {i + 1}: id {sid} is used twice")
        seen.add(sid)
        if extra := set(e) - KEYS:
            errors.append(f"{sid}: unknown field {', '.join(sorted(extra))}")
        name, desc = e.get("name"), e.get("description", "")
        if not isinstance(name, str) or not name.strip():
            errors.append(f"{sid}: name is missing")
        if not isinstance(desc, str):
            errors.append(f"{sid}: description must be text")
        if e.get("windows") is None and e.get("mac") is None:
            errors.append(f"{sid}: needs a windows or a mac file")
        # Optional Romanian name and description for the page: {"name": "...", "description": "..."}.
        ro = e.get("ro") or {}
        if not isinstance(ro, dict) or set(ro) - {"name", "description"} or not all(isinstance(v, str) for v in ro.values()):
            errors.append(f"{sid}: ro must be an object with a text name and description")
            ro = {}
        ro = {k: v.strip() for k, v in ro.items() if v.strip()}
        variants = {p: load_variant(e, p, errors) for p in ("windows", "mac")}
        out.append({"id": sid, "name": (name or "").strip(), "description": (desc or "").strip() if isinstance(desc, str) else "",
                    **({"ro": ro} if ro else {}), **variants})
    if errors:
        sys.exit("scripts/scripts.json has problems:\n  " + "\n  ".join(errors))

    # JSON is valid JavaScript; these escapes keep any script text safe inside a <script> too.
    data = json.dumps(out, ensure_ascii=False, indent=1)
    data = data.replace("</", "<\\/").replace("<!--", "\\u003c!--").replace("\u2028", "\\u2028").replace("\u2029", "\\u2029")
    with open(OUT, "w", encoding="utf-8", newline="\n") as f:
        f.write("// Built by tools/build_scripts.py from scripts/scripts.json. Don't edit it here.\n")
        f.write(f"window.SCRIPT_LIBRARY = {data};\n")
    print(f"Wrote web/scripts.js with {len(out)} script{'' if len(out) == 1 else 's'}.")


if __name__ == "__main__":
    build()

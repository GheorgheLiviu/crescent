"""Reads and writes web/apps.js, the app catalog the page loads (window.CRESCENT_APPS).

Everything after "window.CRESCENT_APPS =" is plain JSON. save() always writes the same layout
(one app per line), so tools that rewrite the file don't reshuffle it.
"""
import json, os

PATH = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "web", "apps.js")
PREFIX = "window.CRESCENT_APPS = "
HEADER = """// The apps the Crescent page offers. Edit this file to add, remove or rename apps; after the
// "=" it is plain JSON (double quotes, no comma after the last item). See README "Add an app".
//   groups:        the list on the page, in order. "ro" is the Romanian name (optional for apps).
//                  Word, Excel, PowerPoint, Outlook, OneNote and Access use the IDs Microsoft.Office/Word and so on.
//   templates:     the "Start from a template" cards: names, descriptions, tile colour (hue 0-359)
//                  and the IDs of their apps.
//   starter:       apps ticked when the page opens.
//   downloads:     apps winget can't install: the maker's link, the name shown in Installed apps
//                  (to skip it when present) and the maker's name.
//   fixedLocation: apps whose setup ignores a chosen folder. Rebuild it with
//                  python tools/location_support.py --write
//   mac:           the Homebrew package of each app that has a Mac version, as cask:name or
//                  formula:name. Apps not listed here don't show on the Mac page.
//                  An app with "macOnly": true has no Windows version and shows only on the Mac page.
//   repos:         the source code link of each open-source app, shown as a small link on its row.
"""


def load(path=PATH):
    text = open(path, encoding="utf-8").read()
    start = text.index(PREFIX) + len(PREFIX)
    return json.loads(text[start:text.rindex("}") + 1])


def _j(value):
    return json.dumps(value, ensure_ascii=False)


def _obj(d, indent):
    """A dict on one line, or with its "apps" list spread one item per line."""
    if "apps" not in d:
        return _j(d)
    head = ", ".join(f"{_j(k)}: {_j(v)}" for k, v in d.items() if k != "apps")
    pad = " " * (indent + 2)
    items = ",\n".join(pad + (_j(a) if not isinstance(a, dict) else _j(a)) for a in d["apps"])
    return "{" + head + ', "apps": [\n' + items + "\n" + " " * indent + "]}"


def dumps(data):
    lines = []
    for key, value in data.items():
        if isinstance(value, list) and value and isinstance(value[0], dict):
            body = ",\n".join("    " + _obj(v, 4) for v in value)
            lines.append(f"  {_j(key)}: [\n{body}\n  ]")
        elif isinstance(value, list):
            body = ",\n".join("    " + _j(v) for v in value)
            lines.append(f"  {_j(key)}: [\n{body}\n  ]")
        elif isinstance(value, dict):
            body = ",\n".join(f"    {_j(k)}: {_j(v)}" for k, v in value.items())
            lines.append(f"  {_j(key)}: {{\n{body}\n  }}")
        else:
            lines.append(f"  {_j(key)}: {_j(value)}")
    return "{\n" + ",\n".join(lines) + "\n}"


def save(data, path=PATH):
    text = HEADER + PREFIX + dumps(data) + ";\n"
    json.loads(text[len(HEADER) + len(PREFIX):text.rindex("}") + 1])  # must stay valid JSON
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(text)


def catalog_ids(data):
    """[(name, package id)] for every Windows catalog app, Word/Excel/PowerPoint folded into Microsoft.Office."""
    out, seen = [], set()
    for g in data["groups"]:
        for a in g["apps"]:
            if a.get("macOnly"):
                continue
            pkg = a["id"].split("/")[0]
            if pkg not in seen:
                seen.add(pkg)
                out.append((a["name"], pkg))
    return out


if __name__ == "__main__":  # rewrites the file in the standard layout
    save(load())
    print("web/apps.js rewritten")

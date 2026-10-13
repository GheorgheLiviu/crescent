#!/usr/bin/env python3
"""Check the Crescent app list against Microsoft's winget catalog.

Reports apps whose winget ID no longer exists, and version-pinned apps
(Python 3.x, Temurin JDK N, PostgreSQL N) that have a newer release line.

Usage: python3 tools/check_catalog.py [path/to/apps.js] [--cache DIR]
Prints a Markdown report; exits 1 when something needs attention.
"""
import argparse
import os
import re
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import apps_data  # noqa: E402

REPO = "https://github.com/microsoft/winget-pkgs"

# Version-pinned packages: (pattern for our ID, template for the sibling ID).
# The third item says which release lines count (Java: long-term support only).
FAMILIES = [
    (re.compile(r"^Python\.Python\.3\.(\d+)$"), "Python.Python.3.{}", lambda k: True),
    (re.compile(r"^EclipseAdoptium\.Temurin\.(\d+)\.JDK$"), "EclipseAdoptium.Temurin.{}.JDK", lambda k: (k - 21) % 4 == 0),
    (re.compile(r"^PostgreSQL\.PostgreSQL\.(\d+)$"), "PostgreSQL.PostgreSQL.{}", lambda k: True),
]


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], check=True, capture_output=True, text=True).stdout


def sync_repo(cache):
    """Keep a tree-only shallow clone of winget-pkgs (no file contents)."""
    if not os.path.isdir(os.path.join(cache, ".git")):
        subprocess.run(["git", "clone", "-q", "--depth", "1", "--filter=blob:none", "--no-checkout", REPO, cache], check=True)
    else:
        git(cache, "fetch", "-q", "--depth", "1", "origin", "HEAD")
        git(cache, "update-ref", "HEAD", "FETCH_HEAD")
    return cache


def manifest_dir(pkg_id):
    return "manifests/{}/{}".format(pkg_id[0].lower(), pkg_id.replace(".", "/"))


def versions(repo, pkg_id):
    """Versions that have a real manifest for exactly this ID."""
    base = manifest_dir(pkg_id)
    out = git(repo, "ls-tree", "-r", "--name-only", "HEAD", base + "/")
    pat = re.compile(r"^{}/([^/]+)/{}\.yaml$".format(re.escape(base), re.escape(pkg_id)))
    found = {m.group(1) for line in out.splitlines() if (m := pat.match(line))}
    return sorted(found, key=lambda v: [(0, int(x), "") if x.isdigit() else (1, 0, x) for x in re.split(r"[.\-+]", v)])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("apps", nargs="?", default=apps_data.PATH, help="the catalog file (web/apps.js)")
    ap.add_argument("--cache", default=os.path.join(os.path.expanduser("~"), ".cache", "winget-pkgs"))
    args = ap.parse_args()

    repo = sync_repo(args.cache)
    head = git(repo, "log", "-1", "--format=%h %cs").strip()
    missing, newer, ok = [], [], 0

    data = apps_data.load(args.apps)
    ids = apps_data.catalog_ids(data)
    direct = set(data["downloads"])  # downloaded from their maker, not in winget-pkgs
    listed = {pkg for _, pkg in ids}
    for name, pkg in ids:
        if re.fullmatch(r"[0-9A-Z]{12}", pkg) or pkg in direct:  # Store app or maker download
            ok += 1
            continue
        if not versions(repo, pkg):
            missing.append((name, pkg))
            continue
        ok += 1
        for pat, template, counts in FAMILIES:
            m = pat.match(pkg)
            if not m:
                continue
            n = int(m.group(1))
            later = []
            for k in range(n + 1, n + 15):
                if not counts(k) or template.format(k) in listed:
                    continue
                v = versions(repo, template.format(k))
                if v:
                    later.append((template.format(k), v[-1]))
            if later:
                newer.append((name, pkg, later))

    lines = ["# Crescent catalog check", "", f"Checked against winget-pkgs at {head}.", ""]
    lines.append(f"- {ok} app IDs found in the catalog")
    lines.append(f"- {len(missing)} app IDs missing")
    lines.append(f"- {len(newer)} version-pinned apps with a newer release")
    if missing:
        lines += ["", "## Missing from winget", ""]
        lines += [f"- {name} (`{pkg}`)" for name, pkg in missing]
    if newer:
        lines += ["", "## Newer releases available", ""]
        for name, pkg, later in newer:
            alts = ", ".join(f"`{i}` {v}" for i, v in later)
            lines.append(f"- {name} (`{pkg}`): {alts}")
    print("\n".join(lines))
    return 1 if missing or newer else 0


if __name__ == "__main__":
    sys.exit(main())

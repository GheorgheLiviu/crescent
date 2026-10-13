#!/usr/bin/env python3
"""List catalog apps whose install folder can't be chosen, from their winget manifests.

winget's --location only works when the installer accepts a folder: Inno, NSIS and portable
packages do, and MSI, WiX and other EXE installers only when the manifest names a location switch
(winget's default TARGETDIR for MSI is ignored by most MSIs, which install under Program Files).
MSIX and Store apps never do. Prints the IDs that can't move and, with --write, updates fixedLocation
in web/apps.js.

Usage: python3 tools/location_support.py [--write] [--cache DIR]
"""
import argparse, os, re, sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import apps_data  # noqa: E402
import check_catalog as c  # noqa: E402

MOVABLE_TYPES = {"inno", "nullsoft", "portable"}
# Installers that accept a folder on paper but install to their own place anyway.
KNOWN_FIXED = {"Google.Chrome"}


def can_move(repo, pkg):
    d = c.manifest_dir(pkg) if hasattr(c, "manifest_dir") else "manifests/{}/{}".format(pkg[0].lower(), pkg.replace(".", "/"))
    names = c.git(repo, "ls-tree", "--name-only", "HEAD", d + "/").split()
    vers = [n.rsplit("/", 1)[-1] for n in names]
    vers = sorted((v for v in vers if re.match(r"\d", v)),
                  key=lambda v: [(0, int(x), "") if x.isdigit() else (1, 0, x) for x in re.split(r"[.\-+]", v)])
    if not vers:
        return False
    y = c.git(repo, "show", f"HEAD:{d}/{vers[-1]}/{pkg}.installer.yaml")
    types = set(re.findall(r"^\s*-?\s*InstallerType:\s*(\S+)", y, re.M))
    nested = set(re.findall(r"NestedInstallerType:\s*(\S+)", y))
    kinds = (types - {"zip"}) | (nested if "zip" in types else set())
    if not kinds or kinds & {"msix", "appx"}:
        return False
    return bool(re.search(r"^\s*InstallLocation:", y, re.M)) or kinds <= MOVABLE_TYPES


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--write", action="store_true")
    ap.add_argument("--cache", default=os.path.join(os.path.expanduser("~"), ".cache", "winget-pkgs"))
    ap.add_argument("apps", nargs="?", default=apps_data.PATH, help="the catalog file (web/apps.js)")
    args = ap.parse_args()
    repo = c.sync_repo(args.cache)
    data = apps_data.load(args.apps)
    fixed = []
    for _, pkg in apps_data.catalog_ids(data):
        # Store apps, Microsoft 365 and maker downloads never take a folder from winget.
        if re.fullmatch(r"[0-9A-Z]{12}", pkg) or pkg == "Microsoft.Office" or pkg in data["downloads"]:
            continue  # the page and the installer already treat these as fixed
        if pkg in KNOWN_FIXED or not can_move(repo, pkg):
            fixed.append(pkg)
    print("\n".join(fixed))
    if args.write:
        data["fixedLocation"] = fixed
        apps_data.save(data, args.apps)


if __name__ == "__main__":
    main()

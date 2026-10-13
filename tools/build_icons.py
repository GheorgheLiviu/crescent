#!/usr/bin/env python3
"""Build web/icons.js: window.APP_ICONS = {wingetId: dataURI} for the page and the installer window.

Sources (brand logos belong to their owners):
  di  homarr-labs/dashboard-icons, Apache-2.0, full-colour PNG
  sh  selfhst/icons, CC BY 4.0, full-colour PNG
  si  simple-icons, CC0, single-colour SVG drawn white on its brand colour
  gh  a GitHub account's avatar, for open-source projects with no site of their own
  url an icon file in the app's own repository
  fav the maker's own site icon (by domain), through Google's favicon service (ddg: DuckDuckGo's),
      for apps none of the collections has
An optional third value draws a light dashboard-icons logo on a tile of that colour.

Apps with no entry keep the letter tile. Run from any folder:
  python3 tools/build_icons.py
"""
import base64, io, json, os, re, sys, urllib.request
from PIL import Image, ImageDraw

ICONS = {
    "Google.Chrome": ("di", "google-chrome"),
    "Mozilla.Firefox": ("di", "firefox"),
    "Brave.Brave": ("di", "brave"),
    "Opera.Opera": ("di", "opera"),
    "Vivaldi.Vivaldi": ("di", "vivaldi"),
    "Discord.Discord": ("di", "discord"),
    "SlackTechnologies.Slack": ("di", "slack"),
    "Zoom.Zoom": ("di", "zoom"),
    "Microsoft.Teams": ("di", "microsoft-teams"),
    "Telegram.TelegramDesktop": ("di", "telegram"),
    "OpenWhisperSystems.Signal": ("di", "signal"),
    "9NKSQGP7F2NH": ("di", "whatsapp"),
    "VideoLAN.VLC": ("si", "vlcmediaplayer"),
    "Spotify.Spotify": ("di", "spotify"),
    "OBSProject.OBSStudio": ("si", "obsstudio"),
    "Audacity.Audacity": ("di", "audacity"),
    "HandBrake.HandBrake": ("di", "handbrake"),
    "Microsoft.Office": ("di", "microsoft-365"),
    "Microsoft.Office/Word": ("di", "microsoft-word"),
    "Microsoft.Office/Excel": ("di", "microsoft-excel"),
    "Microsoft.Office/PowerPoint": ("di", "microsoft-powerpoint"),
    "Microsoft.Office/Outlook": ("di", "microsoft-outlook"),
    "Microsoft.Office/OneNote": ("di", "microsoft-onenote"),
    "Microsoft.Office/Access": ("di", "microsoft-access"),
    "TheDocumentFoundation.LibreOffice": ("si", "libreoffice"),
    "Adobe.Acrobat.Reader.64-bit": ("sh", "adobe-acrobat"),
    "Notion.Notion": ("di", "notion"),
    "Obsidian.Obsidian": ("di", "obsidian"),
    "Google.GoogleDrive": ("di", "google-drive"),
    "Dropbox.Dropbox": ("di", "dropbox"),
    "Synology.DriveClient": ("di", "synology-drive"),
    "7zip.7zip": ("di", "7zip"),
    "Microsoft.PowerToys": ("sh", "microsoft-powertoys"),
    "ShareX.ShareX": ("si", "sharex"),
    "WinDirStat.WinDirStat": ("di", "windirstat"),
    "Bitwarden.Bitwarden": ("di", "bitwarden"),
    "KeePassXCTeam.KeePassXC": ("di", "keepassxc"),
    "AgileBits.1Password": ("di", "1password"),
    "Anthropic.Claude": ("di", "claude-ai"),
    "Anthropic.ClaudeCode": ("si", "claudecode"),
    "OpenAI.Codex": ("di", "openai-light", "000000"),
    "Microsoft.VisualStudioCode": ("di", "visual-studio-code"),
    "Git.Git": ("di", "git"),
    "Microsoft.WindowsTerminal": ("sh", "windows-terminal"),
    "Notepad++.Notepad++": ("sh", "notepad-plus-plus"),
    "SublimeHQ.SublimeText.4": ("si", "sublimetext"),
    "OpenJS.NodeJS.LTS": ("di", "nodejs"),
    "pnpm.pnpm": ("si", "pnpm"),
    "Yarn.Yarn": ("si", "yarn"),
    "Oven-sh.Bun": ("si", "bun"),
    "DenoLand.Deno": ("si", "deno"),
    "Chocolatey.Chocolatey": ("si", "chocolatey"),
    "Devolutions.UniGetUI": ("url", "https://raw.githubusercontent.com/Devolutions/UniGetUI/main/media/icon.png"),
    "Docker.DockerDesktop": ("di", "docker"),
    "Postman.Postman": ("di", "postman"),
    "Python.Python.3.14": ("di", "python"),
    "astral-sh.uv": ("di", "astral"),
    "JetBrains.PyCharm": ("si", "pycharm"),
    "EclipseAdoptium.Temurin.21.JDK": ("si", "eclipseadoptium"),
    "EclipseAdoptium.Temurin.25.JDK": ("si", "eclipseadoptium"),
    "JetBrains.IntelliJIDEA": ("di", "intellij"),
    "PostgreSQL.PostgreSQL.18": ("di", "postgresql"),
    "DBeaver.DBeaver.Community": ("si", "dbeaver"),
    "PostgreSQL.pgAdmin": ("di", "pgadmin"),
    "SQLite.SQLite": ("si", "sqlite"),
    "DBBrowserForSQLite.DBBrowserForSQLite": ("di", "sqlitebrowser"),
    "MongoDB.Server": ("di", "mongodb"),
    "MongoDB.Compass.Full": ("di", "mongodb"),
    "GIMP.GIMP": ("di", "gimp"),
    "Inkscape.Inkscape": ("si", "inkscape"),
    "BlenderFoundation.Blender": ("di", "blender"),
    "Valve.Steam": ("di", "steam"),
    "LizardByte.Sunshine": ("di", "sunshine"),
    "VirtualDrivers.Virtual-Display-Driver": ("gh", "VirtualDrivers"),
    "MoonlightGameStreamingProject.Moonlight": ("fav", "moonlight-stream.org"),
    "Vorssaint": ("fav", "vorssaint.com"),
    "Keka": ("fav", "keka.io"),
    "DeskPad": ("url", "https://raw.githubusercontent.com/Stengo/DeskPad/main/DeskPad/Assets.xcassets/AppIcon.appiconset/Icon-256.png"),
    "BetterDisplay": ("fav", "betterdisplay.pro"),
    "EpicGames.EpicGamesLauncher": ("di", "epic-games"),
    "GOG.Galaxy": ("si", "gogdotcom"),
    "Ubisoft.Connect": ("si", "ubisoft"),
    "ElectronicArts.EADesktop": ("di", "electronic-arts"),
    "SteelSeries.GG": ("si", "steelseries"),
    "Reolink.Reolink": ("di", "reolink"),
    "Apple.iTunes": ("si", "itunes"),
    "ONLYOFFICE.DesktopEditors": ("di", "onlyoffice"),
    "TeamViewer.TeamViewer": ("sh", "teamviewer"),
    "AnyDesk.AnyDesk": ("di", "anydesk"),
    "qBittorrent.qBittorrent": ("di", "qbittorrent"),
    "Malwarebytes.Malwarebytes": ("si", "malwarebytes"),
    "Microsoft.VCRedist.2015+.x64": ("si", "cplusplus"),
    "Microsoft.DotNet.DesktopRuntime.8": ("si", "dotnet"),
    "Microsoft.DotNet.DesktopRuntime.9": ("si", "dotnet"),
    "Blizzard.BattleNet": ("si", "battledotnet"),
    "9MV0B5HZVK9Z": ("di", "xbox"),
    "Logitech.GHUB": ("di", "logitech", "ffffff"),  # dark logo: on a white tile
    "Corsair.iCUE.5": ("si", "corsair"),
    "CodecGuide.K-LiteCodecPack.Standard": ("ddg", "codecguide.com"),
    "RARLab.WinRAR": ("fav", "win-rar.com"),
    "voidtools.Everything": ("fav", "voidtools.com"),
    "CharlesMilette.TranslucentTB": ("fav", "translucenttb.github.io"),
    "Rufus.Rufus": ("fav", "rufus.ie"),
    "CPUID.CPU-Z": ("fav", "cpuid.com"),
    "REALiX.HWiNFO": ("fav", "hwinfo.com"),
    "CrystalDewWorld.CrystalDiskInfo": ("fav", "crystalmark.info"),
    "RevoUninstaller.RevoUninstaller": ("fav", "revouninstaller.com"),
    "Microsoft.DirectX": ("fav", "microsoft.com"),
    "Microsoft.VisualStudio.2022.BuildTools": ("si", "cplusplus"),
    "dotPDN.PaintDotNet": ("fav", "getpaint.net"),
    "9NK75KF67S2N": ("fav", "tobii.com", "ffffff"),
    "Samsung.Magician": ("fav", "semiconductor.samsung.com"),
}

URL = {
    "di": "https://raw.githubusercontent.com/homarr-labs/dashboard-icons/main/png/{}.png",
    "sh": "https://raw.githubusercontent.com/selfhst/icons/main/png/{}.png",
    "si": "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/{}.svg",
    "fav": "https://www.google.com/s2/favicons?domain={}&sz=128",
    "ddg": "https://icons.duckduckgo.com/ip3/{}.ico",
    "gh": "https://github.com/{}.png?size=128",
    "url": "{}",
}
SI_DATA = "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/data/simple-icons.json"
SIZE = 64


def get(url):
    # Some sites refuse requests without a browser-like User-Agent.
    with urllib.request.urlopen(urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"}), timeout=30) as r:
        return r.read()


def png_uri(raw, tile=None):
    im = Image.open(io.BytesIO(raw)).convert("RGBA")
    bbox = im.getbbox()  # trim transparent margins so every logo fills its tile alike
    if bbox:
        im = im.crop(bbox)
    scale = SIZE / max(im.size)  # up as well as down, so small site icons fill the tile too
    im = im.resize((max(1, round(im.width * scale)), max(1, round(im.height * scale))), Image.LANCZOS)
    canvas = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    if tile:  # logo inset on a rounded tile, for logos that only come in black or white
        ImageDraw.Draw(canvas).rounded_rectangle((0, 0, SIZE - 1, SIZE - 1), 14, fill="#" + tile)
        im.thumbnail((SIZE * 5 // 8, SIZE * 5 // 8), Image.LANCZOS)
    canvas.paste(im, ((SIZE - im.width) // 2, (SIZE - im.height) // 2), im)
    out = io.BytesIO()
    canvas.save(out, "PNG", optimize=True)
    return "data:image/png;base64," + base64.b64encode(out.getvalue()).decode()


def si_slug(title):
    return re.sub(r"[^a-z0-9]", "", title.lower().replace("+", "plus").replace(".", "dot").replace("&", "and"))


def svg_uri(raw, hexcolor):
    path = re.search(rb'<path d="([^"]+)"', raw).group(1).decode()
    svg = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">'
           f'<rect width="64" height="64" rx="14" fill="#{hexcolor}"/>'
           f'<g transform="translate(14 14) scale(1.5)"><path fill="#fff" d="{path}"/></g></svg>')
    return "data:image/svg+xml;base64," + base64.b64encode(svg.encode()).decode()


def main():
    data = json.loads(get(SI_DATA))
    entries = data["icons"] if isinstance(data, dict) else data
    hexes = {(e.get("slug") or si_slug(e["title"])): e["hex"] for e in entries}
    out, missing = {}, []
    for app, (src, slug, *tile) in ICONS.items():
        try:
            raw = get(URL[src].format(slug))
            out[app] = svg_uri(raw, hexes[slug]) if src == "si" else png_uri(raw, *tile)
        except Exception as e:  # keep going; the page falls back to the letter tile
            missing.append(f"{app} ({src}:{slug}): {e}")
    out_file = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "web", "icons.js")
    with open(out_file, "w", encoding="utf-8", newline="\n") as f:
        f.write("// Generated by tools/build_icons.py. Logos belong to their owners; sources: dashboard-icons\n"
                "// (Apache-2.0), selfh.st icons (CC BY 4.0), Simple Icons (CC0), makers' site icons.\n")
        f.write("window.APP_ICONS = " + json.dumps(out, separators=(",", ":")) + ";\n")
    print(f"{len(out)} icons written, {sum(map(len, out.values()))//1024} KB")
    for m in missing:
        print("missing:", m, file=sys.stderr)


if __name__ == "__main__":
    main()

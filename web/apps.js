// The apps the Crescent page offers. Edit this file to add, remove or rename apps; after the
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
window.CRESCENT_APPS = {
  "groups": [
    {"name": "Browsers", "ro": "Browsere", "apps": [
      {"name": "Google Chrome", "id": "Google.Chrome"},
      {"name": "Mozilla Firefox", "id": "Mozilla.Firefox"},
      {"name": "Brave", "id": "Brave.Brave"},
      {"name": "Opera", "id": "Opera.Opera"},
      {"name": "Vivaldi", "id": "Vivaldi.Vivaldi"}
    ]},
    {"name": "Messaging", "ro": "Mesagerie", "apps": [
      {"name": "Discord", "id": "Discord.Discord"},
      {"name": "Slack", "id": "SlackTechnologies.Slack"},
      {"name": "Zoom", "id": "Zoom.Zoom"},
      {"name": "Microsoft Teams", "id": "Microsoft.Teams"},
      {"name": "Telegram", "id": "Telegram.TelegramDesktop"},
      {"name": "Signal", "id": "OpenWhisperSystems.Signal"},
      {"name": "WhatsApp", "id": "9NKSQGP7F2NH"}
    ]},
    {"name": "Media", "apps": [
      {"name": "VLC media player", "id": "VideoLAN.VLC"},
      {"name": "Spotify", "id": "Spotify.Spotify"},
      {"name": "OBS Studio", "id": "OBSProject.OBSStudio"},
      {"name": "Audacity", "id": "Audacity.Audacity"},
      {"name": "HandBrake", "id": "HandBrake.HandBrake"},
      {"name": "K-Lite Codec Pack", "id": "CodecGuide.K-LiteCodecPack.Standard"},
      {"name": "iTunes", "id": "Apple.iTunes"}
    ]},
    {"name": "Microsoft Office", "apps": [
      {"name": "Word", "id": "Microsoft.Office/Word"},
      {"name": "Excel", "id": "Microsoft.Office/Excel"},
      {"name": "PowerPoint", "id": "Microsoft.Office/PowerPoint"},
      {"name": "Outlook (classic)", "id": "Microsoft.Office/Outlook", "ro": "Outlook (clasic)"},
      {"name": "OneNote", "id": "Microsoft.Office/OneNote"},
      {"name": "Access", "id": "Microsoft.Office/Access"}
    ]},
    {"name": "Productivity", "ro": "Productivitate", "apps": [
      {"name": "LibreOffice", "id": "TheDocumentFoundation.LibreOffice"},
      {"name": "Adobe Acrobat Reader", "id": "Adobe.Acrobat.Reader.64-bit"},
      {"name": "Notion", "id": "Notion.Notion"},
      {"name": "Obsidian", "id": "Obsidian.Obsidian"},
      {"name": "Google Drive", "id": "Google.GoogleDrive"},
      {"name": "Dropbox", "id": "Dropbox.Dropbox"},
      {"name": "Synology Drive Client", "id": "Synology.DriveClient"},
      {"name": "ONLYOFFICE", "id": "ONLYOFFICE.DesktopEditors"}
    ]},
    {"name": "Utilities", "ro": "Utilitare", "apps": [
      {"name": "7-Zip", "id": "7zip.7zip"},
      {"name": "WinRAR", "id": "RARLab.WinRAR"},
      {"name": "Everything search", "id": "voidtools.Everything", "ro": "Everything (căutare)"},
      {"name": "PowerToys", "id": "Microsoft.PowerToys"},
      {"name": "ShareX", "id": "ShareX.ShareX"},
      {"name": "WinDirStat", "id": "WinDirStat.WinDirStat"},
      {"name": "Bitwarden", "id": "Bitwarden.Bitwarden"},
      {"name": "KeePassXC", "id": "KeePassXCTeam.KeePassXC"},
      {"name": "1Password", "id": "AgileBits.1Password"},
      {"name": "TranslucentTB", "id": "CharlesMilette.TranslucentTB"},
      {"name": "TeamViewer", "id": "TeamViewer.TeamViewer"},
      {"name": "AnyDesk", "id": "AnyDesk.AnyDesk"},
      {"name": "qBittorrent", "id": "qBittorrent.qBittorrent"},
      {"name": "BetterDisplay", "id": "BetterDisplay", "macOnly": true}
    ]},
    {"name": "System tools", "ro": "Unelte de sistem", "apps": [
      {"name": "Rufus", "id": "Rufus.Rufus"},
      {"name": "CPU-Z", "id": "CPUID.CPU-Z"},
      {"name": "HWiNFO", "id": "REALiX.HWiNFO"},
      {"name": "CrystalDiskInfo", "id": "CrystalDewWorld.CrystalDiskInfo"},
      {"name": "Revo Uninstaller", "id": "RevoUninstaller.RevoUninstaller"},
      {"name": "Malwarebytes", "id": "Malwarebytes.Malwarebytes"}
    ]},
    {"name": "Runtimes", "ro": "Componente runtime", "apps": [
      {"name": "Visual C++ Redistributable", "id": "Microsoft.VCRedist.2015+.x64"},
      {"name": ".NET Desktop Runtime 8", "id": "Microsoft.DotNet.DesktopRuntime.8"},
      {"name": ".NET Desktop Runtime 9", "id": "Microsoft.DotNet.DesktopRuntime.9"},
      {"name": "DirectX", "id": "Microsoft.DirectX"}
    ]},
    {"name": "AI assistants", "ro": "Asistenți AI", "apps": [
      {"name": "Claude", "id": "Anthropic.Claude"},
      {"name": "Claude Code", "id": "Anthropic.ClaudeCode"},
      {"name": "Codex CLI", "id": "OpenAI.Codex"}
    ]},
    {"name": "Developer tools", "ro": "Unelte pentru programatori", "apps": [
      {"name": "Visual Studio Code", "id": "Microsoft.VisualStudioCode"},
      {"name": "Git", "id": "Git.Git"},
      {"name": "Windows Terminal", "id": "Microsoft.WindowsTerminal"},
      {"name": "Notepad++", "id": "Notepad++.Notepad++"},
      {"name": "Sublime Text", "id": "SublimeHQ.SublimeText.4"},
      {"name": "Docker Desktop", "id": "Docker.DockerDesktop"},
      {"name": "Postman", "id": "Postman.Postman"}
    ]},
    {"name": "Package managers", "ro": "Manageri de pachete", "apps": [
      {"name": "Node.js LTS (npm)", "id": "OpenJS.NodeJS.LTS"},
      {"name": "pnpm", "id": "pnpm.pnpm"},
      {"name": "Yarn", "id": "Yarn.Yarn"},
      {"name": "Bun", "id": "Oven-sh.Bun"},
      {"name": "Deno", "id": "DenoLand.Deno"},
      {"name": "Chocolatey", "id": "Chocolatey.Chocolatey"},
      {"name": "UniGetUI", "id": "Devolutions.UniGetUI"}
    ]},
    {"name": "Python", "apps": [
      {"name": "Python 3.14", "id": "Python.Python.3.14"},
      {"name": "uv (Python packages)", "id": "astral-sh.uv", "ro": "uv (pachete Python)"},
      {"name": "PyCharm", "id": "JetBrains.PyCharm"},
      {"name": "C++ Build Tools (large)", "id": "Microsoft.VisualStudio.2022.BuildTools", "ro": "C++ Build Tools (mare)"}
    ]},
    {"name": "Java", "apps": [
      {"name": "Java JDK 21 (Temurin)", "id": "EclipseAdoptium.Temurin.21.JDK"},
      {"name": "Java JDK 25 (Temurin)", "id": "EclipseAdoptium.Temurin.25.JDK"},
      {"name": "IntelliJ IDEA", "id": "JetBrains.IntelliJIDEA"}
    ]},
    {"name": "Databases", "ro": "Baze de date", "apps": [
      {"name": "PostgreSQL 18", "id": "PostgreSQL.PostgreSQL.18"},
      {"name": "DBeaver", "id": "DBeaver.DBeaver.Community"},
      {"name": "pgAdmin", "id": "PostgreSQL.pgAdmin"},
      {"name": "SQLite", "id": "SQLite.SQLite"},
      {"name": "DB Browser for SQLite", "id": "DBBrowserForSQLite.DBBrowserForSQLite"},
      {"name": "MongoDB Server", "id": "MongoDB.Server"},
      {"name": "MongoDB Compass", "id": "MongoDB.Compass.Full"}
    ]},
    {"name": "Graphics", "ro": "Grafică", "apps": [
      {"name": "GIMP", "id": "GIMP.GIMP"},
      {"name": "Paint.NET", "id": "dotPDN.PaintDotNet"},
      {"name": "Inkscape", "id": "Inkscape.Inkscape"},
      {"name": "Blender", "id": "BlenderFoundation.Blender"}
    ]},
    {"name": "Gaming", "ro": "Jocuri", "apps": [
      {"name": "Steam", "id": "Valve.Steam"},
      {"name": "Epic Games Launcher", "id": "EpicGames.EpicGamesLauncher"},
      {"name": "GOG Galaxy", "id": "GOG.Galaxy"},
      {"name": "Ubisoft Connect", "id": "Ubisoft.Connect"},
      {"name": "EA app", "id": "ElectronicArts.EADesktop"},
      {"name": "Battle.net", "id": "Blizzard.BattleNet"},
      {"name": "Xbox", "id": "9MV0B5HZVK9Z"}
    ]},
    {"name": "Open-source gaming", "ro": "Jocuri open source", "apps": [
      {"name": "Sunshine", "id": "LizardByte.Sunshine"},
      {"name": "Moonlight", "id": "MoonlightGameStreamingProject.Moonlight"}
    ]},
    {"name": "Open-source tools", "ro": "Unelte open source", "apps": [
      {"name": "Virtual Display Driver", "id": "VirtualDrivers.Virtual-Display-Driver", "ro": "Virtual Display Driver (monitor virtual)"},
      {"name": "Keka", "id": "Keka", "macOnly": true},
      {"name": "Vorssaint", "id": "Vorssaint", "macOnly": true},
      {"name": "DeskPad", "id": "DeskPad", "macOnly": true}
    ]},
    {"name": "Devices", "ro": "Dispozitive", "apps": [
      {"name": "SteelSeries GG", "id": "SteelSeries.GG"},
      {"name": "Tobii Experience", "id": "9NK75KF67S2N"},
      {"name": "Reolink", "id": "Reolink.Reolink"},
      {"name": "Samsung Magician", "id": "Samsung.Magician"},
      {"name": "Logitech G HUB", "id": "Logitech.GHUB"},
      {"name": "Corsair iCUE", "id": "Corsair.iCUE.5"}
    ]}
  ],
  "templates": [
    {"name": "Quick start", "ro": "Pornire rapidă", "description": "Everyday apps for a new PC: browsers, media, Office, PDF and handy tools", "descriptionRo": "Aplicații de zi cu zi pentru un PC nou: browsere, media, Office, PDF și unelte utile", "hue": 265, "apps": [
      "Google.Chrome",
      "Mozilla.Firefox",
      "VideoLAN.VLC",
      "Spotify.Spotify",
      "Zoom.Zoom",
      "Microsoft.Office/Word",
      "Microsoft.Office/Excel",
      "Microsoft.Office/PowerPoint",
      "Adobe.Acrobat.Reader.64-bit",
      "7zip.7zip",
      "voidtools.Everything",
      "Microsoft.PowerToys",
      "Bitwarden.Bitwarden",
      "Microsoft.VCRedist.2015+.x64"
    ]},
    {"name": "Python", "ro": "Python", "description": "Write and run Python code", "descriptionRo": "Scrie și rulează cod Python", "hue": 205, "apps": [
      "Python.Python.3.14",
      "astral-sh.uv",
      "Microsoft.VisualStudioCode",
      "Git.Git",
      "Microsoft.WindowsTerminal",
      "JetBrains.PyCharm"
    ]},
    {"name": "Java", "ro": "Java", "description": "Build and run Java projects", "descriptionRo": "Construiește și rulează proiecte Java", "hue": 18, "apps": [
      "EclipseAdoptium.Temurin.21.JDK",
      "JetBrains.IntelliJIDEA",
      "Microsoft.VisualStudioCode",
      "Git.Git",
      "Microsoft.WindowsTerminal"
    ]},
    {"name": "Database", "ro": "Bază de date", "description": "Local SQL database and a browser for it", "descriptionRo": "Bază de date SQL locală și un program pentru ea", "hue": 150, "apps": [
      "PostgreSQL.PostgreSQL.18",
      "DBeaver.DBeaver.Community"
    ]}
  ],
  "starter": [
    "Google.Chrome",
    "7zip.7zip",
    "VideoLAN.VLC",
    "Discord.Discord"
  ],
  "downloads": {
    "Samsung.Magician": {"url": "https://download.semiconductor.samsung.com/resources/software-resources/Samsung_Magician_Installer_Official_9.0.1.950.exe", "detect": "Samsung Magician", "from": "Samsung"}
  },
  "fixedLocation": [
    "Google.Chrome",
    "Brave.Brave",
    "Opera.Opera",
    "Discord.Discord",
    "SlackTechnologies.Slack",
    "Zoom.Zoom",
    "Microsoft.Teams",
    "Spotify.Spotify",
    "Notion.Notion",
    "Google.GoogleDrive",
    "Dropbox.Dropbox",
    "Microsoft.PowerToys",
    "WinDirStat.WinDirStat",
    "AgileBits.1Password",
    "CharlesMilette.TranslucentTB",
    "AnyDesk.AnyDesk",
    "Microsoft.VCRedist.2015+.x64",
    "Microsoft.DotNet.DesktopRuntime.8",
    "Microsoft.DotNet.DesktopRuntime.9",
    "Microsoft.DirectX",
    "Anthropic.Claude",
    "Microsoft.WindowsTerminal",
    "Docker.DockerDesktop",
    "Postman.Postman",
    "Yarn.Yarn",
    "Chocolatey.Chocolatey",
    "Microsoft.VisualStudio.2022.BuildTools",
    "MongoDB.Server",
    "MongoDB.Compass.Full",
    "EpicGames.EpicGamesLauncher",
    "LizardByte.Sunshine",
    "MoonlightGameStreamingProject.Moonlight",
    "Logitech.GHUB",
    "Corsair.iCUE.5"
  ],
  "mac": {
    "Google.Chrome": "cask:google-chrome",
    "Mozilla.Firefox": "cask:firefox",
    "Brave.Brave": "cask:brave-browser",
    "Opera.Opera": "cask:opera",
    "Vivaldi.Vivaldi": "cask:vivaldi",
    "Discord.Discord": "cask:discord",
    "SlackTechnologies.Slack": "cask:slack",
    "Zoom.Zoom": "cask:zoom",
    "Microsoft.Teams": "cask:microsoft-teams",
    "Telegram.TelegramDesktop": "cask:telegram-desktop",
    "OpenWhisperSystems.Signal": "cask:signal",
    "9NKSQGP7F2NH": "cask:whatsapp",
    "VideoLAN.VLC": "cask:vlc",
    "Spotify.Spotify": "cask:spotify",
    "OBSProject.OBSStudio": "cask:obs",
    "Audacity.Audacity": "cask:audacity",
    "HandBrake.HandBrake": "cask:handbrake-app",
    "Microsoft.Office/Word": "cask:microsoft-word",
    "Microsoft.Office/Excel": "cask:microsoft-excel",
    "Microsoft.Office/PowerPoint": "cask:microsoft-powerpoint",
    "Microsoft.Office/Outlook": "cask:microsoft-outlook",
    "Microsoft.Office/OneNote": "cask:microsoft-onenote",
    "TheDocumentFoundation.LibreOffice": "cask:libreoffice",
    "Adobe.Acrobat.Reader.64-bit": "cask:adobe-acrobat-reader",
    "Notion.Notion": "cask:notion",
    "Obsidian.Obsidian": "cask:obsidian",
    "Google.GoogleDrive": "cask:google-drive",
    "Dropbox.Dropbox": "cask:dropbox",
    "Synology.DriveClient": "cask:synology-drive",
    "7zip.7zip": "formula:sevenzip",
    "Bitwarden.Bitwarden": "cask:bitwarden",
    "KeePassXCTeam.KeePassXC": "cask:keepassxc",
    "AgileBits.1Password": "cask:1password",
    "Anthropic.Claude": "cask:claude",
    "Anthropic.ClaudeCode": "cask:claude-code",
    "OpenAI.Codex": "cask:codex",
    "Microsoft.VisualStudioCode": "cask:visual-studio-code",
    "Git.Git": "formula:git",
    "SublimeHQ.SublimeText.4": "cask:sublime-text",
    "OpenJS.NodeJS.LTS": "formula:node@22",
    "Docker.DockerDesktop": "cask:docker-desktop",
    "Postman.Postman": "cask:postman",
    "Python.Python.3.14": "formula:python@3.14",
    "astral-sh.uv": "formula:uv",
    "JetBrains.PyCharm": "cask:pycharm",
    "EclipseAdoptium.Temurin.21.JDK": "cask:temurin@21",
    "EclipseAdoptium.Temurin.25.JDK": "cask:temurin@25",
    "JetBrains.IntelliJIDEA": "cask:intellij-idea",
    "PostgreSQL.PostgreSQL.18": "formula:postgresql@18",
    "DBeaver.DBeaver.Community": "cask:dbeaver-community",
    "PostgreSQL.pgAdmin": "cask:pgadmin4",
    "SQLite.SQLite": "formula:sqlite",
    "DBBrowserForSQLite.DBBrowserForSQLite": "cask:db-browser-for-sqlite",
    "MongoDB.Server": "formula:mongodb/brew/mongodb-community",
    "MongoDB.Compass.Full": "cask:mongodb-compass",
    "GIMP.GIMP": "cask:gimp",
    "Inkscape.Inkscape": "cask:inkscape",
    "BlenderFoundation.Blender": "cask:blender",
    "Vorssaint": "cask:vorssaint",
    "Keka": "cask:keka",
    "DeskPad": "cask:deskpad",
    "BetterDisplay": "cask:betterdisplay",
    "Valve.Steam": "cask:steam",
    "LizardByte.Sunshine": "formula:lizardbyte/homebrew/sunshine",
    "MoonlightGameStreamingProject.Moonlight": "cask:moonlight",
    "EpicGames.EpicGamesLauncher": "cask:epic-games",
    "GOG.Galaxy": "cask:gog-galaxy",
    "ElectronicArts.EADesktop": "cask:ea",
    "SteelSeries.GG": "cask:steelseries-gg",
    "Reolink.Reolink": "cask:reolink",
    "Samsung.Magician": "cask:samsung-magician",
    "ONLYOFFICE.DesktopEditors": "cask:onlyoffice",
    "TeamViewer.TeamViewer": "cask:teamviewer",
    "AnyDesk.AnyDesk": "cask:anydesk",
    "qBittorrent.qBittorrent": "cask:qbittorrent",
    "Malwarebytes.Malwarebytes": "cask:malwarebytes",
    "Blizzard.BattleNet": "cask:battle-net",
    "Logitech.GHUB": "cask:logitech-g-hub",
    "pnpm.pnpm": "formula:pnpm",
    "Yarn.Yarn": "formula:yarn",
    "Oven-sh.Bun": "formula:bun",
    "DenoLand.Deno": "formula:deno"
  },
  "repos": {
    "Mozilla.Firefox": "https://github.com/mozilla-firefox/firefox",
    "Brave.Brave": "https://github.com/brave/brave-browser",
    "Telegram.TelegramDesktop": "https://github.com/telegramdesktop/tdesktop",
    "OpenWhisperSystems.Signal": "https://github.com/signalapp/Signal-Desktop",
    "VideoLAN.VLC": "https://code.videolan.org/videolan/vlc",
    "OBSProject.OBSStudio": "https://github.com/obsproject/obs-studio",
    "Audacity.Audacity": "https://github.com/audacity/audacity",
    "HandBrake.HandBrake": "https://github.com/HandBrake/HandBrake",
    "TheDocumentFoundation.LibreOffice": "https://git.libreoffice.org/core",
    "ONLYOFFICE.DesktopEditors": "https://github.com/ONLYOFFICE/DesktopEditors",
    "7zip.7zip": "https://github.com/ip7z/7zip",
    "Microsoft.PowerToys": "https://github.com/microsoft/PowerToys",
    "ShareX.ShareX": "https://github.com/ShareX/ShareX",
    "WinDirStat.WinDirStat": "https://github.com/windirstat/windirstat",
    "Bitwarden.Bitwarden": "https://github.com/bitwarden/clients",
    "KeePassXCTeam.KeePassXC": "https://github.com/keepassxreboot/keepassxc",
    "CharlesMilette.TranslucentTB": "https://github.com/TranslucentTB/TranslucentTB",
    "qBittorrent.qBittorrent": "https://github.com/qbittorrent/qBittorrent",
    "Rufus.Rufus": "https://github.com/pbatard/rufus",
    "CrystalDewWorld.CrystalDiskInfo": "https://github.com/hiyohiyo/CrystalDiskInfo",
    "Microsoft.DotNet.DesktopRuntime.8": "https://github.com/dotnet/runtime",
    "Microsoft.DotNet.DesktopRuntime.9": "https://github.com/dotnet/runtime",
    "OpenAI.Codex": "https://github.com/openai/codex",
    "Microsoft.VisualStudioCode": "https://github.com/microsoft/vscode",
    "Git.Git": "https://github.com/git-for-windows/git",
    "Microsoft.WindowsTerminal": "https://github.com/microsoft/terminal",
    "Notepad++.Notepad++": "https://github.com/notepad-plus-plus/notepad-plus-plus",
    "OpenJS.NodeJS.LTS": "https://github.com/nodejs/node",
    "Python.Python.3.14": "https://github.com/python/cpython",
    "astral-sh.uv": "https://github.com/astral-sh/uv",
    "EclipseAdoptium.Temurin.21.JDK": "https://github.com/adoptium/jdk21u",
    "EclipseAdoptium.Temurin.25.JDK": "https://github.com/adoptium/jdk25u",
    "PostgreSQL.PostgreSQL.18": "https://github.com/postgres/postgres",
    "DBeaver.DBeaver.Community": "https://github.com/dbeaver/dbeaver",
    "PostgreSQL.pgAdmin": "https://github.com/pgadmin-org/pgadmin4",
    "SQLite.SQLite": "https://github.com/sqlite/sqlite",
    "DBBrowserForSQLite.DBBrowserForSQLite": "https://github.com/sqlitebrowser/sqlitebrowser",
    "GIMP.GIMP": "https://gitlab.gnome.org/GNOME/gimp",
    "Inkscape.Inkscape": "https://gitlab.com/inkscape/inkscape",
    "BlenderFoundation.Blender": "https://projects.blender.org/blender/blender",
    "LizardByte.Sunshine": "https://github.com/LizardByte/Sunshine",
    "MoonlightGameStreamingProject.Moonlight": "https://github.com/moonlight-stream/moonlight-qt",
    "VirtualDrivers.Virtual-Display-Driver": "https://github.com/VirtualDrivers/Virtual-Display-Driver",
    "Keka": "https://github.com/aonez/Keka",
    "Vorssaint": "https://github.com/vorssaint/vorssaint-utils",
    "DeskPad": "https://github.com/Stengo/DeskPad",
    "pnpm.pnpm": "https://github.com/pnpm/pnpm",
    "Yarn.Yarn": "https://github.com/yarnpkg/yarn",
    "Oven-sh.Bun": "https://github.com/oven-sh/bun",
    "DenoLand.Deno": "https://github.com/denoland/deno",
    "Chocolatey.Chocolatey": "https://github.com/chocolatey/choco",
    "Devolutions.UniGetUI": "https://github.com/Devolutions/UniGetUI"
  }
};

// Retakes the screenshots in docs/screenshots/ from the page and the installer window (with demo data,
// the Windows side mocked as in tests/installer-window.test.mjs). Needs the test packages
// (tests/: npm install, npx playwright install chromium). Run from the project folder:
//   node tools/screenshots.mjs
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const OUT = path.join(ROOT, "docs", "screenshots");
const { chromium } = createRequire(path.join(ROOT, "tests", "package.json"))("playwright");
const iconsJs = fs.readFileSync(path.join(ROOT, "web", "icons.js"), "utf8");
const ICONS = JSON.parse(iconsJs.slice(iconsJs.indexOf("{"), iconsJs.lastIndexOf("}") + 1));

const APPS = [
  ["Google.Chrome", "Google Chrome", { fixed: true }], ["VideoLAN.VLC", "VLC media player"], ["Valve.Steam", "Steam"],
  ["Microsoft.VisualStudioCode", "Visual Studio Code"], ["Samsung.Magician", "Samsung Magician", { fixed: true, manual: true }],
].map(([id, name, extra]) => ({ id, name, icon: ICONS[id], ...extra }));
const SCRIPTS = [
  { name: "monthly-printer-test.ps1", kind: "ps1", lines: 82, body: "# Monthly printer test page, so the ink or toner doesn't dry out." },
  { name: "dark-mode.ps1", kind: "ps1", lines: 11, body: "# Dark mode for Windows and apps, for the signed-in user." },
];

const browser = await chromium.launch();
async function open(file, { width, height, colorScheme = "dark", init, arg } = {}) {
  const page = await browser.newPage({ viewport: { width, height }, colorScheme, deviceScaleFactor: 1 });
  if (init) await page.addInitScript(init, arg);
  await page.goto(pathToFileURL(path.join(ROOT, file)).href);
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.evaluate(() => document.fonts && document.fonts.ready);
  return page;
}
// The progress bar fills with a short animation; wait for it to end.
const shot = async (page, name) => { await page.waitForTimeout(600); await page.screenshot({ path: path.join(OUT, name) }); console.log("wrote", name); };

// The installer window: window.AI set as the Go side would, bindings mocked.
const installer = (colorScheme, scripts) => open("installer/ui.html", {
  width: 680, height: 640, colorScheme, arg: { apps: APPS, scripts },
  init: ({ apps, scripts }) => {
    window.AI = { apps, autoUpdate: false, ...(scripts && { scripts }) };
    window.aiStart = window.aiCancel = window.aiClose = window.aiRunScripts = () => {};
    window.aiPickFolder = () => setTimeout(() => window.ui.picked("D:\\"), 0);
  },
});

let p = await installer("light");
await shot(p, "installer-ready.png");
await p.click("[data-mode=folder]"); await p.waitForTimeout(50);
await shot(p, "installer-location.png");
await p.close();

p = await installer("dark");
await p.click("[data-mode=folder]"); await p.waitForTimeout(50);
await shot(p, "installer-location-dark.png");
await p.close();

p = await installer("light", SCRIPTS);
await shot(p, "installer-scripts-list.png");
await p.close();

p = await installer("dark", SCRIPTS);
await p.click("#primary");
await p.evaluate(() => {
  ui.progress(0, 5, "Installing Google Chrome (1 of 5)"); ui.outcome(0, "installed");
  ui.progress(1, 5, "Installing VLC media player (2 of 5)"); ui.outcome(1, "skipped");
  ui.progress(2, 5, "Installing Steam (3 of 5)");
});
await shot(p, "installer-installing.png");
await p.evaluate(() => {
  ui.outcome(2, "installed"); ui.progress(3, 5, "Installing Visual Studio Code (4 of 5)"); ui.outcome(3, "installed");
  ui.progress(4, 5, "Finish the Samsung Magician setup window (5 of 5)"); ui.outcome(4, "installed"); ui.progress(5, 5, "");
  ui.scriptStarted(0); ui.scriptFinished({ index: 0, note: "Your script monthly-printer-test.ps1 finished.", ok: true });
  ui.scriptStarted(1); ui.scriptFinished({ index: 1, note: "Your script dark-mode.ps1 finished.", ok: true });
  ui.done({ installed: 4, skipped: 1, failed: [], notes: ["Your script monthly-printer-test.ps1 finished.", "Your script dark-mode.ps1 finished."], cancelled: false, scriptsRan: [true, true] });
});
await shot(p, "installer-done-dark.png");
await p.close();

// The page, in the default dark theme.
p = await open("web/index.html", { width: 1280, height: 860 });
await p.evaluate(() => { selected.add("Valve.Steam"); selected.add("Microsoft.VisualStudioCode"); update(); });
await shot(p, "page-main.png");
await p.evaluate(() => {
  scripts.find(s => s.key === "lib:monthly-printer-test").on = true;
  addOwnScript("my-setup.ps1", "ps1", "git config --global user.name \"Your Name\"\ncode --install-extension ms-python.python");
  renderScripts(); update();
  const top = document.querySelector("#scriptsSec").getBoundingClientRect().top + scrollY - 70;
  scrollTo(0, top);
});
await p.waitForTimeout(100);
await shot(p, "page-scripts-list.png");
await p.close();

// The Mac page: the platform choice is remembered in localStorage.
p = await open("web/index.html", { width: 1280, height: 860, init: () => localStorage.setItem("ai-platform", "mac") });
await shot(p, "page-mac.png");
await p.close();

await browser.close();

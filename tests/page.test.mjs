// Tests for the web page (web/index.html): picking apps, the package panel, notices and the
// installer file it builds.
import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { playwright, open, ROOT } from "./helpers.mjs";

let browser;
before(async () => { browser = await playwright().chromium.launch(); });
after(async () => { await browser?.close(); });

const page = (opts) => open(browser, "web/index.html", opts);

// Builds the installer in the page and reads its app list back from the end of the file.
const built = p => p.evaluate(() => {
  const b = buildExe(), n = b.length;
  const magic = new TextDecoder().decode(b.subarray(n - 8));
  const len = new DataView(b.buffer, b.byteOffset).getUint32(n - 12, true);
  return { magic, baseLen: n - 12 - len, payload: JSON.parse(new TextDecoder().decode(b.subarray(n - 12 - len, n - 12))) };
});
const ids = p => p.evaluate(() => selectedApps().map(a => a.id));
const count = p => p.$eval("#n", e => +e.textContent);
const tick = (p, id) => p.click(`#catalog label[data-row="${id}"]`);

test("loads with the starter apps and no errors", async () => {
  const p = await page();
  assert.deepEqual(await ids(p), ["Google.Chrome", "7zip.7zip", "VideoLAN.VLC", "Discord.Discord"]);
  assert.equal(await count(p), 4);
  assert.equal(await p.isEnabled("#dl"), true);
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("ticking and unticking an app updates the package", async () => {
  const p = await page();
  await tick(p, "Valve.Steam");
  assert.equal(await count(p), 5);
  assert.ok((await ids(p)).includes("Valve.Steam"));
  assert.match(await p.textContent("#code"), /Steam/);
  await tick(p, "Valve.Steam");
  assert.equal(await count(p), 4);
  await p.close();
});

test("search narrows the list", async () => {
  const p = await page();
  await p.fill("#q", "vlc");
  const rows = await p.$$eval("#catalog .row .nm", els => els.map(e => e.textContent));
  assert.deepEqual(rows, ["VLC media player"]);
  await p.fill("#q", "no such app");
  assert.match(await p.textContent("#catalog"), /No apps match/);
  await p.close();
});

test("every template app is in the catalog", async () => {
  const p = await page();
  assert.deepEqual(await p.evaluate(() => PRESETS.flatMap(x => x[2]).filter(id => !names.has(id))), []);
  await p.close();
});

test("Quick start adds its 14 apps, with Office as one Microsoft 365 install", async () => {
  const p = await page();
  await p.click("#clear");
  await p.click('[data-preset="0"]');
  assert.equal(await count(p), 14);
  const apps = await p.evaluate(() => selectedApps());
  assert.equal(apps.length, 12, "Word, Excel and PowerPoint become one entry");
  assert.deepEqual(apps.find(a => a.id === "Microsoft.Office").office, ["Word", "Excel", "PowerPoint"]);
  assert.match(await p.textContent('#presets .tpl:first-child .apps'), /^Google Chrome · Mozilla Firefox · VLC media player · Spotify · Zoom · Word · and 8 more$/);
  await p.click('[data-preset="0"]');
  assert.equal(await count(p), 0);
  await p.close();
});

test("removing a template keeps apps picked by hand or in another added template", async () => {
  const p = await page();
  await p.click("#clear");
  await tick(p, "JetBrains.PyCharm"); // by hand, also in the Python template
  await p.click('[data-preset="1"]'); // Python
  await p.click('[data-preset="2"]'); // Java: shares VS Code, Git and Windows Terminal with Python
  await p.click('[data-preset="1"]'); // remove Python
  const left = await ids(p);
  for (const id of ["Microsoft.VisualStudioCode", "Git.Git", "Microsoft.WindowsTerminal"]) assert.ok(left.includes(id), `${id} stays for Java`);
  assert.ok(left.includes("JetBrains.PyCharm"), "picked by hand, so it stays");
  for (const id of ["Python.Python.3.14", "astral-sh.uv"]) assert.ok(!left.includes(id), `${id} goes with Python`);
  await p.click('[data-preset="2"]'); // remove Java too
  assert.deepEqual(await ids(p), ["JetBrains.PyCharm"]);
  await p.close();
});

test("Manage templates: save the selection, keep it after a reload, delete it", async () => {
  const p = await page();
  const cards = () => p.$$eval("#presets .tpl h3", els => els.map(e => e.textContent));
  await p.click("#tplManage");
  assert.equal(await p.isVisible("#tplDlg"), true);
  await p.click("#tmSave");
  assert.match(await p.textContent("#tmMsg"), /name/, "a name is needed");
  await p.fill("#tmName", "My PC");
  await p.click("#tmSave");
  assert.match(await p.textContent("#tmMsg"), /My PC saved as a template/);
  assert.match(await p.textContent("#tmOwn"), /My PC.*4 apps/);
  await p.fill("#tmName", "my pc");
  await p.click("#tmSave");
  assert.match(await p.textContent("#tmMsg"), /already a template/);
  await p.click('#tplDlg .dlg-f [data-close]');
  assert.deepEqual((await cards()).slice(-1), ["My PC"], "a card after the included ones");

  await p.reload();
  assert.deepEqual((await cards()).slice(-1), ["My PC"], "kept in this browser");
  await p.click("#clear");
  await p.click(`[data-preset="${(await cards()).length - 1}"]`);
  assert.deepEqual(await ids(p), ["Google.Chrome", "7zip.7zip", "VideoLAN.VLC", "Discord.Discord"], "the card adds its apps");
  await p.click("#tplManage");
  await p.click('#tmOwn [data-tm-del="0"]');
  assert.match(await p.textContent("#tmMsg"), /My PC deleted/);
  await p.reload();
  assert.ok(!(await cards()).includes("My PC"));
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("Manage templates: hide and show an included template", async () => {
  const p = await page();
  const cards = () => p.$$eval("#presets .tpl h3", els => els.map(e => e.textContent));
  await p.click("#tplManage");
  await p.click('#tmBuiltin [data-tm-hide="3"]'); // Database
  assert.ok(!(await cards()).includes("Database"));
  assert.match(await p.textContent("#tmBuiltin"), /Database\s*Hidden\s*Show/);
  await p.reload();
  assert.ok(!(await cards()).includes("Database"), "stays hidden after a reload");
  await p.click("#tplManage");
  await p.click('#tmBuiltin [data-tm-hide="3"]');
  assert.ok((await cards()).includes("Database"));
  await p.close();
});

test("Manage templates: import a templates file, and refuse anything else", async () => {
  const p = await page();
  await p.click("#tplManage");
  const file = (name, json) => ({ name, mimeType: "application/json", buffer: Buffer.from(JSON.stringify(json)) });
  await p.setInputFiles("#tmFile", file("x.json", { hello: 1 }));
  await p.waitForFunction(() => document.querySelector("#tmMsg").textContent);
  assert.match(await p.textContent("#tmMsg"), /isn't an Crescent templates file/);
  await p.setInputFiles("#tmFile", file("crescent-templates.json", { crescentTemplates: 1,
    own: [{ name: "Gaming", apps: ["Valve.Steam", "Some.Custom", "bad id!"] }, { name: "", apps: ["Git.Git"] }], hidden: ["Java", "Nope"] }));
  await p.waitForFunction(() => /Imported/.test(document.querySelector("#tmMsg").textContent));
  assert.match(await p.textContent("#tmMsg"), /Imported 1 template\./);
  const store = await p.evaluate(() => tplStore);
  assert.deepEqual(store, { own: [{ name: "Gaming", apps: ["Valve.Steam", "Some.Custom"] }], hidden: ["Java"] }, "bad IDs, nameless templates and unknown names are dropped");
  await p.click('#tplDlg .dlg-f [data-close]');
  await p.click("#clear");
  const i = (await p.$$eval("#presets .tpl h3", els => els.map(e => e.textContent))).indexOf("Gaming");
  await p.click(`[data-preset="${i}"]`);
  assert.deepEqual(await ids(p), ["Valve.Steam", "Some.Custom"]);
  assert.match(await p.textContent("#cats"), /Added by you/, "an ID from a winget ID appears in Added by you");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("the theme button switches to light and back, and remembers it", async () => {
  const p = await page();
  const bg = () => p.evaluate(() => getComputedStyle(document.body).backgroundColor);
  assert.equal(await bg(), "rgb(18, 13, 28)");
  assert.equal(await p.getAttribute("#themeBtn", "aria-label"), "Switch to the light theme");
  await p.click("#themeBtn");
  assert.equal(await bg(), "rgb(244, 243, 250)");
  await p.reload();
  assert.equal(await bg(), "rgb(244, 243, 250)", "remembered");
  assert.equal(await p.getAttribute("#themeBtn", "aria-label"), "Switch to the dark theme");
  await p.click("#themeBtn");
  assert.equal(await bg(), "rgb(18, 13, 28)");
  await p.close();
});

test("your own script can be renamed, and two scripts never share a name", async () => {
  const p = await page();
  await p.click("#scriptAdd");
  await p.fill("#scriptDraftName", "setup tools");
  await p.fill("#scriptBody", "Write-Host one");
  await p.click("#scriptSave");
  await p.click("#scriptAdd");
  await p.fill("#scriptDraftName", "Setup Tools.ps1");
  await p.fill("#scriptBody", "Write-Host two");
  await p.click("#scriptSave");
  assert.deepEqual((await payloadScripts(p)).map(s => s.name), ["setup tools.ps1", "Setup Tools-2.ps1"], "the extension is added, a taken name gets -2");
  await p.click('#scriptList [aria-label="Edit setup tools.ps1"]');
  await p.fill("#scriptDraftName", "renamed.ps1");
  await p.click("#scriptSave");
  assert.deepEqual((await payloadScripts(p)).map(s => s.name), ["renamed.ps1", "Setup Tools-2.ps1"]);
  await p.close();
});

test("notices only mention the tools picked, and maker downloads are listed last", async () => {
  const p = await page();
  await p.evaluate(() => { selected.clear(); selected.add("Samsung.Magician"); selected.add("EclipseAdoptium.Temurin.21.JDK"); selected.add("Valve.Steam"); update(); });
  const cli = await p.$$eval("#notices li", els => els.map(e => e.textContent).find(t => t.startsWith("Ready for the command line")));
  assert.equal(cli, "Ready for the command lineJava sets JAVA_HOME.");
  assert.deepEqual(await ids(p), ["EclipseAdoptium.Temurin.21.JDK", "Valve.Steam", "Samsung.Magician"], "Samsung last, as the installer runs it");
  assert.match(await p.textContent("#code li:last-child"), /Samsung Magician/);
  await p.close();
});

test("the catalog comes from web/apps.js", async () => {
  const p = await page();
  const r = await p.evaluate(() => ({ groups: APPS.groups.length, apps: CATALOG.flatMap(g => g[1]).filter(a => !MAC_ONLY.has(a[1])).length, templates: PRESETS.length,
    rows: document.querySelectorAll("#catalog .row").length, cards: document.querySelectorAll("#presets .tpl").length }));
  assert.ok(r.groups > 10 && r.apps > 90, JSON.stringify(r));
  assert.equal(r.rows, r.apps, "every Windows app is shown");
  assert.equal(r.cards, r.templates, "every template is shown");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("a quick setup adds its apps and removes them again", async () => {
  const p = await page();
  await p.click('[data-preset="1"]');
  const withPython = await ids(p);
  for (const id of ["Python.Python.3.14", "astral-sh.uv", "JetBrains.PyCharm"]) assert.ok(withPython.includes(id), id);
  await p.click('[data-preset="1"]');
  assert.ok(!(await ids(p)).includes("Python.Python.3.14"));
  await p.close();
});

test("Word and Excel become one Microsoft 365 install", async () => {
  const p = await page();
  await p.evaluate(() => { selected.clear(); selected.add("Microsoft.Office/Word"); selected.add("Microsoft.Office/Excel"); update(); });
  const apps = await p.evaluate(() => selectedApps());
  assert.equal(apps.length, 1);
  assert.equal(apps[0].id, "Microsoft.Office");
  assert.deepEqual(apps[0].office, ["Word", "Excel"]);
  assert.equal(apps[0].fixed, true);
  assert.match(apps[0].name, /Microsoft 365 \(Word, Excel\)/);
  await p.close();
});

test("apps that can't change folder or need clicks are marked", async () => {
  const p = await page();
  const apps = await p.evaluate(() => {
    ["Valve.Steam", "9NKSQGP7F2NH", "Samsung.Magician"].forEach(id => selected.add(id));
    return Object.fromEntries(selectedApps().map(a => [a.id, a]));
  });
  assert.equal(apps["Google.Chrome"].fixed, true);
  assert.equal(apps["Valve.Steam"].fixed, undefined);
  assert.equal(apps["9NKSQGP7F2NH"].fixed, true, "Store apps can't move");
  assert.equal(apps["Samsung.Magician"].fixed, true);
  assert.match(apps["Samsung.Magician"].url, /^https:\/\/download\.semiconductor\.samsung\.com\/.+\.exe$/);
  assert.equal(apps["Samsung.Magician"].detect, "Samsung Magician");
  await p.close();
});

test("the installer is the base program plus the app list", async () => {
  const p = await page();
  const base = fs.readFileSync(path.join(ROOT, "installer", "Crescent-base.exe"));
  const r = await built(p);
  assert.equal(r.magic, "AUTOINS1");
  assert.equal(r.baseLen, base.length, "base program unchanged");
  assert.deepEqual(r.payload.apps.map(a => a.id), ["Google.Chrome", "7zip.7zip", "VideoLAN.VLC", "Discord.Discord"]);
  assert.equal(r.payload.autoUpdate, false, "weekly updates are off by default");
  assert.ok(r.payload.apps.every(a => !a.icon || a.icon.startsWith("data:image/")));
  await p.close();
});

test("installer-base.js holds the same program as Crescent-base.exe", () => {
  const js = fs.readFileSync(path.join(ROOT, "web", "installer-base.js"), "utf8");
  const b64 = js.match(/CRESCENT_EXE_B64="([^"]+)"/)[1];
  assert.ok(Buffer.from(b64, "base64").equals(fs.readFileSync(path.join(ROOT, "installer", "Crescent-base.exe"))),
    "rebuild installer-base.js after rebuilding the exe (see README, Build the installer)");
});

test("weekly updates switch goes into the installer", async () => {
  const p = await page();
  assert.equal(await p.isChecked("#autoUpdate"), false);
  await p.click('label[for="autoUpdate"]');
  assert.equal((await built(p)).payload.autoUpdate, true);
  await p.close();
});

test("adding an app by winget ID", async () => {
  const p = await page();
  await p.fill("#customId", "not an id!");
  await p.click("#addCustom");
  assert.match(await p.textContent("#customErr"), /doesn't look like a winget ID/);
  await p.fill("#customId", "Mozilla.Thunderbird");
  await p.click("#addCustom");
  assert.ok((await ids(p)).includes("Mozilla.Thunderbird"));
  assert.match(await p.textContent("#cats"), /Added by you/);
  await p.fill("#customId", "valve.steam");
  await p.click("#addCustom");
  assert.match(await p.textContent("#customErr"), /already in the list/);
  assert.ok((await ids(p)).includes("Valve.Steam"), "matches the catalog ID whatever the case");
  await p.close();
});

test("clearing the selection disables the download", async () => {
  const p = await page();
  await p.click("#clear");
  assert.equal(await count(p), 0);
  assert.equal(await p.isDisabled("#dl"), true);
  await p.close();
});

test("notices follow the selection", async () => {
  const p = await page();
  const notices = () => p.$$eval("#notices li b", els => els.map(e => e.textContent));
  let n = await notices();
  assert.ok(n.includes("Windows may show a warning"));
  assert.ok(n.includes("Choose where apps go"));
  assert.ok(!n.some(t => /needs a few clicks/.test(t)));
  await p.evaluate(() => { selected.add("Samsung.Magician"); selected.add("PostgreSQL.PostgreSQL.18"); update(); });
  n = await notices();
  assert.ok(n.includes("Samsung Magician needs a few clicks"));
  assert.ok(n.includes("PostgreSQL password"));
  await p.close();
});

test("fits a phone screen without sideways scrolling", async () => {
  const p = await page({ width: 400, height: 800 });
  // with scripts ticked and one long script open
  await p.evaluate(() => { addOwnScript("a-script-with-a-rather-long-file-name-to-wrap.ps1", "ps1", "x".repeat(400)); renderScripts(); update(); });
  await p.click('#scriptList .sc-row[data-key="own:1"] [data-view]');
  await p.click('#scriptList .sc-row[data-key="lib:dark-mode"] input');
  const wide = await p.evaluate(() => document.documentElement.scrollWidth);
  assert.ok(wide <= 400, `page is ${wide}px wide`);
  await p.close();
});

test("dark is the default theme, and data-theme=light switches to light", async () => {
  const light = await page(), dark = await page({ colorScheme: "dark" });
  const bg = q => q.evaluate(() => getComputedStyle(document.body).backgroundColor);
  assert.equal(await bg(light), "rgb(18, 13, 28)", "dark even when Windows is set to light");
  assert.equal(await bg(dark), "rgb(18, 13, 28)");
  await light.evaluate(() => { document.documentElement.dataset.theme = "light"; });
  assert.equal(await bg(light), "rgb(244, 243, 250)");
  await light.close(); await dark.close();
});

test("every English text has a Romanian one, and the other way round", async () => {
  const p = await page();
  const keys = await p.evaluate(() => ({ en: Object.keys(I18N.en), ro: Object.keys(I18N.ro),
    cats: APPS.groups.filter(g => !g.ro && !["Media", "Microsoft Office", "Python", "Java"].includes(g.name)).map(g => g.name),
    templates: APPS.templates.filter(t => !t.ro || !t.descriptionRo).map(t => t.name) }));
  assert.deepEqual(keys.en.filter(k => !keys.ro.includes(k)), [], "missing in ro");
  assert.deepEqual(keys.ro.filter(k => !keys.en.includes(k)), [], "missing in en");
  assert.deepEqual(keys.cats, [], "categories without a Romanian name (in web/apps.js)");
  assert.deepEqual(keys.templates, [], "templates without a Romanian name or description (in web/apps.js)");
  await p.close();
});

test("ENG | RO switches the page, remembers it, and the installer keeps English names", async () => {
  const p = await page();
  assert.equal(await p.textContent("h1"), "Choose the apps for your new PC", "English for an English browser");
  await p.click('.lang [data-lang="ro"]');
  assert.equal(await p.getAttribute("html", "lang"), "ro");
  assert.equal(await p.textContent("h1"), "Alege aplicațiile pentru noul tău PC");
  assert.equal(await p.getAttribute("#q", "placeholder"), "Caută aplicații");
  assert.equal(await p.textContent("#dl"), "Descarcă instalatorul");
  const n = await p.evaluate(() => CATALOG.flatMap(g => g[1]).filter(a => !MAC_ONLY.has(a[1])).length); // counted, so new apps need no test change
  assert.equal(await p.textContent("#listSub"), await p.evaluate(n => roN(n, "aplicație", "aplicații"), n)); // roN has its own test below
  assert.match(await p.textContent('#scriptList .sc-row[data-key="lib:dark-mode"] .nm'), /^Mod întunecat/);
  assert.match(await p.textContent("#notices"), /Windows poate afișa un avertisment/);
  await p.click('#catalog label[data-row="astral-sh.uv"]');
  assert.match(await p.textContent("#code"), /uv \(pachete Python\)/);
  const names = await p.evaluate(() => selectedApps().map(a => a.name));
  assert.ok(names.includes("uv (Python packages)"), "the installer window stays English");
  await p.reload();
  assert.equal(await p.textContent("h1"), "Alege aplicațiile pentru noul tău PC", "the choice is remembered");
  await p.click('.lang [data-lang="en"]');
  assert.equal(await p.textContent("h1"), "Choose the apps for your new PC");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("Romanian counts take \"de\" from 20 on", async () => {
  const p = await page();
  const r = await p.evaluate(() => [1, 2, 19, 20, 75, 101, 120, 200].map(n => roN(n, "aplicație", "aplicații")));
  assert.deepEqual(r, ["1 aplicație", "2 aplicații", "19 aplicații", "20 de aplicații", "75 de aplicații", "101 aplicații", "120 de aplicații", "200 de aplicații"]);
  await p.close();
});

// Scripts: the project's library (scripts.js, built from scripts/) and the user's own, in one list.
const LIBRARY = JSON.parse(fs.readFileSync(path.join(ROOT, "scripts", "scripts.json"), "utf8"));
const libBody = rel => fs.readFileSync(path.join(ROOT, "scripts", rel), "utf8").replace(/^\uFEFF/, "").replace(/\r\n/g, "\n");
const scriptRows = p => p.$$eval("#scriptList .sc-row", rows => rows.map(r => ({
  key: r.dataset.key, name: r.querySelector(".nm").firstChild.textContent, on: r.querySelector("input").checked,
})));
const tickScript = (p, key) => p.click(`#scriptList .sc-row[data-key="${key}"] input`);
const payloadScripts = async p => (await built(p)).payload.scripts;
const notices = p => p.$$eval("#notices li", els => els.map(e => e.textContent));
// Opens the page with a script library of the test's own in place of scripts.js (undefined: none).
const pageWithLibrary = (lib, opts) => page({ ...opts, arg: lib, init: lib => {
  Object.defineProperty(window, "SCRIPT_LIBRARY", { get: () => lib, set() {}, configurable: true });
} });

test("the project's scripts are listed, all off", async () => {
  const p = await page();
  const win = LIBRARY.filter(e => e.windows);
  assert.ok(win.length >= 2, "scripts/scripts.json has the starter scripts");
  const rows = await scriptRows(p);
  assert.deepEqual(rows.map(r => r.key), win.map(e => "lib:" + e.id), "in scripts.json order");
  assert.deepEqual(rows.map(r => r.name), win.map(e => e.name));
  assert.ok(rows.every(r => !r.on), "library scripts start unticked");
  assert.equal(await payloadScripts(p), undefined, "no scripts unless one is ticked");
  assert.match(await p.textContent('#cats [data-goto="scriptsSec"]'), new RegExp(`^Scripts${win.length}$`));
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("scripts.js matches scripts/", () => {
  // Rebuild with: python3 tools/build_scripts.py
  const js = fs.readFileSync(path.join(ROOT, "web", "scripts.js"), "utf8");
  const lib = JSON.parse(js.slice(js.indexOf("["), js.lastIndexOf("]") + 1));
  assert.deepEqual(lib.map(e => e.id), LIBRARY.map(e => e.id));
  for (const [i, e] of LIBRARY.entries()) {
    assert.equal(lib[i].name, e.name);
    assert.equal(lib[i].windows?.body ?? null, e.windows ? libBody(e.windows) : null, `${e.id} is up to date`);
    if (e.windows) assert.equal(lib[i].windows.name, path.basename(e.windows));
  }
});

test("ticked scripts go into the installer in list order", async () => {
  const p = await page();
  await tickScript(p, "lib:dark-mode");
  await tickScript(p, "lib:show-file-extensions");
  const sc = await payloadScripts(p);
  assert.deepEqual(sc.map(s => s.name), ["show-file-extensions.ps1", "dark-mode.ps1"], "the list's order, not the order ticked");
  assert.deepEqual(sc[1], { name: "dark-mode.ps1", kind: "ps1", body: libBody("windows/dark-mode.ps1") });
  assert.match(await p.textContent("#notices"), /Your scripts run last.*runs show-file-extensions\.ps1 and dark-mode\.ps1 as administrator, one after another/);
  assert.match(await p.textContent('#cats [data-goto="scriptsSec"]'), new RegExp(`2/${LIBRARY.filter(e => e.windows).length}$`));
  await tickScript(p, "lib:show-file-extensions");
  assert.deepEqual((await payloadScripts(p)).map(s => s.name), ["dark-mode.ps1"]);
  assert.ok((await notices(p)).some(t => /^Your script runs last.*runs dark-mode\.ps1 as administrator in its own window/.test(t)));
  await p.close();
});

test("the Package list shows ticked scripts after the apps, and × turns one off", async () => {
  const p = await page();
  await tickScript(p, "lib:show-file-extensions");
  await tickScript(p, "lib:dark-mode");
  const items = () => p.$$eval("#code li", els => els.map(e => e.querySelector(".no").textContent + " " + e.querySelector(".nm").textContent));
  assert.deepEqual(await items(), ["1 Google Chrome", "2 7-Zip", "3 VLC media player", "4 Discord", "5 Show file extensions", "6 Dark mode"]);
  assert.equal(await count(p), 4, "the count is apps only");
  assert.equal(await p.textContent(".pk-top .js-nscripts"), "+ 2 scripts");
  await p.click('#code [aria-label="Turn off Show file extensions"]');
  assert.deepEqual((await items()).slice(4), ["5 Dark mode"]);
  assert.equal(await p.isChecked('#scriptList input[data-script="lib:show-file-extensions"]'), false);
  assert.equal(await p.textContent(".pk-top .js-nscripts"), "+ 1 script");
  await p.click("#clear");
  assert.equal(await payloadScripts(p), undefined, "Clear selection turns scripts off too");
  assert.equal(await p.textContent(".pk-top .js-nscripts"), "");
  await p.close();
});

test("View shows a script's text, never as HTML", async () => {
  const p = await page();
  const view = '#scriptList .sc-row[data-key="lib:dark-mode"] [data-view]', pre = '#scriptList .sc-row[data-key="lib:dark-mode"] pre';
  assert.equal(await p.isVisible(pre), false);
  await p.click(view);
  assert.equal(await p.isVisible(pre), true);
  assert.equal(await p.textContent(pre), libBody("windows/dark-mode.ps1"));
  assert.equal(await p.getAttribute(view, "aria-expanded"), "true");
  await p.click(view);
  assert.equal(await p.isVisible(pre), false);

  const body = '<img src=x onerror="window.hacked=1"><b>bold</b> & "quotes"';
  await p.evaluate(b => { addOwnScript("<i>x</i>.ps1", "ps1", b); renderScripts(); update(); }, body);
  await p.click('#scriptList .sc-row[data-key="own:1"] [data-view]');
  const shown = await p.$eval('#scriptList .sc-row[data-key="own:1"] pre', el => ({ text: el.textContent, kids: el.children.length }));
  assert.deepEqual(shown, { text: body, kids: 0 });
  assert.equal(await p.evaluate(() => document.querySelectorAll("#scriptList img, #scriptList i, #scriptList b, #code i").length), 0);
  assert.equal(await p.evaluate(() => window.hacked), undefined);
  assert.equal((await payloadScripts(p))[0].name, "script.ps1", "a name with < > becomes script.ps1");
  await p.close();
});

test("adding two of your own scripts, editing and removing one", async () => {
  const p = await page();
  await p.click("#scriptAdd");
  assert.equal(await p.isVisible("#scriptDlg"), true);
  await p.fill("#scriptBody", "   ");
  await p.click("#scriptSave");
  assert.match(await p.textContent("#scriptMsg"), /empty/);
  assert.equal(await p.isVisible("#scriptDlg"), true, "an empty script isn't saved");
  await p.fill("#scriptBody", "Write-Host 'Gata'\ngit --version");
  await p.click("#scriptSave");
  assert.equal(await p.isVisible("#scriptDlg"), false);

  await p.click("#scriptAdd");
  assert.equal(await p.inputValue("#scriptDraftName"), "my-script-2.ps1", "a new script gets a name not used yet");
  await p.check('input[name="scriptKind"][value="cmd"]');
  assert.equal(await p.inputValue("#scriptDraftName"), "my-script-2.cmd", "switching kind changes the extension");
  await p.fill("#scriptBody", "@echo off\r\necho two");
  await p.click("#scriptSave");

  let rows = await scriptRows(p);
  assert.deepEqual(rows.slice(-2), [{ key: "own:1", name: "my-script.ps1", on: true }, { key: "own:2", name: "my-script-2.cmd", on: true }], "your scripts come after the library, ticked");
  assert.match(await p.textContent('#scriptList .sc-row[data-key="own:1"] .ds'), /Added by you · 2 lines/);
  await tickScript(p, "lib:dark-mode");
  assert.deepEqual(await payloadScripts(p), [
    { name: "dark-mode.ps1", kind: "ps1", body: libBody("windows/dark-mode.ps1") },
    { name: "my-script.ps1", kind: "ps1", body: "Write-Host 'Gata'\ngit --version" },
    { name: "my-script-2.cmd", kind: "cmd", body: "@echo off\r\necho two" },
  ]);

  await p.click('#scriptList [aria-label="Edit my-script.ps1"]');
  assert.equal(await p.inputValue("#scriptBody"), "Write-Host 'Gata'\ngit --version");
  assert.equal(await p.textContent("#scriptDlgTitle"), "Edit your script");
  await p.check('input[name="scriptKind"][value="cmd"]');
  await p.keyboard.press("Escape");
  assert.equal((await payloadScripts(p))[1].kind, "ps1", "Esc discards the edit");
  await p.click('#scriptList [aria-label="Edit my-script.ps1"]');
  await p.fill("#scriptBody", "Write-Host 'Gata!'");
  await p.click("#scriptSave");
  assert.deepEqual((await payloadScripts(p)).map(s => s.name + ": " + s.body.split("\n")[0]),
    ["dark-mode.ps1: # Dark mode for Windows and apps, for the signed-in user.", "my-script.ps1: Write-Host 'Gata!'", "my-script-2.cmd: @echo off\r"], "an edit keeps its place");

  await p.click('#scriptList [aria-label="Remove my-script.ps1"]');
  assert.deepEqual((await payloadScripts(p)).map(s => s.name), ["dark-mode.ps1", "my-script-2.cmd"]);
  assert.equal(await p.evaluate(() => document.activeElement.dataset.script), "own:2", "focus moves to the next script");
  await p.click('#scriptList [aria-label="Remove my-script-2.cmd"]');
  assert.equal(await p.evaluate(() => document.activeElement.id), "scriptAdd");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("opening a .bat file sets Command Prompt and keeps its name", async () => {
  const p = await page();
  await p.click("#scriptAdd");
  await p.setInputFiles("#scriptFile", { name: "setup tools.bat", mimeType: "text/plain", buffer: Buffer.from("@echo off\r\necho gata\r\n") });
  await p.waitForFunction(() => document.querySelector("#scriptBody").value.includes("echo gata"));
  assert.equal(await p.isChecked('input[name="scriptKind"][value="cmd"]'), true);
  await p.click("#scriptSave");
  const [s] = await payloadScripts(p);
  assert.equal(s.name, "setup tools.bat");
  assert.equal(s.kind, "cmd");
  await p.close();
});

test("a script with a NUL character is refused", async () => {
  const p = await page();
  await p.click("#scriptAdd");
  await p.$eval("#scriptBody", el => { el.value = "echo a\u0000b"; el.dispatchEvent(new Event("input", { bubbles: true })); });
  await p.click("#scriptSave");
  assert.match(await p.textContent("#scriptMsg"), /NUL/);
  assert.equal(await payloadScripts(p), undefined);
  await p.close();
});

test("script names follow the installer's rules", async () => {
  const p = await page();
  const names = await p.evaluate(() => [
    ["setup.ps1", "ps1"], ["setup.bat", "cmd"], ["setup.txt", "cmd"], ["C:\\Users\\me\\x.ps1", "ps1"],
    ["con.cmd", "cmd"], ["LPT1 .bat", "cmd"], ["nul.ps1", "ps1"], ["-x.ps1", "ps1"], ["a".repeat(61) + ".ps1", "ps1"],
  ].map(([n, k]) => scriptName(n, k)));
  assert.deepEqual(names, ["setup.ps1", "setup.bat", "setup.cmd", "x.ps1", "script.cmd", "script.cmd", "script.ps1", "script.ps1", "script.ps1"]);
  await p.close();
});

test("an installer takes at most 20 scripts", async () => {
  const p = await page();
  await p.evaluate(() => { for (let i = 1; i <= 20; i++) addOwnScript(`s${i}.cmd`, "cmd", "echo " + i); renderScripts(); update(); });
  await tickScript(p, "lib:dark-mode");
  assert.equal(await p.isChecked('#scriptList input[data-script="lib:dark-mode"]'), false);
  assert.match(await p.textContent("#scriptsNote"), /up to 20 scripts/);
  await p.click("#scriptAdd");
  assert.equal(await p.isVisible("#scriptDlg"), false, "no room for another");
  assert.equal((await payloadScripts(p)).length, 20);
  assert.equal(await p.textContent(".pk-top .js-nscripts"), "+ 20 scripts");
  await p.close();
});

test("the Scripts entry in the sidebar goes to the Scripts section", async () => {
  const p = await page();
  await p.click('#cats [data-goto="scriptsSec"]');
  await p.waitForFunction(() => document.activeElement.id === "h-scripts");
  await p.waitForFunction(() => Math.abs(document.querySelector("#scriptsSec").getBoundingClientRect().top - 72) < 4);
  assert.equal(await p.$eval('#cats [aria-current="true"]', b => b.dataset.cat), "All", "it isn't an app category");
  await p.close();
});

test("the templates and scripts sections fold, stay folded after a reload, and the sidebar opens Scripts", async () => {
  const p = await page();
  await p.click("#tplSec .fold"); await p.click("#scriptsSec .fold");
  assert.equal(await p.isVisible("#presets"), false);
  await p.reload();
  assert.equal(await p.isVisible("#presets"), false);
  assert.equal(await p.getAttribute("#scriptsSec .fold", "aria-expanded"), "false");
  await p.click('#cats [data-goto="scriptsSec"]');
  assert.equal(await p.isVisible("#scriptList"), true);
  await p.close();
});

test("the library can be missing or have Mac-only scripts", async () => {
  let p = await pageWithLibrary(undefined);
  assert.deepEqual(await scriptRows(p), []);
  assert.match(await p.textContent("#scriptList"), /No scripts yet/);
  assert.deepEqual(p.errors, []);
  await p.close();

  p = await pageWithLibrary([
    { id: "mac-only", name: "Mac only", description: "", windows: null, mac: { name: "x.sh", kind: "sh", body: "echo" } },
    { id: "html", name: "<b>Bold</b>", description: "<img src=x>", windows: { name: "tidy.cmd", kind: "cmd", body: "echo tidy" }, mac: null },
    { id: "empty", name: "Empty", description: "", windows: { name: "e.ps1", kind: "ps1", body: "  " }, mac: null },
  ]);
  assert.deepEqual(await scriptRows(p), [{ key: "lib:html", name: "<b>Bold</b>", on: false }], "only usable Windows scripts");
  assert.equal(await p.$$eval("#scriptList b, #scriptList img", els => els.length), 0);
  await tickScript(p, "lib:html");
  assert.deepEqual(await payloadScripts(p), [{ name: "tidy.cmd", kind: "cmd", body: "echo tidy" }]);
  assert.deepEqual(p.errors, []);
  await p.close();
});

// unzip reads a stored (uncompressed) zip like the page makes: { name: { mode, data } }.
function unzip(buf) {
  const end = buf.length - 22, files = {};
  assert.equal(buf.readUInt32LE(end), 0x06054b50);
  for (let i = 0, o = buf.readUInt32LE(end + 16); i < buf.readUInt16LE(end + 10); i++) {
    const nameLen = buf.readUInt16LE(o + 28), size = buf.readUInt32LE(o + 20), local = buf.readUInt32LE(o + 42);
    const name = buf.subarray(o + 46, o + 46 + nameLen).toString("utf8"), start = local + 30 + buf.readUInt16LE(local + 26);
    files[name] = { mode: buf.readUInt32LE(o + 38) >>> 16, data: buf.subarray(start, start + size) };
    o += 46 + nameLen;
  }
  return files;
}

test("the Mac page offers Homebrew apps and builds Crescent.app", async () => {
  const p = await page();
  await tick(p, "voidtools.Everything");
  await tick(p, "Microsoft.Office/Word");
  assert.equal(await p.$('#catalog label[data-row="Vorssaint"]'), null, "Mac-only app");
  await p.click('#platSw [data-plat="mac"]');
  assert.notEqual(await p.$('#catalog label[data-row="Vorssaint"]'), null, "Mac-only app");
  assert.equal(await p.$('#catalog label[data-row="voidtools.Everything"]'), null, "no Mac version");
  assert.match(await p.textContent("#toast"), /1 app has no Mac version/);
  assert.deepEqual(await ids(p), ["Google.Chrome", "7zip.7zip", "VideoLAN.VLC", "Discord.Discord", "Microsoft.Office/Word"]);
  assert.match(await p.textContent("h1"), /new Mac/);
  assert.equal(await p.textContent("#fileName"), "Crescent.app");
  assert.match(await p.textContent("#notices"), /Homebrew comes first.*Open Anyway/s);
  assert.deepEqual(await scriptRows(p), LIBRARY.filter(e => e.mac).map(e => ({ key: "lib-mac:" + e.id, name: e.name, on: false })));
  await tickScript(p, "lib-mac:dark-mode");
  await p.fill("#customId", "Not.AName");
  await p.click("#addCustom");
  assert.match(await p.textContent("#customErr"), /Homebrew name/);
  await p.fill("#customId", "node@22");
  await p.click("#addCustom");

  await p.waitForFunction(() => window.CRESCENT_MAC_B64);
  const zip = unzip(Buffer.from(await p.evaluate(() => new Promise(done => {
    const r = new FileReader(); r.onload = () => done(r.result.split(",")[1]); r.readAsDataURL(buildMacApp());
  })), "base64"));
  const app = "Crescent.app/Contents/";
  assert.deepEqual(Object.keys(zip), [app + "Info.plist", app + "MacOS/Crescent", app + "Resources/apps.json"]);
  assert.equal(zip[app + "MacOS/Crescent"].mode, 0o100755, "the program can run");
  const js = fs.readFileSync(path.join(ROOT, "web", "installer-mac.js"), "utf8");
  assert.ok(zip[app + "MacOS/Crescent"].data.equals(Buffer.from(js.match(/CRESCENT_MAC_B64="([^"]+)"/)[1], "base64")));
  assert.match(zip[app + "Info.plist"].data.toString(), /<key>CFBundleExecutable<\/key><string>Crescent<\/string>/);
  const payload = JSON.parse(zip[app + "Resources/apps.json"].data);
  assert.deepEqual(payload.apps.map(a => a.id), ["cask:google-chrome", "formula:sevenzip", "cask:vlc", "cask:discord", "cask:microsoft-word", "node@22"]);
  assert.deepEqual(payload.scripts.map(s => [s.name, s.kind]), [["dark-mode.sh", "sh"]]);

  await p.click('#platSw [data-plat="win"]');
  assert.ok(await p.$('#catalog label[data-row="voidtools.Everything"]'));
  assert.ok(!(await ids(p)).includes("node@22"), "Homebrew names don't carry over");
  assert.equal(await p.evaluate(() => localStorage.getItem("ai-platform")), "win");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("open-source apps link to their source code without ticking the app", async () => {
  const p = await page();
  const repos = await p.evaluate(() => APPS.repos);
  const links = await p.$$eval("#catalog .row a.repo", as => as.map(a => [a.closest(".row").dataset.row, a.href, a.target]));
  assert.ok(links.length > 10);
  for (const [id, href, target] of links) {
    assert.equal(href, new URL(repos[id]).href, id);
    assert.equal(target, "_blank");
  }
  assert.equal(await p.$('#catalog label[data-row="Google.Chrome"] a.repo'), null, "closed-source apps have no link");
  await p.evaluate(() => document.addEventListener("click", e => e.target.closest("a.repo") && e.preventDefault()));
  await p.click('#catalog label[data-row="Mozilla.Firefox"] a.repo');
  assert.equal(await p.isChecked('#catalog input[data-id="Mozilla.Firefox"]'), false, "the link doesn't tick the app");
});

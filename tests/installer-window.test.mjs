// Tests for the installer window (installer/ui.html) with the Go side mocked: the Ready screen,
// install location, the optional scripts, progress and the summary.
import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import { playwright, open } from "./helpers.mjs";

let browser;
before(async () => { browser = await playwright().chromium.launch(); });
after(async () => { await browser?.close(); });

const APPS = [
  { id: "Google.Chrome", name: "Google Chrome", fixed: true },
  { id: "VideoLAN.VLC", name: "VLC media player" },
  { id: "Valve.Steam", name: "Steam" },
  { id: "Samsung.Magician", name: "Samsung Magician", fixed: true, manual: true },
];
const SCRIPTS = [
  { name: "setup.ps1", kind: "ps1", lines: 2, body: "Write-Host 'Gata, ăîșț'\n# <b>&amp;</b>" },
  { name: "tools.cmd", kind: "cmd", lines: 1, body: "@echo off" },
];

// window opens ui.html as the Go side would: window.AI set before the page's scripts run, and the
// bindings replaced by mocks that record their calls in window.calls.
async function window_({ apps = APPS, scripts, autoUpdate = false, colorScheme } = {}) {
  return open(browser, "installer/ui.html", {
    width: 680, height: 640, colorScheme, arg: { apps, scripts, autoUpdate },
    init: ({ apps, scripts, autoUpdate }) => {
      window.AI = { apps, autoUpdate, ...(scripts && { scripts }) };
      window.calls = []; window.nextFolder = [];
      const rec = name => (...args) => { window.calls.push([name, ...args]); };
      window.aiStart = rec("aiStart"); window.aiCancel = rec("aiCancel"); window.aiClose = rec("aiClose"); window.aiRunScripts = rec("aiRunScripts");
      window.aiPickFolder = (title, start) => { window.calls.push(["aiPickFolder", title, start]); setTimeout(() => window.ui.picked(window.nextFolder.shift() || ""), 0); };
    },
  });
}
const calls = (p, name) => p.evaluate(n => window.calls.filter(c => c[0] === n).map(c => c.slice(1)), name);
const rows = p => p.$$eval("#apps li", els => els.map(li => ({ name: li.querySelector(".nm").textContent, dest: li.querySelector(".dest").textContent, change: !!li.querySelector("[data-pick]") })));
const pickNext = (p, folder) => p.evaluate(f => window.nextFolder.push(f), folder);
const settle = p => p.waitForTimeout(20);
const scriptRows = p => p.$$eval("#slist li", els => els.map(li => ({ name: li.querySelector(".path").textContent, kind: li.querySelector(".kind").textContent,
  on: li.querySelector("input").checked, shown: li.offsetParent !== null })));
const done = (p, r) => p.evaluate(r => ui.done({ installed: 4, skipped: 0, failed: [], notes: [], cancelled: false, scriptsRan: [], ...r }), r);

test("Ready screen lists the apps and marks the ones that can't move", async () => {
  const p = await window_();
  assert.equal(await p.textContent("#title"), "Ready to install");
  assert.match(await p.textContent("#lede"), /install 4 apps/);
  const r = await rows(p);
  assert.deepEqual(r.map(x => x.name), ["Google Chrome", "VLC media player", "Steam", "Samsung Magician"]);
  assert.deepEqual(r.map(x => x.change), [false, true, true, false]);
  assert.equal(r[0].dest, "Usual place only");
  assert.equal(r[3].dest, "You choose in its own setup");
  assert.equal(await p.isVisible("#scripts"), false, "no scripts section without scripts");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("one folder for all apps, with one app elsewhere", async () => {
  const p = await window_();
  await p.click("[data-mode=folder]"); await settle(p); // picker cancelled: nothing changes
  assert.equal((await rows(p))[1].dest, "Usual place");
  await pickNext(p, "D:\\");
  await p.click("[data-mode=folder]"); await settle(p);
  assert.match(await p.textContent("#whereBody"), /D:\\Program Files/);
  await pickNext(p, "E:\\Games\\Steam");
  await p.click('[data-pick="2"]'); await settle(p);
  const r = await rows(p);
  assert.equal(r[1].dest, "D:\\Program Files\\VLC media player");
  assert.equal(r[2].dest, "E:\\Games\\Steam", "no doubled Steam\\Steam");
  await p.click("#primary");
  assert.deepEqual(await calls(p, "aiStart"), [[["", "D:\\Program Files\\VLC media player", "E:\\Games\\Steam", ""], []]]);
  await p.close();
});

test("a network folder is refused", async () => {
  const p = await window_();
  await pickNext(p, "\\\\nas\\share");
  await p.click("[data-mode=folder]"); await settle(p);
  assert.match(await p.textContent("#whereHint"), /one of this PC's drives/);
  await p.click("#primary");
  assert.deepEqual((await calls(p, "aiStart"))[0][0], ["", "", "", ""]);
  await p.close();
});

test("Reset puts an app back in the shared choice", async () => {
  const p = await window_();
  await pickNext(p, "F:\\Tools");
  await p.click('[data-pick="1"]'); await settle(p);
  assert.equal((await rows(p))[1].dest, "F:\\Tools\\VLC media player");
  await p.click('[data-reset="1"]');
  assert.equal((await rows(p))[1].dest, "Usual place");
  await p.close();
});

test("the location box is hidden when no app can move", async () => {
  const p = await window_({ apps: [APPS[0], APPS[3]] });
  assert.equal(await p.isVisible("#where"), false);
  await p.close();
});

test("installing, then the summary", async () => {
  const p = await window_({ autoUpdate: true });
  assert.match(await p.textContent("#opt"), /Weekly updates/);
  await p.click("#primary");
  assert.equal(await p.textContent("#title"), "Installing your apps");
  await p.evaluate(() => { ui.progress(0, 4, "Installing Google Chrome (1 of 4)"); ui.outcome(0, "installed"); ui.progress(1, 4, "Installing VLC media player (2 of 4)"); ui.outcome(1, "failed"); });
  assert.equal(await p.textContent("#pct"), "25%");
  await p.evaluate(() => ui.done({ installed: 1, skipped: 0, failed: ["VLC media player"], notes: ["A note <b>"], cancelled: false, scriptsRan: [] }));
  assert.equal(await p.textContent("#title"), "Finished, with some problems");
  assert.match(await p.textContent("#summary"), /1 installed.*0 already installed.*1 failed/);
  assert.match(await p.textContent("#notes"), /Failed: VLC media player/);
  assert.match(await p.textContent("#notes"), /A note <b>/, "notes are shown as text");
  assert.equal(await p.isVisible("#runScriptsBtn"), false);
  await p.click("#primary");
  assert.equal((await calls(p, "aiClose")).length, 1);
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("Cancel waits for a setup the user is clicking through", async () => {
  const p = await window_();
  await p.click("#primary");
  await p.evaluate(() => { ui.progress(3, 4, "Finish the Samsung Magician setup window (4 of 4)"); ui.outcome(3, "waiting"); });
  await p.click("#cancel");
  assert.equal(await p.textContent("#status"), "Stopping when you close the Samsung Magician setup window");
  assert.equal((await calls(p, "aiCancel")).length, 1);
  await p.close();
});

test("scripts are listed, on by default, and each can be switched off", async () => {
  const p = await window_({ scripts: SCRIPTS });
  assert.equal(await p.isVisible("#scripts"), true);
  assert.deepEqual(await scriptRows(p), [
    { name: "setup.ps1", kind: "PowerShell · 2 lines", on: true, shown: true },
    { name: "tools.cmd", kind: "Command Prompt · 1 line", on: true, shown: true },
  ]);
  assert.equal(await p.textContent("#scriptsCount"), "2 of 2 on");
  assert.equal(await p.textContent("#scriptsHint"), "Run as administrator, one after another, each in its own window, after the apps are installed.");
  await p.click('#slist [data-run="1"]');
  assert.equal(await p.textContent("#scriptsCount"), "1 of 2 on");
  assert.equal(await p.$eval("#slist li:nth-child(2)", li => li.classList.contains("off")), true);
  await p.click("#primary");
  assert.deepEqual(await calls(p, "aiStart"), [[["", "", "", ""], [true, false]]]);
  assert.equal(await p.isVisible("#scripts"), false, "hidden once installing starts");
  assert.match(await p.textContent("#lede"), /Your script runs when they're done\./);
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("View shows a script as plain text, not HTML", async () => {
  const p = await window_({ scripts: SCRIPTS });
  await p.click('#slist [data-view="0"]');
  assert.equal(await p.isVisible("#scriptCode"), true);
  assert.equal(await p.textContent("#scriptCode"), SCRIPTS[0].body);
  assert.equal(await p.$eval("#scriptCode", e => e.childElementCount), 0, "no markup from the script");
  assert.equal(await p.textContent('#slist [data-view="0"]'), "Hide");
  assert.equal(await p.getAttribute('#slist [data-view="0"]', "aria-expanded"), "true");
  assert.deepEqual((await scriptRows(p)).map(r => r.shown), [true, false], "only the open script's row");
  await p.click('#slist [data-view="0"]');
  assert.equal(await p.isVisible("#scriptCode"), false);
  assert.deepEqual((await scriptRows(p)).map(r => r.shown), [true, true]);
  await p.click('#slist [data-view="1"]');
  assert.equal(await p.textContent("#scriptCode"), "@echo off");
  await p.close();
});

test("scripts run after the apps, in order, then the summary", async () => {
  const p = await window_({ scripts: SCRIPTS });
  await p.click("#primary");
  assert.deepEqual((await calls(p, "aiStart"))[0][1], [true, true]);
  assert.match(await p.textContent("#lede"), /Your scripts run when they're done\./);
  await p.evaluate(() => { ui.progress(4, 4, "Done"); ui.scriptStarted(0); });
  assert.equal(await p.textContent("#status"), "Running script 1 of 2: setup.ps1");
  assert.equal(await p.textContent("#lede"), "Your script is running in its own window. Crescent continues when it closes.");
  assert.equal(await p.textContent("#pct"), "100%");
  await p.evaluate(() => { ui.scriptFinished({ index: 0, note: "Your script setup.ps1 finished.", ok: true }); ui.scriptStarted(1); });
  assert.equal(await p.textContent("#status"), "Running script 2 of 2: tools.cmd");
  await p.evaluate(() => ui.scriptFinished({ index: 1, note: "Your script tools.cmd finished.", ok: true }));
  await done(p, { notes: ["Your script setup.ps1 finished.", "Your script tools.cmd finished."], scriptsRan: [true, true] });
  assert.equal(await p.textContent("#title"), "All set");
  assert.deepEqual(await p.$$eval("#notes li", els => els.map(e => e.textContent)), ["Your script setup.ps1 finished.", "Your script tools.cmd finished."]);
  assert.equal(await p.isVisible("#runScriptsBtn"), false, "nothing left to run");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("Cancel while a script runs waits for its window, and the rest can run later", async () => {
  const p = await window_({ scripts: SCRIPTS });
  await p.click("#primary");
  await p.evaluate(() => ui.scriptStarted(0));
  await p.click("#cancel");
  assert.equal(await p.textContent("#status"), "Stopping when your script's window closes");
  assert.equal((await calls(p, "aiCancel")).length, 1);
  await p.evaluate(() => ui.scriptFinished({ index: 0, note: "Your script setup.ps1 finished.", ok: true }));
  await done(p, { cancelled: true, notes: ["Your script setup.ps1 finished."], scriptsRan: [true, false] });
  assert.equal(await p.isVisible("#runScriptsBtn"), true);
  assert.equal(await p.textContent("#runScriptsBtn"), "Run script", "one left");
  await p.close();
});

test("Run scripts on the last screen runs the ones that were off, once", async () => {
  const p = await window_({ scripts: SCRIPTS });
  await p.click('#slist [data-run="0"]'); await p.click('#slist [data-run="1"]');
  assert.equal(await p.textContent("#scriptsCount"), "0 of 2 on");
  assert.match(await p.textContent("#scriptsHint"), /None will run now/);
  await p.click("#primary");
  assert.deepEqual((await calls(p, "aiStart"))[0][1], [false, false]);
  assert.doesNotMatch(await p.textContent("#lede"), /script/);
  await done(p, { scriptsRan: [false, false] });
  assert.equal(await p.isVisible("#runScriptsBtn"), true);
  assert.equal(await p.textContent("#runScriptsBtn"), "Run scripts");
  await p.click("#runScriptsBtn");
  await p.click("#runScriptsBtn", { force: true }).catch(() => {}); // a double click runs them once
  assert.equal((await calls(p, "aiRunScripts")).length, 1);
  assert.equal(await p.isDisabled("#runScriptsBtn"), true);
  await p.evaluate(() => ui.scriptStarted(0));
  assert.equal(await p.textContent("#runScriptsBtn"), "Running…");
  assert.match(await p.textContent("#notes"), /Running setup\.ps1 in its own window…/);
  await p.evaluate(() => { ui.scriptFinished({ index: 0, note: "Your script setup.ps1 ended with exit code 1.", ok: false }); ui.scriptStarted(1); });
  await p.evaluate(() => ui.scriptFinished({ index: 1, note: "Your script tools.cmd finished.", ok: true }));
  const notes = await p.$$eval("#notes li", els => els.map(e => [e.textContent, e.classList.contains("bad")]));
  assert.deepEqual(notes, [["Your script setup.ps1 ended with exit code 1.", true], ["Your script tools.cmd finished.", false]]);
  assert.equal(await p.isVisible("#runScriptsBtn"), true, "until Go says they're all done");
  await p.evaluate(() => ui.scriptsDone());
  assert.equal(await p.isVisible("#runScriptsBtn"), false);
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("ticked scripts get a row under the apps that follows their state", async () => {
  const p = await window_({ scripts: SCRIPTS });
  const srows = () => p.$$eval("#apps [data-script-row]", els => els.map(li => [li.querySelector(".nm").textContent, li.querySelector(".st").textContent, li.querySelector(".dest").textContent]));
  assert.deepEqual(await srows(), [], "no script rows on the Ready screen");
  await p.click('#slist [data-run="1"]'); // tools.cmd off
  await p.click("#primary");
  assert.deepEqual(await srows(), [["setup.ps1", "Queued", "PowerShell script"]], "only ticked scripts, after the apps");
  await p.evaluate(() => { ui.progress(4, 4, ""); ui.scriptStarted(0); });
  assert.deepEqual(await srows(), [["setup.ps1", "Running", "PowerShell script"]]);
  await p.evaluate(() => ui.scriptFinished({ index: 0, note: "Your script setup.ps1 ended with exit code 2.", ok: false }));
  assert.deepEqual(await srows(), [["setup.ps1", "Failed", "Your script setup.ps1 ended with exit code 2."]]);
  await done(p, { scriptsRan: [true, false], notes: ["Your script setup.ps1 ended with exit code 2."] });
  assert.equal((await srows()).length, 1, "rows stay on the last screen");
  await p.click("#runScriptsBtn");
  await p.evaluate(() => { ui.scriptStarted(1); ui.scriptFinished({ index: 1, note: "Your script tools.cmd finished.", ok: true }); });
  assert.deepEqual((await srows())[1], ["tools.cmd", "Finished", "Your script tools.cmd finished."], "a script run later gets its row");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("Cancel marks scripts that didn't start as skipped", async () => {
  const p = await window_({ scripts: SCRIPTS });
  await p.click("#primary");
  await p.click("#cancel");
  await done(p, { cancelled: true, scriptsRan: [false, false] });
  assert.deepEqual(await p.$$eval("#apps [data-script-row] .st", els => els.map(e => e.textContent)), ["Skipped", "Skipped"]);
  await p.close();
});

test("one script reads naturally", async () => {
  const p = await window_({ scripts: SCRIPTS.slice(0, 1) });
  assert.equal(await p.textContent("#scriptsCount"), "On");
  assert.equal(await p.textContent("#scriptsHint"), "Runs as administrator in its own window after the apps are installed.");
  await p.click("#primary");
  assert.deepEqual((await calls(p, "aiStart"))[0][1], [true]);
  await p.evaluate(() => ui.scriptStarted(0));
  assert.equal(await p.textContent("#status"), "Running your script setup.ps1");
  await p.close();
});

test("a long list of scripts scrolls in its own box and the app list stays in view", async () => {
  const scripts = Array.from({ length: 8 }, (_, i) => ({ name: `step-${i + 1}.ps1`, kind: "ps1", lines: 1, body: `Write-Host ${i + 1}` }));
  scripts[7].name = "<img src=x onerror=window.pwned=1>.ps1";
  const p = await window_({ scripts });
  const box = await p.evaluate(() => {
    const r = s => document.querySelector(s).getBoundingClientRect(), l = document.querySelector("#slist");
    return { list: l.clientHeight, listFull: l.scrollHeight, panel: r("#panel").height, app: r("#apps li").bottom,
      panelBottom: r("#panel").bottom, footerTop: r("footer").top, page: document.documentElement.scrollHeight };
  });
  assert.ok(box.listFull > box.list && box.list <= 132, `the list scrolls (${box.list} of ${box.listFull}px)`);
  assert.ok(box.panel >= 98, `the app list keeps room for two apps (${box.panel}px)`);
  assert.ok(box.app <= box.panelBottom && box.panelBottom <= box.footerTop, "the first app is fully in view");
  assert.equal(box.page, 640, "nothing spills out of the window");
  await p.$eval("#slist", l => { l.scrollTop = l.scrollHeight; });
  assert.equal(await p.$eval("#slist li:last-child .path", e => e.textContent), scripts[7].name, "names are text");
  assert.equal(await p.evaluate(() => !!document.querySelector("#slist img") || !!window.pwned), false);
  await p.click('#slist [data-view="6"]');
  assert.equal(await p.textContent("#scriptCode"), "Write-Host 7");
  assert.ok(await p.$eval("#panel", e => e.getBoundingClientRect().height) >= 50, "an app still shows while a script is open");
  assert.deepEqual(p.errors, []);
  await p.close();
});

test("dark mode changes the colours", async () => {
  const light = await window_(), dark = await window_({ colorScheme: "dark" });
  const bg = q => q.evaluate(() => getComputedStyle(document.body).backgroundColor);
  assert.notEqual(await bg(light), await bg(dark));
  await light.close(); await dark.close();
});

test("on the Mac the window talks about the Mac and Homebrew, with no folder choice", async () => {
  const p = await open(browser, "installer/ui.html", { width: 680, height: 640, init: () => {
    window.AI = { mac: true, apps: [{ id: "cask:vlc", name: "VLC media player", fixed: true }],
      scripts: [{ name: "dark-mode.sh", kind: "sh", lines: 1, body: "osascript" }] };
  } });
  assert.equal(await p.textContent("header span"), "Homebrew");
  assert.match(await p.textContent("#lede"), /on this Mac/);
  assert.equal(await p.isHidden("#where"), true);
  assert.match(await p.textContent("#slist .kind"), /^Shell/);
  assert.match(await p.textContent("#scriptsHint"), /Show details/);
  assert.deepEqual(p.errors, []);
  await p.close();
});

// End to end: the page builds an installer, and the installer program reads its app list back,
// runs winget for each app and then the ticked scripts in order. The program is built for this computer
// (its plain-text version) and winget is replaced by a stand-in that records what it was asked.
// Needs Go and a Unix shell, so it is skipped on Windows.
import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { playwright, open, ROOT } from "./helpers.mjs";

const hasGo = (() => { try { execFileSync("go", ["version"]); return true; } catch { return false; } })();
const skip = process.platform === "win32" ? "needs a Unix shell" : !hasGo ? "needs Go" : false;

let browser, dir;
before(async () => {
  if (skip) return;
  dir = fs.mkdtempSync(path.join(os.tmpdir(), "crescent-e2e-"));
  browser = await playwright().chromium.launch();
});
after(async () => {
  await browser?.close();
  if (dir) fs.rmSync(dir, { recursive: true, force: true });
});

test("the page's installer installs every app, then runs the scripts in order", { skip }, async () => {
  const base = path.join(dir, "base");
  execFileSync("go", ["build", "-o", base, "."], { cwd: path.join(ROOT, "installer"), stdio: "inherit" });

  const p = await open(browser, "web/index.html");
  const exe = await p.evaluate(b64 => {
    selected.clear();
    ["Valve.Steam", "9NKSQGP7F2NH", "Microsoft.Office/Word"].forEach(id => selected.add(id));
    // Two of the user's own scripts (the library's are PowerShell, which needs pwsh here). Off
    // Windows a script runs with sh, which sees each line's Windows line ending; the trailing #
    // turns it into a comment.
    scripts.forEach(s => s.on = false);
    addOwnScript("first.cmd", "cmd", 'echo "first in $PWD" >> "$HOME/script-ran.txt" #\nexit 0 #');
    addOwnScript("second.bat", "cmd", 'echo "second in $PWD" >> "$HOME/script-ran.txt" #\nexit 0 #');
    renderScripts(); update();
    window.CRESCENT_EXE_B64 = b64; baseExe = null; // build from this computer's version
    const b = buildExe();
    let s = ""; for (let i = 0; i < b.length; i += 0x8000) s += String.fromCharCode(...b.subarray(i, i + 0x8000));
    return btoa(s);
  }, fs.readFileSync(base).toString("base64"));
  assert.deepEqual(p.errors, []);
  await p.close();

  const installer = path.join(dir, "Crescent");
  fs.writeFileSync(installer, Buffer.from(exe, "base64"), { mode: 0o755 });
  const bin = path.join(dir, "bin"), log = path.join(dir, "winget.log"), home = path.join(dir, "home");
  fs.mkdirSync(bin); fs.mkdirSync(home);
  fs.writeFileSync(path.join(bin, "winget"), `#!/bin/sh\necho "$*" >> "${log}"\n`, { mode: 0o755 });

  const out = execFileSync(installer, { env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, HOME: home }, encoding: "utf8" });
  const calls = fs.readFileSync(log, "utf8").trim().split("\n");
  assert.equal(calls.length, 3, out);
  assert.match(calls[0], /^install --id Valve\.Steam --exact --silent .*--source winget/);
  assert.match(calls[1], /^install --id 9NKSQGP7F2NH .*--source msstore/);
  assert.match(calls[2], /^install --id Microsoft\.Office .*--override \/configure /);
  assert.match(out, /Installed: 3/);
  assert.match(out, /Your script first\.cmd finished\.[\s\S]*Your script second\.bat finished\./);
  assert.deepEqual(fs.readFileSync(path.join(home, "script-ran.txt"), "utf8").trim().split("\n"),
    [`first in ${home}`, `second in ${home}`], "both ran, in order, in the user's home folder");
});

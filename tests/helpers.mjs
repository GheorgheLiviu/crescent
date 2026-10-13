// Shared helpers for the browser tests: load Playwright, open the page or the installer window
// from disk, and fail on any page error.
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const require = createRequire(import.meta.url);
export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export function playwright() {
  try { return require("playwright"); } catch {}
  return require("/opt/node22/lib/node_modules/playwright"); // the cloud environment's global copy
}

// open loads a file from the project in a new page. Web fonts are blocked so tests run offline.
// options.init runs before the page's own scripts (to inject window.AI or mock bindings).
export async function open(browser, file, { colorScheme = "light", width = 1280, height = 900, init, arg } = {}) {
  const page = await browser.newPage({ viewport: { width, height }, colorScheme });
  const errors = [];
  page.on("pageerror", e => errors.push(e.message));
  page.on("console", m => { if (m.type() === "error" && !/Failed to load resource/.test(m.text())) errors.push(m.text()); });
  await page.route(/fonts\.(googleapis|gstatic)/, r => r.abort());
  if (init) await page.addInitScript(init, arg);
  await page.goto(pathToFileURL(path.join(ROOT, file)).href);
  page.errors = errors;
  return page;
}

// readTrailer splits an installer built by the page into the base program and its app list:
// <exe><JSON><uint32 LE length>"AUTOINS1".
export function readTrailer(bytes) {
  const buf = Buffer.from(bytes);
  const magic = buf.subarray(buf.length - 8).toString("latin1");
  const len = buf.readUInt32LE(buf.length - 12);
  const start = buf.length - 12 - len;
  return { magic, base: buf.subarray(0, start), payload: JSON.parse(buf.subarray(start, buf.length - 12).toString("utf8")) };
}

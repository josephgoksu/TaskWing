// Fast checks for ui/: every JS file parses, and nothing loads from the network.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { execFileSync } from "node:child_process";

const files = [];
const walk = (d) => readdirSync(d).forEach((f) => { const p = join(d, f); statSync(p).isDirectory() ? walk(p) : files.push(p); });
walk("ui");
let bad = 0;
for (const f of files) {
  if (f.endsWith(".js")) {
    // ui/js holds ES modules. `node --check file.js` passes a module with a
    // syntax error, so check it as module source on stdin.
    try { execFileSync(process.execPath, ["--input-type=module", "--check"], { input: readFileSync(f), stdio: "pipe" }); }
    catch (e) { bad++; console.error(`syntax: ${f}\n${e.stderr}`); }
  }
  if (/\.(js|css|html)$/.test(f)) {
    const text = readFileSync(f, "utf8");
    const hits = text.match(/(?:src|href)\s*=\s*["']https?:\/\/|url\(\s*["']?https?:\/\/|@import\s+["']?https?:|\bfrom\s+["']https?:\/\/|\bimport\s*\(\s*["']https?:/g);
    if (hits) { bad++; console.error(`remote load in ${f}: ${hits.join(", ")}`); }
  }
}
console.log(bad ? `${bad} problem(s)` : `ok: ${files.length} files`);
process.exit(bad ? 1 : 0);

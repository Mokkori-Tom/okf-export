// Screenshots of the same pages from okf-export (<out>/okf) and mdBook
// (<out>/mdbook), at a desktop and a phone width and in two themes, put side
// by side in <out>/report.html. Pages are those in both outputs; x.md is
// x.html in each. Run by compare.sh.
const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const out = path.resolve(process.argv[2] || "compare-out");
const sides = [
  { name: "okf-export", dir: path.join(out, "okf"), themeKey: "okf-theme", sidebarKey: "okf-sidebar" },
  { name: "mdBook", dir: path.join(out, "mdbook"), themeKey: "mdbook-theme", sidebarKey: "mdbook-sidebar" },
].filter((s) => fs.existsSync(s.dir));
const views = [
  { name: "desktop", width: 1280, height: 800 },
  { name: "phone", width: 390, height: 844 },
];
const themes = ["light", "navy"];

function pages(dir) {
  const list = [];
  (function walk(d) {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) walk(p);
      else if (e.name.endsWith(".html") && !/^(print|toc|404)\.html$/.test(e.name) && !e.name.endsWith(".md.html")) list.push(path.relative(dir, p));
    }
  })(dir);
  return list.sort();
}

(async () => {
  if (!sides.length) throw new Error("nothing to compare in " + out);
  const common = pages(sides[0].dir).filter((p) => sides.every((s) => fs.existsSync(path.join(s.dir, p))));
  const shots = path.join(out, "shots");
  fs.mkdirSync(shots, { recursive: true });
  const browser = await chromium.launch();
  const rows = [];
  for (const page of common) {
    for (const view of views) {
      for (const theme of themes) {
        const cells = [];
        for (const side of sides) {
          const ctx = await browser.newContext({ viewport: { width: view.width, height: view.height } });
          await ctx.addInitScript(([k, v, sk]) => { localStorage.setItem(k, v); localStorage.setItem(sk, "visible"); }, [side.themeKey, theme, side.sidebarKey]);
          const p = await ctx.newPage();
          await p.goto("file://" + path.join(side.dir, page));
          await p.waitForTimeout(300);
          const file = [page.replace(/[\/\\]/g, "_"), view.name, theme, side.name].join("-") + ".png";
          await p.screenshot({ path: path.join(shots, file) });
          cells.push({ side: side.name, file });
          await ctx.close();
        }
        rows.push({ page, view: view.name, theme, cells });
      }
    }
  }
  await browser.close();
  const esc = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  const html = `<!DOCTYPE html><meta charset="utf-8"><title>okf-export vs mdBook</title>
<style>body{font:14px sans-serif;margin:16px}h2{font-size:15px;margin:24px 0 8px}.row{display:flex;gap:12px;align-items:flex-start}
figure{margin:0;flex:1;min-width:0}figcaption{font-weight:bold;margin-bottom:4px}img{width:100%;border:1px solid #ccc}</style>
<h1>okf-export vs mdBook</h1><p>${common.length} pages × ${views.length} sizes × ${themes.length} themes</p>
${rows.map((r) => `<h2>${esc(r.page)} — ${r.view}, ${r.theme}</h2><div class="row">${r.cells
    .map((c) => `<figure><figcaption>${esc(c.side)}</figcaption><img loading="lazy" src="shots/${encodeURIComponent(c.file)}"></figure>`)
    .join("")}</div>`).join("\n")}`;
  fs.writeFileSync(path.join(out, "report.html"), html);
})().catch((e) => { console.error(e); process.exit(1); });

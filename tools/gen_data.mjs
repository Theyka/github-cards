// One-time data generator: converts upstream reference sources into embedded JSON.
// Run from the project root with the `reference/` folder present (see reference/README-note).
import fs from "node:fs";
import path from "node:path";

const root = new URL("..", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");
const ref = path.join(root, "reference", "upstream");
const out = path.join(root, "internal", "cards", "data");
fs.mkdirSync(out, { recursive: true });

// 1) github-readme-stats (github-stats-extended) themes
{
  const src = fs.readFileSync(path.join(ref, "ghs", "themes__index.ts"), "utf8");
  const start = src.indexOf("export const themes = ") + "export const themes = ".length;
  const end = src.indexOf("} as const satisfies");
  const objText = src.slice(start, end) + "}";
  const themes = new Function("return (" + objText + ")")();
  fs.writeFileSync(path.join(out, "themes_ghstats.json"), JSON.stringify(themes, null, 2) + "\n");
  console.log("ghstats themes:", Object.keys(themes).length);
}

// 2) streak-stats themes (PHP -> JSON)
{
  const src = fs.readFileSync(path.join(ref, "streak", "themes.php"), "utf8");
  const themes = {};
  const blockRe = /^\s{4}"([^"]+)"\s*=>\s*\[([\s\S]*?)\n\s{4}\],?/gm;
  let m;
  while ((m = blockRe.exec(src))) {
    const colorRe = /"([^"]+)"\s*=>\s*"([^"]*)"/g;
    const theme = {};
    let cm;
    while ((cm = colorRe.exec(m[2]))) theme[cm[1]] = cm[2];
    themes[m[1]] = theme;
  }
  fs.writeFileSync(path.join(out, "themes_streak.json"), JSON.stringify(themes, null, 2) + "\n");
  console.log("streak themes:", Object.keys(themes).length);
}

// 3) Segoe UI width table + streak CSS color names + language colors
{
  const renderSrc = fs.readFileSync(path.join(ref, "ghs", "common__render.ts"), "utf8");
  const wm = renderSrc.match(/const widths = \[([\s\S]*?)\];/);
  const widths = wm[1].split(",").map((s) => s.trim()).filter(Boolean).map(Number);
  const avgM = renderSrc.match(/const avg = ([\d.]+);/);
  fs.writeFileSync(
    path.join(out, "segoe_widths.json"),
    JSON.stringify({ widths, avg: Number(avgM[1]) }),
  );
  console.log("segoe widths:", widths.length, "avg:", avgM[1]);

  const colorsSrc = fs.readFileSync(path.join(ref, "streak", "colors.php"), "utf8");
  const names = [...colorsSrc.matchAll(/"([a-z]+)"/g)].map((x) => x[1]).filter((n) => n !== "php");
  fs.writeFileSync(path.join(out, "streak_css_colors.json"), JSON.stringify([...new Set(names)]));
  console.log("css colors:", names.length);

  fs.copyFileSync(
    path.join(ref, "ghs", "common__languageColors.json"),
    path.join(out, "language_colors.json"),
  );
  console.log("language colors copied");
}

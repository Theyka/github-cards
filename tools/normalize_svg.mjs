// Normalizes an SVG for structural comparison: collapses whitespace and
// replaces numbers with N so data-value differences do not show up.
// Usage: node tools/normalize_svg.mjs <input> <output>
import { readFileSync, writeFileSync } from "node:fs";

const [, , input, output] = process.argv;
const svg = readFileSync(input, "utf8");

const normalized = svg
  .replace(/\d+(\.\d+)?/g, "N")
  .replace(/\s+/g, " ")
  .trim();

writeFileSync(output, normalized + "\n");
console.log(`${input} -> ${output} (${normalized.length} chars)`);

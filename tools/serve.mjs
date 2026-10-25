// Minimal static file server for local visual checks.
// Usage: node tools/serve.mjs [port] [root]
import { createServer } from "node:http";
import { readFileSync, existsSync, statSync } from "node:fs";
import { extname, join, normalize } from "node:path";

const port = Number(process.argv[2] ?? 8090);
const root = process.argv[3] ?? process.cwd();

const types = {
  ".html": "text/html; charset=utf-8",
  ".svg": "image/svg+xml",
  ".js": "text/javascript",
  ".json": "application/json",
  ".png": "image/png",
  ".css": "text/css",
};

createServer((req, res) => {
  try {
    const urlPath = decodeURIComponent(new URL(req.url, "http://localhost").pathname);
    const target = normalize(join(root, urlPath));
    if (!target.startsWith(normalize(root))) {
      res.writeHead(403).end("forbidden");
      return;
    }
    if (!existsSync(target) || statSync(target).isDirectory()) {
      res.writeHead(404).end("not found");
      return;
    }
    const data = readFileSync(target);
    res.writeHead(200, { "Content-Type": types[extname(target)] ?? "application/octet-stream" });
    res.end(data);
  } catch (err) {
    res.writeHead(500).end(String(err));
  }
}).listen(port, () => console.log(`serving ${root} at http://localhost:${port}/`));

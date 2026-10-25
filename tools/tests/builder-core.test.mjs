import { test } from "node:test";
import assert from "node:assert/strict";
import { defaultDesign, buildOutputs, validateDesign, serializeDesign, restoreDesign, escapeHTML } from "../../internal/server/web/builder-core.mjs";

const origin = "https://githubstats.theyka.net";

test("stats output uses the active origin and correct endpoint", () => {
  const output = buildOutputs(defaultDesign(), origin);
  const url = new URL(output.url);
  assert.equal(url.origin, origin);
  assert.equal(url.pathname, "/api");
  assert.equal(url.searchParams.get("username"), "Theyka");
  assert.equal(url.searchParams.get("show_icons"), "true");
  assert.equal(url.searchParams.get("disable_animations"), "true");
  assert.equal(url.searchParams.get("embed_width"), null);
  assert.equal(url.searchParams.get("repo"), null);
  assert.match(output.html, /<p align="center">/);
  assert.match(output.html, /width="400"/);
  assert.doesNotMatch(output.html, /<table/);
});

test("pin output links to its repository without leaking stats controls", () => {
  const state = defaultDesign("pin");
  state.repo = "some-project";
  state.hide_rank = true;
  const output = buildOutputs(state, origin);
  const url = new URL(output.url);
  assert.equal(url.pathname, "/api/pin/");
  assert.equal(url.searchParams.get("repo"), "some-project");
  assert.equal(url.searchParams.get("show_owner"), "true");
  assert.equal(url.searchParams.get("hide_rank"), null);
  assert.equal(output.link, "https://github.com/Theyka/some-project");
});

test("streak uses user, daily options, and its own color parameter names", () => {
  const state = defaultDesign("streak");
  state.timezone = "Europe/Paris";
  state.exclude_days = "Sat,Sun";
  state.colors.background = "#141321";
  const output = buildOutputs(state, origin);
  const url = new URL(output.url);
  assert.equal(url.pathname, "/streak/");
  assert.equal(url.searchParams.get("user"), "Theyka");
  assert.equal(url.searchParams.get("username"), null);
  assert.equal(url.searchParams.get("timezone"), "Europe/Paris");
  assert.equal(url.searchParams.get("exclude_days"), "Sat,Sun");
  assert.equal(url.searchParams.get("background"), "141321");
  assert.equal(url.searchParams.get("bg_color"), null);
  state.mode = "weekly";
  assert.equal(new URL(buildOutputs(state, origin).url).searchParams.get("exclude_days"), null);
});

test("color overrides normalize hex and encode gradients as a single parameter", () => {
  const state = defaultDesign();
  state.colors = { title_color: "#ABCDEF", bg_color: "90,141321,302b63", fire: "ff0000" };
  const url = new URL(buildOutputs(state, origin).url);
  assert.equal(url.searchParams.get("title_color"), "ABCDEF");
  assert.equal(url.searchParams.get("bg_color"), "90,141321,302b63");
  assert.equal(url.searchParams.get("fire"), null);
  assert.equal(url.hash, "");
});

test("HTML escapes attributes and the image URL", () => {
  const state = defaultDesign();
  state.custom_title = 'A "quoted" title <script>alert(1)</script> & more';
  const output = buildOutputs(state, origin);
  assert.match(output.html, /&amp;theme=/);
  assert.match(output.html, /&quot;quoted&quot;/);
  assert.match(output.html, /&lt;script&gt;/);
  assert.doesNotMatch(output.html, /<script>/);
  assert.equal(new URL(output.url).searchParams.get("custom_title"), state.custom_title);
  assert.equal(escapeHTML("'&<>\""), "&#39;&amp;&lt;&gt;&quot;");
});

test("Markdown escapes bracket characters in a title", () => {
  const state = defaultDesign();
  state.custom_title = "Work [in progress]";
  assert.match(buildOutputs(state, origin).markdown, /Work \\\[in progress\\\]/);
});

test("HTML embed width does not change the SVG URL", () => {
  const state = defaultDesign();
  const before = buildOutputs(state, origin);
  state.embed_width = "600";
  const after = buildOutputs(state, origin);
  assert.equal(before.url, after.url);
  assert.match(after.html, /width="600"/);
});

test("hidden stats are whitelisted and serialized in stable order", () => {
  const state = defaultDesign();
  state.hide = ["issues", "stars", "not-a-stat"];
  const url = new URL(buildOutputs(state, origin).url);
  assert.equal(url.searchParams.get("hide"), "stars,issues");
});

test("design fragments round-trip all three card types", () => {
  for (const type of ["stats", "pin", "streak"]) {
    const state = defaultDesign(type);
    state.username = "octocat";
    state.embed_width = "500";
    state.hide_border = true;
    state.hide = ["issues"];
    state.colors[type === "streak" ? "ring" : "bg_color"] = "abcdef";
    assert.deepEqual(restoreDesign("#" + serializeDesign(state)), state);
  }
});

test("restoring fragments ignores unknown fields and colors", () => {
  const state = restoreDesign("#card=pin&username=octocat&token=secret&__proto__=bad&color.fire=ff0000&hide=stars,bad");
  assert.equal(state.type, "pin");
  assert.equal(state.username, "octocat");
  assert.equal(Object.hasOwn(state, "token"), false);
  assert.equal(Object.hasOwn(state, "__proto__"), false);
  assert.deepEqual(state.colors, {});
  assert.deepEqual(state.hide, ["stars"]);
  assert.equal(restoreDesign("#card=unknown").type, "stats");
});

test("invalid input cannot generate an embed", () => {
  for (const username of ["", "has spaces", "a&theme=dark", "<script>", "-octocat", "octo--cat", "x".repeat(40)]) {
    const state = defaultDesign();
    state.username = username;
    assert.ok(validateDesign(state).some((error) => error.field === "username"), username);
    assert.throws(() => buildOutputs(state, origin));
  }
  const state = defaultDesign("pin");
  state.repo = "owner/repo";
  assert.ok(validateDesign(state).some((error) => error.field === "repo"));
  assert.throws(() => buildOutputs(defaultDesign(), "javascript:alert(1)"));
});

test("numbers and layout options are validated", () => {
  const state = defaultDesign();
  state.embed_width = "";
  state.border_radius = "NaN";
  state.card_width = "200";
  state.commits_year = "20xx";
  const fields = validateDesign(state).map((error) => error.field);
  assert.ok(fields.includes("embed_width"));
  assert.ok(fields.includes("border_radius"));
  assert.ok(fields.includes("card_width"));
  assert.ok(fields.includes("commits_year"));
});

test("invalid colors and empty streak layouts are rejected", () => {
  const state = defaultDesign("streak");
  state.colors.ring = "not-a-color";
  state.hide_total_contributions = state.hide_current_streak = state.hide_longest_streak = true;
  state.timezone = "Not/A-Timezone";
  const fields = validateDesign(state).map((error) => error.field);
  assert.ok(fields.includes("color.ring"));
  assert.ok(fields.includes("hide_total_contributions"));
  assert.ok(fields.includes("timezone"));
});

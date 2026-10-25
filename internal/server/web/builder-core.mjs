// Pure URL, validation, and embed logic, shared by the browser and Node tests.
// No dependencies, API requests, DOM access, or deployment-specific hostname.
export const CARD_TYPES = ["stats", "pin", "streak"];
export const STAT_NAMES = ["stars", "commits", "prs", "issues", "contribs"];

const STATS_COLORS = [
  ["bg_color", "Background"], ["title_color", "Title"],
  ["text_color", "Text"], ["icon_color", "Icons"],
  ["border_color", "Border"], ["ring_color", "Rank ring"],
];
export const COLOR_FIELDS = {
  stats: STATS_COLORS,
  pin: STATS_COLORS.filter(([key]) => key !== "ring_color"),
  streak: [
    ["background", "Background"], ["border", "Border"],
    ["ring", "Streak ring"], ["fire", "Fire"],
    ["currStreakNum", "Current streak number"], ["sideNums", "Side numbers"],
    ["currStreakLabel", "Current streak label"], ["sideLabels", "Side labels"],
    ["dates", "Dates"], ["stroke", "Dividers"],
    ["excludeDaysLabel", "Excluded days label"],
  ],
};

export const FALLBACK_THEMES = {
  stats: {
    default: { title_color: "2f80ed", text_color: "434d58", icon_color: "4c71f2", bg_color: "fffefe", border_color: "e4e2e2" },
    radical: { title_color: "fe428e", text_color: "a9fef7", icon_color: "f8d847", bg_color: "141321", border_color: "e4e2e2" },
  },
  streak: {
    default: { background: "#FFFEFE", border: "#E4E2E2", stroke: "#E4E2E2", ring: "#FB8C00", fire: "#FB8C00", currStreakNum: "#151515", sideNums: "#151515", currStreakLabel: "#FB8C00", sideLabels: "#151515", dates: "#464646", excludeDaysLabel: "#464646" },
    radical: { background: "#141321", border: "#E4E2E2", stroke: "#E4E2E2", ring: "#FE428E", fire: "#FE428E", currStreakNum: "#F8D847", sideNums: "#FE428E", currStreakLabel: "#F8D847", sideLabels: "#FE428E", dates: "#A9FEF7", excludeDaysLabel: "#A9FEF7" },
  },
};

export function defaultDesign(type = "stats") {
  if (!CARD_TYPES.includes(type)) throw new TypeError("Unknown card type");
  return {
    type, username: "Theyka", repo: "Turnstile-Solver", theme: "radical",
    embed_width: "400", border_radius: "4.5", card_width: "",
    hide_border: false, disable_animations: true,
    show_icons: true, text_bold: true, hide_title: false, hide_rank: false,
    custom_title: "", rank_icon: "default", number_format: "short", hide: [],
    show_owner: true, description_lines_count: "",
    mode: "daily", short_numbers: false, timezone: "", starting_year: "", exclude_days: "",
    hide_total_contributions: false, hide_current_streak: false, hide_longest_streak: false,
    commits_year: "", include_all_commits: false, contribs_include_own_repos: false, exclude_repo: "",
    colors: {},
  };
}

export function normalizeColor(value) {
  return String(value ?? "").trim().replace(/^#/, "");
}

export function validateDesign(state) {
  const errors = [];
  const add = (field, message) => errors.push({ field, message });
  if (!CARD_TYPES.includes(state.type)) {
    add("type", "Choose a supported card type.");
    return errors;
  }
  const username = String(state.username).trim();
  if (!/^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$/.test(username) || username.includes("--")) {
    add("username", "Enter a GitHub username (1–39 letters, numbers, or single hyphens).");
  }
  if (state.type === "pin" && !/^[a-zA-Z0-9_.-]{1,100}$/.test(state.repo.trim())) {
    add("repo", "Enter a repository name, without the owner or a URL.");
  }
  if (!state.theme || !/^[a-zA-Z0-9_-]+$/.test(state.theme)) add("theme", "Choose a theme.");
  const inRange = (value, min, max, integer = false) =>
    String(value).trim() !== "" && Number.isFinite(Number(value)) &&
    Number(value) >= min && Number(value) <= max && (!integer || Number.isInteger(Number(value)));
  if (!inRange(state.embed_width, 200, 1200, true)) add("embed_width", "Embed width must be a whole number from 200 to 1200.");
  if (!inRange(state.border_radius, 0, 100)) add("border_radius", "Corner radius must be between 0 and 100.");
  if (state.card_width !== "") {
    let minimum = 300;
    if (state.type === "stats") minimum = state.hide_rank ? 304 : (state.show_icons ? 437 : 420);
    if (state.type === "streak") minimum = 100 * ["hide_total_contributions", "hide_current_streak", "hide_longest_streak"].filter((key) => !state[key]).length;
    if (!inRange(state.card_width, minimum, 1200, true)) add("card_width", `SVG width must be a whole number from ${minimum} to 1200 for this layout.`);
  }
  if (state.type === "stats") {
    if (state.hide_rank && STAT_NAMES.every((name) => state.hide.includes(name))) add("hide_rank", "Keep at least one stat or the rank circle visible.");
    if (!["default", "percentile", "github"].includes(state.rank_icon)) add("rank_icon", "Choose a rank display.");
    if (!["short", "long"].includes(state.number_format)) add("number_format", "Choose a number format.");
    if (state.commits_year && !/^\d{4}$/.test(state.commits_year)) add("commits_year", "Commits year must have four digits.");
  }
  if (state.type === "pin" && !["", "1", "2", "3"].includes(state.description_lines_count)) add("description_lines_count", "Choose 1, 2, or 3 description lines, or Automatic.");
  if (state.type === "streak") {
    if (["hide_total_contributions", "hide_current_streak", "hide_longest_streak"].every((key) => state[key])) add("hide_total_contributions", "Keep at least one streak column visible.");
    if (!["daily", "weekly"].includes(state.mode)) add("mode", "Choose daily or weekly mode.");
    if (state.starting_year && !inRange(state.starting_year, 2005, new Date().getFullYear(), true)) add("starting_year", "Starting year must be between 2005 and the current year.");
    if (state.timezone) {
      try { new Intl.DateTimeFormat("en", { timeZone: state.timezone }); }
      catch { add("timezone", "Enter a valid timezone, such as UTC or Europe/Paris."); }
    }
    if (state.mode === "daily" && state.exclude_days && state.exclude_days.split(",").some((day) => !/^(sun|mon|tue|wed|thu|fri|sat)$/i.test(day.trim().slice(0, 3)))) add("exclude_days", "Use comma-separated days, such as Sat,Sun.");
  }
  for (const [key, label] of COLOR_FIELDS[state.type]) {
    const value = normalizeColor(state.colors[key]);
    if (!value) continue;
    if (/^(?:[a-f\d]{3}|[a-f\d]{4}|[a-f\d]{6}|[a-f\d]{8})$/i.test(value)) continue;
    if (key === "background" || key === "bg_color") {
      const [angle, ...stops] = value.split(",");
      const validAngle = state.type === "streak" ? /^-?\d+$/.test(angle) : /^-?\d+(?:\.\d+)?$/.test(angle);
      if (validAngle && stops.length >= 2 && stops.every((stop) => /^(?:[a-f\d]{3}|[a-f\d]{4}|[a-f\d]{6}|[a-f\d]{8})$/i.test(stop))) continue;
    }
    add(`color.${key}`, `${label}: use a 3, 4, 6, or 8-digit hex color${key === "background" || key === "bg_color" ? ", or an angle followed by hex gradient stops" : ""}.`);
  }
  return errors;
}

export function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

export function buildOutputs(state, origin) {
  const errors = validateDesign(state);
  if (errors.length) throw new TypeError(errors[0].message);
  const base = new URL(origin);
  if (!["https:", "http:"].includes(base.protocol)) throw new TypeError("Use an HTTP or HTTPS origin");
  const path = { stats: "/api", pin: "/api/pin/", streak: "/streak/" }[state.type];
  const url = new URL(path, base.origin);
  const params = url.searchParams;
  params.set(state.type === "streak" ? "user" : "username", state.username.trim());
  const set = (key) => params.set(key, String(state[key]));
  const optional = (key) => { if (String(state[key]).trim() !== "") set(key); };
  for (const key of ["theme", "border_radius", "hide_border", "disable_animations"]) set(key);
  optional("card_width");
  if (state.type === "stats") {
    for (const key of ["show_icons", "text_bold", "hide_title", "hide_rank", "rank_icon", "number_format", "include_all_commits", "contribs_include_own_repos"]) set(key);
    for (const key of ["custom_title", "commits_year", "exclude_repo"]) optional(key);
    const hidden = STAT_NAMES.filter((name) => state.hide.includes(name));
    if (hidden.length) params.set("hide", hidden.join(","));
  } else if (state.type === "pin") {
    params.set("repo", state.repo.trim());
    set("show_owner");
    optional("description_lines_count");
  } else {
    for (const key of ["mode", "short_numbers", "hide_total_contributions", "hide_current_streak", "hide_longest_streak"]) set(key);
    for (const key of ["timezone", "starting_year"]) optional(key);
    if (state.mode === "daily") optional("exclude_days");
  }
  for (const [key] of COLOR_FIELDS[state.type]) {
    const value = normalizeColor(state.colors[key]);
    if (value) params.set(key, value);
  }
  const username = state.username.trim();
  const link = `https://github.com/${encodeURIComponent(username)}${state.type === "pin" ? "/" + encodeURIComponent(state.repo.trim()) : ""}`;
  const alt = state.type === "pin" ? `${username}/${state.repo.trim()}` : state.type === "streak" ? `${username}'s contribution streak` : (state.custom_title || `${username}'s GitHub stats`);
  const width = String(Number(state.embed_width));
  const html = `<p align="center">\n  <a href="${escapeHTML(link)}">\n    <img width="${width}" src="${escapeHTML(url.href)}" alt="${escapeHTML(alt)}" />\n  </a>\n</p>`;
  const markdownAlt = alt.replace(/[\[\]\\]/g, "\\$&");
  return { url: url.href, html, markdown: `[![${markdownAlt}](${url.href})](${link})`, alt, link, width };
}

// A fragment (not a root query string) preserves old /?username= image URLs.
// Restore only known fields; arbitrary shared-link keys never become API params.
export function serializeDesign(state) {
  const params = new URLSearchParams({ card: state.type });
  const defaults = defaultDesign(state.type);
  for (const key of Object.keys(defaults)) {
    if (["type", "colors", "hide"].includes(key)) continue;
    if (state[key] !== defaults[key]) params.set(key, String(state[key]));
  }
  if (state.hide.length) params.set("hide", state.hide.join(","));
  for (const [key] of COLOR_FIELDS[state.type]) {
    const color = normalizeColor(state.colors[key]);
    if (color) params.set(`color.${key}`, color);
  }
  return params.toString();
}

export function restoreDesign(fragment) {
  const params = new URLSearchParams(fragment.replace(/^#/, ""));
  const type = CARD_TYPES.includes(params.get("card")) ? params.get("card") : "stats";
  const state = defaultDesign(type);
  for (const key of Object.keys(state)) {
    if (["type", "colors", "hide"].includes(key) || !params.has(key)) continue;
    const value = params.get(key).slice(0, 500);
    state[key] = typeof state[key] === "boolean" ? value === "true" : value;
  }
  state.hide = (params.get("hide") || "").split(",").filter((name) => STAT_NAMES.includes(name));
  for (const [key] of COLOR_FIELDS[type]) {
    if (params.has(`color.${key}`)) state.colors[key] = normalizeColor(params.get(`color.${key}`).slice(0, 500));
  }
  return state;
}

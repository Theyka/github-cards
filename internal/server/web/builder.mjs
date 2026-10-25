import {
  CARD_TYPES, STAT_NAMES, COLOR_FIELDS, FALLBACK_THEMES,
  defaultDesign, normalizeColor, validateDesign, buildOutputs, serializeDesign, restoreDesign,
} from "./builder-core.mjs";

const $ = (id) => document.getElementById(id);
const form = $("builder-form");
form.noValidate = true;
let catalog = FALLBACK_THEMES;
const designs = Object.fromEntries(CARD_TYPES.map((type) => [type, defaultDesign(type)]));
let currentType = "stats";
let format = "url";
let outputs = null;
let previewTimer;
let previewController;
let previewSequence = 0;
let objectURL;
let lastPreviewURL;
let lastPreviewError = "";
let lastPreviewSize = "";

function themesFor(type) {
  return catalog[type === "streak" ? "streak" : "stats"];
}

function readDesign() {
  const state = { ...designs[currentType], colors: {} };
  for (const element of form.elements) {
    if (!element.name || element.matches(":disabled") || ["type", "visible_stat"].includes(element.name)) continue;
    if (element.dataset.colorKey) {
      const color = normalizeColor(element.value);
      if (color) state.colors[element.dataset.colorKey] = color;
    } else {
      state[element.name] = element.type === "checkbox" ? element.checked : element.value;
    }
  }
  if (currentType === "stats") {
    const visible = new FormData(form).getAll("visible_stat");
    state.hide = STAT_NAMES.filter((name) => !visible.includes(name));
  }
  return state;
}

function paletteFor(state) {
  const themes = themesFor(state.type);
  const theme = { ...themes.default, ...themes[state.theme] };
  if (state.type !== "streak") theme.ring_color ||= theme.title_color;
  return theme;
}

function hexSwatch(value) {
  const first = normalizeColor(value || "").split(",");
  let hex = first.length > 1 ? first[1] : first[0];
  if (/^[a-f\d]{3,4}$/i.test(hex)) hex = hex.slice(0, 3).split("").map((char) => char + char).join("");
  if (/^[a-f\d]{6,8}$/i.test(hex)) return "#" + hex.slice(0, 6);
  return "#ffffff";
}

function renderThemes(state) {
  const themes = themesFor(state.type);
  // Preserve a shared preset while the complete catalog is still loading.
  const names = [...new Set([...Object.keys(themes), state.theme])].sort();
  $("theme").replaceChildren(...names.map((name) => new Option(name, name, false, name === state.theme)));
  $("theme-count").textContent = `${Object.keys(themes).length} palettes`;
  renderShortcuts(state);
}

function renderShortcuts(state) {
  const themes = themesFor(state.type);
  const names = ["radical", "tokyonight", "github_dark", "github-dark", "transparent"].filter((name) => Object.hasOwn(themes, name));
  $("theme-shortcuts").replaceChildren(...names.map((name) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "theme-shortcut";
    button.setAttribute("aria-pressed", String(state.theme === name));
    const swatch = document.createElement("span");
    swatch.className = "theme-swatch";
    swatch.setAttribute("aria-hidden", "true");
    swatch.style.setProperty("--swatch", hexSwatch(themes[name].bg_color || themes[name].background));
    button.append(swatch, document.createTextNode(name.replaceAll("_", " ").replaceAll("-", " ")));
    button.addEventListener("click", () => {
      $("theme").value = name;
      $("theme").dispatchEvent(new Event("input", { bubbles: true }));
    });
    return button;
  }));
}

function renderColors(state) {
  const palette = paletteFor(state);
  $("color-fields").replaceChildren(...COLOR_FIELDS[state.type].map(([key, title]) => {
    const row = document.createElement("div");
    row.className = "color-field";
    const label = document.createElement("label");
    label.htmlFor = `color-${key}`;
    label.textContent = title;
    const controls = document.createElement("div");
    controls.className = "color-input";
    const swatch = document.createElement("input");
    swatch.type = "color";
    swatch.value = hexSwatch(state.colors[key] || palette[key]);
    swatch.dataset.swatch = key;
    swatch.setAttribute("aria-label", `${title} color picker`);
    const text = document.createElement("input");
    text.id = `color-${key}`;
    text.type = "text";
    text.name = `color.${key}`;
    text.dataset.colorKey = key;
    text.value = state.colors[key] || "";
    text.placeholder = normalizeColor(palette[key] || "Theme default");
    text.maxLength = 150;
    text.autocomplete = "off";
    text.spellcheck = false;
    controls.append(swatch, text);
    row.append(label, controls);
    return row;
  }));
}

function writeDesign(state) {
  for (const fieldset of form.querySelectorAll("fieldset[data-for]")) {
    const active = fieldset.dataset.for === state.type;
    fieldset.hidden = !active;
    fieldset.disabled = !active;
  }
  $("repo-field").hidden = state.type !== "pin";
  $("repo").disabled = state.type !== "pin";
  $("repo").required = state.type === "pin";
  $("username-label").textContent = state.type === "pin" ? "Owner / organization" : "GitHub username";
  renderThemes(state);
  for (const element of form.elements) {
    if (element.name === "type") element.checked = element.value === state.type;
    else if (element.name === "visible_stat") element.checked = !state.hide.includes(element.value);
    else if (Object.hasOwn(state, element.name)) {
      if (element.type === "checkbox") element.checked = state[element.name];
      else element.value = state[element.name];
    }
  }
  $("card-width").placeholder = { stats: "467", pin: "400", streak: "495" }[state.type];
  renderColors(state);
}

function cancelPreview() {
  clearTimeout(previewTimer);
  previewSequence++;
  previewController?.abort();
  $("preview-stage").setAttribute("aria-busy", "false");
}

function setPreviewStatus(message, error = false) {
  $("preview-status").textContent = message;
  $("preview-status").dataset.status = error ? "error" : "ready";
}

function syncOutputs(updateLocation = true) {
  const state = designs[currentType];
  const errors = validateDesign(state);
  for (const element of form.querySelectorAll('[aria-invalid="true"]')) element.removeAttribute("aria-invalid");
  for (const error of errors) {
    const element = Array.from(form.elements).find((input) => input.name === error.field);
    element?.setAttribute("aria-invalid", "true");
  }
  $("validation-message").hidden = !errors.length;
  $("validation-message").textContent = errors[0]?.message || "";
  outputs = errors.length ? null : buildOutputs(state, window.location.origin);
  $("copy-output").disabled = !outputs;
  $("share-design").disabled = !outputs;
  $("open-card").hidden = !outputs;
  if (outputs) $("open-card").href = outputs.url;
  $("output-value").textContent = outputs ? outputs[format] : "Your embed will appear here once the settings are valid.";
  $("copy-status").textContent = "";
  if (updateLocation) {
    try { history.replaceState(null, "", "#" + serializeDesign(state)); }
    catch { /* Sandboxed previews may disallow changing browser history. */ }
  }
  return errors;
}

function schedulePreview() {
  if (!outputs) {
    setPreviewStatus("Check your settings to preview this card.");
    return;
  }
  if (outputs.url === lastPreviewURL) {
    setPreviewStatus(lastPreviewError || lastPreviewSize, Boolean(lastPreviewError));
    return;
  }
  setPreviewStatus($("auto-preview").checked ? "Updating your design…" : "Settings changed. Select Update preview.");
  if ($("auto-preview").checked) previewTimer = setTimeout(renderPreview, 800);
}

async function renderPreview() {
  const errors = syncOutputs(false);
  if (errors.length) return;
  cancelPreview();
  const sequence = previewSequence;
  const requested = outputs;
  const controller = new AbortController();
  previewController = controller;
  const timeout = setTimeout(() => controller.abort(new Error("The preview timed out. Try again in a moment.")), 45000);
  $("preview-stage").setAttribute("aria-busy", "true");
  setPreviewStatus("Loading live GitHub data…");
  try {
    const response = await fetch(requested.url, { signal: controller.signal, headers: { Accept: "image/svg+xml" } });
    if (!response.ok) throw new Error(`The server returned HTTP ${response.status}. Please try again.`);
    if (!response.headers.get("content-type")?.includes("image/svg+xml")) throw new Error("The server did not return an SVG. Check the deployment and try again.");
    const svg = await response.text();
    if (sequence !== previewSequence) return;
    // Inspect a style-free copy. DOMParser associates SVG styles with the
    // current document's strict CSP even though this detached tree is never
    // inserted. The original SVG is displayed unchanged in an isolated <img>.
    const inspectableSVG = svg.replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, "").replace(/\sstyle=(?:"[^"]*"|'[^']*')/gi, "");
    const documentSVG = new DOMParser().parseFromString(inspectableSVG, "image/svg+xml");
    const root = documentSVG.documentElement;
    if (root.localName !== "svg" || documentSVG.querySelector("parsererror")) throw new Error("The server returned an invalid SVG. Please try again.");
    const errorText = documentSVG.querySelector('[data-testid="message"]');
    const streakError = documentSVG.querySelector("#cut-off-area");
    lastPreviewError = errorText ? errorText.textContent.trim().replace(/\s+/g, " ") : streakError ? "GitHub could not return this card. See the message in the preview." : "";
    const viewBox = root.getAttribute("viewBox")?.trim().split(/\s+/);
    const width = root.getAttribute("width")?.replace(/px$/, "") || viewBox?.[2];
    const height = root.getAttribute("height")?.replace(/px$/, "") || viewBox?.[3];
    lastPreviewSize = width && height ? `${width} × ${height} · SVG · ready to embed` : "SVG · ready to embed";
    const nextURL = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
    const previousURL = objectURL;
    objectURL = nextURL;
    $("card-preview").src = nextURL;
    $("card-preview").alt = requested.alt;
    $("card-preview").hidden = false;
    $("preview-placeholder").hidden = true;
    if (previousURL) URL.revokeObjectURL(previousURL);
    lastPreviewURL = requested.url;
    setPreviewStatus(lastPreviewError || lastPreviewSize, Boolean(lastPreviewError));
  } catch (error) {
    if (sequence !== previewSequence) return;
    if (controller.signal.aborted && controller.signal.reason?.name === "AbortError") return;
    setPreviewStatus(controller.signal.reason?.message || error.message || "Could not load the preview. Try Update preview again.", true);
  } finally {
    clearTimeout(timeout);
    if (sequence === previewSequence) $("preview-stage").setAttribute("aria-busy", "false");
  }
}

function edited({ resetColors = false } = {}) {
  cancelPreview();
  designs[currentType] = readDesign();
  if (resetColors) {
    designs[currentType].colors = {};
    renderColors(designs[currentType]);
    renderShortcuts(designs[currentType]);
  }
  syncOutputs();
  schedulePreview();
}

form.addEventListener("input", (event) => {
  const target = event.target;
  if (target.name === "type") return;
  if (target.dataset.swatch) $("color-" + target.dataset.swatch).value = target.value.slice(1);
  if (target.dataset.colorKey) {
    const picker = form.querySelector(`[data-swatch="${target.dataset.colorKey}"]`);
    picker.value = hexSwatch(target.value || paletteFor(designs[currentType])[target.dataset.colorKey]);
  }
  edited({ resetColors: target.id === "theme" });
});

form.addEventListener("change", (event) => {
  if (event.target.name !== "type") return;
  cancelPreview();
  const username = designs[currentType].username;
  designs[currentType] = readDesign();
  currentType = event.target.value;
  designs[currentType].username = username;
  writeDesign(designs[currentType]);
  syncOutputs();
  schedulePreview();
});

form.addEventListener("submit", (event) => {
  event.preventDefault();
  designs[currentType] = readDesign();
  const errors = syncOutputs();
  if (errors.length) {
    const field = Array.from(form.elements).find((element) => element.name === errors[0].field);
    const details = field?.closest("details");
    if (details) details.open = true;
    field?.focus();
    return;
  }
  renderPreview();
  if (matchMedia("(max-width: 760px)").matches) $("workspace").scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth", block: "start" });
});

$("auto-preview").addEventListener("change", () => {
  cancelPreview();
  schedulePreview();
});

$("reset-colors").addEventListener("click", () => {
  designs[currentType] = readDesign();
  designs[currentType].colors = {};
  renderColors(designs[currentType]);
  edited();
});

$("reset-design").addEventListener("click", () => {
  cancelPreview();
  const { username, repo } = designs[currentType];
  designs[currentType] = { ...defaultDesign(currentType), username, repo };
  writeDesign(designs[currentType]);
  syncOutputs();
  schedulePreview();
});

for (const button of document.querySelectorAll(".background-choice")) {
  button.addEventListener("click", () => {
    $("preview-stage").dataset.background = button.dataset.background;
    for (const choice of document.querySelectorAll(".background-choice")) choice.setAttribute("aria-pressed", String(choice === button));
  });
}

function chooseFormat(button) {
  format = button.dataset.format;
  for (const tab of document.querySelectorAll("[data-format]")) {
    tab.setAttribute("aria-selected", String(tab === button));
    tab.tabIndex = tab === button ? 0 : -1;
  }
  $("output-code").setAttribute("aria-labelledby", button.id);
  $("copy-output").textContent = { url: "Copy image URL ⧉", html: "Copy HTML ⧉", markdown: "Copy Markdown ⧉" }[format];
  $("output-value").textContent = outputs ? outputs[format] : "Your embed will appear here once the settings are valid.";
  $("copy-status").textContent = "";
}
const tabs = Array.from(document.querySelectorAll("[data-format]"));
for (const button of tabs) {
  button.addEventListener("click", () => chooseFormat(button));
  button.addEventListener("keydown", (event) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const index = tabs.indexOf(button);
    const next = event.key === "Home" ? tabs[0] : event.key === "End" ? tabs.at(-1) : tabs[(index + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length];
    chooseFormat(next);
    next.focus();
  });
}

async function copy(text, message) {
  try {
    if (!navigator.clipboard?.writeText) throw new Error("Clipboard API unavailable");
    await navigator.clipboard.writeText(text);
  } catch {
    // Plain HTTP self-hosted instances may not have the secure Clipboard API.
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.className = "clipboard-fallback";
    textarea.readOnly = true;
    document.body.append(textarea);
    textarea.select();
    textarea.setSelectionRange(0, text.length);
    let copied = false;
    try { copied = document.execCommand("copy"); }
    catch { /* Fall back to selecting the text for a manual copy. */ }
    textarea.remove();
    if (!copied) {
      $("output-value").textContent = text;
      const range = document.createRange();
      range.selectNodeContents($("output-value"));
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      $("copy-status").textContent = "Automatic copy is unavailable. The text is selected—copy it manually.";
      return;
    }
  }
  $("copy-status").textContent = message;
}

$("copy-output").addEventListener("click", () => {
  if (outputs) copy(outputs[format], `${{ url: "Image URL", html: "HTML embed", markdown: "Markdown embed" }[format]} copied. Make it part of your README.`);
});
$("share-design").addEventListener("click", () => {
  if (outputs) copy(`${location.origin}${location.pathname}#${serializeDesign(designs[currentType])}`, "Design link copied. Anyone can open it and keep customizing.");
});

function loadSharedDesign() {
  const state = restoreDesign(location.hash);
  currentType = state.type;
  designs[currentType] = state;
  writeDesign(state);
  syncOutputs(false);
}

window.addEventListener("hashchange", () => {
  cancelPreview();
  loadSharedDesign();
  schedulePreview();
});
window.addEventListener("pagehide", (event) => {
  cancelPreview();
  if (!event.persisted && objectURL) URL.revokeObjectURL(objectURL);
});

async function init() {
  loadSharedDesign();
  renderPreview();
  try {
    const response = await fetch("/builder/themes", { signal: AbortSignal.timeout(10000) });
    if (!response.ok) throw new Error("Theme catalog unavailable");
    const themes = await response.json();
    if (!themes.stats?.radical || !themes.streak?.radical) throw new Error("Invalid theme catalog");
    catalog = themes;
    writeDesign(designs[currentType]);
    syncOutputs(false);
  } catch {
    $("catalog-notice").textContent = "The full theme catalog could not load. Core themes are still available; reload to try again.";
    $("catalog-notice").hidden = false;
  }
}

init();

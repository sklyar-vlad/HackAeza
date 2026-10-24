// Sidebar, top bar and theme shared by both documentation pages.
import { ops, spec, tags } from "./contract.js";

export const view = document.body.dataset.view;
export const $ = (selector) => document.querySelector(selector);
export const esc = (value) =>
  String(value ?? "").replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);

// Outline icons from the my.aeza.net UI kit, same as frontend/src/shared/ui/Icon.tsx.
const ICONS = {
  home: "M5.78 10.22V18c0 1.1.9 2 2 2H12M5.78 10.22l5.51-5.51a1 1 0 0 1 1.42 0l4.79 4.79M5.78 10.22 4 12M18.22 10.22V18c0 1.1-.9 2-2 2H12M18.22 10.22 20 12M18.22 10.22l-.72-.72M17.5 9.5V6M12 20v-5",
  doc: "M19 8.63V18a3 3 0 0 1-3 3H8a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3h5.5L19 8.63zM13.5 3v4.63c0 .55.45 1 1 1H19M9 13h6M9 17h6",
  book: "M4 19.5A2.5 2.5 0 0 1 6.5 17H20M4 19.5A2.5 2.5 0 0 0 6.5 22H20V2H6.5A2.5 2.5 0 0 0 4 4.5v15z",
  chevron: "M6 9l6 6 6-6",
  copy: "M9 9h11v11H9zM5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1",
  check: "M20 6 9 17l-5-5",
  sun: "M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10zM12 1v2M12 21v2M4.2 4.2l1.4 1.4M18.4 18.4l1.4 1.4M1 12h2M21 12h2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4",
  moon: "M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z",
};
export const icon = (name, size = 20) =>
  `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="${ICONS[name]}"/></svg>`;

const base = view === "reference" ? "" : "./";
export const schemaHref = (name) =>
  `${base}#/schemas/${encodeURIComponent(name)}`;
export const opHref = (op) =>
  view === "swagger"
    ? `#/${encodeURIComponent(op.tags[0])}/${encodeURIComponent(op.operationId)}`
    : `#/op/${encodeURIComponent(op.operationId)}`;

// ---------- theme (shared with the panel through its persisted prefs) ----------
const PREFS = "aeza-risk-prefs";
function showTheme() {
  const dark = document.documentElement.dataset.theme === "dark";
  $("#theme span").textContent = dark ? "Светлая тема" : "Тёмная тема";
  $("#theme path").setAttribute("d", ICONS[dark ? "sun" : "moon"]);
}
showTheme();
$("#theme").addEventListener("click", () => {
  const theme =
    document.documentElement.dataset.theme === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = theme;
  showTheme();
  try {
    const prefs = JSON.parse(localStorage.getItem(PREFS)) ?? {
      state: {},
      version: 0,
    };
    prefs.state = { ...prefs.state, theme };
    localStorage.setItem(PREFS, JSON.stringify(prefs));
  } catch {}
});

// ---------- menu & keyboard ----------
const side = $("#side");
const setMenu = (open) => {
  side.classList.toggle("open", open);
  $("#menu").setAttribute("aria-expanded", String(open));
};
$("#menu").addEventListener("click", () =>
  setMenu(!side.classList.contains("open")),
);
side.addEventListener("click", (event) => {
  if (event.target.closest("a")) setMenu(false);
});
document.addEventListener("keydown", (event) => {
  const typing = /^(INPUT|TEXTAREA|SELECT)$/.test(
    document.activeElement?.tagName,
  );
  if (event.key === "/" && !typing && !event.metaKey && !event.ctrlKey) {
    event.preventDefault();
    setMenu(true);
    $("#search").focus();
  }
  if (event.key === "Escape" && document.activeElement === $("#search")) {
    $("#search").value = "";
    search();
  }
});
$(`[data-view-link="${view}"]`).setAttribute("aria-current", "page");
if (view === "swagger") {
  $("#mode-link").href = "./#/";
  $("#mode-link span").textContent = "API Reference";
  $("#mode-link path").setAttribute("d", ICONS.book);
}

// ---------- navigation ----------
export const expanded = new Set();
export function renderNav(kind, id) {
  const current = (k, i) =>
    kind === k && (i === undefined || id === i) ? ' aria-current="page"' : "";
  $("#nav").innerHTML = `
    <a class="item" href="${base}#/"${current("overview")}><span>Обзор</span>${icon("home")}</a>
    <a class="item" href="${base}#/schemas"${current("schemas")}><span>Схемы данных</span>${icon("doc")}</a>
    <div class="nav-label">Разделы API</div>
    ${tags
      .map((tag) => {
        const open = expanded.has(tag.name);
        return `<div>
          <button class="item nav-tag" type="button" data-tag="${esc(tag.name)}" aria-expanded="${open}"><span>${esc(tag.title)}</span><small>${tag.ops.length}</small>${icon("chevron", 14)}</button>
          <div class="nav-ops"${open ? "" : " hidden"}>${tag.ops
            .map(
              (op) =>
                `<a class="item nav-op" href="${opHref(op)}"${current("op", op.operationId)}><span class="verb ${op.method}">${op.method}</span><span>${esc(op.summary)}</span></a>`,
            )
            .join("")}</div>
        </div>`;
      })
      .join("")}`;
}
$("#nav").addEventListener("click", (event) => {
  const button = event.target.closest(".nav-tag");
  if (!button) return;
  const open = !expanded.delete(button.dataset.tag);
  if (open) expanded.add(button.dataset.tag);
  button.setAttribute("aria-expanded", String(open));
  button.nextElementSibling.hidden = !open;
});

// ---------- search ----------
function search() {
  const query = $("#search").value.trim().toLowerCase();
  $("#nav").hidden = !!query;
  $("#results").hidden = !query;
  if (!query || !spec) return;
  const found = [
    ...ops
      .filter((op) =>
        `${op.method} ${op.path} ${op.summary} ${op.operationId}`
          .toLowerCase()
          .includes(query),
      )
      .map(
        (op) =>
          `<a class="item" role="listitem" href="${opHref(op)}"><span class="verb ${op.method}">${op.method}</span><span>${esc(op.summary)}</span></a>`,
      ),
    ...Object.keys(spec.components.schemas)
      .filter((name) => name.toLowerCase().includes(query))
      .map(
        (name) =>
          `<a class="item" role="listitem" href="${schemaHref(name)}"><span class="verb">схема</span><span>${esc(name)}</span></a>`,
      ),
  ];
  $("#results").innerHTML =
    found.slice(0, 30).join("") || '<p class="empty">Ничего не найдено</p>';
}
$("#search").addEventListener("input", search);
$("#results").addEventListener("click", (event) => {
  if (!event.target.closest("a")) return;
  $("#search").value = "";
  search();
});

export function showLoadError(target) {
  target.innerHTML =
    '<p class="note">Не удалось загрузить контракт. Обновите страницу или скачайте OpenAPI YAML.</p>';
}

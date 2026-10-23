"use strict";
const toggle = document.getElementById("theme-toggle");
function setTheme(theme) {
  document.documentElement.dataset.theme = theme;
  document.body.classList.toggle("dark-mode", theme === "dark");
  document.body.classList.toggle("light-mode", theme === "light");
  toggle.textContent = theme === "dark" ? "Светлая тема" : "Тёмная тема";
  toggle.setAttribute("aria-pressed", String(theme === "dark"));
}
setTheme(document.documentElement.dataset.theme);
toggle.addEventListener("click", () => {
  const theme = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
  try { localStorage.setItem("hackaeza-theme", theme); } catch (_) {}
  setTheme(theme);
});
const header = document.querySelector(".brand-header");
const updateHeaderHeight = () => document.documentElement.style.setProperty("--header-height", `${header.offsetHeight}px`);
updateHeaderHeight();
new ResizeObserver(updateHeaderHeight).observe(header);
if (document.getElementById("reference")) {
  if (typeof Scalar === "undefined") {
    document.getElementById("reference-status").textContent = "Не удалось загрузить справочник. Обновите страницу с помощью Ctrl+Shift+R.";
  } else {
  Scalar.createApiReference("#reference", {
    url: "./openapi.yaml",
    baseServerURL: location.origin,
    theme: "none",
    layout: "modern",
    defaultOpenFirstTag: false,
    darkMode: document.documentElement.dataset.theme === "dark",
    hideDarkModeToggle: true,
    hideClientButton: true,
    withDefaultFonts: false,
    showDeveloperTools: "never",
    agent: { disabled: true },
    telemetry: false,
    persistAuth: false,
    hideTestRequestButton: true,
    hideModels: true,
    defaultHttpClient: { targetKey: "http", clientKey: "http1.1" },
    featuredClients: ["http/http1.1"],
    documentDownloadType: "none",
    onLoaded: () => {
      document.getElementById("reference-status")?.remove();
      setupSections();
    }
  });
  }
}
if (document.getElementById("swagger-ui")) SwaggerUIBundle({
  url: "./openapi.yaml",
  dom_id: "#swagger-ui",
  deepLinking: true,
  filter: true,
  defaultModelsExpandDepth: -1,
  persistAuthorization: false,
  validatorUrl: null,
  syntaxHighlight: false
});

function setupSections() {
  const container = document.getElementById("reference");
  const pager = document.getElementById("section-pager");
  if (pager.dataset.ready) return;
  const groups = [...container.querySelectorAll('[data-sidebar-id]')]
    .filter(node => /\/tag\/[^/]+$/.test(node.dataset.sidebarId))
    .map(node => ({
      slug: node.dataset.sidebarId.split("/").pop(),
      title: node.querySelector('a')?.textContent.trim() || ""
    }));
  if (!groups.length) return;
  pager.dataset.ready = "true";
  const previous = document.getElementById("section-previous");
  const next = document.getElementById("section-next");
  let current = location.hash.match(/^#tag\/([^/]+)/)?.[1] || groups[0].slug;
  let expandedSection;
  const openedButtons = new WeakSet();
  function update() {
    let index = groups.findIndex(group => group.slug === current);
    if (index < 0) { index = 0; current = groups[0].slug; }
    if (expandedSection !== current) {
      expandedSection = current;
      for (const node of container.querySelectorAll('[data-sidebar-id]')) {
        const slug = node.dataset.sidebarId.match(/\/tag\/([^/]+)$/)?.[1];
        const button = node.querySelector('button[aria-expanded="true"]');
        if (slug && slug !== current && button) button.click();
      }
    }
    for (const node of container.querySelectorAll('.references-container > div')) {
      const id = node.id || node.querySelector('section[id]')?.id || "";
      const slug = id.match(/\/tag\/([^/]+)/)?.[1];
      const hidden = slug !== current;
      if (node.hasAttribute("data-doc-hidden") !== hidden) node.toggleAttribute("data-doc-hidden", hidden);
      const more = !hidden && node.querySelector('.show-more');
      if (more && !openedButtons.has(more)) {
        openedButtons.add(more);
        more.click();
      }
    }
    previous.disabled = index === 0;
    next.disabled = index === groups.length - 1;
    previous.textContent = index > 0 ? `← ${groups[index - 1].title}` : "Начало";
    next.textContent = index < groups.length - 1 ? `${groups[index + 1].title} →` : "Конец";
    pager.hidden = false;
  }
  function go(offset) {
    const index = groups.findIndex(group => group.slug === current) + offset;
    if (!groups[index]) return;
    current = groups[index].slug;
    update();
    location.hash = `tag/${current}`;
    window.scrollTo({ top: 0, behavior: "instant" });
  }
  previous.addEventListener("click", () => go(-1));
  next.addEventListener("click", () => go(1));
  window.addEventListener("hashchange", () => {
    const slug = location.hash.match(/^#tag\/([^/]+)/)?.[1];
    if (slug) { current = slug; update(); }
  });
  container.addEventListener("click", event => {
    const link = event.target.closest('a[href^="#tag/"]');
    if (link) {
      current = link.getAttribute("href").split("/")[1];
      update();
    }
  });
  new MutationObserver(update).observe(container, { childList: true, subtree: true });
  update();
}

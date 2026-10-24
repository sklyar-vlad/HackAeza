"use strict";
(() => {
  const view = document.body.dataset.view;
  const root = document.documentElement;
  const media = matchMedia("(prefers-color-scheme: dark)");
  const key = "hackaeza-docs-theme";
  const labels = {
    Auth: ["Вход и сессии", "key"],
    Customers: ["Клиенты", "users"],
    Analytics: ["Аналитика", "chart"],
    Model: ["Модель риска", "sliders"],
    Source: ["Источник данных", "database"],
    Updates: ["Обновления", "refresh"],
    Operations: ["Эксплуатация", "activity"],
    Documentation: ["Документация", "book"],
  };
  let metadata;
  let reference;
  let swagger;
  let schemaCatalog;
  let schemaCatalogPromise;
  let schemaResourcesPromise;
  let schemaModule;
  let schemaDefinitions;
  let preference = "system";
  let sidebarPreviousFocus;
  let observer;
  let scheduled = false;
  const expanded = new WeakSet();
  const localizedContainers = new WeakSet();
  const $ = (selector) => document.querySelector(selector);
  const icon = (name) =>
    `<svg class="icon" aria-hidden="true"><use href="./assets/icons.svg#${name}"></use></svg>`;
  const create = (tag, className, text) => {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (text !== undefined) element.textContent = text;
    return element;
  };
  try {
    const saved =
      localStorage.getItem(key) ||
      localStorage.getItem("hackaeza-aeza-preview-theme");
    if (["light", "dark", "system"].includes(saved)) preference = saved;
  } catch (_) {}

  function applyTheme() {
    const dark =
      preference === "dark" || (preference === "system" && media.matches);
    root.dataset.theme = dark ? "dark" : "light";
    document.body.classList.toggle("dark-mode", dark);
    document.body.classList.toggle("light-mode", !dark);
    document.querySelectorAll('[name="docs-theme"]').forEach((input) => {
      input.checked = input.value === preference;
    });
    if (reference)
      reference.updateConfiguration({
        ...reference.getConfiguration(),
        darkMode: dark,
        forceDarkModeState: dark ? "dark" : "light",
      });
  }
  applyTheme();
  document.querySelectorAll('[name="docs-theme"]').forEach((input) =>
    input.addEventListener("change", () => {
      preference = input.value;
      try {
        localStorage.setItem(key, preference);
      } catch (_) {}
      applyTheme();
    }),
  );
  media.addEventListener("change", () => {
    if (preference === "system") applyTheme();
  });
  addEventListener("storage", (event) => {
    if (event.key === key) {
      preference = ["light", "dark", "system"].includes(event.newValue)
        ? event.newValue
        : "system";
      applyTheme();
    }
  });

  const navigation = $("#docs-navigation");
  const mobile = matchMedia("(max-width: 767px)");
  function setSidebar(open) {
    document.body.classList.toggle("sidebar-open", open);
    $("#sidebar-overlay").hidden = !open;
    $("#open-menu").setAttribute("aria-expanded", String(open));
    navigation.inert = mobile.matches && !open;
    if (mobile.matches && open) {
      sidebarPreviousFocus = document.activeElement;
      navigation.setAttribute("role", "dialog");
      navigation.setAttribute("aria-modal", "true");
      $("#close-menu").focus();
    } else {
      navigation.removeAttribute("role");
      navigation.removeAttribute("aria-modal");
      if (sidebarPreviousFocus) {
        sidebarPreviousFocus.focus();
        sidebarPreviousFocus = null;
      }
    }
  }
  $("#open-menu").addEventListener("click", () => setSidebar(true));
  $("#close-menu").addEventListener("click", () => setSidebar(false));
  $("#sidebar-overlay").addEventListener("click", () => setSidebar(false));
  mobile.addEventListener("change", () => setSidebar(false));
  setSidebar(false);
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") setSidebar(false);
    if (
      event.key === "/" &&
      !event.ctrlKey &&
      !event.metaKey &&
      !["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement?.tagName)
    ) {
      event.preventDefault();
      if (mobile.matches) setSidebar(true);
      $("#docs-search").focus();
    }
    if (
      event.key === "Tab" &&
      mobile.matches &&
      document.body.classList.contains("sidebar-open")
    ) {
      const targets = [
        ...navigation.querySelectorAll("a[href],button,input"),
      ].filter(
        (el) => !el.disabled && !el.hidden && el.getBoundingClientRect().width,
      );
      const first = targets[0],
        last = targets.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      }
      if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
  });
  document.querySelectorAll("[data-view-link]").forEach((link) => {
    if (link.dataset.viewLink === view)
      link.setAttribute("aria-current", "page");
  });
  if (view === "swagger") {
    $("#interactive-link").href = "./";
    $("#interactive-link span").textContent = "API Reference";
    $("#interactive-link use").setAttribute("href", "./assets/icons.svg#book");
  }

  function tagHref(tag) {
    return view === "swagger"
      ? `#/${encodeURIComponent(tag.name)}`
      : `#tag/${tag.slug}`;
  }
  function operationHref(operation) {
    const tag = metadata.tags.find((tag) => operation.tags.includes(tag.name));
    if (!tag) return view === "swagger" ? "#overview" : "#overview";
    return view === "swagger"
      ? `#/${encodeURIComponent(tag.name)}/${encodeURIComponent(operation.operationId)}`
      : `#tag/${tag.slug}/${operation.method}${operation.path}`;
  }
  function currentTag() {
    const hash = location.hash;
    if (view === "reference")
      return metadata.tags.find((tag) => hash.startsWith(`#tag/${tag.slug}`));
    let name;
    try {
      name = decodeURIComponent(hash.split("/")[1] ?? "");
    } catch (_) {
      return undefined;
    }
    return metadata.tags.find((tag) => tag.name === name);
  }
  function addNavItem(href, label, iconName, id, count) {
    const link = create("a", "nav-item");
    link.href = href;
    link.dataset.section = id;
    const caption = create("span", "nav-caption");
    caption.append(create("span", "", label));
    if (count !== undefined)
      caption.append(create("small", "nav-count", String(count)));
    link.append(caption);
    link.insertAdjacentHTML("beforeend", icon(iconName));
    $("#section-navigation").append(link);
  }
  function renderNavigation() {
    const nav = $("#section-navigation");
    nav.replaceChildren();
    addNavItem("#overview", "Обзор API", "home", "overview");
    metadata.tags.forEach((tag) =>
      addNavItem(
        tagHref(tag),
        labels[tag.name]?.[0] ?? tag.name,
        labels[tag.name]?.[1] ?? "layers",
        tag.slug,
        tag.count,
      ),
    );
    addNavItem(
      "#schemas",
      "Схемы данных",
      "layers",
      "schemas",
      metadata.schemaCount,
    );
    nav.addEventListener("click", (event) => {
      if (event.target.closest("a")) {
        if (mobile.matches) setSidebar(false);
        window.scrollTo({ top: 0, behavior: "instant" });
      }
    });
  }
  function renderOverview() {
    const container = $("#docs-overview");
    if (!container) return;
    const required = metadata.operations.filter(
      (operation) => operation.required,
    ).length;
    const metrics = create("div", "docs-stats");
    for (const [label, value, note, iconName] of [
      [
        "Версия контракта",
        `v${metadata.info.version}`,
        `OpenAPI ${metadata.openapi}`,
        "layers",
      ],
      [
        "HTTP-операции",
        metadata.operations.length,
        `${required} обязательных по кейсу`,
        "code",
      ],
      ["Разделы API", metadata.tags.length, "От входа до эксплуатации", "book"],
      [
        "Схемы данных",
        metadata.schemaCount,
        "Входные и выходные структуры",
        "database",
      ],
    ]) {
      const card = create("section", "metric-card");
      const glyph = create("span", "metric-icon");
      glyph.innerHTML = icon(iconName);
      const body = create("div");
      body.append(
        create("p", "metric-label", label),
        create("strong", "metric-value", String(value)),
        create("p", "metric-note", note),
      );
      card.append(glyph, body);
      metrics.append(card);
    }
    container.append(metrics);
    const row = create("div", "overview-grid");
    const intro = create("section", "docs-card");
    intro.append(
      create("h2", "", "Как работать с API"),
      create(
        "p",
        "section-description",
        "Три шага от спецификации к первому запросу.",
      ),
    );
    const steps = [
      [
        "01",
        "Изучите контракт",
        "Выберите раздел слева. Для каждой операции описаны параметры, ответы и ошибки.",
      ],
      [
        "02",
        "Получите сессию",
        "Вход по логину и паролю возвращает Bearer-токен. Дополнительный OIDC описан отдельно.",
      ],
      [
        "03",
        "Проверьте запрос",
        "Откройте Swagger UI, нажмите «Авторизоваться» и откройте параметры запроса для нужной операции.",
      ],
    ];
    for (const [number, title, text] of steps) {
      const step = create("div", "guide-step");
      step.append(create("span", "step-number", number));
      const content = create("div");
      content.append(create("h3", "", title), create("p", "", text));
      step.append(content);
      intro.append(step);
    }
    const action = create("a", "button primary", "Открыть Swagger UI");
    action.href = "./swagger.html";
    action.insertAdjacentHTML("beforeend", icon("arrow-right"));
    intro.append(action);
    const principles = create("section", "docs-card");
    principles.append(
      create("h2", "", "Соглашения API"),
      create("p", "section-description", "Единые правила для всех разделов."),
    );
    for (const [name, value] of [
      ["Формат", "JSON · camelCase"],
      ["Денежные значения", "Целые евроценты"],
      ["Даты источника", "UTC · миллисекунды"],
      ["Авторизация", "Bearer-сессия"],
      ["Без оценки", "score = null · level = unknown"],
    ]) {
      const pair = create("div", "convention-row");
      pair.append(create("span", "", name), create("strong", "", value));
      principles.append(pair);
    }
    const notice = create("div", "docs-notice");
    notice.innerHTML = icon("info");
    notice.append(
      create(
        "p",
        "",
        "Risk-score — объяснимый индекс, не калиброванная вероятность. Денежная оценка является сценарной.",
      ),
    );
    principles.append(notice);
    row.append(intro, principles);
    container.append(row);
    const sections = create("section", "docs-card");
    sections.append(
      create("h2", "", "Разделы документации"),
      create(
        "p",
        "section-description",
        "Операции сгруппированы по бизнес-функциям.",
      ),
    );
    const grid = create("div", "module-grid");
    metadata.tags.forEach((tag) => {
      const link = create("a", "module-card");
      link.href = tagHref(tag);
      const glyph = create("span", "module-icon");
      glyph.innerHTML = icon(labels[tag.name]?.[1] ?? "layers");
      link.append(glyph);
      const content = create("div");
      content.append(
        create("h3", "", labels[tag.name]?.[0] ?? tag.name),
        create("p", "", tag.description ?? ""),
        create("small", "", `${tag.count} HTTP-операций`),
      );
      link.append(content);
      link.insertAdjacentHTML("beforeend", icon("arrow-right"));
      grid.append(link);
    });
    sections.append(grid);
    container.append(sections);
    const description = create("details", "contract-details");
    description.append(
      create("summary", "", "Полное описание контракта"),
      create("p", "", metadata.info.description),
    );
    container.append(description);
  }
  function search() {
    const query = $("#docs-search").value.trim().toLowerCase();
    let visible = 0;
    document.querySelectorAll("#section-navigation a").forEach((link) => {
      const tag = metadata.tags.find(
        (tag) => tag.slug === link.dataset.section,
      );
      const matches =
        !query ||
        link.textContent.toLowerCase().includes(query) ||
        (tag &&
          metadata.operations.some(
            (operation) =>
              operation.tags.includes(tag.name) &&
              `${operation.method} ${operation.path} ${operation.summary}`
                .toLowerCase()
                .includes(query),
          ));
      link.hidden = !matches;
      if (matches) visible++;
    });
    const results = $("#search-results");
    results.replaceChildren();
    const matched = query
      ? metadata.operations
          .filter((operation) =>
            `${operation.method} ${operation.path} ${operation.summary}`
              .toLowerCase()
              .includes(query),
          )
          .slice(0, 12)
      : [];
    for (const operation of matched) {
      const link = create("a", "search-result");
      link.href = operationHref(operation);
      link.append(
        create("span", "http-method", operation.method),
        create("code", "", operation.path),
        create("small", "", operation.summary),
      );
      link.addEventListener("click", () => {
        if (mobile.matches) setSidebar(false);
      });
      results.append(link);
    }
    results.hidden = !matched.length;
    $("#search-empty").hidden = !query || visible > 0 || matched.length > 0;
  }
  $("#docs-search").addEventListener("input", () => {
    if (metadata) search();
  });

  function filterReference(tag) {
    const container = $("#reference");
    if (!container) return;
    for (const section of container.querySelectorAll("section[id]")) {
      if (/\/tag\/[^/]+$/.test(section.id) && !section.querySelector("h3"))
        section.setAttribute("data-tag-introduction", "");
    }
    for (const node of container.querySelectorAll(
      ".references-container > div",
    )) {
      const id = node.id || node.querySelector("section[id]")?.id || "";
      const match = id.match(/\/tag\/([^/]+)/);
      const hidden = !match || match[1] !== tag?.slug;
      node.toggleAttribute("data-doc-hidden", hidden);
      if (!hidden) {
        for (const button of node.querySelectorAll(".show-more")) {
          if (!expanded.has(button)) {
            expanded.add(button);
            button.click();
          }
        }
      }
    }
  }
  function renderRoute() {
    if (!metadata) return;
    const tag = currentTag();
    const schema =
      location.hash === "#schemas" || location.hash.startsWith("#schemas/");
    $("#schema-catalog").hidden = !schema;
    if (schema) showSchemaCatalog();
    if (view === "swagger") {
      $("#swagger-ui").hidden = schema;
      $(".swagger-page .docs-notice").hidden = schema;
    }
    const selected = tag?.slug ?? (schema ? "schemas" : "overview");
    document.querySelectorAll("#section-navigation a").forEach((link) => {
      if (link.dataset.section === selected)
        link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    const title = schema
      ? "Схемы данных"
      : view === "swagger"
        ? "Swagger UI"
        : tag
          ? (labels[tag.name]?.[0] ?? tag.name)
          : "Документация API";
    $("#page-title").textContent = title;
    $("#current-section").textContent = schema
      ? "Схемы данных"
      : view === "swagger"
        ? "Swagger UI"
        : tag
          ? (labels[tag.name]?.[0] ?? tag.name)
          : "Обзор API";
    $("#page-description").textContent = schema
      ? "Поля, типы и ограничения. Связанные структуры открываются по ссылке без вложенных деревьев."
      : view === "swagger"
        ? "Интерактивная спецификация: авторизация, параметры и выполнение HTTP-запросов."
        : tag
          ? tag.description
          : "Всё о данных, расчёте рисков и операциях платформы HackAeza.";
    document.title = `${title} — HackAeza`;
    if (view === "reference") {
      $("#docs-overview").hidden = !!tag || schema;
      $("#reference-content").hidden = !tag || schema;
      filterReference(tag);
      const pager = $("#section-pager");
      pager.hidden = !tag;
      if (tag) {
        const index = metadata.tags.indexOf(tag);
        $("#section-previous").disabled = index === 0;
        $("#section-next").disabled = index === metadata.tags.length - 1;
        $("#section-previous span").textContent = index
          ? (labels[metadata.tags[index - 1].name]?.[0] ??
            metadata.tags[index - 1].name)
          : "Назад";
        $("#section-next span").textContent =
          index < metadata.tags.length - 1
            ? (labels[metadata.tags[index + 1].name]?.[0] ??
              metadata.tags[index + 1].name)
            : "Далее";
      }
    }
  }
  function loadSchemaResources() {
    if (!schemaResourcesPromise)
      schemaResourcesPromise = Promise.all([
        import("./schema-catalog.js"),
        fetch("./schemas.json").then((response) => {
          if (!response.ok) throw new Error("schemas unavailable");
          return response.json();
        }),
      ])
        .then(([module, schemas]) => {
          schemaModule = module;
          schemaDefinitions = schemas;
        })
        .catch((error) => {
          schemaResourcesPromise = undefined;
          throw error;
        });
    return schemaResourcesPromise;
  }
  async function showSchemaCatalog() {
    if (!schemaCatalogPromise) {
      const container = $("#schema-catalog");
      container.textContent = "Загружаем схемы…";
      schemaCatalogPromise = loadSchemaResources()
        .then(() => {
          schemaCatalog = schemaModule.mountCatalog(
            container,
            schemaDefinitions,
          );
        })
        .catch(() => {
          schemaCatalogPromise = undefined;
          container.textContent =
            "Не удалось загрузить схемы. Перейдите в раздел ещё раз или скачайте OpenAPI YAML.";
        });
    }
    await schemaCatalogPromise;
    schemaCatalog?.navigate();
  }
  // Keep a single disclosure per operation. Tag headings are plain navigation,
  // rather than another accordion containing the operation accordion.
  function flatOperationTags() {
    return {
      statePlugins: {
        layout: {
          wrapActions: {
            parseDeepLinkHash: (original) => (hash) => {
              if (/^#\/?schemas(?:\/|$)/.test(hash)) return;
              return original(hash);
            },
          },
        },
      },
      wrapComponents: {
        OperationTag: (_Original, system) => (props) => {
          const h = system.React.createElement;
          const tag = metadata.tags.find((tag) => tag.name === props.tag);
          return h(
            "section",
            { className: "opblock-tag-section is-open" },
            h(
              "h2",
              { className: "opblock-tag", id: `operations-tag-${props.tag}` },
              h(
                "a",
                { href: `#/${encodeURIComponent(props.tag)}` },
                labels[props.tag]?.[0] ?? props.tag,
              ),
              h("small", null, tag?.description ?? ""),
            ),
            props.children,
          );
        },
        ModelWrapper: (_Original, system) => (props) => {
          const h = system.React.createElement;
          const source = props.schema?.toJS?.() ?? props.schema ?? {};
          const name = schemaModule.refName(source.$ref ?? source.$$ref);
          const schema = schemaModule.mergeSchema(
            name && schemaDefinitions[name] ? schemaDefinitions[name] : source,
            schemaDefinitions,
          );
          function type(value) {
            const target = schemaModule.refName(value.$ref ?? value.$$ref);
            if (target && schemaDefinitions[target])
              return h(
                "a",
                {
                  href: schemaModule.schemaHref(target),
                  className: "schema-type-link",
                },
                target,
              );
            if (value.items)
              return h(
                system.React.Fragment,
                null,
                "array<",
                type(value.items),
                ">",
              );
            return value.type ?? (value.properties ? "object" : "—");
          }
          const fields = schemaModule.schemaFields(schema, schemaDefinitions);
          return h(
            "div",
            { className: "operation-flat-schema" },
            name &&
              h(
                "p",
                null,
                h(
                  "a",
                  {
                    href: schemaModule.schemaHref(name),
                    className: "schema-type-link",
                  },
                  `Открыть ${name} в справочнике`,
                ),
              ),
            fields.length
              ? h(
                  "div",
                  { className: "schema-table-scroll" },
                  h(
                    "table",
                    { className: "schema-table" },
                    h(
                      "caption",
                      { className: "sr-only" },
                      "Поля схемы операции",
                    ),
                    h(
                      "thead",
                      null,
                      h(
                        "tr",
                        null,
                        ...["Поле", "Тип", "Описание и ограничения"].map(
                          (title) =>
                            h("th", { key: title, scope: "col" }, title),
                        ),
                      ),
                    ),
                    h(
                      "tbody",
                      null,
                      ...fields.map(({ path, field, required }) =>
                        h(
                          "tr",
                          { key: path },
                          h(
                            "th",
                            { scope: "row" },
                            h("code", null, path),
                            required && h("small", null, "Обязательно"),
                          ),
                          h(
                            "td",
                            { className: "schema-field-type" },
                            type(field),
                          ),
                          h(
                            "td",
                            { className: "schema-field-description" },
                            h("p", null, field.description ?? ""),
                            h(
                              "ul",
                              { className: "schema-constraints" },
                              ...schemaModule
                                .constraints(field)
                                .map((note, index) =>
                                  h("li", { key: index }, note),
                                ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                )
              : h("p", null, type(source)),
            !fields.length &&
              h(
                "ul",
                { className: "schema-constraints" },
                ...schemaModule
                  .constraints(schema)
                  .map((note, index) => h("li", { key: index }, note)),
              ),
          );
        },
      },
    };
  }
  addEventListener("hashchange", renderRoute);
  $("#section-previous")?.addEventListener("click", () => {
    const index = metadata.tags.indexOf(currentTag());
    if (index > 0) location.hash = tagHref(metadata.tags[index - 1]);
    window.scrollTo({ top: 0, behavior: "instant" });
  });
  $("#section-next")?.addEventListener("click", () => {
    const index = metadata.tags.indexOf(currentTag());
    if (index < metadata.tags.length - 1)
      location.hash = tagHref(metadata.tags[index + 1]);
    window.scrollTo({ top: 0, behavior: "instant" });
  });

  function watchReference() {
    if (observer) return;
    observer = new MutationObserver(() => {
      if (!scheduled) {
        scheduled = true;
        requestAnimationFrame(() => {
          scheduled = false;
          filterReference(currentTag());
        });
      }
    });
    observer.observe($("#reference"), { childList: true, subtree: true });
  }
  function localizeInterface(container) {
    if (localizedContainers.has(container)) return;
    localizedContainers.add(container);
    const translations = {
      "Available authorizations": "Авторизация API",
      Authorize: "Авторизоваться",
      Logout: "Выйти",
      "Edit Value": "Изменить",
      Close: "Закрыть",
      "Try it out": "Параметры запроса",
      Execute: "Выполнить запрос",
      Cancel: "Отменить",
      Clear: "Очистить",
      Reset: "Сбросить",
      Parameters: "Параметры",
      "Query Parameters": "Параметры запроса",
      "Path Parameters": "Параметры пути",
      "Request Body": "Тело запроса",
      "Request body": "Тело запроса",
      Responses: "Ответы",
      "Response body": "Тело ответа",
      "Response headers": "Заголовки ответа",
      "Request URL": "URL запроса",
      "Server response": "Ответ сервера",
      "Example Value": "Пример",
      Schema: "Схема",
      Models: "Схемы данных",
      Schemas: "Схемы данных",
      Servers: "Сервер API",
      Name: "Параметр",
      Description: "Описание",
      "Value:": "Значение:",
      Code: "Код",
      Details: "Подробности",
      Links: "Связи",
      "No links": "Нет связей",
      "Media type": "Тип содержимого",
      "No parameters": "Нет параметров",
      "Examples:": "Примеры:",
      Examples: "Примеры",
      "Auth Required": "Нужна авторизация",
      "Copy as Markdown": "Копировать Markdown",
      Body: "Тело",
      Headers: "Заголовки",
      Cookies: "Cookies",
      "Show More": "Показать ещё",
      Copy: "Копировать",
      "Copy path to clipboard": "Скопировать путь",
      "Copied!": "Скопировано",
      Copied: "Скопировано",
      Download: "Скачать",
      "Expand all": "Развернуть всё",
      "Collapse all": "Свернуть всё",
      "Filter by tag or path": "Фильтр по разделу или пути",
    };
    let pending = false;
    const watcher = new MutationObserver(() => {
      if (!pending) {
        pending = true;
        requestAnimationFrame(translate);
      }
    });
    function translate() {
      pending = false;
      watcher.disconnect();
      const candidates = container.querySelectorAll(
        "button,label,h4,thead th,.modal-ux-header h3,.opblock-section-header h4,.responses-inner h4,.responses-inner h5,.servers-title,.section-header-label,.property-heading,.response-card-header,.request-card-header,.opblock-description-wrapper p,.examples-select__section-label",
      );
      for (const candidate of candidates) {
        const walk = document.createTreeWalker(candidate, NodeFilter.SHOW_TEXT);
        while (walk.nextNode()) {
          const node = walk.currentNode;
          if (
            node.parentElement.closest(
              "pre,code,textarea,.microlight,.scalar-code-block",
            )
          )
            continue;
          const text = node.nodeValue.trim();
          if (translations[text])
            node.nodeValue = node.nodeValue.replace(text, translations[text]);
        }
      }
      for (const element of container.querySelectorAll(
        "[placeholder],[aria-label],[title]",
      )) {
        for (const attribute of ["placeholder", "aria-label", "title"]) {
          const text = element.getAttribute(attribute);
          if (translations[text])
            element.setAttribute(attribute, translations[text]);
        }
      }
      for (const heading of container.querySelectorAll(".opblock-tag > a")) {
        const text = heading.textContent.trim();
        if (labels[text]) heading.textContent = labels[text][0];
      }
      watcher.observe(container, {
        childList: true,
        subtree: true,
        characterData: true,
      });
    }
    translate();
  }
  async function bootstrap() {
    try {
      const response = await fetch("./metadata.json");
      if (!response.ok) throw new Error("metadata unavailable");
      metadata = await response.json();
      $("#toolbar-version").textContent = `v${metadata.info.version}`;
      $("#footer-version").textContent =
        `Контракт v${metadata.info.version} · OpenAPI ${metadata.openapi}`;
      renderNavigation();
      renderOverview();
      renderRoute();
      if (view === "reference") {
        if (typeof Scalar === "undefined")
          throw new Error("reference unavailable");
        reference = Scalar.createApiReference("#reference", {
          url: "./openapi.yaml",
          baseServerURL: location.origin,
          theme: "none",
          layout: "modern",
          showSidebar: false,
          hideSearch: true,
          defaultOpenFirstTag: false,
          darkMode: root.dataset.theme === "dark",
          forceDarkModeState: root.dataset.theme,
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
            $("#reference-status").hidden = true;
            watchReference();
            localizeInterface($("#reference"));
            renderRoute();
          },
        });
      } else {
        if (typeof SwaggerUIBundle === "undefined")
          throw new Error("swagger unavailable");
        await loadSchemaResources();
        swagger = SwaggerUIBundle({
          url: "./openapi.yaml",
          dom_id: "#swagger-ui",
          deepLinking: true,
          filter: true,
          docExpansion: "list",
          defaultModelsExpandDepth: -1,
          defaultModelExpandDepth: 2,
          persistAuthorization: false,
          validatorUrl: null,
          syntaxHighlight: false,
          displayRequestDuration: true,
          plugins: [flatOperationTags],
          onComplete: () => {
            localizeInterface($("#swagger-ui"));
            renderRoute();
          },
        });
      }
    } catch (error) {
      const node = create(
        "div",
        "docs-notice error",
        "Не удалось загрузить документацию. Проверьте соединение и обновите страницу.",
      );
      node.setAttribute("role", "alert");
      $("#docs-main")?.append(node);
      if ($("#reference-status")) {
        $("#reference-content").hidden = false;
        $("#reference-status").textContent = node.textContent;
      }
      console.error("Documentation initialization failed:", error.message);
    }
  }
  addEventListener("DOMContentLoaded", bootstrap, { once: true });
})();

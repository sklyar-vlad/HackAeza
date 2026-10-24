// API Reference page: overview, operation with "try it", schema catalog.
import {
  TITLES,
  deref,
  describe,
  example,
  fields,
  limits,
  loadContract,
  merge,
  ops,
  refName,
  spec,
  tags,
  usedIn,
} from "./contract.js";
import {
  $,
  esc,
  expanded,
  icon,
  opHref,
  renderNav,
  schemaHref,
  showLoadError,
} from "./shell.js";

const PLACES = { path: "путь", query: "query", header: "заголовок" };
const GROUPS = {
  path: "Параметры пути",
  query: "Query-параметры",
  header: "Заголовки",
};
const STEPS = [
  [
    "Войдите",
    "Логин и пароль сотрудника дают Bearer-токен сессии.",
    'POST /api/v1/auth/login\n{ "username": "analyst", "password": "…" }',
  ],
  [
    "Авторизуйтесь",
    "Передавайте токен в заголовке каждого защищённого запроса.",
    "Authorization: Bearer <accessToken>",
  ],
  [
    "Запросите рейтинг",
    "Клиенты с высоким риском и наибольшими потерями.",
    "GET /api/v1/customers?riskLevel=high\n  &sortBy=potentialLoss",
  ],
];
const plural = (n, forms) =>
  `${n} ${forms[n % 10 === 1 && n % 100 !== 11 ? 0 : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20) ? 1 : 2]}`;
const statusClass = (code) =>
  code.startsWith("2")
    ? "ok"
    : ["409", "422", "429"].includes(code)
      ? "warn"
      : code.startsWith("5")
        ? ""
        : "bad";
const pretty = (value) =>
  typeof value === "string" ? value : JSON.stringify(value, null, 2);
// POSIX single-quoting: safe for any JSON body or header value.
const shq = (value) => `'${String(value).replaceAll("'", `'\\''`)}'`;

function typeHtml(schema = {}) {
  const name = refName(schema);
  const nullable = schema.nullable ? " | null" : "";
  if (name) return `<a class="ref" href="${schemaHref(name)}">${esc(name)}</a>`;
  if (schema.allOf?.length === 1) return typeHtml(schema.allOf[0]) + nullable;
  if (schema.oneOf || schema.anyOf)
    return (schema.oneOf ?? schema.anyOf)
      .map((part) => typeHtml(part))
      .join(" | ");
  if (schema.type === "array") return `${typeHtml(schema.items)}[]` + nullable;
  if (typeof schema.additionalProperties === "object")
    return `map&lt;string, ${typeHtml(schema.additionalProperties)}&gt;`;
  if (schema.enum) return esc(schema.enum.join(" | ")) + nullable;
  const type = schema.type ?? (schema.properties ? "object" : "any");
  return esc(schema.format ? `${type} · ${schema.format}` : type) + nullable;
}
const limitsHtml = (schema) => {
  const notes = limits(schema);
  return notes.length
    ? `<span class="limits">${esc(notes.join(" · "))}</span>`
    : "";
};
const verb = (op, extra = "") =>
  `<span class="verb ${extra}${op.method}">${op.method}</span>`;

// ---------- overview ----------
function overview() {
  const kpis = [
    [
      "Операций",
      ops.length,
      `в ${plural(tags.length, ["разделе", "разделах", "разделах"])}`,
    ],
    [
      "Обязательных по кейсу",
      ops.filter((op) => op.required).length,
      "x-required-by-case",
    ],
    [
      "Схем данных",
      Object.keys(spec.components.schemas).length,
      "components/schemas",
    ],
    ["Формат", "JSON", "camelCase, деньги в евроцентах"],
  ];
  return `<div class="stack rise">
    <header>
      <div class="home-word" aria-hidden="true">docs</div>
      <h1>документация api</h1>
      <p class="lead">Контракт риск-панели: вход, рейтинг клиентов, сводка рисков, настройки модели и импорт снимка. Запросы из документации уходят на настоящий сервер.</p>
    </header>
    <section class="grid" style="--min: 150px" aria-label="Контракт в цифрах">
      ${kpis.map(([label, value, sub]) => `<div class="card"><div class="kpi-label">${label}</div><div class="kpi-value">${value}</div><div class="kpi-sub">${sub}</div></div>`).join("")}
    </section>
    <section class="card">
      <div class="card-head"><h2>Первый запрос за три шага</h2><span class="muted">Токен действует до выхода или истечения срока</span></div>
      <ol class="steps grid" style="--min: 240px">
        ${STEPS.map(([title, text, code]) => `<li><strong>${title}</strong><span class="muted">${text}</span><pre class="code">${esc(code)}</pre></li>`).join("")}
      </ol>
    </section>
    <section class="grid" style="--min: 360px" aria-label="Разделы API">
      ${tags
        .map(
          (tag) => `<article class="card stack tight">
            <div class="card-head"><h2>${esc(tag.title)}</h2><span class="muted">${plural(tag.ops.length, ["операция", "операции", "операций"])}</span></div>
            <p class="muted">${esc(tag.description)}</p>
            <div class="op-list">${tag.ops
              .map(
                (op) =>
                  `<a class="row" href="${opHref(op)}">${verb(op)}<span>${esc(op.summary)}<code>${esc(op.path)}</code></span>${op.required ? '<span class="tag" title="Обязательная операция кейса">кейс</span>' : ""}</a>`,
              )
              .join("")}</div>
          </article>`,
        )
        .join("")}
    </section>
  </div>`;
}

// ---------- operation ----------
const fieldRow = (name, schema, required, description) =>
  `<div><div><code>${esc(name)}</code>${required ? '<span class="req">обязательно</span>' : ""}<div class="type">${typeHtml(schema)}</div></div><div class="desc">${esc(description) || "—"}${limitsHtml(schema)}</div></div>`;

function response(code, raw, open) {
  const { description, content = {} } = deref(raw);
  const [type, media] = Object.entries(content)[0] ?? [];
  const sample = example(media);
  const name = refName(media?.schema);
  const head = `<span class="dot ${statusClass(code)}"></span><code>${code}</code><span class="muted">${esc(description)}</span>`;
  if (sample === undefined && !name)
    return `<div class="response"><button class="row" type="button" disabled>${head}</button></div>`;
  return `<div class="response">
    <button class="row" type="button" aria-expanded="${open}">${head}${icon("chevron", 16)}</button>
    <div class="response-body"${open ? "" : " hidden"}>
      <span class="muted">${esc(type)}${name ? ` · <a class="ref" href="${schemaHref(name)}">${esc(name)}</a>` : ""}</span>
      ${sample !== undefined ? `<pre class="code">${esc(pretty(sample))}</pre>` : ""}
    </div>
  </div>`;
}

function operation(op) {
  const [bodyType, bodyMedia] =
    Object.entries(deref(op.requestBody)?.content ?? {})[0] ?? [];
  const bodyName = refName(bodyMedia?.schema);
  const groups = Object.entries(GROUPS)
    .map(([place, title]) => [
      title,
      op.params.filter((param) => param.in === place),
    ])
    .filter(([, params]) => params.length);
  const bodyRows =
    bodyMedia &&
    (fields(bodyMedia.schema)
      .map(({ path, field, required }) =>
        fieldRow(path, field, required, describe(field)),
      )
      .join("") ||
      `<div><div class="type">${typeHtml(bodyMedia.schema)}</div></div>`);
  return `<div class="stack rise">
    <header class="stack tight">
      <div class="op-line">${verb(op, "lg ")}<code>${esc(op.path)}</code><button class="icon-btn" type="button" data-copy="${esc(op.path)}" aria-label="Скопировать путь">${icon("copy", 16)}</button></div>
      <h1>${esc(op.summary)}</h1>
      <div class="badges">
        <span class="badge"><span class="dot ${op.required ? "ok" : ""}"></span>${op.required ? "обязательная операция кейса" : "расширение проекта"}</span>
        <span class="badge"><span class="dot ${op.secured ? "warn" : ""}"></span>${op.secured ? "Bearer-токен" : "без авторизации"}</span>
      </div>
      ${op.description ? `<p class="lead">${esc(op.description)}</p>` : ""}
    </header>
    <div class="op-grid">
      <div class="stack tight">
        ${groups.map(([title, params]) => `<section class="card fields"><h2>${title}</h2>${params.map((param) => fieldRow(param.name, param.schema ?? {}, param.required, param.description)).join("")}</section>`).join("")}
        ${bodyMedia ? `<section class="card fields"><h2>Тело запроса${bodyName ? ` · <a class="ref" href="${schemaHref(bodyName)}">${esc(bodyName)}</a>` : ""}</h2>${bodyRows}</section>` : ""}
        <section class="card"><h2>Ответы</h2>${Object.entries(
          op.responses ?? {},
        )
          .map(([code, raw], index) => response(code, raw, index === 0))
          .join("")}</section>
      </div>
      ${tryPanel(op, bodyType, bodyMedia)}
    </div>
  </div>`;
}

// ---------- try it ----------
let current;
let lang = "curl";
let token = "";
try {
  // The panel serves the docs on the same origin under /docs: reuse its session.
  token =
    JSON.parse(localStorage.getItem("aeza-risk-session")).state.token ?? "";
} catch {}

function tryPanel(op, bodyType, bodyMedia) {
  const stream = Object.keys(
    deref(op.responses?.["200"])?.content ?? {},
  ).includes("text/event-stream");
  const sample = example(bodyMedia);
  const body = sample === undefined ? "" : pretty(sample);
  const param = (p) => {
    const value = p.name === "Idempotency-Key" ? crypto.randomUUID() : "";
    const hint =
      p.example ??
      p.schema?.example ??
      p.schema?.default ??
      p.schema?.enum?.join(" | ") ??
      "";
    return `<label class="lbl"><span><code>${esc(p.name)}</code> · ${PLACES[p.in] ?? esc(p.in)}${p.required ? " · обязательно" : ""}</span><input class="field" name="${esc(`${p.in}:${p.name}`)}" value="${value}" placeholder="${esc(hint)}" autocomplete="off"${p.required ? " required" : ""} /></label>`;
  };
  return `<aside class="card try" aria-label="Попробовать запрос">
    <div class="card-head"><h2>Попробовать</h2><span class="muted">реальный сервер</span></div>
    <form id="try" data-type="${esc(bodyType ?? "")}">
      ${op.secured ? `<label class="lbl">Bearer-токен<input class="field" name="token" type="password" value="${esc(token)}" placeholder="из POST /api/v1/auth/login" autocomplete="off" /></label>` : ""}
      ${op.params.map(param).join("")}
      ${bodyMedia ? `<label class="lbl">Тело запроса · ${esc(bodyType)}<textarea class="field code" name="body" rows="${Math.min(body.split("\n").length + 1, 16)}" spellcheck="false">${esc(body)}</textarea></label>` : ""}
      ${stream ? '<p class="note">Это SSE-поток: подключайтесь через fetch streaming с заголовком Authorization.</p>' : '<button class="btn" type="submit">Отправить запрос</button>'}
    </form>
    <div id="result" aria-live="polite"></div>
    <div class="snippet">
      <div class="segmented small" role="tablist" aria-label="Пример кода">
        ${[
          ["curl", "cURL"],
          ["fetch", "JavaScript"],
        ]
          .map(
            ([id, label]) =>
              `<button type="button" role="tab" data-lang="${id}" aria-selected="${lang === id}">${label}</button>`,
          )
          .join("")}
      </div>
      <pre class="code" id="snippet"></pre>
    </div>
  </aside>`;
}

function request(form) {
  const data = new FormData(form);
  const query = new URLSearchParams();
  const headers = {};
  let path = current.path;
  for (const param of current.params) {
    const value = String(data.get(`${param.in}:${param.name}`) ?? "").trim();
    if (!value) continue;
    if (param.in === "path")
      path = path.replace(`{${param.name}}`, encodeURIComponent(value));
    if (param.in === "query") query.set(param.name, value);
    if (param.in === "header") headers[param.name] = value;
  }
  const body = String(data.get("body") ?? "").trim() || undefined;
  if (body) headers["Content-Type"] = form.dataset.type;
  const search = query.toString();
  return { url: path + (search ? `?${search}` : ""), headers, body };
}

function updateSnippet() {
  const form = $("#try");
  if (!form) return;
  const { url, headers, body } = request(form);
  let payload = body;
  try {
    payload = JSON.stringify(JSON.parse(body));
  } catch {}
  const large = payload?.length > 240;
  const auth = current.secured;
  const json = Object.keys(
    deref(Object.values(current.responses ?? {})[0])?.content ?? {},
  ).some((type) => type.includes("json"));
  const entries = Object.entries(headers);
  $("#snippet").textContent =
    lang === "curl"
      ? [
          `curl -X ${current.method} ${shq(location.origin + url)}`,
          // Double quotes so the shell expands $TOKEN.
          auth && '  -H "Authorization: Bearer $TOKEN"',
          ...entries.map(([key, value]) => `  -H ${shq(`${key}: ${value}`)}`),
          payload && `  -d ${large ? "@body.json" : shq(payload)}`,
        ]
          .filter(Boolean)
          .join(" \\\n")
      : [
          `const response = await fetch(${JSON.stringify(url)}, {`,
          `  method: "${current.method}",`,
          (auth || entries.length) && "  headers: {",
          auth && "    Authorization: `Bearer ${token}`,",
          ...entries.map(
            ([key, value]) =>
              `    ${JSON.stringify(key)}: ${JSON.stringify(value)},`,
          ),
          (auth || entries.length) && "  },",
          payload &&
            `  body: ${large ? "JSON.stringify(body)" : `JSON.stringify(${payload})`},`,
          "});",
          `const data = await response.${json ? "json" : "text"}();`,
        ]
          .filter(Boolean)
          .join("\n");
}

async function send(form) {
  const { url, headers, body } = request(form);
  if (current.secured && token) headers.Authorization = `Bearer ${token}`;
  const button = form.querySelector('[type="submit"]');
  button.disabled = true;
  button.textContent = "Отправляем…";
  const started = performance.now();
  let html;
  try {
    const response = await fetch(url, {
      method: current.method,
      headers,
      body,
      signal: AbortSignal.timeout(30000),
    });
    const text = await response.text();
    let shown = text;
    let saved = "";
    try {
      const data = JSON.parse(text);
      shown = JSON.stringify(data, null, 2);
      if (current.operationId === "login" && response.ok && data.accessToken) {
        token = data.accessToken;
        saved =
          '<span class="muted">Токен подставлен в остальные запросы</span>';
      }
    } catch {}
    html = `<div class="result-head"><span class="dot ${statusClass(String(response.status))}"></span><strong>${response.status} ${esc(response.statusText)}</strong><span class="muted">за ${Math.round(performance.now() - started)} мс</span></div>${saved}${shown ? `<pre class="code">${esc(shown.slice(0, 20000))}</pre>` : ""}`;
  } catch (error) {
    html = `<div class="result-head"><span class="dot bad"></span><strong>Нет ответа</strong><span class="muted">${error.name === "TimeoutError" ? "прошло 30 секунд" : "сервер недоступен"}</span></div>`;
  }
  $("#result").innerHTML = `<div class="result">${html}</div>`;
  button.disabled = false;
  button.textContent = "Отправить запрос";
}

// ---------- schemas ----------
function schemas(requested) {
  const all = spec.components.schemas;
  const names = Object.keys(all);
  const name = all[requested]
    ? requested
    : names.includes("Customer")
      ? "Customer"
      : names[0];
  const merged = merge(all[name]);
  const rows = fields(all[name]);
  const parents = (all[name].allOf ?? []).map(refName).filter(Boolean);
  const kind = [
    rows.length ? "object" : typeHtml(merged),
    parents.length &&
      `allOf ${parents.map((parent) => `<a class="ref" href="${schemaHref(parent)}">${esc(parent)}</a>`).join(", ")}`,
    ...limits(merged).map(esc),
  ].filter(Boolean);
  const used = usedIn(name);
  return `<div class="stack rise">
    <header>
      <h1>схемы данных</h1>
      <p class="lead">Деньги — целые евроценты, даты — UTC. Пустая оценка — это <code>null</code>, а не ноль.</p>
    </header>
    <div class="schema-grid">
      <nav class="card schema-list" aria-label="Схемы">
        ${names.map((item) => `<a class="item" href="${schemaHref(item)}"${item === name ? ' aria-current="page"' : ""}><span>${esc(item)}</span><small>${Object.keys(merge(all[item]).properties).length || ""}</small></a>`).join("")}
      </nav>
      <section class="card">
        <div class="card-head schema-head"><h2>${esc(name)}</h2><span class="type">${kind.join(" · ")}</span></div>
        ${merged.description ? `<p class="lead">${esc(merged.description)}</p>` : ""}
        ${
          rows.length
            ? `<div class="table-wrap"><table class="table">
                <thead><tr><th scope="col">Поле</th><th scope="col">Тип</th><th scope="col">Описание</th></tr></thead>
                <tbody>${rows
                  .map(
                    ({ path, field, required }) =>
                      `<tr><td><code>${esc(path)}</code>${required ? '<span class="req" title="Обязательное поле">*</span>' : ""}</td><td class="type">${typeHtml(field)}</td><td class="desc">${esc(describe(field)) || "—"}${limitsHtml(field)}</td></tr>`,
                  )
                  .join("")}</tbody>
              </table></div>`
            : ""
        }
        ${used.length ? `<div class="used-in"><span class="muted">Используется в</span>${used.map((op) => `<a href="${opHref(op)}">${verb(op)}${esc(op.path)}</a>`).join("")}</div>` : ""}
      </section>
    </div>
  </div>`;
}

// ---------- router ----------
function render(navigated) {
  let kind, id;
  try {
    [, kind = "", id = ""] = decodeURIComponent(location.hash.slice(1)).split(
      "/",
    );
  } catch {}
  current = kind === "op" && ops.find((op) => op.operationId === id);
  let html, crumb;
  if (current) {
    expanded.add(current.tags[0]);
    html = operation(current);
    crumb = TITLES[current.tags[0]];
  } else if (kind === "schemas") {
    html = schemas(id);
    crumb = "Схемы данных";
  } else {
    kind = "overview";
    html = overview();
    crumb = "API Reference";
  }
  $("#view").innerHTML = html;
  $("#crumb").textContent = crumb;
  document.title = `${current ? current.summary : kind === "overview" ? "Документация API" : crumb} // aéza`;
  renderNav(kind, id);
  updateSnippet();
  const schema = $(".schema-list [aria-current]");
  if (schema)
    schema.parentElement.scrollTop =
      schema.offsetTop - schema.parentElement.clientHeight / 2;
  if (navigated) {
    scrollTo(0, 0);
    $("#main").focus({ preventScroll: true });
  }
}

const root = $("#view");
root.addEventListener("click", (event) => {
  const toggle = event.target.closest(".response > .row:not([disabled])");
  if (toggle) {
    const open = toggle.getAttribute("aria-expanded") !== "true";
    toggle.setAttribute("aria-expanded", String(open));
    toggle.nextElementSibling.hidden = !open;
  }
  const copy = event.target.closest("[data-copy]");
  if (copy) {
    navigator.clipboard?.writeText(copy.dataset.copy);
    copy.innerHTML = icon("check", 16);
    setTimeout(() => (copy.innerHTML = icon("copy", 16)), 1500);
  }
  const tab = event.target.closest("[data-lang]");
  if (tab) {
    lang = tab.dataset.lang;
    root
      .querySelectorAll("[data-lang]")
      .forEach((item) =>
        item.setAttribute("aria-selected", String(item === tab)),
      );
    updateSnippet();
  }
});
root.addEventListener("input", (event) => {
  if (event.target.name === "token") token = event.target.value.trim();
  updateSnippet();
});
root.addEventListener("submit", (event) => {
  event.preventDefault();
  if (event.target.reportValidity()) send(event.target);
});

loadContract().then(
  () => {
    render(false);
    addEventListener("hashchange", () => render(true));
  },
  () => showLoadError(root),
);

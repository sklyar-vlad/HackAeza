// Swagger UI page: the stock UI in the panel's style, with the shared sidebar.
import { TITLES, loadContract } from "./contract.js";
import { $, renderNav, showLoadError } from "./shell.js";

// Swagger UI has no Russian locale: swap its fixed labels in place.
const RU = {
  Authorize: "Авторизоваться",
  "Available authorizations": "Авторизация",
  Logout: "Выйти",
  Close: "Закрыть",
  "Try it out": "Попробовать",
  Execute: "Выполнить",
  Clear: "Очистить",
  Cancel: "Отменить",
  Parameters: "Параметры",
  "No parameters": "Без параметров",
  Name: "Параметр",
  Description: "Описание",
  "Request body": "Тело запроса",
  Responses: "Ответы",
  Code: "Код",
  Links: "Связи",
  "No links": "Нет связей",
  "Media type": "Тип содержимого",
  "Example Value": "Пример",
  Schema: "Схема",
  Details: "Подробности",
  "Request URL": "URL запроса",
  "Request duration": "Время запроса",
  "Server response": "Ответ сервера",
  "Response body": "Тело ответа",
  "Response headers": "Заголовки ответа",
  Undocumented: "Не описан",
  "Value:": "Значение:",
  "Edit Value": "Изменить",
  "Examples:": "Примеры:",
  Examples: "Примеры",
  required: "обязательно",
  "Controls Accept header.": "Задаёт заголовок Accept.",
  ...TITLES,
};
const SKIP = "pre, code, textarea, .markdown, .renderedMarkdown";

function translate(nodes) {
  for (const added of nodes) {
    const walker = document.createTreeWalker(added, NodeFilter.SHOW_TEXT);
    for (let node = walker.currentNode; node; node = walker.nextNode()) {
      if (node.nodeType !== Node.TEXT_NODE) continue;
      const text = node.nodeValue.trim();
      if (RU[text] && !node.parentElement?.closest(SKIP))
        node.nodeValue = node.nodeValue.replace(text, RU[text]);
    }
  }
}

const root = $("#swagger-ui");
$("#crumb").textContent = "Swagger UI";
// Only added or changed text is translated, not the whole tree on every update.
new MutationObserver((records) =>
  translate(
    records.flatMap((record) =>
      record.type === "characterData"
        ? [record.target]
        : [...record.addedNodes],
    ),
  ),
).observe(root, { childList: true, subtree: true, characterData: true });

const ui = SwaggerUIBundle({
  url: "./openapi.yaml",
  dom_id: "#swagger-ui",
  deepLinking: true,
  docExpansion: "list",
  defaultModelsExpandDepth: -1,
  tryItOutEnabled: true,
  displayRequestDuration: true,
  persistAuthorization: false,
  syntaxHighlight: false,
  validatorUrl: null,
});

// Swagger reads deep links only on load; the sidebar changes the hash later.
addEventListener("hashchange", () => {
  const [, tag, id] = location.hash.split("/").map(decodeURIComponent);
  if (!tag || !id) return;
  ui.layoutActions.show(["operations", tag, id], true);
  requestAnimationFrame(() =>
    document
      .getElementById(`operations-${tag}-${id}`)
      ?.scrollIntoView({ behavior: "smooth", block: "start" }),
  );
});

loadContract().then(
  () => renderNav("swagger"),
  () => showLoadError($("#nav")),
);

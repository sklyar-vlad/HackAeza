const element = (tag, className, text) => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
};
export const refName = (ref) =>
  ref?.startsWith("#/components/schemas/")
    ? ref
        .slice("#/components/schemas/".length)
        .replaceAll("~1", "/")
        .replaceAll("~0", "~")
    : undefined;
export const schemaHref = (name) => `#schemas/${encodeURIComponent(name)}`;

// allOf inheritance is presented as one table. References to other objects
// remain links, so recursive schemas never turn into infinite nested trees.
export function mergeSchema(schema, schemas, seen = new Set()) {
  if (!schema) return {};
  const name = refName(schema.$ref ?? schema.$$ref);
  if (name && seen.has(name)) return { ...schema };
  const next = new Set(seen);
  if (name) next.add(name);
  const parts = [
    ...(name && schemas[name]
      ? [mergeSchema(schemas[name], schemas, next)]
      : []),
    ...(schema.allOf ?? []).map((part) => mergeSchema(part, schemas, next)),
    schema,
  ];
  const merged = Object.assign({}, ...parts);
  merged.properties = Object.assign(
    {},
    ...parts.map((part) => part.properties),
  );
  merged.required = [...new Set(parts.flatMap((part) => part.required ?? []))];
  return merged;
}

export function constraints(schema) {
  const notes = [];
  if (schema.format) notes.push(`Формат: ${schema.format}`);
  if (schema.nullable) notes.push("Допускается null");
  if (schema.enum)
    notes.push(
      `Значения: ${schema.enum.map((x) => JSON.stringify(x)).join(" · ")}`,
    );
  if (schema.minimum !== undefined)
    notes.push(`${schema.exclusiveMinimum ? ">" : "≥"} ${schema.minimum}`);
  if (schema.maximum !== undefined)
    notes.push(`${schema.exclusiveMaximum ? "<" : "≤"} ${schema.maximum}`);
  if (schema.multipleOf !== undefined)
    notes.push(`Кратно ${schema.multipleOf}`);
  if (schema.minLength !== undefined)
    notes.push(`Длина от ${schema.minLength}`);
  if (schema.maxLength !== undefined)
    notes.push(`Длина до ${schema.maxLength}`);
  if (schema.pattern) notes.push(`Шаблон: ${schema.pattern}`);
  if (schema.minItems !== undefined)
    notes.push(`Элементов от ${schema.minItems}`);
  if (schema.maxItems !== undefined)
    notes.push(`Элементов до ${schema.maxItems}`);
  if (schema.uniqueItems) notes.push("Уникальные элементы");
  if (schema.minProperties !== undefined)
    notes.push(`Полей от ${schema.minProperties}`);
  if (schema.maxProperties !== undefined)
    notes.push(`Полей до ${schema.maxProperties}`);
  if (schema.additionalProperties === false)
    notes.push("Дополнительные поля запрещены");
  if (schema.readOnly) notes.push("Только чтение");
  if (schema.writeOnly) notes.push("Только запись");
  if (schema.deprecated) notes.push("Устарело");
  if (schema.default !== undefined)
    notes.push(`По умолчанию: ${JSON.stringify(schema.default)}`);
  if (schema.example !== undefined)
    notes.push(`Пример: ${JSON.stringify(schema.example)}`);
  for (const [key, label] of [
    ["oneOf", "Ровно один вариант"],
    ["anyOf", "Хотя бы один вариант"],
  ]) {
    if (schema[key])
      notes.push(
        `${label}: ${schema[key].map((part) => JSON.stringify(part)).join(" | ")}`,
      );
  }
  if (schema.not) notes.push(`Исключено: ${JSON.stringify(schema.not)}`);
  return notes;
}

export function schemaFields(schema, schemas) {
  const result = [];
  function visit(current, prefix, ancestors) {
    const merged = mergeSchema(current, schemas);
    for (const [name, field] of Object.entries(merged.properties ?? {})) {
      const path = prefix ? `${prefix}.${name}` : name;
      result.push({
        path,
        field,
        required: merged.required.includes(name),
        ancestors,
      });
      if (!field.$ref && !field.$$ref && field.properties)
        visit(field, path, [...ancestors, path]);
      if (!field.items?.$ref && !field.items?.$$ref && field.items?.properties)
        visit(field.items, `${path}[]`, [...ancestors, path]);
    }
  }
  visit(schema, "", []);
  return result;
}

function appendType(target, schema, schemas) {
  const name = refName(schema.$ref);
  if (name && schemas[name]) {
    const link = element("a", "schema-type-link", name);
    link.href = schemaHref(name);
    target.append(link);
  } else if (schema.type === "array" || schema.items) {
    target.append(document.createTextNode("array<"));
    appendType(target, schema.items ?? {}, schemas);
    target.append(document.createTextNode(">"));
  } else if (typeof schema.additionalProperties === "object") {
    target.append(document.createTextNode("map<string, "));
    appendType(target, schema.additionalProperties, schemas);
    target.append(document.createTextNode(">"));
  } else
    target.append(
      document.createTextNode(
        schema.type ?? (schema.properties ? "object" : "—"),
      ),
    );
}

export function mountCatalog(container, schemas) {
  const names = Object.keys(schemas);
  const tools = element("div", "schema-tools");
  const searchLabel = element("label", "schema-control");
  searchLabel.append(element("span", "", "Найти схему или поле"));
  const search = element("input");
  search.type = "search";
  search.placeholder = "Account, balance, risk…";
  searchLabel.append(search);
  const selectLabel = element("label", "schema-control");
  selectLabel.append(element("span", "", "Схема контракта"));
  const select = element("select");
  selectLabel.append(select);
  const count = element("span", "schema-count");
  tools.append(searchLabel, selectLabel, count);
  const body = element("article", "schema-document");
  const status = element("p", "schema-empty");
  status.setAttribute("role", "status");
  status.hidden = true;
  container.replaceChildren(tools, status, body);
  let active = "";
  function fillOptions(query = "") {
    const matches = names.filter((name) =>
      `${name} ${JSON.stringify(schemas[name])} ${JSON.stringify(schemaFields(schemas[name], schemas))}`
        .toLowerCase()
        .includes(query),
    );
    select.replaceChildren(
      ...matches.map((name) => {
        const option = element("option", "", name);
        option.value = name;
        return option;
      }),
    );
    count.textContent = `${matches.length} / ${names.length}`;
    select.disabled = !matches.length;
    status.hidden = !!matches.length;
    status.textContent = matches.length
      ? ""
      : "Схемы не найдены. Измените запрос.";
    body.hidden = !matches.length;
    if (matches.includes(active)) select.value = active;
    return matches;
  }
  function show(name) {
    active = name;
    select.value = name;
    const source = schemas[name];
    const merged = mergeSchema(source, schemas);
    body.replaceChildren();
    const heading = element("div", "schema-heading");
    heading.append(element("h2", "", name));
    const type = element("code", "schema-root-type");
    appendType(type, { ...merged, $ref: undefined }, schemas);
    heading.append(type);
    body.append(heading);
    if (merged.description)
      body.append(element("p", "schema-description", merged.description));
    const notes = constraints(merged);
    if (notes.length) {
      const list = element("ul", "schema-constraints");
      notes.forEach((note) => list.append(element("li", "", note)));
      body.append(list);
    }
    const fields = schemaFields(source, schemas);
    if (fields.length) {
      const scroll = element("div", "schema-table-scroll");
      const table = element("table", "schema-table");
      const caption = element("caption", "sr-only", `Поля ${name}`);
      const head = element("thead");
      const row = element("tr");
      for (const title of [
        "Поле",
        "Тип",
        "Обязательно",
        "Описание и ограничения",
      ]) {
        const th = element("th", "", title);
        th.scope = "col";
        row.append(th);
      }
      head.append(row);
      const tbody = element("tbody");
      fields.forEach(({ path, field, required, ancestors }) => {
        const tr = element("tr");
        const nameCell = element("th");
        nameCell.scope = "row";
        nameCell.append(element("code", "", path));
        const typeCell = element("td", "schema-field-type");
        appendType(typeCell, field, schemas);
        const requiredCell = element(
          "td",
          "",
          required ? (ancestors.length ? "Внутри объекта" : "Да") : "Нет",
        );
        const details = element("td", "schema-field-description");
        const resolved = mergeSchema(field, schemas);
        if (resolved.description)
          details.append(element("p", "", resolved.description));
        const limits = constraints(resolved);
        if (limits.length) {
          const list = element("ul", "schema-constraints");
          limits.forEach((note) => list.append(element("li", "", note)));
          details.append(list);
        }
        if (!details.childNodes.length) details.textContent = "—";
        tr.append(nameCell, typeCell, requiredCell, details);
        tbody.append(tr);
      });
      table.append(caption, head, tbody);
      scroll.append(table);
      body.append(scroll);
    }
    const original = element(
      "a",
      "schema-original",
      "Скачать исходный OpenAPI YAML",
    );
    original.href = "./openapi.yaml";
    original.download = "openapi.yaml";
    body.append(original);
  }
  function navigate() {
    let name;
    try {
      name = decodeURIComponent(location.hash.split("/").slice(1).join("/"));
    } catch {
      name = "";
    }
    if (!schemas[name]) name = schemas.Account ? "Account" : names[0];
    search.value = "";
    fillOptions();
    if (name) show(name);
  }
  search.addEventListener("input", () => {
    const matches = fillOptions(search.value.trim().toLowerCase());
    if (matches.length) show(matches.includes(active) ? active : matches[0]);
  });
  select.addEventListener("change", () => {
    location.hash = schemaHref(select.value);
  });
  navigate();
  return { navigate };
}

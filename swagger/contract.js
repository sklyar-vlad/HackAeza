// OpenAPI contract helpers: no DOM, shared by the reference pages and the sidebar.
export let spec, ops, tags;

export const TITLES = {
  Auth: "Вход и сессия",
  Customers: "Клиенты",
  Analytics: "Аналитика",
  Model: "Модель риска",
  Source: "Источник данных",
  Updates: "Живые обновления",
  Operations: "Задания и мониторинг",
  Documentation: "Документация",
};

export async function loadContract() {
  const response = await fetch("./spec.json");
  if (!response.ok) throw new Error(`spec.json: HTTP ${response.status}`);
  spec = await response.json();
  ops = Object.entries(spec.paths).flatMap(([path, item]) =>
    ["get", "post", "put", "patch", "delete"]
      .filter((method) => item[method])
      .map((method) => {
        const op = item[method];
        return {
          ...op,
          method: method.toUpperCase(),
          path,
          params: [...(item.parameters ?? []), ...(op.parameters ?? [])].map(
            deref,
          ),
          secured: (op.security ?? spec.security ?? []).length > 0,
          required: op["x-required-by-case"] === true,
        };
      }),
  );
  tags = spec.tags.map((tag) => ({
    ...tag,
    title: TITLES[tag.name] ?? tag.name,
    ops: ops.filter((op) => op.tags?.[0] === tag.name),
  }));
}

const lookup = (ref) =>
  ref
    .slice(2)
    .split("/")
    .reduce(
      (node, key) => node?.[key.replaceAll("~1", "/").replaceAll("~0", "~")],
      spec,
    );
export const deref = (node) => (node?.$ref ? deref(lookup(node.$ref)) : node);
export const refName = (node) =>
  node?.$ref?.startsWith("#/components/schemas/")
    ? node.$ref.slice("#/components/schemas/".length)
    : null;

// allOf parts are flattened into one object; $ref fields stay links.
export function merge(schema = {}, seen = new Set()) {
  const name = refName(schema);
  if (name)
    return seen.has(name)
      ? {}
      : merge(spec.components.schemas[name], new Set(seen).add(name));
  const parts = [
    ...(schema.allOf ?? []).map((part) => merge(part, seen)),
    schema,
  ];
  return {
    ...Object.assign({}, ...parts),
    allOf: undefined,
    properties: Object.assign({}, ...parts.map((part) => part.properties)),
    required: parts.flatMap((part) => part.required ?? []),
  };
}

// Inline objects are expanded as dotted paths; referenced schemas are not.
export function fields(schema, prefix = "") {
  const merged = merge(schema);
  return Object.entries(merged.properties).flatMap(([name, field]) => {
    const path = prefix + name;
    const row = { path, field, required: merged.required.includes(name) };
    if (field.properties) return [row, ...fields(field, `${path}.`)];
    if (field.items?.properties)
      return [row, ...fields(field.items, `${path}[].`)];
    return [row];
  });
}

export function limits(s) {
  return [
    s.minimum !== undefined && `${s.exclusiveMinimum ? ">" : "≥"} ${s.minimum}`,
    s.maximum !== undefined && `${s.exclusiveMaximum ? "<" : "≤"} ${s.maximum}`,
    s.minLength !== undefined && `длина от ${s.minLength}`,
    s.maxLength !== undefined && `длина до ${s.maxLength}`,
    s.minItems !== undefined && `элементов от ${s.minItems}`,
    s.maxItems !== undefined && `элементов до ${s.maxItems}`,
    s.minProperties !== undefined && `полей от ${s.minProperties}`,
    s.pattern && `шаблон ${s.pattern}`,
    s.default !== undefined && `по умолчанию ${JSON.stringify(s.default)}`,
    s.readOnly && "только чтение",
    s.writeOnly && "только запись",
    s.deprecated && "устарело",
  ].filter(Boolean);
}

export const describe = (schema) =>
  schema.description ?? merge(schema).description;

export function example(media) {
  if (!media) return undefined;
  if (media.example !== undefined) return media.example;
  const first = Object.values(media.examples ?? {})[0];
  return first ? deref(first).value : deref(media.schema)?.example;
}

// Schemas reachable from an operation, through components/responses, parameters and nested refs.
function collect(node, found = new Set(), seen = new Set()) {
  if (Array.isArray(node)) node.forEach((item) => collect(item, found, seen));
  else if (node && typeof node === "object") {
    if (node.$ref) {
      if (seen.has(node.$ref)) return found;
      seen.add(node.$ref);
      if (refName(node)) found.add(refName(node));
      collect(lookup(node.$ref), found, seen);
    } else
      for (const [key, value] of Object.entries(node))
        if (key !== "examples" && key !== "example")
          collect(value, found, seen);
  }
  return found;
}
let usage;
export function usedIn(name) {
  usage ??= new Map(
    ops.map((op) => [op, collect([op.params, op.requestBody, op.responses])]),
  );
  return ops.filter((op) => usage.get(op).has(name));
}

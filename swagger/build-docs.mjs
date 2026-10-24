import fs from "node:fs";
import crypto from "node:crypto";
import path from "node:path";
import { parse } from "yaml";

const site = "/reference/site";
const spec = parse(fs.readFileSync("/reference/openapi.yaml", "utf8"));
const exactSchemas =
  parse(fs.readFileSync("/reference/openapi.yaml", "utf8"), {
    intAsBigInt: true,
  }).components?.schemas ?? {};
const methods = new Set([
  "get",
  "put",
  "post",
  "patch",
  "delete",
  "head",
  "options",
  "trace",
]);
const operations = Object.entries(spec.paths).flatMap(([url, item]) =>
  Object.entries(item)
    .filter(([method]) => methods.has(method))
    .map(([method, operation]) => ({
      method: method.toUpperCase(),
      path: url,
      summary: operation.summary ?? url,
      operationId: operation.operationId,
      tags: operation.tags ?? [],
      required: operation["x-required-by-case"] === true,
    })),
);
const tags = spec.tags.map((tag) => ({
  ...tag,
  slug: tag.name.toLowerCase().replace(/\s+/g, "-"),
  count: operations.filter((operation) => operation.tags.includes(tag.name))
    .length,
}));
const metadata = {
  info: spec.info,
  openapi: spec.openapi,
  tags,
  operations,
  schemaCount: Object.keys(spec.components?.schemas ?? {}).length,
};
fs.writeFileSync(path.join(site, "metadata.json"), JSON.stringify(metadata));
fs.writeFileSync(
  path.join(site, "schemas.json"),
  JSON.stringify(exactSchemas, (_key, value) => {
    if (typeof value !== "bigint") return value;
    return Number.isSafeInteger(Number(value))
      ? Number(value)
      : value.toString();
  }),
);
const shell = fs.readFileSync(path.join(site, "shell.html"), "utf8");
for (const page of ["index.html", "swagger.html"]) {
  let html = fs.readFileSync(path.join(site, page), "utf8");
  const content = html.match(
    /<!-- page-content -->([\s\S]*?)<!-- \/page-content -->/,
  );
  if (!content) throw new Error(`Missing content in ${page}`);
  html = html.replace(
    content[0],
    shell.replace("<!-- docs-content -->", content[1]),
  );
  for (const asset of [
    "theme.js",
    "theme.css",
    "tokens.css",
    "assets/icons.svg",
    "assets/favicon.svg",
  ]) {
    const hash = crypto
      .createHash("sha256")
      .update(fs.readFileSync(path.join(site, asset)))
      .digest("hex")
      .slice(0, 12);
    html = html.replaceAll(`./${asset}`, `./${asset}?v=${hash}`);
  }
  fs.writeFileSync(path.join(site, page), html);
}

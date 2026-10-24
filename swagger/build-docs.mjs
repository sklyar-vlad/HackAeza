// Usage: node build-docs.mjs <openapi.yaml> <out dir>
// Writes spec.json and both pages with the shared head and shell.
import fs from "node:fs";
import path from "node:path";
import { parse } from "yaml";

const [specPath, out] = process.argv.slice(2);
const site = path.dirname(new URL(import.meta.url).pathname);
const read = (file) => fs.readFileSync(path.join(site, file), "utf8");

// int64 limits do not fit a double: keep unsafe integers as strings.
const spec = parse(fs.readFileSync(specPath, "utf8"), { intAsBigInt: true });
fs.mkdirSync(out, { recursive: true });
fs.writeFileSync(
  path.join(out, "spec.json"),
  JSON.stringify(spec, (_key, value) =>
    typeof value !== "bigint"
      ? value
      : Number.isSafeInteger(Number(value))
        ? Number(value)
        : value.toString(),
  ),
);

const head = read("head.html");
const shell = read("shell.html").replace(
  "{{version}}",
  `v${spec.info.version} · OpenAPI ${spec.openapi}`,
);
for (const page of ["index.html", "swagger.html"]) {
  const html = read(page)
    .replace("<!-- head -->", head.trim())
    .replace(/<!-- shell -->([\s\S]*?)<!-- \/shell -->/, (_match, content) =>
      shell.replace("<!-- content -->", content.trim()),
    );
  fs.writeFileSync(path.join(out, page), html);
}

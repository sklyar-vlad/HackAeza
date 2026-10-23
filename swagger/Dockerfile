FROM node:22-alpine AS reference
WORKDIR /reference
RUN npm pack @scalar/api-reference@1.75.1 --ignore-scripts && tar -xzf scalar-api-reference-1.75.1.tgz package/dist/browser/standalone.js
COPY swagger /reference/site
RUN node -e 'const fs=require("node:fs"),crypto=require("node:crypto"); for(const page of ["index.html","swagger.html"]){ let html=fs.readFileSync("site/"+page,"utf8"); for(const asset of ["theme.js","theme.css","assets/icons.svg"]){ const hash=crypto.createHash("sha256").update(fs.readFileSync("site/"+asset)).digest("hex").slice(0,12); html=html.replaceAll("./"+asset,"./"+asset+"?v="+hash); } fs.writeFileSync("site/"+page,html); }'

FROM swaggerapi/swagger-ui:latest
COPY --from=reference /reference/package/dist/browser/standalone.js /usr/share/nginx/html/reference.js
COPY api/openapi.yaml /usr/share/nginx/html/openapi.yaml
COPY --from=reference /reference/site/index.html /reference/site/swagger.html /reference/site/theme.css /reference/site/theme.js /usr/share/nginx/html/
COPY swagger/assets /usr/share/nginx/html/assets

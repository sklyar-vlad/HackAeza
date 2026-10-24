FROM node:22-alpine AS reference
WORKDIR /reference
RUN npm pack @scalar/api-reference@1.75.1 --ignore-scripts && tar -xzf scalar-api-reference-1.75.1.tgz package/dist/browser/standalone.js
RUN npm install --no-save --ignore-scripts --no-audit --no-fund yaml@2.8.1
COPY swagger /reference/site
COPY docs/openapi.yaml /reference/openapi.yaml
RUN node /reference/site/build-docs.mjs

FROM swaggerapi/swagger-ui:latest
COPY --from=reference /reference/package/dist/browser/standalone.js /usr/share/nginx/html/reference.js
COPY docs/openapi.yaml /usr/share/nginx/html/openapi.yaml
COPY --from=reference /reference/site/index.html /reference/site/swagger.html /reference/site/tokens.css /reference/site/theme.css /reference/site/theme.js /reference/site/metadata.json /usr/share/nginx/html/
COPY --from=reference /reference/site/schema-catalog.mjs /usr/share/nginx/html/schema-catalog.js
COPY --from=reference /reference/site/schemas.json /usr/share/nginx/html/
COPY swagger/assets /usr/share/nginx/html/assets

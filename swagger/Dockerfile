FROM node:22-alpine AS docs
WORKDIR /docs
RUN npm install --no-save --ignore-scripts --no-audit --no-fund yaml@2.8.1
COPY swagger/build-docs.mjs swagger/head.html swagger/shell.html swagger/index.html swagger/swagger.html docs/openapi.yaml ./
RUN node build-docs.mjs openapi.yaml out

FROM swaggerapi/swagger-ui:v5.34.0
ENV HTTP_PORT=8081
COPY swagger/nginx.conf /etc/nginx/templates/default.conf.template
COPY docs/openapi.yaml swagger/docs.css swagger/swagger.css swagger/contract.js swagger/shell.js swagger/reference.js swagger/swagger.js frontend/public/favicon.ico frontend/public/favicon.svg frontend/public/apple-touch-icon.png /usr/share/nginx/html/
COPY --from=docs /docs/out/ /usr/share/nginx/html/
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s CMD wget -q -O /dev/null http://127.0.0.1:8080/swagger.html || exit 1

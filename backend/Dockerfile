FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /server .

FROM alpine:3.24
USER 65532:65532
COPY --from=build /server /server
EXPOSE 8081
ENTRYPOINT ["/server"]

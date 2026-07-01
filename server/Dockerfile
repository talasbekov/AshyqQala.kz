# Multi-stage сборка cmd/api (Story 1.7 — отложено из 1.3). Distroless static (CGO off, pgx — чистый Go).
# ⚠️ Build-контекст = КОРЕНЬ РЕПО (deploy/docker-compose.yml: context: .., dockerfile: server/Dockerfile),
# а НЕ server/ — чтобы ЗАПЕЧЬ в образ registry/ (methodology_params/astana_districts/lexicon), который
# лежит вне server/ и нужен api в рантайме (config.REGISTRY_ROOT). Контекст сужен repo-root .dockerignore
# (allow-list server+registry) — иначе в демон улетают web/node_modules (214M), .git и пр.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
# registry/ — статическая конфигурация рантайма, ЗАПЕЧЕНА в образ (не монтируется). Из build-контекста (repo-root).
COPY registry/ /registry/
ENV REGISTRY_ROOT=/registry
EXPOSE 8080
ENTRYPOINT ["/api"]

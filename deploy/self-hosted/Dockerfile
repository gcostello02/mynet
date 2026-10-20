# Public self-hosted MyNet coordinator image (signaling + embedded admin UI).
FROM node:22-bookworm-slim AS web
WORKDIR /src/web/admin
COPY web/admin/package.json web/admin/package-lock.json ./
RUN npm ci
COPY web/admin/ ./
RUN npm run build

FROM golang:1.23-bookworm AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/dashboard/dist ./internal/dashboard/dist
RUN CGO_ENABLED=0 go build -o /out/mynet-server ./cmd/mynet-server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/mynet-server /usr/local/bin/mynet-server
COPY deploy/self-hosted/entrypoint.sh /usr/local/bin/mynet-coordinator-entrypoint.sh
RUN chmod +x /usr/local/bin/mynet-coordinator-entrypoint.sh
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mynet-coordinator-entrypoint.sh"]

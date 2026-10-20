# Linux staging image: mynet + mynet-server + TUN prerequisites.
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
RUN CGO_ENABLED=0 go build -o /out/mynet ./cmd/mynet && \
    CGO_ENABLED=0 go build -o /out/mynet-server ./cmd/mynet-server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates iproute2 iputils-ping curl jq iptables \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/mynet /out/mynet-server /usr/local/bin/
COPY deploy/staging/entrypoint-client.sh /usr/local/bin/mynet-staging-client-entrypoint.sh
COPY deploy/staging/entrypoint-coordinator.sh /usr/local/bin/mynet-staging-coord-entrypoint.sh
RUN chmod +x /usr/local/bin/mynet-staging-*-entrypoint.sh
ENV MYNET_STATE_DIR=/var/lib/mynet
RUN mkdir -p "$MYNET_STATE_DIR" && chmod 700 "$MYNET_STATE_DIR"
ENTRYPOINT ["/usr/local/bin/mynet-staging-client-entrypoint.sh"]

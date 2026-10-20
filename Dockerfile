# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json* ./frontend/
WORKDIR /src/frontend
RUN npm install
COPY frontend/ ./
RUN mkdir -p /src/backend/internal/static/dist && npm run build

FROM golang:1.27.3-alpine AS backend
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/backend/internal/static/dist ./internal/static/dist
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
  -ldflags="-s -w -X github.com/MarcelRuh/zimaos-backup-manager/internal/version.Version=${VERSION}" \
  -o /out/zimaos-backup-manager ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget su-exec \
  && addgroup -S zbm \
  && adduser -S -G zbm -h /data zbm \
  && mkdir -p /data \
  && chown -R zbm:zbm /data
COPY --from=backend /out/zimaos-backup-manager /usr/local/bin/zimaos-backup-manager
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
# Entrypoint starts as root only to fix volume ownership, then drops to zbm.
USER root
ENV DATA_DIR=/data \
    LISTEN_ADDR=:8787 \
    TZ=Europe/Berlin \
    ZIMAOS_API_BASE_URL=http://host.docker.internal
EXPOSE 8787
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8787/api/health || exit 1
ENTRYPOINT ["/entrypoint.sh"]

# syntax=docker/dockerfile:1

FROM node:22-alpine AS web-builder
WORKDIR /app/web-v2
COPY web-v2/package.json web-v2/package-lock.json ./
RUN npm ci
COPY web-v2/ ./
RUN npm run build

FROM golang:1.22-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=web-builder /app/web-v2/dist ./web-v2/dist
COPY web-v2/static.go ./web-v2/static.go
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X cpa-usage/internal/version.Version=${VERSION}" \
    -o /out/cpa-usage ./cmd/server/main.go

FROM alpine:3.20
WORKDIR /
RUN apk add --no-cache ca-certificates tzdata su-exec \
	&& addgroup -S app \
	&& adduser -S -G app app \
	&& mkdir -p /data \
	&& chown -R app:app /data
COPY --from=go-builder /out/cpa-usage /app/cpa-usage
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN sed -i 's/\r$//' /usr/local/bin/docker-entrypoint.sh \
	&& chmod +x /usr/local/bin/docker-entrypoint.sh
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD base_path="${APP_BASE_PATH:-}" && base_path="${base_path%/}" && wget -q --spider "http://127.0.0.1:${APP_PORT:-8080}${base_path}/healthz" || exit 1
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/app/cpa-usage"]

# Dashboard: static export.
FROM node:24-alpine AS web
WORKDIR /src
COPY page/package.json page/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY page/ ./
RUN npm run build

# Collector: CGO is required by the SQLite driver, so this stage runs natively
# for each target platform.
FROM golang:1.27-alpine AS api
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/mikrotik-dns .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && mkdir -p /data
COPY --from=api /out/mikrotik-dns /usr/local/bin/mikrotik-dns
COPY --from=web /src/out /usr/share/mikrotik-dns/web
ENV HTTP_ADDR=:3000 \
    SYSLOG_ADDR=:5354 \
    DATABASE_PATH=/data/queries.db \
    WEB_DIR=/usr/share/mikrotik-dns/web
VOLUME ["/data"]
EXPOSE 3000/tcp 5354/udp
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -qO- http://127.0.0.1:3000/healthz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/mikrotik-dns"]

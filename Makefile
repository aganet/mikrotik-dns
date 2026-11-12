IMAGE ?= ghcr.io/aganet/mikrotik-dns
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help build build-web build-api run test lint docker up down logs clean dev-api dev-web

help:
	@echo "build       build the dashboard and the collector binary (serves ./page/out)"
	@echo "run         run the locally built collector on :3000 / :5354"
	@echo "test        run Go tests and the TypeScript type-check"
	@echo "docker      build the Docker image"
	@echo "up / down   start or stop with Docker Compose"
	@echo "dev-api     run the collector on :8080 without the dashboard"
	@echo "dev-web     run the Next.js dev server on :3000, proxying /api to :8080"
	@echo "clean       remove build artifacts"

build: build-web build-api

build-web:
	cd page && npm ci --no-audit --no-fund && npm run build

build-api:
	CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o mikrotik-dns .

run: build-api
	WEB_DIR=page/out ./mikrotik-dns

test:
	go vet ./...
	go test -race ./...
	cd page && npx tsc --noEmit

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

dev-api:
	HTTP_ADDR=:8080 LOG_LEVEL=debug go run .

dev-web:
	cd page && npm run dev

clean:
	rm -rf mikrotik-dns page/.next page/out

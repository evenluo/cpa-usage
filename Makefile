WEB_DIR := ./web

.PHONY: dev-backend dev-frontend test-backend test-frontend fmt-backend vet-backend build-backend build-frontend lint-frontend typecheck-frontend ensure-frontend-embed-dir verify verify-backend verify-frontend verify-docker

dev-backend:
	go run ./cmd/server/main.go --env .env

dev-frontend:
	npm --prefix $(WEB_DIR) run dev

ensure-frontend-embed-dir:
	mkdir -p $(WEB_DIR)/dist
	touch $(WEB_DIR)/dist/.gitkeep

test-backend: ensure-frontend-embed-dir
	go test ./cmd/... ./internal/...

test-frontend:
	$(MAKE) typecheck-frontend

fmt-backend:
	go fmt ./cmd/... ./internal/...

vet-backend: ensure-frontend-embed-dir
	go vet ./cmd/... ./internal/...

build-backend: ensure-frontend-embed-dir
	mkdir -p ./bin
	go build -o ./bin/cpa-usage ./cmd/server

build-frontend:
	npm --prefix $(WEB_DIR) run build

lint-frontend:
	npm --prefix $(WEB_DIR) run lint

typecheck-frontend:
	npm --prefix $(WEB_DIR) run typecheck

verify: verify-backend verify-frontend

verify-backend: test-backend vet-backend

verify-frontend:
	npm --prefix $(WEB_DIR) ci
	$(MAKE) lint-frontend
	$(MAKE) typecheck-frontend
	$(MAKE) build-frontend

verify-docker:
	docker build -t cpa-usage:ci .

WEB_DIR := ./web

.PHONY: dev-backend dev-frontend test-backend test-frontend install-playwright test-frontend-mobile fmt-backend vet-backend build-backend build-frontend lint-frontend typecheck-frontend ensure-frontend-embed-dir verify verify-backend verify-frontend verify-docker render-dokploy-compose verify-dokploy-compose dokploy-migrate-cpa-usage-compose

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
	npm --prefix $(WEB_DIR) run test
	$(MAKE) typecheck-frontend

install-playwright:
	@if [ "$$(uname)" = "Linux" ]; then \
		npm --prefix $(WEB_DIR) exec playwright install --with-deps chromium; \
	else \
		npm --prefix $(WEB_DIR) exec playwright install chromium; \
	fi

test-frontend-mobile: build-frontend
	$(MAKE) install-playwright
	npm --prefix $(WEB_DIR) run test:e2e:mobile

fmt-backend:
	go fmt ./cmd/... ./internal/...

vet-backend: ensure-frontend-embed-dir
	go vet ./cmd/... ./internal/...

build-backend: ensure-frontend-embed-dir
	mkdir -p ./bin
	go build -o ./bin/cpa-usage ./cmd/server

build-frontend:
	npm --prefix $(WEB_DIR) run build
	$(MAKE) ensure-frontend-embed-dir

lint-frontend:
	npm --prefix $(WEB_DIR) run lint

typecheck-frontend:
	npm --prefix $(WEB_DIR) run typecheck

verify: verify-backend verify-frontend

verify-backend: test-backend vet-backend

verify-frontend:
	npm --prefix $(WEB_DIR) ci
	$(MAKE) lint-frontend
	$(MAKE) test-frontend
	$(MAKE) test-frontend-mobile

verify-docker:
	docker build -t cpa-usage:ci .

render-dokploy-compose:
	scripts/render-dokploy-compose.sh $${CPA_USAGE_VERSION:?set CPA_USAGE_VERSION} $${OUTPUT:-.tmp/dokploy/cpa-usage.compose.yml}

verify-dokploy-compose:
	scripts/verify-dokploy-compose.sh

dokploy-migrate-cpa-usage-compose:
	scripts/dokploy-migrate-cpa-usage-compose.sh

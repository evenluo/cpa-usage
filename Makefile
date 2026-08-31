WEB_DIR := ./web
ENV_FILE ?= .env

.DEFAULT_GOAL := help

.PHONY: help dev-app dev-backend dev-frontend test-backend test-frontend install-playwright test-frontend-mobile fmt-backend vet-backend build-backend build-frontend lint-frontend typecheck-frontend ensure-frontend-embed-dir verify verify-backend verify-frontend benchmark-cpa-data-surface verify-docker render-dokploy-compose verify-dokploy-compose-static verify-dokploy-compose test-dokploy-release verify-dokploy-release dokploy-migrate-cpa-usage-compose

help:
	@printf '%s\n' \
		'Development:' \
		'  make dev-app       Build the frontend, then serve integrated UI/API' \
		'  make dev-backend   Serve the last built frontend and API' \
		'  make dev-frontend  Run isolated Vite HMR (no API proxy or E2E)' \
		'' \
		'Verification:' \
		'  make verify          Run backend and frontend verification' \
		'  make verify-backend  Run backend tests and vet' \
		'  make verify-frontend Run frontend lint, tests, typecheck, and mobile E2E' \
		'  make benchmark-cpa-data-surface  Run manual synthetic performance evidence' \
		'  make verify-docker   Build the deployment image' \
		'  make verify-dokploy-release  Run canonical local release proof'

dev-app: build-frontend
	$(MAKE) dev-backend

dev-backend:
	go run ./cmd/server/main.go --env "$(ENV_FILE)"

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

# Manual review evidence only. Keep timing comparisons on the same idle host;
# this target is intentionally not part of verify or CI.
benchmark-cpa-data-surface:
	GOMAXPROCS=1 go test ./internal/service -run '^$$' -bench='^(BenchmarkReplaySafeRedisUsageMessage|BenchmarkRedisUsageInboxBatchEndToEnd)$$' -benchmem -count=3 -benchtime=1x
	GOMAXPROCS=1 go test ./internal/repository -run '^TestRequestEvidenceQueryPlansAvoidFullScans$$' -bench='^(BenchmarkListUsageEventsHighCardinalityCombinedFilters|BenchmarkInsertUsageEventsAttemptAmplification)$$' -benchmem -count=3 -benchtime=1x
	GOMAXPROCS=1 go test ./internal/repository/migration -run '^$$' -bench='^BenchmarkAddUsageAttemptFieldsMigrationHighCardinality$$' -benchmem -count=3 -benchtime=1x
	npm --prefix $(WEB_DIR) run bench:live-capacity
	npm --prefix $(WEB_DIR) run perf:bundle
	$(MAKE) ensure-frontend-embed-dir

verify-docker:
	docker build -t cpa-usage:ci .

render-dokploy-compose:
	scripts/render-dokploy-compose.sh $${CPA_USAGE_VERSION:?set CPA_USAGE_VERSION} $${OUTPUT:-.tmp/dokploy/cpa-usage.compose.yml}

verify-dokploy-compose-static:
	scripts/verify-dokploy-compose-static.sh $${COMPOSE_FILE:-}

verify-dokploy-compose:
	scripts/verify-dokploy-compose.sh $${COMPOSE_FILE:-}

test-dokploy-release:
	scripts/test-dokploy-release.sh

verify-dokploy-release: verify-dokploy-compose test-dokploy-release

dokploy-migrate-cpa-usage-compose:
	scripts/dokploy-migrate-cpa-usage-compose.sh

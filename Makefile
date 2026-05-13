.PHONY: dev-backend dev-frontend test-backend test-frontend fmt-backend vet-backend build-backend build-frontend verify verify-backend verify-frontend verify-docker

dev-backend:
	go run ./cmd/server/main.go --env .env

dev-frontend:
	npm --prefix ./web run dev

test-backend:
	go test ./cmd/... ./internal/...

test-frontend:
	npm --prefix ./web run test

fmt-backend:
	go fmt ./cmd/... ./internal/...

vet-backend:
	go vet ./cmd/... ./internal/...

build-backend:
	mkdir -p ./bin
	go build -o ./bin/cpa-usage ./cmd/server

build-frontend:
	npm --prefix ./web run build

verify: verify-backend verify-frontend

verify-backend: test-backend vet-backend

verify-frontend:
	npm --prefix ./web ci
	$(MAKE) test-frontend
	npm --prefix ./web run lint
	npm --prefix ./web run typecheck
	$(MAKE) build-frontend

verify-docker:
	docker build -t cpa-usage:ci .

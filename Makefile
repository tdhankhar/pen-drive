APP_NAME=pen-drive

# S3 Storage Target: r2 (default) or minio
# Usage: make backend-run S3_TARGET=minio
# MinIO credentials are loaded from .env.minio (shared with docker-compose)
S3_TARGET ?= r2

ifeq ($(S3_TARGET),minio)
	# Load MinIO env vars from .env.minio
	include .env.minio
	S3_OVERRIDES := S3_ENDPOINT=$(S3_ENDPOINT) \
	                S3_ACCESS_KEY_ID=$(S3_ACCESS_KEY_ID) \
	                S3_SECRET_ACCESS_KEY=$(S3_SECRET_ACCESS_KEY) \
	                S3_PING_BUCKET=$(S3_PING_BUCKET) \
	                S3_USE_PATH_STYLE=$(S3_USE_PATH_STYLE)
else
	S3_OVERRIDES :=
endif

.PHONY: backend-run backend-build backend-test backend-lint backend-tidy backend-dev-up backend-dev-down backend-openapi backend-s3-setup backend-dev-minio frontend-build frontend-lint

backend-run:
	cd backend && $(S3_OVERRIDES) go run ./cmd/api

backend-build:
	cd backend && go build ./...

backend-test:
	cd backend && go test ./...

backend-lint:
	cd backend && gofmt -w $$(find . -name '*.go')
	cd backend && go vet ./...

backend-tidy:
	cd backend && go mod tidy

backend-openapi:
	cd backend && go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g ./cmd/api/main.go -o ./docs/openapi --parseDependency --parseInternal

backend-dev-up:
	docker-compose up -d postgres

backend-dev-down:
	docker-compose down

backend-s3-setup:
	@docker exec pen-drive-minio-1 mc alias set local http://localhost:9000 minioadmin minioadmin && \
	docker exec pen-drive-minio-1 mc mb local/pen-drive || true

backend-dev-minio: backend-dev-up backend-s3-setup
	$(MAKE) backend-run S3_TARGET=minio

frontend-build:
	cd frontend && npm run build

frontend-lint:
	cd frontend && npm run lint

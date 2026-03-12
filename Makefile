APP_NAME=pen-drive

.PHONY: backend-run backend-build backend-test backend-lint backend-tidy backend-dev-up backend-dev-down backend-openapi backend-s3-setup frontend-build frontend-lint

backend-run:
	cd backend && go run ./cmd/api

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

frontend-build:
	cd frontend && npm run build

frontend-lint:
	cd frontend && npm run lint

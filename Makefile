APP_NAME=pen-drive

.PHONY: backend-run backend-build backend-test backend-lint backend-tidy

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

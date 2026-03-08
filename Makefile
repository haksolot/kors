.PHONY: dev migrate generate test test-unit test-integration lint swagger

dev:
	docker-compose up -d

migrate:
	go run github.com/pressly/goose/v3/cmd/goose -dir kors-api/migrations postgres "postgres://kors:kors_dev_secret@localhost:5432/kors?sslmode=disable" up

generate:
	cd kors-api && go run github.com/99designs/gqlgen generate

swagger:
	cd kors-api && go run github.com/swaggo/swag/cmd/swag init -g cmd/server/main.go

test-unit:
	cd kors-api && go test ./internal/domain/... ./internal/usecase/... -v -count=1

test-integration:
	cd kors-api && TESTCONTAINERS_RYUK_DISABLED=true go test ./internal/adapter/postgres/... -v -count=1 -timeout 120s

test:
	make test-unit
	make test-integration
	cd kors-events && go test ./... -v -count=1
	cd kors-worker && go test ./... -v -count=1

lint:
	cd kors-api && go vet ./...
	cd kors-events && go vet ./...
	cd kors-worker && go vet ./...

test-graphql:
	powershell -NoProfile -Command "Invoke-RestMethod -Uri 'http://localhost/query' -Method Post -ContentType 'application/json' -Body '{\"query\": \"mutation { registerResourceType(input: { name: \\\"tool_final\\\", description: \\\"Final Tool\\\", jsonSchema: {\\\"type\\\": \\\"object\\\"}, transitions: {\\\"idle\\\": [\\\"in_use\\\"]} }) { success } }\"}'"

